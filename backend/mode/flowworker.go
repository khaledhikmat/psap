package mode

import (
	"context"
	"log"

	"github.com/khaledhikmat/psap/service"
	"github.com/khaledhikmat/psap/service/flow/activities/launch"
	"github.com/khaledhikmat/psap/service/flow/activities/notification"
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
		log.Fatalln("Unable to create client", err)
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
	w.RegisterActivity(launch.NewActivities(svcs.CfgSvc, svcs.DataSvc))
	w.RegisterActivity(notification.NewActivities(svcs.CfgSvc, svcs.DataSvc))

	err = w.Run(worker.InterruptCh())
	if err != nil {
		return err
	}

	return nil
}
