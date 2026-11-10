package minions

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/khaledhikmat/psap/service"
)

func eventually(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestSimpleMinionLifecycle(t *testing.T) {
	m := NewSimpleMinion("m1", "mgr1", service.ServicesFactory{}, 100)
	if err := m.Deposit(1); !errors.Is(err, ErrNotRunning) {
		t.Fatalf("signal before start = %v, want ErrNotRunning", err)
	}
	if err := m.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := m.Start(context.Background()); err == nil {
		t.Fatal("second Start succeeded")
	}

	if err := m.Deposit(50); err != nil {
		t.Fatal(err)
	}
	if err := m.Withdraw(30); err != nil {
		t.Fatal(err)
	}
	if err := m.Signal(MinionRequest{Command: "Nope"}); err != nil { // logged, not fatal
		t.Fatal(err)
	}
	eventually(t, "balance 120", func() bool { return m.GetState().Balance == 120 })

	m.Stop()
	m.Stop() // idempotent

	st := m.GetState()
	if st.Running || !st.InitialActivitiesDone || !st.FinalActivitiesDone {
		t.Fatalf("state after stop = %+v", st.MinionState)
	}
	if st.SignalsCount != 3 || st.EndTime.IsZero() || st.StartTime.IsZero() {
		t.Fatalf("signals=%d start=%v end=%v", st.SignalsCount, st.StartTime, st.EndTime)
	}
	if st.ID != "m1" || st.ManagerID != "mgr1" || st.Kind != "simple" {
		t.Fatalf("identity = %q %q %q", st.ID, st.ManagerID, st.Kind)
	}
	if err := m.Deposit(1); !errors.Is(err, ErrNotRunning) {
		t.Fatalf("signal after stop = %v, want ErrNotRunning", err)
	}
}

// counterState is a custom T with its own minion behavior, built only from overriding processors.
type counterState struct {
	MinionState
	Count int
}

func TestGenericMinionOverridesAndTimers(t *testing.T) {
	var saves, initial, final atomic.Int32
	m := NewMinion("c1", "mgr", "counter", service.ServicesFactory{}, &counterState{})
	m.TickInterval = 5 * time.Millisecond
	m.SaveInterval = 20 * time.Millisecond
	m.OnSignal = func(ctx context.Context, s *counterState, req MinionRequest) error {
		s.Count++
		return nil
	}
	m.OnSave = func(ctx context.Context, s *counterState) error { saves.Add(1); return nil }
	m.OnRunInitialActivities = func(ctx context.Context, s *counterState) error { initial.Add(1); return nil }
	m.OnRunFinalActivities = func(ctx context.Context, s *counterState) error { final.Add(1); return nil }

	var fired atomic.Int32
	m.OnStart = func(mn *Minion[*counterState]) {
		// Two timers at the very same time must both survive.
		at := time.Now().Add(10 * time.Millisecond)
		mn.RegisterTimer(at, func(ctx context.Context, t time.Time) (bool, time.Duration) { fired.Add(1); return false, 0 })
		mn.RegisterTimer(at, func(ctx context.Context, t time.Time) (bool, time.Duration) { fired.Add(1); return false, 0 })
		// A timer that reschedules itself twice.
		mn.RegisterTimer(at, func(ctx context.Context, t time.Time) (bool, time.Duration) {
			n := fired.Add(100)
			return n < 300, 5 * time.Millisecond
		})
	}

	if err := m.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	for range 5 {
		if err := m.Signal(MinionRequest{Command: "inc"}); err != nil {
			t.Fatal(err)
		}
	}
	eventually(t, "timers", func() bool { return fired.Load() == 302 })
	eventually(t, "periodic save", func() bool { return saves.Load() >= 1 })

	m.Stop()
	if initial.Load() != 1 || final.Load() != 1 {
		t.Fatalf("activities initial=%d final=%d, want 1/1", initial.Load(), final.Load())
	}
	var count int
	m.View(func(s *counterState) { count = s.Count })
	if count != 5 {
		t.Fatalf("count = %d, want 5", count)
	}
	if got := m.State().TimerInvocationsCount; got != 5 { // 2 plain + 3 runs of the rescheduling one
		t.Fatalf("timer invocations = %d, want 5", got)
	}
}

func TestLoadHook(t *testing.T) {
	loaded := func(ctx context.Context, s *counterState) error {
		s.Count = 7
		s.ManagerID = "stale-manager" // must not survive: the manager may have changed
		return nil
	}

	m := NewMinion("c2", "new-manager", "counter", service.ServicesFactory{}, &counterState{})
	m.OnLoad = loaded
	if err := m.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer m.Stop()
	var count int
	m.View(func(s *counterState) { count = s.Count })
	if count != 7 || m.State().ManagerID != "new-manager" {
		t.Fatalf("count=%d manager=%q", count, m.State().ManagerID)
	}

	failing := NewMinion("c3", "mgr", "counter", service.ServicesFactory{}, &counterState{})
	boom := errors.New("boom")
	failing.OnLoad = func(ctx context.Context, s *counterState) error { return boom }
	if err := failing.Start(context.Background()); !errors.Is(err, boom) {
		t.Fatalf("Start = %v, want boom", err)
	}
	failing.Stop() // must not hang after a failed Start
}

func TestStopOnParentCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	m := NewSimpleMinion("m2", "mgr", service.ServicesFactory{}, 0)
	if err := m.Start(ctx); err != nil {
		t.Fatal(err)
	}
	cancel()
	eventually(t, "minion to stop", func() bool { return !m.GetState().Running && !m.GetState().EndTime.IsZero() })
}
