package minions

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/khaledhikmat/psap/service"
)

type loadProc func[T any](ID, kind string, svcs service.ServicesFactory) T
type saveProc func[T any](ID, kind string, svcs service.ServicesFactory, T) error

// BaseMinion represents a basic minion with a state
type baseMinion struct {
	svcs  service.ServicesFactory
	canx  context.Context
	done  chan bool
	inbox chan MinionRequest
	state MinionState

	Load  loadProc
	Save  saveProc
}

func NewBaseMinion(ID, managerID string, svcs service.ServicesFactory) IMinion {
	s := MinionState{
		ID:           ID,
		ManagerID:    managerID,
		Kind:         "base",
		StartTime:    new(time.Time),
		EndTime:      new(time.Time),
		Timers:       make(map[time.Time]MinionTimerProcessor),
		SignalsCount: 0,
	}

	minion := &baseMinion{
		svcs:  svcs,
		state: s,
	}
	return minion
}

func (a *baseMinion) Start(canx context.Context) error {
	a.canx = canx
	a.inbox = make(chan MinionRequest, 64) // Mailbox
	a.done = make(chan bool)

	// Load the existing state if available, otherwise keep the initialized start and end times
	// Exclude the manager ID because the manager may have been changed!!!
	managerID := a.state.ManagerID
	err := a.state.Load(a.state.ID, a.state.Kind, a.svcs)
	if err != nil {
		*a.state.StartTime = time.Now()
		a.state.EndTime = new(time.Time)
	}
	// Restore the manager ID after loading the state
	a.state.ManagerID = managerID

	timer1Proc := func(ctx context.Context, t time.Time) (bool, time.Duration) {
		// Default timer processor, can be customized
		// Do stuff
		// Do not reschedule this timer
		fmt.Printf("Timer1 triggered at: %v\n", t)
		return false, time.Duration(0)
	}

	timer2Proc := func(ctx context.Context, t time.Time) (bool, time.Duration) {
		// Default timer processor, can be customized
		// Do stuff
		// re-schedule it for later
		fmt.Printf("Timer2 triggered at: %v\n", t)
		return true, 2 * time.Second
	}

	// Register timers if any
	a.registerTimer(time.Now().Add(1*time.Second), timer1Proc)
	a.registerTimer(time.Now().Add(2*time.Second), timer2Proc)

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
		if a.done != nil {
			close(a.done)
		}

		a.state.Save(a.svcs)
	}()

	a.state.Running = true

	// Run initial activities
	a.runInitialActivities()

	for {
		select {
		case <-a.canx.Done():
			goto resume
		case <-a.done:
			goto resume
		case <-time.After(time.Duration(3 * time.Second)):
			// Timeout to handle timers, heartbeat, etc
			// Determine which timers that need to be triggered
			now := time.Now()
			fmt.Printf("Timeout occurred at: %v\n", now)
			for t, processor := range a.state.Timers {
				if t.Before(now) || t.Equal(now) {
					a.state.TimerInvocationsCount++
					rerun, duration := processor(a.canx, t)
					delete(a.state.Timers, t)
					if rerun && duration > 0 {
						fmt.Printf("Rescheduling timer for: %v\n", now.Add(duration))
						a.registerTimer(now.Add(duration), processor)
					}
				}
			}

			// Persist every minute
			a.state.Save(a.svcs)
		case request, ok := <-a.inbox:
			a.state.SignalsCount++

			if !ok {
				goto resume
			}
			switch request.Command {
			case "Deposit":
				// Assuming Args[0] contains the amount as string
				// Convert to float64 and add to balance
				// Error handling omitted for brevity
				fmt.Printf("Deposit request: %+v\n", request)
				if len(request.Args) > 0 {
					amount, _ := request.Args[0].(string)
					fmt.Printf("Deposit amount: %s\n", amount)
					amountFloat, _ := strconv.ParseFloat(amount, 64)
					a.state.Balance += amountFloat
				}
			case "Withdraw":
				// Assuming Args[0] contains the amount as string
				// Convert to float64 and subtract from balance
				// Error handling omitted for brevity
				fmt.Printf("Withdraw request: %+v\n", request)
				if len(request.Args) > 0 {
					amount, _ := request.Args[0].(string)
					amountFloat, _ := strconv.ParseFloat(amount, 64)
					a.state.Balance -= amountFloat
				}
			}
		}
	}

resume:
	a.runFinalActivities()
	a.state.Running = false
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

func (a *simpleMinion) GetState() SimpleMinionState {
	return a.state
}

func (a *simpleMinion) Signal(request MinionRequest) error {
	fmt.Printf("Signaling request: %+v\n", request)
	if !a.state.Running {
		return fmt.Errorf("minion is not running")
	}
	a.inbox <- request
	return nil
}

func (a *simpleMinion) Stop() {
	// Only upon close, set the end time. This indicates that the minion has finished its lifecycle.
	*a.state.EndTime = time.Now()
	fmt.Printf("Stopping minion: %+v\n", a.state)
	a.done <- true
}

// Convenient helper to execute a command with arbitrary arguments
// They still have to be signaled to perform the action via the Signal method
// in order to avoid race conditions and ensure proper lifecycle management of the minion.

func (a *simpleMinion) Deposit(amount float64) error {
	p := MinionRequest{
		ID:        a.state.ID,
		ManagerID: a.state.ManagerID,
		Kind:      a.state.Kind,
		Command:   "Deposit",
		Args:      []any{fmt.Sprintf("%f", amount)},
	}
	return a.Signal(p)
}

func (a *simpleMinion) Withdraw(amount float64) error {
	p := MinionRequest{
		ID:        a.state.ID,
		ManagerID: a.state.ManagerID,
		Kind:      a.state.Kind,
		Command:   "Withdraw",
		Args:      []any{fmt.Sprintf("%f", amount)},
	}
	return a.Signal(p)
}
