package minions

import (
	"context"
	"time"

	"github.com/khaledhikmat/psap/service"
)

type ISimpleMinion interface {
	IMinion
	Execute(args ...any) error
}

type SimpleMinionState struct {
	MinionState
	balance float64
}

func (a SimpleMinionState) Load(id, managerID, kind string, svcs service.ServicesFactory) error {
	// Implement the execution logic for the simple minion
	// This could involve processing the arguments and updating the state
	return nil
}

func (a SimpleMinionState) Save(svcs service.ServicesFactory) error {
	// Implement the save logic for the simple minion state
	// This could involve persisting the state to a database or other storage
	return nil
}

// SimpleMinion represents a basic minion with a state
type simpleMinion struct {
	svcs   service.ServicesFactory
	canx   context.Context
	done   chan bool
	inbox  chan MinionRequest
	outbox chan MinionResponse
	state  SimpleMinionState
}

func NewSimpleMinion(id, managerID, kind string, svcs service.ServicesFactory) ISimpleMinion {
	s := SimpleMinionState{
		ID:        id,
		ManagerID: managerID,
		Kind:      kind,
		StartTime: new(time.Time),
		EndTime:   new(time.Time),
		Timers:    make(map[time.Time]MinionTimerProcessor),
		balance:   0.0,
	}

	minion := &simpleMinion{
		svcs:  svcs,
		state: s,
	}
	return minion
}

func (a *simpleMinion) Start(canx context.Context) error {
	a.canx = canx
	a.inbox = make(chan MinionRequest, 64) // Mailbox
	a.outbox = make(chan MinionResponse)   // Outbox
	a.done = make(chan bool)

	err := a.state.Load(a.state.ID, a.state.ManagerID, a.state.Kind, a.svcs)
	if err != nil {
		*a.state.StartTime = time.Now()
		a.state.EndTime = new(time.Time)
	}

	timer1Proc := func(ctx context.Context, t time.Time) (bool, time.Duration) {
		// Default timer processor, can be customized
		// Do stuff
		// Do not reschedule this timer
		return false, time.Duration(0)
	}

	timer2Proc := func(ctx context.Context, t time.Time) (bool, time.Duration) {
		// Default timer processor, can be customized
		// Do stuff
		// re-schedule it for later
		return true, 200 * time.Second
	}

	// Register timers if any
	a.registerTimer(time.Now().Add(100*time.Second), timer1Proc)
	a.registerTimer(time.Now().Add(200*time.Second), timer2Proc)

	go a.run()
	return nil
}

func (a *simpleMinion) registerTimer(t time.Time, processor MinionTimerProcessor) {
	a.state.Timers[t] = processor
}

// Sequential message loop (the actor's lifecycle)
func (a *simpleMinion) run() {
	defer func() {
		if a.inbox != nil {
			close(a.inbox)
		}
		if a.outbox != nil {
			close(a.outbox)
		}
		if a.done != nil {
			close(a.done)
		}

		a.state.Save(a.svcs)
	}()

	// Run initial activities
	a.runInitialActivities()

	for {
		select {
		case <-a.canx.Done():
			goto resume
		case <-a.done:
			goto resume
		case <-time.After(time.Duration(60 * time.Second)):
			// Timeout to handle timers, heartbeat, etc
			// Determine which timers that need to be triggered
			now := time.Now()
			for t, processor := range a.state.Timers {
				if t.Before(now) || t.Equal(now) {
					a.state.TimerInvocationsCount++
					rerun, duration := processor(a.canx, t)
					delete(a.state.Timers, t)
					if rerun && duration > 0 {
						a.registerTimer(time.Now().Add(duration), processor)
					}
				}
			}

			// Persist every minute
			a.state.Save(a.svcs)
		case request, ok := <-a.inbox:
			a.state.CommandsCount++

			if !ok {
				goto resume
			}
			switch request.Command {
			case "Deposit":
				// Assuming Args[0] contains the amount as string
				// Convert to float64 and add to balance
				// Error handling omitted for brevity
			case "Withdraw":
				// Assuming Args[0] contains the reply channel as string (not typical, but for illustration)
				// In practice, you might pass a channel directly in the Payload struct
			}
		}
	}

resume:
	a.runFinalActivities()
}

func (a *simpleMinion) runInitialActivities() {
	// Placeholder for initial activities
}

func (a *simpleMinion) runFinalActivities() {
	// Placeholder for final activities
}

func (a *simpleMinion) State() MinionState {
	return a.state.MinionState
}

func (a *simpleMinion) Query(request MinionRequest) (MinionResponse, error) {
	a.inbox <- request
	return <-a.outbox, nil
}

func (a *simpleMinion) Signal(request MinionRequest) error {
	a.inbox <- request
	return nil
}

func (a *simpleMinion) Close() {
	// Only upon close, set the end time. This indicates that the minion has finished its lifecycle.
	*a.state.EndTime = time.Now()
	a.done <- true
}

// Convenient helper to execute a command with arbitrary arguments
func (a *simpleMinion) Execute(args ...any) error {
	p := MinionRequest{
		Command: "Execute",
		Args:    args,
	}
	return a.Signal(p)
}
