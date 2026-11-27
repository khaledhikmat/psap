package workflows

import (
	"time"

	"go.temporal.io/sdk/workflow"
)

// waitCase is something a workflow can wait on: a signal or a timeout.
type waitCase interface {
	addTo(s workflow.Selector)
}

type signalCase struct {
	ch      workflow.ReceiveChannel
	receive func(c workflow.ReceiveChannel)
}

// onSignal waits for the named signal, decodes its payload into T and passes it to fn.
func onSignal[T any](ctx workflow.Context, name string, fn func(T)) waitCase {
	return &signalCase{
		ch: workflow.GetSignalChannel(ctx, name),
		receive: func(c workflow.ReceiveChannel) {
			var payload T
			c.Receive(ctx, &payload)
			fn(payload)
		},
	}
}

func (c *signalCase) addTo(s workflow.Selector) {
	s.AddReceive(c.ch, func(ch workflow.ReceiveChannel, _ bool) { c.receive(ch) })
}

type timeoutCase struct {
	ctx      workflow.Context
	d        time.Duration
	fn       func() bool
	f        workflow.Future
	disabled bool
}

// onTimeout calls fn every time d elapses. The timer survives across waitUntil iterations
// (signals do not reset it) and is re-armed only after it fires. fn returns false to stop
// the timer for good. A non-positive duration disables the timeout.
func onTimeout(ctx workflow.Context, d time.Duration, fn func() bool) waitCase {
	return &timeoutCase{ctx: ctx, d: d, fn: fn, disabled: d <= 0}
}

func (t *timeoutCase) addTo(s workflow.Selector) {
	if t.disabled {
		return
	}
	if t.f == nil {
		t.f = workflow.NewTimer(t.ctx, t.d)
	}
	s.AddFuture(t.f, func(workflow.Future) {
		t.f = nil
		t.disabled = !t.fn()
	})
}

// waitUntil blocks, dispatching signals and timeouts to their handlers, until done() returns
// true or the workflow is cancelled (then it returns ctx.Err()). If Temporal suggests
// Continue-As-New, pending signals are drained first (so none are lost) and the error
// returned by continueAsNew is returned.
func waitUntil(ctx workflow.Context, done func() bool, continueAsNew func() error, cases ...waitCase) error {
	for !done() {
		s := workflow.NewSelector(ctx)
		s.AddReceive(ctx.Done(), func(workflow.ReceiveChannel, bool) {})
		for _, c := range cases {
			c.addTo(s)
		}
		s.Select(ctx)
		if ctx.Err() != nil {
			return ctx.Err()
		}

		if !done() && workflow.GetInfo(ctx).GetContinueAsNewSuggested() {
			for s.HasPending() && !done() {
				s.Select(ctx)
			}
			if !done() {
				return continueAsNew()
			}
		}
	}
	return nil
}
