package flow

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/khaledhikmat/psap/model"
	"github.com/khaledhikmat/psap/service/config"
	"github.com/khaledhikmat/psap/service/data"
	launchactivities "github.com/khaledhikmat/psap/service/flow/activities/launch"
	"github.com/khaledhikmat/psap/service/flow/helpers"
	"github.com/khaledhikmat/psap/service/flow/workflows"
	"github.com/khaledhikmat/psap/service/lgr"
	"golang.org/x/xerrors"

	enumspb "go.temporal.io/api/enums/v1"
	"go.temporal.io/api/serviceerror"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/temporal"
)

var (
	// ErrLaunchInProgress is returned by InitiateLaunch when a concurrent InitiateLaunch for the
	// same asset started its launch first.
	ErrLaunchInProgress = xerrors.New("asset launch already in progress for this asset")
	// ErrInvalidLaunchState is returned when a launch step is attempted in the wrong state,
	// e.g. submitting answers after they were finalized, or validating a tier before.
	ErrInvalidLaunchState = xerrors.New("launch is not in a state that allows this step")
	// ErrInvalidAnswer is returned for answers to unknown questions or with a reply other than YES/NO.
	ErrInvalidAnswer = xerrors.New("invalid answer")
	// ErrInvalidTier is returned when validating a launch with a tier that does not exist.
	ErrInvalidTier = xerrors.New("invalid tier")
)

// Launch defaults (timeouts in seconds).
// TODO: Source these from config once config.IService exposes them
const (
	launchReminderTimeout   = 60
	launchMaxReminders      = 3
	launchWarningTimeout    = 300
	launchMaxWarnings       = 2
	launchEscalationTimeout = 900
	launchMaxEscalations    = 1

	// How long InitiateLaunch waits for a replaced launch to stop before terminating it.
	launchCancelTimeout = 30 * time.Second
)

type service struct {
	cfgsvc  config.IService
	datasvc data.IService
	tClient client.Client
}

func NewTemporal(cfgsvc config.IService, datasvc data.IService) IService {
	// Create Temporal client here if needed
	t, err := client.Dial(client.Options{})
	if err != nil {
		lgr.Logger.Error(
			"Unable to create Temporal client",
			slog.Any("error", xerrors.New(err.Error())),
		)
		return nil
	}

	return &service{
		cfgsvc:  cfgsvc,
		datasvc: datasvc,
		tClient: t,
	}
}

func (s *service) Finalize() {
	s.tClient.Close()
}

func (s *service) InitiateLaunch(ctx context.Context, assetID string) error {
	asset, err := s.datasvc.RetrieveAssetByID(ctx, assetID)
	if err != nil {
		return err
	}
	if asset.BusinessOwnerID == "" || asset.BusinessManagerID == "" {
		return fmt.Errorf("asset %s needs a business owner and a business manager before it can be launched", assetID)
	}

	// Relaunching (e.g. to re-tier the asset) replaces the asset's running launch, if any.
	if err := s.cancelRunningLaunch(ctx, asset); err != nil {
		return err
	}

	// Only one launch per asset may run at a time. Temporal enforces this atomically through
	// the workflow ID: with the FAIL conflict policy, starting a workflow whose ID is already
	// running returns WorkflowExecutionAlreadyStarted, which only happens here if a concurrent
	// InitiateLaunch started one first.
	workflowID := workflows.LaunchWorkflowID(assetID)
	options := client.StartWorkflowOptions{
		ID:                                       workflowID,
		TaskQueue:                                helpers.WorkerQueueName,
		WorkflowIDConflictPolicy:                 enumspb.WORKFLOW_ID_CONFLICT_POLICY_FAIL,
		WorkflowExecutionErrorWhenAlreadyStarted: true,
	}

	we, err := s.tClient.ExecuteWorkflow(ctx, options, workflows.Launch, workflows.LaunchRequest{
		AssetID:           assetID,
		BusinessOwner:     asset.BusinessOwnerID,
		BusinessManager:   asset.BusinessManagerID,
		ReminderTimeout:   launchReminderTimeout,
		MaxReminders:      launchMaxReminders,
		WarningTimeout:    launchWarningTimeout,
		MaxWarnings:       launchMaxWarnings,
		EscalationTimeout: launchEscalationTimeout,
		MaxEscalations:    launchMaxEscalations,
	})
	var alreadyStarted *serviceerror.WorkflowExecutionAlreadyStarted
	if errors.As(err, &alreadyStarted) {
		return ErrLaunchInProgress
	}
	if err != nil {
		lgr.Logger.Error(
			"Unable to execute workflow",
			slog.Any("error", xerrors.New(err.Error())),
		)
		return err
	}

	lgr.Logger.Info(
		"Started workflow",
		slog.String("WorkflowID", we.GetID()),
		slog.String("RunID", we.GetRunID()),
	)

	return nil
}

// cancelRunningLaunch cancels the asset's running launch workflow and waits for it to stop.
// The workflow marks its launch cancelled itself. If it does not stop within
// launchCancelTimeout, it is terminated and its launch marked cancelled here.
func (s *service) cancelRunningLaunch(ctx context.Context, asset model.Asset) error {
	workflowID := workflows.LaunchWorkflowID(asset.ID)
	desc, err := s.tClient.DescribeWorkflowExecution(ctx, workflowID, "")
	var notFound *serviceerror.NotFound
	if errors.As(err, &notFound) {
		return nil // never launched
	}
	if err != nil {
		return err
	}
	if desc.GetWorkflowExecutionInfo().GetStatus() != enumspb.WORKFLOW_EXECUTION_STATUS_RUNNING {
		return nil
	}

	lgr.Logger.Info("Cancelling running launch", slog.String("WorkflowID", workflowID), slog.String("LaunchID", asset.LastLaunchID))
	if err := s.tClient.CancelWorkflow(ctx, workflowID, ""); err != nil {
		return err
	}

	waitCtx, cancel := context.WithTimeout(ctx, launchCancelTimeout)
	defer cancel()
	err = s.tClient.GetWorkflow(waitCtx, workflowID, "").Get(waitCtx, nil)
	var cancelled *temporal.CanceledError
	if err == nil || errors.As(err, &cancelled) {
		return nil
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if waitCtx.Err() == nil {
		return nil // the workflow ended some other way (e.g. it completed just before the cancel)
	}

	lgr.Logger.Warn("Launch did not stop after cancel; terminating", slog.String("WorkflowID", workflowID))
	err = s.tClient.TerminateWorkflow(ctx, workflowID, "", "relaunched; cancellation timed out")
	if err != nil && !errors.As(err, &notFound) {
		return err
	}
	// The asset's last launch is the one that was running: Create sets it when a launch starts.
	if asset.LastLaunchID == "" {
		return nil
	}
	launch, err := s.datasvc.RetrieveLaunchByID(ctx, asset.LastLaunchID)
	if errors.Is(err, data.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	launch.State = model.CancelledLaunchState
	launch.UpdatedBy = "flow-service"
	return s.datasvc.UpdateLaunch(ctx, launch)
}

func (s *service) QueryLaunch(ctx context.Context, assetID string) (model.Launch, error) {
	result, err := s.queryLaunch(ctx, assetID)
	if err != nil {
		return model.Launch{}, err
	}
	return model.Launch{
		ID:          result.ID,
		AssetID:     result.AssetID,
		TierID:      result.TierID,
		State:       result.State,
		Reminders:   result.Reminders,
		Warnings:    result.Warnings,
		Escalations: result.Escalations,
	}, nil
}

// SubmitLaunchAnswers saves (possibly partial) answers, keyed by QuestionID with a YES/NO Reply.
func (s *service) SubmitLaunchAnswers(ctx context.Context, assetID string, answers []model.Answer) error {
	answers, err := s.checkAnswers(ctx, assetID, answers)
	if err != nil {
		return err
	}
	return s.signal(ctx, assetID, workflows.LaunchSubmitAnswersSignalName, workflows.SubmitAnswersSignal{
		Answers: answers,
	})
}

// FinalizeLaunchAnswers saves the last answers (if any), locks them and calculates the tier.
// Questions still unanswered count as NO.
func (s *service) FinalizeLaunchAnswers(ctx context.Context, assetID string, answers []model.Answer) error {
	answers, err := s.checkAnswers(ctx, assetID, answers)
	if err != nil {
		return err
	}
	return s.signal(ctx, assetID, workflows.LaunchFinalizeAnswersSignalName, workflows.SubmitAnswersSignal{
		Answers: answers,
	})
}

// ValidateTier approves the launch's tier. An empty tierID accepts the calculated tier; any
// other value overrides it.
func (s *service) ValidateTier(ctx context.Context, assetID string, tierID string) error {
	if err := s.expectState(ctx, assetID, model.WaitingForTierApprovalLaunchState); err != nil {
		return err
	}
	if tierID != "" {
		if _, err := s.datasvc.RetrieveTierByID(ctx, tierID); errors.Is(err, data.ErrNotFound) {
			return fmt.Errorf("%w: %s", ErrInvalidTier, tierID)
		} else if err != nil {
			return err
		}
	}
	return s.signal(ctx, assetID, workflows.LaunchValidateTierSignalName, workflows.ValidateTierSignal{
		TierID: tierID,
	})
}

// checkAnswers verifies the launch is waiting for answers and that every answer is for a
// launch question with a valid reply. It returns the answers with normalized replies.
//
// The workflow ignores signals in the wrong state and rejects invalid answers itself, but a
// signal cannot report errors back, so they are checked here first.
func (s *service) checkAnswers(ctx context.Context, assetID string, answers []model.Answer) ([]model.Answer, error) {
	if err := s.expectState(ctx, assetID, model.WaitingForAnswersLaunchState); err != nil {
		return nil, err
	}
	questions, err := s.datasvc.RetrieveQuestionsByCategoryType(ctx, model.LaunchQuestionCategory)
	if err != nil {
		return nil, err
	}
	known := make(map[string]bool, len(questions))
	for _, q := range questions {
		known[q.ID] = true
	}

	checked := make([]model.Answer, len(answers))
	for i, a := range answers {
		if !known[a.QuestionID] {
			return nil, fmt.Errorf("%w: %q is not a launch question", ErrInvalidAnswer, a.QuestionID)
		}
		reply, err := launchactivities.NormalizeReply(a.Reply)
		if err != nil {
			return nil, fmt.Errorf("%w: question %s: %v", ErrInvalidAnswer, a.QuestionID, err)
		}
		checked[i] = model.Answer{QuestionID: a.QuestionID, Reply: reply}
	}
	return checked, nil
}

func (s *service) expectState(ctx context.Context, assetID string, want model.LaunchState) error {
	result, err := s.queryLaunch(ctx, assetID)
	if err != nil {
		return err
	}
	if result.State != want {
		return fmt.Errorf("%w: launch is %s, want %s", ErrInvalidLaunchState, result.State, want)
	}
	return nil
}

// queryLaunch queries the workflow's current state. Unlike WorkflowRun.Get, this does not
// block until the workflow completes.
func (s *service) queryLaunch(ctx context.Context, assetID string) (workflows.LaunchResult, error) {
	var result workflows.LaunchResult
	val, err := s.tClient.QueryWorkflow(ctx, workflows.LaunchWorkflowID(assetID), "", workflows.LaunchStateQueryName)
	if err != nil {
		lgr.Logger.Error(
			"Unable to query workflow",
			slog.Any("error", xerrors.New(err.Error())),
		)
		return result, err
	}
	if err := val.Get(&result); err != nil {
		lgr.Logger.Error(
			"Unable to decode workflow query result",
			slog.Any("error", xerrors.New(err.Error())),
		)
		return result, err
	}
	return result, nil
}

func (s *service) signal(ctx context.Context, assetID string, signalName string, payload any) error {
	err := s.tClient.SignalWorkflow(ctx, workflows.LaunchWorkflowID(assetID), "", signalName, payload)
	if err != nil {
		lgr.Logger.Error(
			"Unable to signal workflow",
			slog.String("signal", signalName),
			slog.Any("error", xerrors.New(err.Error())),
		)
		return err
	}
	return nil
}
