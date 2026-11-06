package mode

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
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

	simpleWF := minions.NewSimpleMinion("workflow-id", "manager-id", "simple", svcs)
	simpleWF.Start(canxCtx)
	defer simpleWF.Stop()

	var wg sync.WaitGroup

	// Concurrently deposit from 10 goroutines without mutexes
	for i := range 10 {
		index := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			if index%2 == 0 {
				simpleWF.Deposit(100.00)
			} else {
				simpleWF.Withdraw(50.00)
			}
		}()
	}

	wg.Wait()
	fmt.Printf("Final state: %v\n", simpleWF.GetState().Balance)
	time.Sleep(20 * time.Second)

	return nil
}
