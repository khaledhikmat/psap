package workflows

import (
	"time"

	"github.com/khaledhikmat/psap/model"
	notificationactivities "github.com/khaledhikmat/psap/service/flow/activities/notification"
	shieldactivities "github.com/khaledhikmat/psap/service/flow/activities/shield"
	"go.temporal.io/sdk/log"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// Signals
const (
	ShieldSubmitAnswersSignalName     = "shield-submit-answers-signal"
	ShieldFinalizeAnswersSignalName   = "shield-finalize-answers-signal"
	ShieldSubmitEvidencesSignalName   = "shield-submit-evidences-signal"
	ShieldFinalizeEvidencesSignalName = "shield-finalize-evidences-signal"
)

// Queries
const (
	ShieldStateQueryName = "shield-state-query"
)

type ShieldRequest struct {
	AssetID           string
	BusinessOwner     string // worker ID; answers the shield questions and gets reminders/warnings
	BusinessManager   string // worker ID; gets escalations
	ReminderTimeout   int
	MaxReminders      int
	WarningTimeout    int
	MaxWarnings       int
	EscalationTimeout int
	MaxEscalations    int

	// Carried over by Continue-As-New. Leave empty when starting a new shield.
	ShieldID       string
	StartedAt      time.Time
	State          model.ShieldState
	Score          int
	Answered       int
	Questions      int
	EvidencesCount int
	Categories     int
	Reminders      int
	Warnings       int
	Escalations    int
}

type ShieldResult struct {
	ID              string
	AssetID         string
	RunningTime     time.Duration
	State           model.ShieldState
	Score           int
	Answered        int
	Questions       int
	EvidencesCount  int
	Categories      int
	BusinessOwner   string
	BusinessManager string
	Reminders       int
	Warnings        int
	Escalations     int
}

// ShieldWorkflowID is the workflow ID of an asset's shield. Using one ID per asset is what
// lets Temporal enforce a single running shield per asset.
func ShieldWorkflowID(assetID string) string {
	return "asset-shield-wf-" + assetID
}

// SubmitAnswersShieldSignal carries replies by question ID.
type SubmitAnswersShieldSignal struct {
	Answers map[string]string // questionID -> reply
}

// SubmitEvidencesShieldSignal carries evidence data by category ID.
type SubmitEvidencesShieldSignal struct {
	Evidences map[string]shieldactivities.ShieldEvidenceData // categoryID -> evidence
}

// Shield creates a shield for an asset and collects the business owner's answers and evidences.
// Answers and evidences can be submitted independently. Once both are finalized, the shield
// calculates the score and completes. Reminders and warnings nudge whoever the shield is waiting
// on; escalations go to the business manager, and the shield times out once they run out.
func Shield(ctx workflow.Context, req ShieldRequest) (ShieldResult, error) {
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: time.Second * 10,
	})
	w := newShield(ctx, req)

	if err := w.setupQuery(); err != nil {
		return ShieldResult{}, err
	}
	if err := w.create(); err != nil {
		if ctx.Err() != nil {
			return w.cancelled()
		}
		return ShieldResult{}, err
	}
	w.persistStep()

	// Main loop: wait for signals (answers/evidences) or timeouts
	for !w.done() {
		if err := w.continueAsNew(); err != nil {
			return ShieldResult{}, err
		}
		w.advanceState()
	}

	if err := w.persist(); err != nil {
		return ShieldResult{}, err
	}
	return w.result(), nil
}

type shield struct {
	ctx         workflow.Context
	req         ShieldRequest
	logger      log.Logger
	shieldActs  *shieldactivities.Activities
	notifyActs  *notificationactivities.Activities
	id          string
	startedAt   time.Time
	state       model.ShieldState
	score       int
	answered    int
	questions   int
	evidences   int
	categories  int
	reminders   int
	warnings    int
	escalations int
	persisted   persistedShield
}

type persistedShield struct {
	score       int
	state       model.ShieldState
	reminders   int
	warnings    int
	escalations int
}

func newShield(ctx workflow.Context, req ShieldRequest) *shield {
	return &shield{
		ctx:         ctx,
		req:         req,
		logger:      workflow.GetLogger(ctx),
		shieldActs:  &shieldactivities.Activities{},
		notifyActs:  &notificationactivities.Activities{},
		id:          req.ShieldID,
		startedAt:   req.StartedAt,
		state:       req.State,
		score:       req.Score,
		answered:    req.Answered,
		questions:   req.Questions,
		evidences:   req.EvidencesCount,
		categories:  req.Categories,
		reminders:   req.Reminders,
		warnings:    req.Warnings,
		escalations: req.Escalations,
	}
}

func (w *shield) setupQuery() error {
	return workflow.SetQueryHandler(w.ctx, ShieldStateQueryName, func() (ShieldResult, error) {
		return w.result(), nil
	})
}

func (w *shield) create() error {
	var res shieldactivities.ShieldCreationResult
	err := workflow.ExecuteActivity(w.ctx, w.shieldActs.Create, shieldactivities.ShieldCreationRequest{
		AssetID: w.req.AssetID,
	}).Get(w.ctx, &res)
	if err != nil {
		return err
	}
	w.id = res.ShieldID
	w.questions = res.Questions
	w.categories = res.Categories
	w.state = model.WaitingForAnswersShieldState
	return nil
}

// persist stores the shield's score, state and nudge counts if they changed since the last call.
func (w *shield) persist() error {
	current := persistedShield{
		score:       w.score,
		state:       w.state,
		reminders:   w.reminders,
		warnings:    w.warnings,
		escalations: w.escalations,
	}
	if current == w.persisted {
		return nil
	}
	err := workflow.ExecuteActivity(w.ctx, w.shieldActs.Persist, shieldactivities.ShieldPersistRequest{
		ShieldID:    w.id,
		Score:       w.score,
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
func (w *shield) persistStep() {
	if err := w.persist(); err != nil {
		w.logger.Error("Failed to persist shield", "assetID", w.req.AssetID, "ShieldID", w.id, "state", w.state, "error", err)
	}
}

// cancelled ends a shield whose workflow was cancelled. It marks the shield cancelled using
// a disconnected context, since activities on the cancelled context would not run, and reports
// the workflow as cancelled.
func (w *shield) cancelled() (ShieldResult, error) {
	disconnectedCtx, cancel := workflow.NewDisconnectedContext(w.ctx)
	defer cancel()
	w.ctx = disconnectedCtx
	w.state = model.CancelledShieldState
	_ = w.persist()
	return ShieldResult{}, temporal.NewCanceledError()
}

// advanceState handles the current state: waits for a signal or timeout, processes it, and
// moves to the next state as needed.
func (w *shield) advanceState() {
	switch w.state {
	case model.WaitingForAnswersShieldState:
		w.waitForAnswers()
	case model.AnswersSubmittedShieldState:
		w.waitForEvidences()
	case model.WaitingForEvidencesShieldState:
		w.waitForEvidences()
	case model.EvidencesSubmittedShieldState:
		w.complete()
	}
}

func (w *shield) waitForAnswers() {
	sigAnswers := workflow.GetSignalChannel(w.ctx, ShieldSubmitAnswersSignalName)
	sigFinalizeAnswers := workflow.GetSignalChannel(w.ctx, ShieldFinalizeAnswersSignalName)

	timeout := seconds(w.req.ReminderTimeout)
	if w.reminders >= w.req.MaxReminders {
		timeout = seconds(w.req.WarningTimeout)
	}
	if w.warnings >= w.req.MaxWarnings {
		timeout = seconds(w.req.EscalationTimeout)
	}

	selector := workflow.NewSelector(w.ctx)
	selector.AddReceive(sigAnswers, func(ch workflow.ReceiveChannel, more bool) {
		var sig SubmitAnswersShieldSignal
		ch.Receive(w.ctx, &sig)
		w.saveAnswers(sig.Answers)
	})
	selector.AddReceive(sigFinalizeAnswers, func(ch workflow.ReceiveChannel, more bool) {
		var sig SubmitAnswersShieldSignal
		ch.Receive(w.ctx, &sig)
		w.finalizeAnswers(sig.Answers)
	})
	selector.AddFuture(workflow.NewTimer(w.ctx, timeout), func(f workflow.Future) {
		w.nudgeTimeout()
	})
	selector.Select(w.ctx)
}

func (w *shield) waitForEvidences() {
	sigEvidences := workflow.GetSignalChannel(w.ctx, ShieldSubmitEvidencesSignalName)
	sigFinalizeEvidences := workflow.GetSignalChannel(w.ctx, ShieldFinalizeEvidencesSignalName)

	timeout := seconds(w.req.ReminderTimeout)
	if w.reminders >= w.req.MaxReminders {
		timeout = seconds(w.req.WarningTimeout)
	}
	if w.warnings >= w.req.MaxWarnings {
		timeout = seconds(w.req.EscalationTimeout)
	}

	selector := workflow.NewSelector(w.ctx)
	selector.AddReceive(sigEvidences, func(ch workflow.ReceiveChannel, more bool) {
		var sig SubmitEvidencesShieldSignal
		ch.Receive(w.ctx, &sig)
		w.saveEvidences(sig.Evidences)
	})
	selector.AddReceive(sigFinalizeEvidences, func(ch workflow.ReceiveChannel, more bool) {
		var sig SubmitEvidencesShieldSignal
		ch.Receive(w.ctx, &sig)
		w.finalizeEvidences(sig.Evidences)
	})
	selector.AddFuture(workflow.NewTimer(w.ctx, timeout), func(f workflow.Future) {
		w.nudgeTimeout()
	})
	selector.Select(w.ctx)
}

func (w *shield) saveAnswers(answers map[string]string) {
	err := workflow.ExecuteActivity(w.ctx, w.shieldActs.SaveAnswers, shieldactivities.ShieldAnswersSaveRequest{
		ShieldID: w.id,
		Answers:  answers,
	}).Get(w.ctx, nil)
	if err != nil {
		w.logger.Error("Failed to save shield answers", "assetID", w.req.AssetID, "ShieldID", w.id, "error", err)
		return
	}
	w.answered += len(answers)
	w.persistStep()
}

func (w *shield) finalizeAnswers(answers map[string]string) {
	err := workflow.ExecuteActivity(w.ctx, w.shieldActs.FinalizeAnswers, shieldactivities.ShieldAnswersFinalizeRequest{
		ShieldID: w.id,
		Answers:  answers,
	}).Get(w.ctx, nil)
	if err != nil {
		w.logger.Error("Failed to finalize shield answers", "assetID", w.req.AssetID, "ShieldID", w.id, "error", err)
		return
	}
	w.answered = w.questions
	if w.state == model.WaitingForAnswersShieldState {
		w.state = model.AnswersSubmittedShieldState
	} else if w.state == model.WaitingForEvidencesShieldState && w.evidences == w.categories {
		// Both done
		w.complete()
		return
	}
	// Move to waiting for evidences if not already there
	if w.state == model.AnswersSubmittedShieldState {
		w.state = model.WaitingForEvidencesShieldState
	}
	w.persistStep()
}

func (w *shield) saveEvidences(evidences map[string]shieldactivities.ShieldEvidenceData) {
	err := workflow.ExecuteActivity(w.ctx, w.shieldActs.SaveEvidences, shieldactivities.ShieldEvidencesSaveRequest{
		ShieldID:  w.id,
		Evidences: evidences,
	}).Get(w.ctx, nil)
	if err != nil {
		w.logger.Error("Failed to save shield evidences", "assetID", w.req.AssetID, "ShieldID", w.id, "error", err)
		return
	}
	w.evidences += len(evidences)
	w.persistStep()
}

func (w *shield) finalizeEvidences(evidences map[string]shieldactivities.ShieldEvidenceData) {
	err := workflow.ExecuteActivity(w.ctx, w.shieldActs.FinalizeEvidences, shieldactivities.ShieldEvidencesFinalizeRequest{
		ShieldID:  w.id,
		Evidences: evidences,
	}).Get(w.ctx, nil)
	if err != nil {
		w.logger.Error("Failed to finalize shield evidences", "assetID", w.req.AssetID, "ShieldID", w.id, "error", err)
		return
	}
	w.evidences = w.categories
	if w.state == model.WaitingForEvidencesShieldState {
		w.state = model.EvidencesSubmittedShieldState
	} else if w.state == model.AnswersSubmittedShieldState && w.answered == w.questions {
		// Both done
		w.complete()
		return
	}
	w.persistStep()
}

func (w *shield) complete() {
	var res shieldactivities.ShieldScoreCalculationResult
	err := workflow.ExecuteActivity(w.ctx, w.shieldActs.CalculateScore, shieldactivities.ShieldScoreCalculationRequest{
		ShieldID: w.id,
	}).Get(w.ctx, &res)
	if err != nil {
		w.logger.Error("Failed to calculate shield score", "assetID", w.req.AssetID, "ShieldID", w.id, "error", err)
		return
	}
	w.score = res.Score
	w.state = model.CompletedShieldState
	w.persistStep()
}

func (w *shield) nudgeTimeout() {
	if w.reminders < w.req.MaxReminders {
		if w.remind() {
			return
		}
	}
	if w.warnings < w.req.MaxWarnings {
		if w.warn() {
			return
		}
	}
	if !w.escalate() {
		// All escalations exhausted
		return
	}
}

// Reminders and warnings go to whoever the shield is waiting on; escalations go to the
// business manager.

func (w *shield) remind() bool {
	recipient, task := w.waitingOn()
	return w.nudge("reminder", &w.reminders, w.req.MaxReminders, recipient, "Reminder: please "+task)
}

func (w *shield) warn() bool {
	recipient, task := w.waitingOn()
	return w.nudge("warning", &w.warnings, w.req.MaxWarnings, recipient, "Warning: please "+task)
}

// escalate gives up on the shield once all escalations have been sent.
func (w *shield) escalate() bool {
	_, task := w.waitingOn()
	if w.nudge("escalation", &w.escalations, w.req.MaxEscalations, w.req.BusinessManager,
		"Escalation: shield of asset "+w.req.AssetID+" is overdue; someone needs to "+task) {
		return true
	}
	w.state = model.TimedOutShieldState
	return false
}

// waitingOn returns who the shield is waiting on, and for what.
func (w *shield) waitingOn() (recipient, task string) {
	switch w.state {
	case model.WaitingForAnswersShieldState:
		return w.req.BusinessOwner, "submit your shield answers"
	case model.WaitingForEvidencesShieldState:
		return w.req.BusinessOwner, "submit your shield evidences"
	case model.AnswersSubmittedShieldState:
		return w.req.BusinessOwner, "submit your shield evidences"
	case model.EvidencesSubmittedShieldState:
		return w.req.BusinessOwner, "submit your shield answers"
	default:
		return w.req.BusinessOwner, "complete the shield"
	}
}

// nudge notifies the recipient and increments count, unless count has reached max.
// It returns false when there was nothing left to send.
func (w *shield) nudge(kind string, count *int, max int, recipient, message string) bool {
	w.logger.Info("Shield "+kind+" timed out", "assetID", w.req.AssetID, "ShieldID", w.id)
	if *count >= max {
		return false
	}

	var res notificationactivities.NotificationResult
	err := workflow.ExecuteActivity(w.ctx, w.notifyActs.Notify, notificationactivities.NotificationRequest{
		Recipient: recipient,
		Message:   message,
	}).Get(w.ctx, &res)
	if err != nil {
		w.logger.Error("Failed to send "+kind, "assetID", w.req.AssetID, "ShieldID", w.id, "error", err)
	}
	*count++
	w.persistStep()
	return true
}

func (w *shield) done() bool {
	return w.state == model.CompletedShieldState || w.state == model.TimedOutShieldState || w.state == model.CancelledShieldState
}

// continueAsNew restarts the workflow with fresh history when we approach Temporal's limits.
func (w *shield) continueAsNew() error {
	info := workflow.GetInfo(w.ctx)
	if info.GetCurrentHistoryLength() < 900 {
		return nil
	}
	next := w.req
	next.ShieldID = w.id
	next.StartedAt = w.startedAt
	next.State = w.state
	next.Score = w.score
	next.Answered = w.answered
	next.Questions = w.questions
	next.EvidencesCount = w.evidences
	next.Categories = w.categories
	next.Reminders = w.reminders
	next.Warnings = w.warnings
	next.Escalations = w.escalations
	return workflow.NewContinueAsNewError(w.ctx, Shield, next)
}

func (w *shield) result() ShieldResult {
	if w.startedAt.IsZero() {
		w.startedAt = workflow.Now(w.ctx)
	}
	return ShieldResult{
		ID:              w.id,
		AssetID:         w.req.AssetID,
		RunningTime:     workflow.Now(w.ctx).Sub(w.startedAt),
		State:           w.state,
		Score:           w.score,
		Answered:        w.answered,
		Questions:       w.questions,
		EvidencesCount:  w.evidences,
		Categories:      w.categories,
		BusinessOwner:   w.req.BusinessOwner,
		BusinessManager: w.req.BusinessManager,
		Reminders:       w.reminders,
		Warnings:        w.warnings,
		Escalations:     w.escalations,
	}
}
