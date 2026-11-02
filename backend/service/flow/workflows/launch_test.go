package workflows

import (
	"testing"
	"time"

	"github.com/khaledhikmat/psap/model"
	launchactivities "github.com/khaledhikmat/psap/service/flow/activities/launch"
	notificationactivities "github.com/khaledhikmat/psap/service/flow/activities/notification"
	"github.com/stretchr/testify/mock"
	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/workflow"
)

type launchApprovalWaitResult struct {
	State   model.LaunchState
	Elapsed time.Duration
}

func launchApprovalTimeoutTestWorkflow(ctx workflow.Context) (launchApprovalWaitResult, error) {
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 10 * time.Second,
	})
	w := newLaunch(ctx, LaunchRequest{
		BusinessManager: "manager-1",
		MaxEscalations:  1,
	})
	w.id = "launch-1"
	w.state = model.WaitingForTierApprovalLaunchState
	w.tierApprovalDeadline = workflow.Now(ctx).Add(launchTierApprovalTimeout)
	w.persisted = persistedLaunch{state: w.state}
	w.launchActs = launchactivities.NewActivities(nil, nil)
	w.notifyActs = notificationactivities.NewActivities(nil, nil)
	startedAt := workflow.Now(ctx)

	if err := w.waitForTierApproval(ctx); err != nil {
		return launchApprovalWaitResult{}, err
	}
	return launchApprovalWaitResult{
		State:   w.state,
		Elapsed: workflow.Now(ctx).Sub(startedAt),
	}, nil
}

func TestLaunchTierApprovalTimesOutAfterSevenDays(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	launchActs := launchactivities.NewActivities(nil, nil)
	notifyActs := notificationactivities.NewActivities(nil, nil)
	env.RegisterWorkflow(launchApprovalTimeoutTestWorkflow)
	env.RegisterActivity(launchActs)
	env.RegisterActivity(notifyActs)
	env.OnActivity(launchActs.Persist, mock.Anything, mock.Anything).Return(nil)
	env.OnActivity(notifyActs.Notify, mock.Anything, mock.Anything).
		Return(notificationactivities.NotificationResult{Success: true}, nil).
		Once()

	env.ExecuteWorkflow(launchApprovalTimeoutTestWorkflow)
	if !env.IsWorkflowCompleted() {
		t.Fatal("workflow did not complete")
	}
	if err := env.GetWorkflowError(); err != nil {
		t.Fatalf("workflow returned an error: %v", err)
	}

	var result launchApprovalWaitResult
	if err := env.GetWorkflowResult(&result); err != nil {
		t.Fatalf("could not read workflow result: %v", err)
	}
	if result.State != model.TimedOutLaunchState {
		t.Errorf("state = %q, want %q", result.State, model.TimedOutLaunchState)
	}
	if result.Elapsed != launchTierApprovalTimeout {
		t.Errorf("elapsed = %s, want %s", result.Elapsed, launchTierApprovalTimeout)
	}
	env.AssertExpectations(t)
}
