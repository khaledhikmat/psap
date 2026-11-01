package workflows

import (
	"time"

	"github.com/khaledhikmat/psap/model"
	launchactivities "github.com/khaledhikmat/psap/service/flow/activities/launch"
	notificationactivities "github.com/khaledhikmat/psap/service/flow/activities/notification"
	enumspb "go.temporal.io/api/enums/v1"
	"go.temporal.io/sdk/log"
	"go.temporal.io/sdk/temporal"
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
	BusinessOwner     string // worker ID; answers the launch questions and gets reminders/warnings
	BusinessManager   string // worker ID; validates the tier and gets escalations
	ReminderTimeout   int
	MaxReminders      int
	WarningTimeout    int
	MaxWarnings       int
	EscalationTimeout int
	MaxEscalations    int

	// Carried over by Continue-As-New. Leave empty when starting a new launch.
	LaunchID    string
	StartedAt   time.Time
	State       model.LaunchState
	TierID      string
	Points      int
	Answered    int
	Questions   int
	Reminders   int
	Warnings    int
	Escalations int
}

type LaunchResult struct {
	ID              string
	AssetID         string
	RunningTime     time.Duration
	State           model.LaunchState
	TierID          string
	Points          int // sum of the points of the selected answers; set when answers are finalized
	Answered        int
	Questions       int
	BusinessOwner   string
	BusinessManager string
	Reminders       int
	Warnings        int
	Escalations     int
}

// LaunchWorkflowID is the workflow ID of an asset's launch. Using one ID per asset is what
// lets Temporal enforce a single running launch per asset.
func LaunchWorkflowID(assetID string) string {
	return "asset-launch-wf-" + assetID
}

// SubmitAnswersSignal carries the selected predefined answers by question.
type SubmitAnswersSignal struct {
	Answers []model.AnswerSubmission
}

// ValidateTierSignal approves the launch's tier. An empty TierID accepts the calculated tier;
// any other value overrides it.
type ValidateTierSignal struct {
	TierID string
}

// Launch creates a launch for an asset and collects the business owner's answers. Once the
// answers are finalized it calculates the tier, then waits for the business manager to
// validate it. Reminders and warnings nudge whoever the launch is waiting on; escalations go
// to the business manager, and the launch times out once they run out.
func Launch(ctx workflow.Context, req LaunchRequest) (LaunchResult, error) {
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: time.Second * 10,
	})
	w := newLaunch(ctx, req)

	if err := w.setupQuery(); err != nil {
		return LaunchResult{}, err
	}
	if err := w.create(); err != nil {
		if ctx.Err() != nil {
			return w.cancelled()
		}
		return LaunchResult{}, err
	}

	err := waitUntil(ctx, w.isDone, w.continueAsNew,
		onSignal(ctx, LaunchSubmitAnswersSignalName, w.onAnswersSubmitted),
		onSignal(ctx, LaunchFinalizeAnswersSignalName, w.onAnswersFinalized),
		onSignal(ctx, LaunchValidateTierSignalName, w.onTierValidated),
		onTimeout(ctx, seconds(req.ReminderTimeout), w.remind),
		onTimeout(ctx, seconds(req.WarningTimeout), w.warn),
		onTimeout(ctx, seconds(req.EscalationTimeout), w.escalate),
	)
	if ctx.Err() != nil {
		return w.cancelled()
	}
	if err != nil {
		return w.result(), err
	}

	if err := w.persist(); err != nil {
		return w.result(), err
	}

	// Auto-start Shield workflow when tier is validated
	if w.state == model.TierValidatedLaunchState {
		w.logger.Info("Tier validated, starting Shield workflow", "assetID", req.AssetID, "LaunchID", w.id)
		if err := w.startShield(); err != nil {
			w.logger.Error("Failed to start Shield workflow", "assetID", req.AssetID, "error", err)
			// Don't fail the launch if Shield fails to start
		}
	}

	w.logger.Info("Launch completed", "assetID", req.AssetID, "LaunchID", w.id, "state", w.state, "tierID", w.tierID)
	return w.result(), nil
}

// launch holds the workflow's durable state. It isn't persisted directly: Temporal
// reconstructs it by replaying history events (activity completions, signals, timers)
// through this code. That's why mutations must only happen via deterministic paths
// (activity results, signal handlers, workflow.* APIs).
type launch struct {
	ctx    workflow.Context
	req    LaunchRequest
	logger log.Logger

	// nil receivers: only used by Temporal to resolve activity names
	launchActs *launchactivities.Activities
	notifyActs *notificationactivities.Activities

	id          string
	startedAt   time.Time
	state       model.LaunchState
	tierID      string
	points      int
	answered    int
	questions   int
	reminders   int
	warnings    int
	escalations int

	persisted persistedLaunch // what the data service last stored for this launch
}

type persistedLaunch struct {
	tierID      string
	state       model.LaunchState
	reminders   int
	warnings    int
	escalations int
}

func newLaunch(ctx workflow.Context, req LaunchRequest) *launch {
	w := &launch{
		ctx:         ctx,
		req:         req,
		logger:      workflow.GetLogger(ctx),
		id:          req.LaunchID,
		startedAt:   req.StartedAt,
		state:       req.State,
		tierID:      req.TierID,
		points:      req.Points,
		answered:    req.Answered,
		questions:   req.Questions,
		reminders:   req.Reminders,
		warnings:    req.Warnings,
		escalations: req.Escalations,
	}
	if w.startedAt.IsZero() {
		w.startedAt = workflow.Now(ctx)
	}
	if w.state == "" {
		w.state = model.WaitingForAnswersLaunchState
	}
	return w
}

func (w *launch) setupQuery() error {
	return workflow.SetQueryHandler(w.ctx, LaunchStateQueryName, func() (LaunchResult, error) {
		return w.result(), nil
	})
}

// create creates the launch, unless this run continues a launch that
// already exists.
func (w *launch) create() error {
	if w.id != "" {
		w.logger.Info("Resuming launch workflow", "assetID", w.req.AssetID, "LaunchID", w.id)
		return nil
	}

	// The Create activity uses the run ID as the launch ID too. Knowing it before the activity
	// completes lets a cancellation during Create still mark the launch cancelled.
	w.id = workflow.GetInfo(w.ctx).WorkflowExecution.RunID
	w.logger.Info("Starting launch workflow", "assetID", w.req.AssetID, "startedAt", w.startedAt)
	var res launchactivities.LaunchCreationResult
	err := workflow.ExecuteActivity(w.ctx, w.launchActs.Create, launchactivities.LaunchCreationRequest{
		AssetID: w.req.AssetID,
	}).Get(w.ctx, &res)
	if err != nil {
		return err
	}
	w.id = res.LaunchID
	w.questions = res.Questions
	w.persisted = persistedLaunch{state: model.WaitingForAnswersLaunchState}
	return nil
}

// persist stores the launch's tier, state and nudge counts, unless they haven't changed
// since the last call.
func (w *launch) persist() error {
	current := persistedLaunch{
		tierID:      w.tierID,
		state:       w.state,
		reminders:   w.reminders,
		warnings:    w.warnings,
		escalations: w.escalations,
	}
	if current == w.persisted {
		return nil
	}
	err := workflow.ExecuteActivity(w.ctx, w.launchActs.Persist, launchactivities.LaunchPersistRequest{
		LaunchID:    w.id,
		TierID:      w.tierID,
		State:       w.state,
		Reminders:   w.reminders,
		Warnings:    w.warnings,
		Escalations: w.escalations,
	}).Get(w.ctx, nil)
	if err != nil {
		return err
	}
	w.persisted = current
	return nil
}

// persistStep stores the state between steps. A failure is only logged: the workflow keeps
// going, and the next step or the final persist stores the state again.
func (w *launch) persistStep() {
	if err := w.persist(); err != nil {
		w.logger.Error("Failed to persist launch", "assetID", w.req.AssetID, "LaunchID", w.id, "state", w.state, "error", err)
	}
}

// cancelled ends a launch whose workflow was cancelled, which happens when the asset is
// relaunched. It marks the launch cancelled using a disconnected context, since activities
// on the cancelled context would not run, and reports the workflow as cancelled.
func (w *launch) cancelled() (LaunchResult, error) {
	ctx, cancel := workflow.NewDisconnectedContext(w.ctx)
	defer cancel()
	w.ctx = ctx
	w.state = model.CancelledLaunchState
	if err := w.persist(); err != nil {
		w.logger.Error("Failed to persist cancelled launch", "assetID", w.req.AssetID, "LaunchID", w.id, "error", err)
	}
	w.logger.Info("Launch cancelled", "assetID", w.req.AssetID, "LaunchID", w.id)
	return w.result(), temporal.NewCanceledError()
}

func (w *launch) isDone() bool {
	return w.state == model.TierValidatedLaunchState || w.state == model.TimedOutLaunchState
}

// Signal handlers. Signals that arrive in the wrong state are ignored; the flow service
// rejects them before they are sent.

// onAnswersSubmitted saves (possibly partial) answers. The tier is not calculated until the
// answers are finalized.
func (w *launch) onAnswersSubmitted(sig SubmitAnswersSignal) {
	if !w.expectState(LaunchSubmitAnswersSignalName, model.WaitingForAnswersLaunchState) {
		return
	}
	w.logger.Info("Answers submitted", "assetID", w.req.AssetID, "LaunchID", w.id, "count", len(sig.Answers))
	w.saveAnswers(sig.Answers)
}

// onAnswersFinalized saves the last answers, locks them and calculates the tier. Unanswered
// questions earn no points.
func (w *launch) onAnswersFinalized(sig SubmitAnswersSignal) {
	if !w.expectState(LaunchFinalizeAnswersSignalName, model.WaitingForAnswersLaunchState) {
		return
	}
	w.logger.Info("Answers finalized", "assetID", w.req.AssetID, "LaunchID", w.id, "count", len(sig.Answers))
	if !w.saveAnswers(sig.Answers) {
		return // stay waiting for answers so the owner can correct them and finalize again
	}

	var res launchactivities.LaunchTierCalculationResult
	err := workflow.ExecuteActivity(w.ctx, w.launchActs.CalculateTier, launchactivities.LaunchTierCalculationRequest{
		LaunchID: w.id,
	}).Get(w.ctx, &res)
	if err != nil {
		w.logger.Error("Failed to calculate tier", "assetID", w.req.AssetID, "LaunchID", w.id, "error", err)
		return
	}
	w.points = res.Points
	w.tierID = res.TierID
	w.state = model.WaitingForTierApprovalLaunchState
	w.logger.Info("Tier calculated", "assetID", w.req.AssetID, "LaunchID", w.id, "points", w.points, "tierID", w.tierID)
	w.persistStep()
}

// onTierValidated completes the launch; Launch's final persist stores it.
func (w *launch) onTierValidated(sig ValidateTierSignal) {
	if !w.expectState(LaunchValidateTierSignalName, model.WaitingForTierApprovalLaunchState) {
		return
	}
	w.logger.Info("Tier validated", "assetID", w.req.AssetID, "LaunchID", w.id, "calculated", w.tierID, "validated", sig.TierID)
	if sig.TierID != "" {
		w.tierID = sig.TierID
	}
	w.state = model.TierValidatedLaunchState
}

func (w *launch) expectState(signal string, want model.LaunchState) bool {
	if w.state == want {
		return true
	}
	w.logger.Warn("Ignoring signal in current state", "signal", signal, "state", w.state, "want", want,
		"assetID", w.req.AssetID, "LaunchID", w.id)
	return false
}

// saveAnswers stores the selections and returns whether that succeeded.
func (w *launch) saveAnswers(answers []model.AnswerSubmission) bool {
	if len(answers) == 0 {
		return true
	}
	var res launchactivities.LaunchAnswersSaveResult
	err := workflow.ExecuteActivity(w.ctx, w.launchActs.SaveAnswers, launchactivities.LaunchAnswersSaveRequest{
		LaunchID:  w.id,
		Answers:   answers,
		UpdatedBy: w.req.BusinessOwner,
	}).Get(w.ctx, &res)
	if err != nil {
		w.logger.Error("Failed to save answers", "assetID", w.req.AssetID, "LaunchID", w.id, "error", err)
		return false
	}
	w.answered = res.Answered
	w.questions = res.Questions
	return true
}

// Timeout handlers. Each returns false once it has nothing left to do, which stops its timer.
// Reminders and warnings go to whoever the launch is waiting on; escalations go to the
// business manager.

func (w *launch) remind() bool {
	recipient, task := w.waitingOn()
	return w.nudge("reminder", &w.reminders, w.req.MaxReminders, recipient, "Reminder: please "+task)
}

func (w *launch) warn() bool {
	recipient, task := w.waitingOn()
	return w.nudge("warning", &w.warnings, w.req.MaxWarnings, recipient, "Warning: please "+task)
}

// escalate gives up on the launch once all escalations have been sent.
func (w *launch) escalate() bool {
	_, task := w.waitingOn()
	if w.nudge("escalation", &w.escalations, w.req.MaxEscalations, w.req.BusinessManager,
		"Escalation: launch of asset "+w.req.AssetID+" is overdue; someone needs to "+task) {
		return true
	}
	w.state = model.TimedOutLaunchState
	return false
}

// waitingOn returns who the launch is waiting on, and for what.
func (w *launch) waitingOn() (recipient, task string) {
	if w.state == model.WaitingForTierApprovalLaunchState {
		return w.req.BusinessManager, "validate the launch tier"
	}
	return w.req.BusinessOwner, "submit your launch answers"
}

// nudge notifies the recipient and increments count, unless count has reached max.
// It returns false when there was nothing left to send.
func (w *launch) nudge(kind string, count *int, max int, recipient, message string) bool {
	w.logger.Info("Launch "+kind+" timed out", "assetID", w.req.AssetID, "LaunchID", w.id)
	if *count >= max {
		return false
	}

	var res notificationactivities.NotificationResult
	err := workflow.ExecuteActivity(w.ctx, w.notifyActs.Notify, notificationactivities.NotificationRequest{
		Recipient: recipient,
		Message:   message,
	}).Get(w.ctx, &res)
	if err != nil {
		w.logger.Error("Failed to send "+kind, "assetID", w.req.AssetID, "LaunchID", w.id, "error", err)
	}
	*count++
	w.persistStep()
	return true
}

// continueAsNew starts a fresh run that carries the current state forward.
func (w *launch) continueAsNew() error {
	next := w.req
	next.LaunchID = w.id
	next.StartedAt = w.startedAt
	next.State = w.state
	next.TierID = w.tierID
	next.Points = w.points
	next.Answered = w.answered
	next.Questions = w.questions
	next.Reminders = w.reminders
	next.Warnings = w.warnings
	next.Escalations = w.escalations
	return workflow.NewContinueAsNewError(w.ctx, Launch, next)
}

// startShield initiates the Shield workflow for the asset using the Temporal service.
func (w *launch) startShield() error {
	// Execute the InitiateShield workflow using ExecuteChildWorkflow
	childCtx := workflow.WithChildOptions(w.ctx, workflow.ChildWorkflowOptions{
		WorkflowID:        ShieldWorkflowID(w.req.AssetID),
		TaskQueue:         workflow.GetInfo(w.ctx).TaskQueueName,
		WorkflowIDReusePolicy: enumspb.WORKFLOW_ID_REUSE_POLICY_ALLOW_DUPLICATE,
	})

	shieldReq := ShieldRequest{
		AssetID:           w.req.AssetID,
		BusinessOwner:     w.req.BusinessOwner,
		BusinessManager:   w.req.BusinessManager,
		ReminderTimeout:   60,
		MaxReminders:      3,
		WarningTimeout:    300,
		MaxWarnings:       2,
		EscalationTimeout: 900,
		MaxEscalations:    1,
	}

	future := workflow.ExecuteChildWorkflow(childCtx, Shield, shieldReq)
	// Don't wait for the child workflow to complete, just start it
	// Get the child workflow execution to ensure it started
	var childWE workflow.Execution
	if err := future.GetChildWorkflowExecution().Get(childCtx, &childWE); err != nil {
		return err
	}
	w.logger.Info("Shield workflow started", "assetID", w.req.AssetID, "shieldWorkflowID", childWE.ID)
	return nil
}

func (w *launch) result() LaunchResult {
	return LaunchResult{
		ID:              w.id,
		AssetID:         w.req.AssetID,
		RunningTime:     workflow.Now(w.ctx).Sub(w.startedAt),
		State:           w.state,
		TierID:          w.tierID,
		Points:          w.points,
		Answered:        w.answered,
		Questions:       w.questions,
		BusinessOwner:   w.req.BusinessOwner,
		BusinessManager: w.req.BusinessManager,
		Reminders:       w.reminders,
		Warnings:        w.warnings,
		Escalations:     w.escalations,
	}
}

func seconds(n int) time.Duration {
	return time.Duration(n) * time.Second
}
