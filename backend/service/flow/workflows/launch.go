package workflows

import (
	"time"

	"github.com/khaledhikmat/psap/model"
	launchactivities "github.com/khaledhikmat/psap/service/flow/activities/launch"
	notificationactivities "github.com/khaledhikmat/psap/service/flow/activities/notification"
	"go.temporal.io/sdk/workflow"
)

// Signals
const (
	LaunchSubmitAnswersSignalName   = "launch-submit-answers-signal"
	LaunchFinalizeAnswersSignalName = "launch-finalize-answers-signal"
	LaunchValidateTierSignalName    = "launch-validate-tier-signal"
)

// Queries
const (
	LaunchStateQueryName = "launch-state-query"
)

type LaunchRequest struct {
	AssetID           string
	Reviewer          string
	ReminderTimeout   int
	MaxReminders      int
	WarningTimeout    int
	MaxWarnings       int
	EscalationTimeout int
	MaxEscalations    int
}

type LaunchResult struct {
	ID          string
	AssetID     string
	RunningTime time.Duration
	Answers     []model.Answer
	State       model.LaunchState
	TierID      string
	Reviewer    string
	Reminders   int
	Warnings    int
	Escalations int
}

type SubmitAnswersSignal struct {
	Answers []model.Answer
}

type ValidateTierSignal struct {
	TierID string
}

func Launch(ctx workflow.Context, request LaunchRequest) (LaunchResult, error) {
	// State
	// These fields ARE the workflow's durable state. They aren't persisted directly —
	// Temporal reconstructs them by replaying history events (activity completions, signals,
	// timers) through this code. That's why mutations must only happen via deterministic
	// paths (activity results, signal handlers, Workflow.* APIs).
	var ID string = ""
	var tierID string = ""
	var answers []model.Answer = []model.Answer{}
	var reminders = 0
	var ReminderTimeout time.Duration = time.Duration(request.ReminderTimeout) * time.Second
	var warnings = 0
	var WarningTimeout time.Duration = time.Duration(request.WarningTimeout) * time.Second
	var escalations = 0
	var EscalationTimeout time.Duration = time.Duration(request.EscalationTimeout) * time.Second

	var state = model.WaitingForAnswersLaunchState
	var startedAt time.Time = workflow.Now(ctx)

	ao := workflow.ActivityOptions{
		StartToCloseTimeout: time.Second * 10,
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	logger := workflow.GetLogger(ctx)
	startedAt = workflow.Now(ctx)
	logger.Info("Starting launch workflow", "assetID", request.AssetID, "startedAt", startedAt)

	// Setup query handlers for the workflow state
	err := workflow.SetQueryHandler(ctx, LaunchStateQueryName, func() (LaunchResult, error) {
		return LaunchResult{
			ID:          ID,
			RunningTime: workflow.Now(ctx).Sub(startedAt),
			State:       state,
			TierID:      tierID,
			Reviewer:    request.Reviewer,
			Answers:     answers,
			Reminders:   reminders,
			Warnings:    warnings,
			Escalations: escalations,
		}, nil
	})
	if err != nil {
		return LaunchResult{}, err
	}

	// Create Launch
	var createResult launchactivities.LaunchCreationResult
	err = workflow.ExecuteActivity(ctx, launchactivities.Create, launchactivities.LaunchCreationRequest{
		AssetID: request.AssetID,
	}).Get(ctx, &createResult)
	if err != nil {
		return LaunchResult{}, err
	}
	ID = createResult.LaunchID
	state = model.WaitingForAnswersLaunchState

	// Block until all answers are submitted, tier validated, timed out (reminder, warning and escalations) or Continue-As-New is suggested
	// Accept partial answers
	for {
		selector := workflow.NewSelector(ctx)
		selector.AddReceive(workflow.GetSignalChannel(ctx, LaunchSubmitAnswersSignalName), func(c workflow.ReceiveChannel, more bool) {
			var signal SubmitAnswersSignal
			c.Receive(ctx, &signal)
			logger.Info("Answers submitted", "assetID", request.AssetID, "LaunchID", ID)
			answers = signal.Answers

			var answersAnalysisResult launchactivities.LaunchAnswersAnalysisResult
			err := workflow.ExecuteActivity(ctx, launchactivities.AnalyzeAnswers, launchactivities.LaunchAnswersAnalysisRequest{
				AssetID:  request.AssetID,
				LaunchID: ID,
				Answers:  answers,
			}).Get(ctx, &answersAnalysisResult)
			if err != nil {
				logger.Error("Failed to analyze answers", "assetID", request.AssetID, "LaunchID", ID, "error", err)
			}
			tierID = answersAnalysisResult.TierID
		})
		selector.AddReceive(workflow.GetSignalChannel(ctx, LaunchFinalizeAnswersSignalName), func(c workflow.ReceiveChannel, more bool) {
			var signal SubmitAnswersSignal
			c.Receive(ctx, &signal)
			logger.Info("Answers Finalized", "assetID", request.AssetID, "LaunchID", ID)
			answers = signal.Answers
			var answersAnalysisResult launchactivities.LaunchAnswersAnalysisResult
			err := workflow.ExecuteActivity(ctx, launchactivities.AnalyzeAnswers, launchactivities.LaunchAnswersAnalysisRequest{
				AssetID:  request.AssetID,
				LaunchID: ID,
				Answers:  answers,
			}).Get(ctx, &answersAnalysisResult)
			if err != nil {
				logger.Error("Failed to analyze answers", "assetID", request.AssetID, "LaunchID", ID, "error", err)
			}
			tierID = answersAnalysisResult.TierID
			state = model.WaitingForTierApprovalLaunchState
		})
		selector.AddReceive(workflow.GetSignalChannel(ctx, LaunchValidateTierSignalName), func(c workflow.ReceiveChannel, more bool) {
			var signal ValidateTierSignal
			c.Receive(ctx, &signal)
			logger.Info("Tier validated", "assetID", request.AssetID, "LaunchID", ID)
			tierID = signal.TierID
			state = model.TierValidatedLaunchState
		})
		selector.AddFuture(workflow.NewTimer(ctx, ReminderTimeout), func(f workflow.Future) {
			logger.Info("Launch reminder timed out", "assetID", request.AssetID, "LaunchID", ID)
			if reminders < request.MaxReminders {
				// Reminder timed out for the first time, notify the reviewer
				var notifyResult notificationactivities.NotificationResult
				err := workflow.ExecuteActivity(ctx, notificationactivities.Notify, notificationactivities.NotificationRequest{
					Recipient: request.Reviewer,
					Message:   "Reminder: please submit your answers",
				}).Get(ctx, &notifyResult)
				if err != nil {
					logger.Error("Failed to send reminder", "assetID", request.AssetID, "LaunchID", ID, "error", err)
				}
				reminders++
			}
		})
		selector.AddFuture(workflow.NewTimer(ctx, WarningTimeout), func(f workflow.Future) {
			logger.Info("Launch warning timed out", "assetID", request.AssetID, "LaunchID", ID)
			if warnings < request.MaxWarnings {
				// Warning timed out for the first time, notify the reviewer
				var notifyResult notificationactivities.NotificationResult
				err := workflow.ExecuteActivity(ctx, notificationactivities.Notify, notificationactivities.NotificationRequest{
					Recipient: request.Reviewer,
					Message:   "Warning: please submit your answers",
				}).Get(ctx, &notifyResult)
				if err != nil {
					logger.Error("Failed to send warning", "assetID", request.AssetID, "LaunchID", ID, "error", err)
				}
				warnings++
			}
		})
		selector.AddFuture(workflow.NewTimer(ctx, EscalationTimeout), func(f workflow.Future) {
			logger.Info("Launch escalation timed out", "assetID", request.AssetID, "LaunchID", ID)
			if escalations < request.MaxEscalations {
				// Escalation timed out for the first time, notify the reviewer
				var notifyResult notificationactivities.NotificationResult
				err := workflow.ExecuteActivity(ctx, notificationactivities.Notify, notificationactivities.NotificationRequest{
					Recipient: request.Reviewer,
					Message:   "Escalation: please submit your answers",
				}).Get(ctx, &notifyResult)
				if err != nil {
					logger.Error("Failed to send escalation", "assetID", request.AssetID, "LaunchID", ID, "error", err)
				}
				escalations++
			} else {
				state = model.TimedOutLaunchState
			}
		})
		selector.Select(ctx)

		if state == model.TierValidatedLaunchState {
			break // break out of the for loop
		}

		if state == model.TimedOutLaunchState {
			return LaunchResult{
				ID:          ID,
				AssetID:     request.AssetID,
				RunningTime: workflow.Now(ctx).Sub(startedAt),
				State:       state,
				TierID:      tierID,
				Reviewer:    request.Reviewer,
				Answers:     answers,
				Reminders:   reminders,
				Warnings:    warnings,
				Escalations: escalations,
			}, nil
		}

		if workflow.GetInfo(ctx).GetContinueAsNewSuggested() {
			// Carry current state forward so it is not reset on the new run
			return LaunchResult{
					ID:          ID,
					AssetID:     request.AssetID,
					RunningTime: workflow.Now(ctx).Sub(startedAt),
					State:       state,
					TierID:      tierID,
					Reviewer:    request.Reviewer,
					Answers:     answers,
					Reminders:   reminders,
					Warnings:    warnings,
					Escalations: escalations,
				}, workflow.NewContinueAsNewError(ctx, Launch, LaunchRequest{
					AssetID:           request.AssetID,
					ReminderTimeout:   request.ReminderTimeout,
					WarningTimeout:    request.WarningTimeout,
					EscalationTimeout: request.EscalationTimeout,
				})
		}
	}

	logger.Info("Launch completed", "assetID", request.AssetID, "LaunchID", ID)

	return LaunchResult{
		ID:          ID,
		AssetID:     request.AssetID,
		RunningTime: workflow.Now(ctx).Sub(startedAt),
		State:       state,
		TierID:      tierID,
		Reviewer:    request.Reviewer,
		Answers:     answers,
		Reminders:   reminders,
		Warnings:    warnings,
		Escalations: escalations,
	}, nil
}
