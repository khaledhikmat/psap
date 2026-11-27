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

// Flow worker manager function
// A single worker tuned for concurrency via options (see MultiWorker for the multi-worker variant)
func Worker(canxCtx context.Context, pod string, svcs service.ServicesFactory) error {
	c, err := client.Dial(client.Options{})
	if err != nil {
		return fmt.Errorf("unable to create Temporal client: %w", err)
	}
	defer c.Close()

	w := worker.New(c, helpers.WorkerQueueName, worker.Options{
		Identity:                               pod + "-worker",
		MaxConcurrentWorkflowTaskPollers:       4,
		MaxConcurrentActivityTaskPollers:       4,
		MaxConcurrentWorkflowTaskExecutionSize: 100,
		MaxConcurrentActivityExecutionSize:     100,
	})

	w.RegisterWorkflow(workflows.Launch)
	w.RegisterWorkflow(workflows.Shield)
	w.RegisterActivity(launch.NewActivities(svcs.CfgSvc, svcs.DataSvc))
	w.RegisterActivity(&shieldactivities.Activities{DataSvc: svcs.DataSvc})
	w.RegisterActivity(notification.NewActivities(svcs.CfgSvc, svcs.DataSvc))

	err = w.Run(worker.InterruptCh())
	if err != nil {
		return err
	}

	return nil
}
