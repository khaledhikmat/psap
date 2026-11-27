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

type launchRevisionResult struct {
	State  model.LaunchState
	TierID string
}

func launchRevisionTestWorkflow(ctx workflow.Context) (launchRevisionResult, error) {
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 10 * time.Second,
	})
	w := newLaunch(ctx, LaunchRequest{
		BusinessOwner:   "owner-1",
		BusinessManager: "manager-1",
	})
	w.id = "launch-1"
	w.state = model.WaitingForTierApprovalLaunchState
	w.tierID = "tier-3"
	w.persisted = persistedLaunch{state: w.state, tierID: w.tierID}
	w.launchActs = launchactivities.NewActivities(nil, nil)
	w.notifyActs = notificationactivities.NewActivities(nil, nil)
	if err := w.setupUpdates(); err != nil {
		return launchRevisionResult{}, err
	}
	if err := w.waitForTierApproval(ctx); err != nil {
		return launchRevisionResult{}, err
	}
	return launchRevisionResult{State: w.state, TierID: w.tierID}, nil
}

type updateResult struct {
	result ReviseAnswersResult
	err    error
}

func (u *updateResult) Accept()          {}
func (u *updateResult) Reject(err error) { u.err = err }
func (u *updateResult) Complete(success interface{}, err error) {
	if err != nil {
		u.err = err
		return
	}
	u.result = success.(ReviseAnswersResult)
}

func TestLaunchManagerRevisionIsRecordedAndRecalculatesTier(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	launchActs := launchactivities.NewActivities(nil, nil)
	notifyActs := notificationactivities.NewActivities(nil, nil)
	env.RegisterWorkflow(launchRevisionTestWorkflow)
	env.RegisterActivity(launchActs)
	env.RegisterActivity(notifyActs)

	answers := []model.AnswerSubmission{{QuestionID: "q3", PredefinedAnswerIDs: []string{"a3-5"}}}
	env.OnActivity(launchActs.SaveAnswers, mock.Anything, launchactivities.LaunchAnswersSaveRequest{
		LaunchID:  "launch-1",
		Answers:   answers,
		UpdatedBy: "manager-1",
	}).Return(launchactivities.LaunchAnswersSaveResult{Answered: 8, Questions: 8}, nil).Once()
	env.OnActivity(launchActs.CalculateTier, mock.Anything, mock.Anything).
		Return(launchactivities.LaunchTierCalculationResult{Points: 450, TierID: "tier-1"}, nil).Once()
	env.OnActivity(launchActs.Persist, mock.Anything, launchactivities.LaunchPersistRequest{
		LaunchID: "launch-1",
		TierID:   "tier-1",
		State:    model.WaitingForTierApprovalLaunchState,
	}).Return(nil).Once()

	var outcome updateResult
	env.RegisterDelayedCallback(func() {
		env.UpdateWorkflow(LaunchReviseAnswersUpdateName, "", &outcome, ReviseAnswersUpdate{Answers: answers})
	}, time.Hour)
	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow(LaunchValidateTierSignalName, ValidateTierSignal{})
	}, 2*time.Hour)

	env.ExecuteWorkflow(launchRevisionTestWorkflow)
	if err := env.GetWorkflowError(); err != nil {
		t.Fatalf("workflow returned an error: %v", err)
	}
	if outcome.err != nil {
		t.Fatalf("revision failed: %v", outcome.err)
	}
	if outcome.result.TierID != "tier-1" || outcome.result.Points != 450 {
		t.Errorf("revision result = %+v, want tier-1 with 450 points", outcome.result)
	}

	var result launchRevisionResult
	if err := env.GetWorkflowResult(&result); err != nil {
		t.Fatalf("could not read workflow result: %v", err)
	}
	if result.State != model.TierValidatedLaunchState || result.TierID != "tier-1" {
		t.Errorf("result = %+v, want validated tier-1", result)
	}
	env.AssertExpectations(t)
}

func TestLaunchRevisionRejectedOutsideTierApproval(t *testing.T) {
	var w launch
	answers := []model.AnswerSubmission{{QuestionID: "q3"}}
	w.state = model.WaitingForAnswersLaunchState
	if err := w.validateRevision(nil, ReviseAnswersUpdate{Answers: answers}); err == nil {
		t.Error("revision while waiting for answers was accepted")
	}
	w.state = model.WaitingForTierApprovalLaunchState
	if err := w.validateRevision(nil, ReviseAnswersUpdate{}); err == nil {
		t.Error("empty revision was accepted")
	}
	if err := w.validateRevision(nil, ReviseAnswersUpdate{Answers: answers}); err != nil {
		t.Errorf("valid revision rejected: %v", err)
	}
}
