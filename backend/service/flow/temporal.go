package flow

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/khaledhikmat/psap/model"
	"github.com/khaledhikmat/psap/service/answers"
	"github.com/khaledhikmat/psap/service/config"
	"github.com/khaledhikmat/psap/service/data"
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
	// ErrInvalidAnswer is returned for answers to unknown questions, or selecting predefined
	// answers that do not belong to the question or that the question does not allow.
	ErrInvalidAnswer = answers.ErrInvalid
	// ErrInvalidTier is returned when validating a launch with a tier that does not exist.
	ErrInvalidTier = xerrors.New("invalid tier")

	// ErrShieldInProgress is returned by InitiateShield when a concurrent InitiateShield for the
	// same asset started its shield first.
	ErrShieldInProgress = xerrors.New("asset shield already in progress for this asset")
	// ErrInvalidShieldState is returned when a shield step is attempted in the wrong state.
	ErrInvalidShieldState = xerrors.New("shield is not in a state that allows this step")
	// ErrIncompleteShield is returned when finalizing shield answers or evidences while
	// required ones are still missing.
	ErrIncompleteShield = xerrors.New("shield is incomplete")
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

// Shield defaults (timeouts in seconds).
const (
	shieldReminderTimeout   = 60
	shieldMaxReminders      = 3
	shieldWarningTimeout    = 300
	shieldMaxWarnings       = 2
	shieldEscalationTimeout = 900
	shieldMaxEscalations    = 1

	// How long InitiateShield waits for a replaced shield to stop before terminating it.
	shieldCancelTimeout = 30 * time.Second
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
	if s.tClient == nil {
		return
	}

	s.tClient.Close()
}

func (s *service) InitiateLaunch(ctx context.Context, assetID string) error {
	asset, err := s.datasvc.RetrieveAssetByID(ctx, assetID)
	if err != nil {
		return err
	}
	if asset.BusinessOwnerID == nil || asset.BusinessManagerID == nil {
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
		BusinessOwner:     *asset.BusinessOwnerID,
		BusinessManager:   *asset.BusinessManagerID,
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

	launchID := ""
	if asset.LastLaunchID != nil {
		launchID = *asset.LastLaunchID
	}
	lgr.Logger.Info("Cancelling running launch", slog.String("WorkflowID", workflowID), slog.String("LaunchID", launchID))
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
	if asset.LastLaunchID == nil {
		return nil
	}
	launch, err := s.datasvc.RetrieveLaunchByID(ctx, *asset.LastLaunchID)
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
		TierID:      &result.TierID,
		State:       result.State,
		Reminders:   result.Reminders,
		Warnings:    result.Warnings,
		Escalations: result.Escalations,
	}, nil
}

// SubmitLaunchAnswers saves (possibly partial) answers: the predefined answers selected for each question.
func (s *service) SubmitLaunchAnswers(ctx context.Context, assetID string, submissions []model.AnswerSubmission) error {
	if err := s.checkAnswers(ctx, assetID, submissions); err != nil {
		return err
	}
	return s.signal(ctx, assetID, workflows.LaunchSubmitAnswersSignalName, workflows.SubmitAnswersSignal{
		Answers: submissions,
	})
}

// FinalizeLaunchAnswers saves the last answers (if any), locks them and calculates the tier.
// Questions still unanswered earn no points.
func (s *service) FinalizeLaunchAnswers(ctx context.Context, assetID string, submissions []model.AnswerSubmission) error {
	if err := s.checkAnswers(ctx, assetID, submissions); err != nil {
		return err
	}
	return s.signal(ctx, assetID, workflows.LaunchFinalizeAnswersSignalName, workflows.SubmitAnswersSignal{
		Answers: submissions,
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

// checkAnswers verifies the launch is waiting for answers and that the submissions are valid
// for the launch questions.
//
// The workflow ignores signals in the wrong state and rejects invalid answers itself, but a
// signal cannot report errors back, so they are checked here first.
func (s *service) checkAnswers(ctx context.Context, assetID string, submissions []model.AnswerSubmission) error {
	if err := s.expectState(ctx, assetID, model.WaitingForAnswersLaunchState); err != nil {
		return err
	}
	return s.validateAnswers(ctx, model.LaunchQuestionCategory, submissions)
}

func (s *service) validateAnswers(ctx context.Context, categoryType model.QuestionCategoryType, submissions []model.AnswerSubmission) error {
	questions, err := s.datasvc.RetrieveQuestionsByCategoryType(ctx, categoryType)
	if err != nil {
		return err
	}
	predefined, err := s.datasvc.RetrievePredefinedAnswersByCategoryType(ctx, categoryType)
	if err != nil {
		return err
	}
	return answers.Validate(questions, predefined, submissions)
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

// Shield methods

// InitiateShield starts a Shield workflow for an asset, or replaces its running shield if any.
// If there's a running shield, it's cancelled and a fresh shield starts from scratch.
func (s *service) InitiateShield(ctx context.Context, assetID string) error {
	asset, err := s.datasvc.RetrieveAssetByID(ctx, assetID)
	if err != nil {
		return err
	}
	if asset.BusinessOwnerID == nil || asset.BusinessManagerID == nil {
		return fmt.Errorf("asset %s needs a business owner and a business manager before it can be shielded", assetID)
	}

	// Reshielding replaces the asset's running shield, if any.
	if err := s.cancelRunningShield(ctx, asset); err != nil {
		return err
	}

	// Only one shield per asset may run at a time.
	workflowID := workflows.ShieldWorkflowID(assetID)
	options := client.StartWorkflowOptions{
		ID:                                       workflowID,
		TaskQueue:                                helpers.WorkerQueueName,
		WorkflowIDConflictPolicy:                 enumspb.WORKFLOW_ID_CONFLICT_POLICY_FAIL,
		WorkflowExecutionErrorWhenAlreadyStarted: true,
	}

	we, err := s.tClient.ExecuteWorkflow(ctx, options, workflows.Shield, workflows.ShieldRequest{
		AssetID:           assetID,
		BusinessOwner:     *asset.BusinessOwnerID,
		BusinessManager:   *asset.BusinessManagerID,
		ReminderTimeout:   shieldReminderTimeout,
		MaxReminders:      shieldMaxReminders,
		WarningTimeout:    shieldWarningTimeout,
		MaxWarnings:       shieldMaxWarnings,
		EscalationTimeout: shieldEscalationTimeout,
		MaxEscalations:    shieldMaxEscalations,
	})

	var alreadyStarted *serviceerror.WorkflowExecutionAlreadyStarted
	if errors.As(err, &alreadyStarted) {
		return ErrShieldInProgress
	}
	if err != nil {
		lgr.Logger.Error(
			"Unable to start workflow",
			slog.Any("error", xerrors.New(err.Error())),
		)
		return err
	}
	lgr.Logger.Info("Shield workflow started", slog.String("WorkflowID", we.GetID()), slog.String("RunID", we.GetRunID()))
	return nil
}

// cancelRunningShield cancels the asset's running shield workflow, if any, and waits for it
// to stop. It terminates the shield if cancellation times out.
func (s *service) cancelRunningShield(ctx context.Context, asset model.Asset) error {
	workflowID := workflows.ShieldWorkflowID(asset.ID)
	var notFound *serviceerror.NotFound
	desc, err := s.tClient.DescribeWorkflowExecution(ctx, workflowID, "")
	if errors.As(err, &notFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if desc.GetWorkflowExecutionInfo().GetStatus() != enumspb.WORKFLOW_EXECUTION_STATUS_RUNNING {
		return nil
	}

	shieldID := ""
	if asset.LastShieldID != nil {
		shieldID = *asset.LastShieldID
	}
	lgr.Logger.Info("Cancelling running shield", slog.String("WorkflowID", workflowID), slog.String("ShieldID", shieldID))
	if err := s.tClient.CancelWorkflow(ctx, workflowID, ""); err != nil {
		return err
	}

	waitCtx, cancel := context.WithTimeout(ctx, shieldCancelTimeout)
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
		return nil
	}

	lgr.Logger.Warn("Shield did not stop after cancel; terminating", slog.String("WorkflowID", workflowID))
	err = s.tClient.TerminateWorkflow(ctx, workflowID, "", "reshielded; cancellation timed out")
	if err != nil && !errors.As(err, &notFound) {
		return err
	}
	// Mark the cancelled shield as cancelled
	if asset.LastShieldID == nil {
		return nil
	}
	shield, err := s.datasvc.RetrieveShieldByID(ctx, *asset.LastShieldID)
	if errors.Is(err, data.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	shield.State = model.CancelledShieldState
	shield.UpdatedBy = "shield-service"
	return s.datasvc.UpdateShield(ctx, shield)
}

// QueryShield queries the shield workflow for its current state and returns it as a Shield.
func (s *service) QueryShield(ctx context.Context, assetID string) (model.Shield, error) {
	result, err := s.queryShield(ctx, assetID)
	if err != nil {
		return model.Shield{}, err
	}
	return model.Shield{
		ID:          result.ID,
		AssetID:     result.AssetID,
		Score:       result.Score,
		State:       result.State,
		Reminders:   result.Reminders,
		Warnings:    result.Warnings,
		Escalations: result.Escalations,
	}, nil
}

// SubmitShieldAnswers sends answers to the shield workflow.
func (s *service) SubmitShieldAnswers(ctx context.Context, assetID string, submissions []model.AnswerSubmission) error {
	if err := s.validateAnswers(ctx, model.ShieldQuestionCategory, submissions); err != nil {
		return err
	}
	return s.signalShield(ctx, assetID, workflows.ShieldSubmitAnswersSignalName, workflows.SubmitAnswersShieldSignal{
		Answers: submissions,
	})
}

// FinalizeShieldAnswers sends final answers and locks the shield answers. Every shield question
// must be answered by then.
func (s *service) FinalizeShieldAnswers(ctx context.Context, assetID string, submissions []model.AnswerSubmission) error {
	if err := s.validateAnswers(ctx, model.ShieldQuestionCategory, submissions); err != nil {
		return err
	}
	if err := s.checkShieldAnswersComplete(ctx, assetID, submissions); err != nil {
		return err
	}
	return s.signalShield(ctx, assetID, workflows.ShieldFinalizeAnswersSignalName, workflows.SubmitAnswersShieldSignal{
		Answers: submissions,
	})
}

// SubmitShieldEvidences sends evidences to the shield workflow.
func (s *service) SubmitShieldEvidences(ctx context.Context, assetID string, evidences []model.EvidenceSubmission) error {
	if err := s.checkEvidences(ctx, assetID, evidences); err != nil {
		return err
	}
	return s.signalShield(ctx, assetID, workflows.ShieldSubmitEvidencesSignalName, workflows.SubmitEvidencesShieldSignal{
		Evidences: evidences,
	})
}

// FinalizeShieldEvidences sends final evidences and locks them. Every question that requires
// evidence must have some by then.
func (s *service) FinalizeShieldEvidences(ctx context.Context, assetID string, evidences []model.EvidenceSubmission) error {
	if err := s.checkEvidences(ctx, assetID, evidences); err != nil {
		return err
	}
	if err := s.checkShieldEvidencesComplete(ctx, assetID, evidences); err != nil {
		return err
	}
	return s.signalShield(ctx, assetID, workflows.ShieldFinalizeEvidencesSignalName, workflows.SubmitEvidencesShieldSignal{
		Evidences: evidences,
	})
}

// checkShieldAnswersComplete verifies that the stored answers plus the final submissions
// cover every shield question.
func (s *service) checkShieldAnswersComplete(ctx context.Context, assetID string, final []model.AnswerSubmission) error {
	shield, err := s.queryShield(ctx, assetID)
	if err != nil {
		return err
	}
	questions, err := s.datasvc.RetrieveQuestionsByCategoryType(ctx, model.ShieldQuestionCategory)
	if err != nil {
		return err
	}
	stored, err := s.datasvc.RetrieveUserAnswersByShieldID(ctx, shield.ID)
	if err != nil {
		return err
	}
	answered := make(map[string]bool, len(questions))
	for _, ua := range stored {
		answered[ua.QuestionID] = len(ua.Selections) > 0
	}
	for _, sub := range final {
		answered[sub.QuestionID] = len(sub.PredefinedAnswerIDs) > 0
	}
	missing := []string{}
	for _, q := range questions {
		if !answered[q.ID] {
			missing = append(missing, q.ID)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("%w: unanswered questions: %s", ErrIncompleteShield, strings.Join(missing, ", "))
	}
	return nil
}

// checkShieldEvidencesComplete verifies that the stored evidences plus the final submissions
// cover every question that requires evidence.
func (s *service) checkShieldEvidencesComplete(ctx context.Context, assetID string, final []model.EvidenceSubmission) error {
	shield, err := s.queryShield(ctx, assetID)
	if err != nil {
		return err
	}
	questions, err := s.datasvc.RetrieveQuestionsByCategoryType(ctx, model.ShieldQuestionCategory)
	if err != nil {
		return err
	}
	stored, err := s.datasvc.RetrieveEvidencesByShieldID(ctx, shield.ID)
	if err != nil {
		return err
	}
	provided := make(map[string]bool, len(questions))
	for _, e := range stored {
		provided[e.QuestionID] = e.Description != "" || e.URL != ""
	}
	for _, e := range final {
		provided[e.QuestionID] = e.Description != "" || e.URL != ""
	}
	missing := []string{}
	for _, q := range questions {
		if q.EvidenceRequired && !provided[q.ID] {
			missing = append(missing, q.ID)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("%w: missing evidence for: %s", ErrIncompleteShield, strings.Join(missing, ", "))
	}
	return nil
}

// checkEvidences verifies that each evidence is for an answered question of the asset's shield.
// The workflow cannot report errors back from a signal, so this is checked before signaling.
func (s *service) checkEvidences(ctx context.Context, assetID string, evidences []model.EvidenceSubmission) error {
	shield, err := s.queryShield(ctx, assetID)
	if err != nil {
		return err
	}
	answered, err := s.datasvc.RetrieveUserAnswersByShieldID(ctx, shield.ID)
	if err != nil {
		return err
	}
	hasAnswer := make(map[string]bool, len(answered))
	for _, ua := range answered {
		hasAnswer[ua.QuestionID] = len(ua.Selections) > 0
	}
	for _, e := range evidences {
		if !hasAnswer[e.QuestionID] {
			return fmt.Errorf("%w: question %q has no answer to attach evidence to", ErrInvalidAnswer, e.QuestionID)
		}
	}
	return nil
}

// queryShield queries the workflow's current state.
func (s *service) queryShield(ctx context.Context, assetID string) (workflows.ShieldResult, error) {
	var result workflows.ShieldResult
	val, err := s.tClient.QueryWorkflow(ctx, workflows.ShieldWorkflowID(assetID), "", workflows.ShieldStateQueryName)
	if err != nil {
		lgr.Logger.Error(
			"Unable to query shield workflow",
			slog.Any("error", xerrors.New(err.Error())),
		)
		return result, err
	}
	if err := val.Get(&result); err != nil {
		lgr.Logger.Error(
			"Unable to decode shield workflow query result",
			slog.Any("error", xerrors.New(err.Error())),
		)
		return result, err
	}
	return result, nil
}

// signalShield sends a signal to a shield workflow.
func (s *service) signalShield(ctx context.Context, assetID string, signalName string, arg interface{}) error {
	err := s.tClient.SignalWorkflow(ctx, workflows.ShieldWorkflowID(assetID), "", signalName, arg)
	if err != nil {
		lgr.Logger.Error(
			"Unable to signal shield workflow",
			slog.String("signal", signalName),
			slog.Any("error", xerrors.New(err.Error())),
		)
		return err
	}
	return nil
}
