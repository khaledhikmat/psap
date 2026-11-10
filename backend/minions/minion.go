package minions

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/khaledhikmat/psap/service"
	"github.com/khaledhikmat/psap/service/lgr"
)

var (
	// ErrNoState is returned by an OnLoad processor when nothing has been saved for the minion
	// yet. The minion then starts fresh. Any other OnLoad error fails Start.
	ErrNoState = errors.New("no saved minion state")
	// ErrNotRunning is returned by Signal when the minion has not started or is stopping.
	ErrNotRunning = errors.New("minion is not running")
)

// State constrains a minion's state type. Go cannot reach struct fields through a type
// parameter, so the state hands out its embedded MinionState instead. Embedding MinionState
// provides Base, but with a pointer receiver, so instantiate with the pointer type:
//
//	type MyState struct {
//		MinionState
//		Balance float64
//	}
//	m := NewMinion(id, managerID, "my", svcs, &MyState{}) // Minion[*MyState]
type State interface {
	Base() *MinionState
}

// Processor types. A generic type cannot be declared as `type x func[T any](...)`; the type
// parameter goes on the type name instead.
type (
	// LoadProc fills state from storage, using state.Base().ID and Kind. Return ErrNoState if
	// nothing was saved. The manager ID is restored by Start afterwards because it may have changed.
	LoadProc[S State] func(ctx context.Context, state S) error
	// SaveProc persists state. It runs periodically and once more when the minion stops.
	SaveProc[S State] func(ctx context.Context, state S) error
	// SignalProc applies one request to state. Returning an error only logs it.
	SignalProc[S State] func(ctx context.Context, state S, req MinionRequest) error
	// ActivitiesProc runs the minion's initial or final activities.
	ActivitiesProc[S State] func(ctx context.Context, state S) error
	// StartProc runs once at Start, after the state is loaded, typically to register timers.
	StartProc[S State] func(m *Minion[S])
)

// Minion is a single-goroutine actor with a typed state S. Its lifecycle (mailbox, timers,
// periodic saves, start and stop) is fixed; what it does is supplied by overriding the On*
// processors after NewMinion and before Start.
//
// Processors run one at a time on the minion's goroutine, so they can use the state freely.
// Everyone else must go through Signal, State or View.
type Minion[S State] struct {
	svcs  service.ServicesFactory
	state S

	OnLoad                 LoadProc[S]
	OnSave                 SaveProc[S]
	OnSignal               SignalProc[S]
	OnStart                StartProc[S]
	OnRunInitialActivities ActivitiesProc[S]
	OnRunFinalActivities   ActivitiesProc[S]

	// TickInterval is how often due timers are checked. SaveInterval is how often OnSave runs.
	TickInterval time.Duration
	SaveInterval time.Duration

	mu      sync.RWMutex // held while a processor or timer runs, and by readers of the state
	ctx     context.Context
	cancel  context.CancelFunc
	inbox   chan MinionRequest
	done    chan struct{}
	started atomic.Bool
	running atomic.Bool
}

// A minion with no state of its own is a Minion[*MinionState].
var _ IMinion = (*Minion[*MinionState])(nil)

// NewMinion creates a minion around state, which carries the minion's own fields (e.g. a
// balance) and must not be nil. The identity fields of its MinionState are set from the
// arguments. All processors start with harmless defaults.
func NewMinion[S State](id, managerID, kind string, svcs service.ServicesFactory, state S) *Minion[S] {
	base := state.Base()
	base.ID, base.ManagerID, base.Kind = id, managerID, kind
	base.StartTime, base.EndTime = new(time.Time), new(time.Time)
	if base.Timers == nil {
		base.Timers = make(map[time.Time]MinionTimerProcessor)
	}

	m := &Minion[S]{
		svcs:         svcs,
		state:        state,
		TickInterval: time.Second,
		SaveInterval: time.Minute,
		inbox:        make(chan MinionRequest, 64),
		done:         make(chan struct{}),
	}
	m.OnLoad = m.defaultLoad
	m.OnSave = m.defaultSave
	m.OnSignal = m.defaultSignal
	return m
}

// Default processors

func (m *Minion[S]) defaultLoad(ctx context.Context, state S) error {
	return ErrNoState
}

func (m *Minion[S]) defaultSave(ctx context.Context, state S) error {
	return nil
}

func (m *Minion[S]) defaultSignal(ctx context.Context, state S, req MinionRequest) error {
	return fmt.Errorf("unknown command %q", req.Command)
}

// Svcs gives processors access to the services the minion was created with.
func (m *Minion[S]) Svcs() service.ServicesFactory {
	return m.svcs
}

// Start loads the state, runs OnStart and launches the minion's goroutine. The minion runs
// until Stop is called or canx is cancelled.
func (m *Minion[S]) Start(canx context.Context) error {
	if !m.started.CompareAndSwap(false, true) {
		return errors.New("minion already started")
	}
	m.ctx, m.cancel = context.WithCancel(canx)

	// Load the existing state if available, otherwise start fresh. The manager ID is excluded
	// because the manager may have been changed!!!
	base := m.state.Base()
	managerID := base.ManagerID
	switch err := m.OnLoad(m.ctx, m.state); {
	case err == nil:
	case errors.Is(err, ErrNoState):
		now := time.Now()
		base.StartTime, base.EndTime = &now, new(time.Time)
	default:
		m.cancel()
		m.started.Store(false)
		return fmt.Errorf("load minion %s: %w", base.ID, err)
	}
	base.ManagerID = managerID
	if base.StartTime == nil {
		base.StartTime = new(time.Time)
	}
	if base.EndTime == nil {
		base.EndTime = new(time.Time)
	}
	if base.Timers == nil {
		base.Timers = make(map[time.Time]MinionTimerProcessor)
	}

	if m.OnStart != nil {
		m.OnStart(m)
	}

	// Accept signals right away: they wait in the inbox until the loop starts.
	m.running.Store(true)
	go m.run()
	return nil
}

// RegisterTimer schedules processor to run at t. Timers live in the state, which only the
// minion's goroutine may touch, so call this from OnStart or from processors, not from outside.
func (m *Minion[S]) RegisterTimer(t time.Time, processor MinionTimerProcessor) {
	timers := m.state.Base().Timers
	for {
		if _, taken := timers[t]; !taken {
			break
		}
		t = t.Add(time.Nanosecond) // the map is keyed by time; never overwrite another timer
	}
	timers[t] = processor
}

// run is the minion's lifecycle: initial activities, then a sequential loop over signals and
// timer ticks, then final activities and a last save.
func (m *Minion[S]) run() {
	defer close(m.done)
	defer m.running.Store(false)
	defer func() {
		if r := recover(); r != nil {
			lgr.Logger.Error("minion panicked", slog.String("id", m.state.Base().ID), slog.Any("panic", r))
		}
	}()

	base := m.state.Base()
	m.locked(func() {
		base.Running = true
		m.runActivities(m.ctx, m.OnRunInitialActivities)
		base.InitialActivitiesDone = true
	})

	ticker := time.NewTicker(m.TickInterval)
	defer ticker.Stop()
	lastSave := time.Now()

loop:
	for {
		select {
		case <-m.ctx.Done():
			break loop
		case req := <-m.inbox:
			m.locked(func() { m.handle(m.ctx, req) })
		case now := <-ticker.C:
			m.locked(func() {
				m.fireTimers(now)
				if now.Sub(lastSave) >= m.SaveInterval {
					m.save(m.ctx)
					lastSave = now
				}
			})
		}
	}

	// The context is cancelled by now, but the last requests, activities and save must still
	// be able to use the database and other services.
	final := context.WithoutCancel(m.ctx)
	m.running.Store(false)
	m.locked(func() {
		for drained := false; !drained; {
			select {
			case req := <-m.inbox:
				m.handle(final, req)
			default:
				drained = true
			}
		}
		m.runActivities(final, m.OnRunFinalActivities)
		base.FinalActivitiesDone = true
		// Only upon stopping, set the end time. This indicates the minion finished its lifecycle.
		*base.EndTime = time.Now()
		base.Running = false
		m.save(final)
	})
}

func (m *Minion[S]) locked(fn func()) {
	m.mu.Lock()
	defer m.mu.Unlock()
	fn()
}

func (m *Minion[S]) handle(ctx context.Context, req MinionRequest) {
	m.state.Base().SignalsCount++
	if err := m.OnSignal(ctx, m.state, req); err != nil {
		lgr.Logger.Error("minion request failed",
			slog.String("id", m.state.Base().ID),
			slog.String("command", req.Command),
			slog.Any("error", err))
	}
}

func (m *Minion[S]) save(ctx context.Context) {
	if m.OnSave == nil {
		return
	}
	if err := m.OnSave(ctx, m.state); err != nil {
		lgr.Logger.Error("minion save failed", slog.String("id", m.state.Base().ID), slog.Any("error", err))
	}
}

func (m *Minion[S]) runActivities(ctx context.Context, proc ActivitiesProc[S]) {
	if proc == nil {
		return
	}
	if err := proc(ctx, m.state); err != nil {
		lgr.Logger.Error("minion activities failed", slog.String("id", m.state.Base().ID), slog.Any("error", err))
	}
}

// fireTimers runs the timers that are due, oldest first. A timer that asks to be rescheduled
// is registered again for the given duration from now.
func (m *Minion[S]) fireTimers(now time.Time) {
	base := m.state.Base()
	var due []time.Time
	for t := range base.Timers {
		if !t.After(now) {
			due = append(due, t)
		}
	}
	slices.SortFunc(due, time.Time.Compare)

	for _, t := range due {
		processor := base.Timers[t]
		delete(base.Timers, t)
		base.TimerInvocationsCount++
		if rerun, after := processor(m.ctx, t); rerun && after > 0 {
			m.RegisterTimer(now.Add(after), processor)
		}
	}
}

// State returns a snapshot of the minion's base state.
func (m *Minion[S]) State() MinionState {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return cloneBase(m.state.Base())
}

// View runs fn with read access to the minion's full state. fn must not keep the state or
// modify it, and must not call back into the minion.
func (m *Minion[S]) View(fn func(state S)) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	fn(m.state)
}

// cloneBase copies the state including everything it points to, so a snapshot stays valid
// while the minion keeps changing its own state.
func cloneBase(b *MinionState) MinionState {
	c := *b
	if b.StartTime != nil {
		start := *b.StartTime
		c.StartTime = &start
	}
	if b.EndTime != nil {
		end := *b.EndTime
		c.EndTime = &end
	}
	c.Timers = maps.Clone(b.Timers)
	return c
}

// Signal queues a request for the minion's goroutine. A request that races with Stop may be
// dropped.
func (m *Minion[S]) Signal(request MinionRequest) error {
	if !m.running.Load() {
		return ErrNotRunning
	}
	select {
	case m.inbox <- request:
		return nil
	case <-m.ctx.Done():
		return ErrNotRunning
	}
}

// Stop stops the minion and waits until its final activities and last save are done. It is
// safe to call more than once, but only after Start has returned.
func (m *Minion[S]) Stop() {
	if !m.started.Load() {
		return
	}
	m.cancel()
	<-m.done
}
