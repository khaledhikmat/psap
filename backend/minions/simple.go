package minions

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/khaledhikmat/psap/service"
)

const simpleKind = "simple"

type ISimpleMinion interface {
	IMinion
	Deposit(amount float64) error
	Withdraw(amount float64) error
	GetState() SimpleMinionState
}

// SimpleMinionState is the simple minion's T: the base state plus a balance.
type SimpleMinionState struct {
	MinionState
	Balance float64
}

// simpleMinion is a Minion[*SimpleMinionState] that overrides the processors it cares about.
// Embedding provides Start, State, Signal and Stop.
type simpleMinion struct {
	*Minion[*SimpleMinionState]
}

func NewSimpleMinion(ID, managerID string, svcs service.ServicesFactory, balance float64) ISimpleMinion {
	m := NewMinion(ID, managerID, simpleKind, svcs, &SimpleMinionState{Balance: balance})
	m.OnLoad = load
	m.OnSave = save
	m.OnSignal = handleRequest
	m.OnStart = registerTimers
	return &simpleMinion{Minion: m}
}

// loadSimple must load the state using the ID and kind, excluding the manager ID since it may
// have changed (Start restores it).
func load(ctx context.Context, state *SimpleMinionState) error {
	// Implement the load logic for the simple minion state, e.g. read it from the database
	// by state.ID and state.Kind and return ErrNoState if it is not there.
	return ErrNoState
}

func save(ctx context.Context, state *SimpleMinionState) error {
	// Implement the save logic for the simple minion state, e.g. persist it to the database.
	return nil
}

func handleRequest(ctx context.Context, state *SimpleMinionState, req MinionRequest) error {
	switch req.Command {
	case "Deposit":
		amount, err := amountArg(req)
		if err != nil {
			return err
		}
		state.Balance += amount
	case "Withdraw":
		amount, err := amountArg(req)
		if err != nil {
			return err
		}
		state.Balance -= amount
	default:
		return fmt.Errorf("unknown command %q", req.Command)
	}
	return nil
}

// amountArg reads the amount, passed as a string in the first argument.
func amountArg(req MinionRequest) (float64, error) {
	if len(req.Args) == 0 {
		return 0, fmt.Errorf("%s needs an amount", req.Command)
	}
	amount, ok := req.Args[0].(string)
	if !ok {
		return 0, fmt.Errorf("%s amount %v is not a string", req.Command, req.Args[0])
	}
	return strconv.ParseFloat(amount, 64)
}

func registerTimers(m *Minion[*SimpleMinionState]) {
	timer1Proc := func(ctx context.Context, t time.Time) (bool, time.Duration) {
		// Do stuff
		// Do not reschedule this timer
		fmt.Printf("Timer1 triggered at: %v\n", t)
		return false, time.Duration(0)
	}

	timer2Proc := func(ctx context.Context, t time.Time) (bool, time.Duration) {
		// Do stuff
		// re-schedule it for later
		fmt.Printf("Timer2 triggered at: %v\n", t)
		return true, 2 * time.Second
	}

	m.RegisterTimer(time.Now().Add(1*time.Second), timer1Proc)
	m.RegisterTimer(time.Now().Add(2*time.Second), timer2Proc)
}

func (a *simpleMinion) GetState() SimpleMinionState {
	var state SimpleMinionState
	a.View(func(s *SimpleMinionState) {
		state = *s
		state.MinionState = cloneBase(&s.MinionState)
	})
	return state
}

// Convenient helpers to execute a command with arbitrary arguments
// They still have to be signaled to perform the action via the Signal method
// in order to avoid race conditions and ensure proper lifecycle management of the minion.

func (a *simpleMinion) Deposit(amount float64) error {
	return a.Signal(a.request("Deposit", amount))
}

func (a *simpleMinion) Withdraw(amount float64) error {
	return a.Signal(a.request("Withdraw", amount))
}

func (a *simpleMinion) request(command string, amount float64) MinionRequest {
	base := a.State()
	return MinionRequest{
		ID:        base.ID,
		ManagerID: base.ManagerID,
		Kind:      base.Kind,
		Command:   command,
		Args:      []any{fmt.Sprintf("%f", amount)},
	}
}
