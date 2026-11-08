package mode

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/khaledhikmat/psap/service"
	"github.com/khaledhikmat/psap/service/lgr"

	"github.com/khaledhikmat/psap/minions"
)

// The minion manager is responsible for hosting minion workflows
func MinionManager(canxCtx context.Context, pod string, svcs service.ServicesFactory) error {
	lgr.Logger.Info(
		"Minion Manager starting...",
		slog.String("pod", pod),
	)

	// Keep track of active workflows in memory so we can enumerate and manage them
	workflows := make(map[string]minions.ISimpleMinion)

	defer func() {
		// Enumerate active workflows
		for id, wf := range workflows {
			fmt.Printf("Workflow ID: %s, Balance: %v\n", id, wf.GetState().Balance)
			wf.Stop()
		}
	}()

	// Dequeues workflow requests and manages minion lifecycles

	// Workflow IDs must be unique across the system - must be UUID or time-based
	simpleWF1 := minions.NewSimpleMinion("workflow-id-1", "manager-id", svcs, 100)
	simpleWF2 := minions.NewSimpleMinion("workflow-id-2", "manager-id", svcs, 100)
	simpleWF3 := minions.NewSimpleMinion("workflow-id-3", "manager-id", svcs, 100)
	workflows["workflow-id-1"] = simpleWF1
	workflows["workflow-id-2"] = simpleWF2
	workflows["workflow-id-3"] = simpleWF3
	simpleWF1.Start(canxCtx)
	simpleWF2.Start(canxCtx)
	simpleWF3.Start(canxCtx)

	// Deposit into all workflows
	simpleWF1.Deposit(50.00)
	simpleWF2.Deposit(50.00)
	simpleWF3.Deposit(50.00)

	// Withdraw from all workflows
	simpleWF1.Withdraw(50.00)
	simpleWF2.Withdraw(50.00)
	simpleWF3.Withdraw(50.00)

	// Keep the manager running for a while to allow workflows to timeout and complete their operations
	time.Sleep(20 * time.Second)

	return nil
}
