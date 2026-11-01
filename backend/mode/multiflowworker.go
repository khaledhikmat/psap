package mode

import (
	"context"
	"fmt"

	"github.com/khaledhikmat/psap/service"
	"github.com/khaledhikmat/psap/service/flow/activities/launch"
	"github.com/khaledhikmat/psap/service/flow/activities/notification"
	shieldactivities "github.com/khaledhikmat/psap/service/flow/activities/shield"
	"github.com/khaledhikmat/psap/service/flow/helpers"
	"github.com/khaledhikmat/psap/service/flow/workflows"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
)

const multiWorkerCount = 3

// Flow worker manager function that runs several workers on the same task queue
// This is more helpful for handling high concurrency scenarios
// where flows pull from different queues concurrently
func MultiWorker(canxCtx context.Context, pod string, svcs service.ServicesFactory) error {
	// A single client is safe to share across workers
	c, err := client.Dial(client.Options{})
	if err != nil {
		return err
	}
	defer c.Close()

	launchActs := launch.NewActivities(svcs.CfgSvc, svcs.DataSvc)
	notifyActs := notification.NewActivities(svcs.CfgSvc, svcs.DataSvc)
	shieldActs := &shieldactivities.Activities{DataSvc: svcs.DataSvc}

	workers := make([]worker.Worker, 0, multiWorkerCount)
	stopAll := func() {
		for _, w := range workers {
			w.Stop()
		}
	}

	for i := 0; i < multiWorkerCount; i++ {
		w := worker.New(c, helpers.WorkerQueueName, worker.Options{
			Identity: fmt.Sprintf("%s-worker-%d", pod, i),
		})
		w.RegisterWorkflow(workflows.Launch)
		w.RegisterWorkflow(workflows.Shield)
		w.RegisterActivity(launchActs)
		w.RegisterActivity(shieldActs)
		w.RegisterActivity(notifyActs)

		// Start is non-blocking, unlike Run
		if err := w.Start(); err != nil {
			stopAll()
			return fmt.Errorf("starting worker %d: %w", i, err)
		}
		workers = append(workers, w)
	}

	select {
	case <-canxCtx.Done():
	case <-worker.InterruptCh():
	}

	stopAll()
	return nil
}
