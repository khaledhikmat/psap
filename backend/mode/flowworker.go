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
// TODO: launch multiple workers
func Worker(canxCtx context.Context, pod string, svcs service.ServicesFactory) error {
	c, err := client.Dial(client.Options{})
	if err != nil {
		log.Fatalln("Unable to create client", err)
	}
	defer c.Close()

	w := worker.New(c, helpers.WorkerQueueName, worker.Options{})

	w.RegisterWorkflow(workflows.Launch)
	w.RegisterActivity(launch.Create)
	w.RegisterActivity(launch.AnalyzeAnswers)
	w.RegisterActivity(notification.Notify)

	err = w.Run(worker.InterruptCh())
	if err != nil {
		return err
	}

	return nil
}
