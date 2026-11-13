package minions

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/khaledhikmat/psap/model"
	"github.com/khaledhikmat/psap/service"
	"github.com/khaledhikmat/psap/service/answers"
	"github.com/khaledhikmat/psap/service/data"
	launchactivities "github.com/khaledhikmat/psap/service/flow/activities/launch"
	notificationactivities "github.com/khaledhikmat/psap/service/flow/activities/notification"
	"github.com/khaledhikmat/psap/service/lgr"
)

const launchKind = "launch"

const launchTierApprovalTimeout = 7 * 24 * time.Hour

var ErrInvalidLaunchMinionState = errors.New("launch minion is not in a state that allows this operation")

type ILaunchMinion interface {
	IMinion
	SubmitAnswers([]model.AnswerSubmission) error
	FinalizeAnswers([]model.AnswerSubmission) error
	ReviseAnswers([]model.AnswerSubmission) error
	ValidateTier(tierID string) error
	GetState() LaunchMinionState
}

type LaunchMinionOptions struct {
	AssetID           string
	BusinessOwner     string
	BusinessManager   string
	ReminderTimeout   int
	MaxReminders      int
	WarningTimeout    int
	MaxWarnings       int
	EscalationTimeout int
	MaxEscalations    int
}

type LaunchMinionState struct {
	MinionState
	AssetID              string
	BusinessOwner        string
	BusinessManager      string
	LaunchState          model.LaunchState
	TierID               string
	TierApprovalDeadline time.Time
	Points               int
	Answered             int
	Questions            int
	Reminders            int
	Warnings             int
	Escalations          int
	ReminderTimeout      int
	MaxReminders         int
	WarningTimeout       int
	MaxWarnings          int
	EscalationTimeout    int
	MaxEscalations       int
}

type launchMinion struct {
	*Minion[*LaunchMinionState]
	activities *launchactivities.Activities
	persisted  launchPersisted
}

var _ ILaunchMinion = (*launchMinion)(nil)

type launchPersisted struct {
	tierID      string
	state       model.LaunchState
	reminders   int
	warnings    int
	escalations int
}

func NewLaunchMinion(id, managerID string, svcs service.ServicesFactory, opts LaunchMinionOptions) ILaunchMinion {
	state := &LaunchMinionState{
		AssetID:           opts.AssetID,
		BusinessOwner:     opts.BusinessOwner,
		BusinessManager:   opts.BusinessManager,
		ReminderTimeout:   opts.ReminderTimeout,
		MaxReminders:      opts.MaxReminders,
		WarningTimeout:    opts.WarningTimeout,
		MaxWarnings:       opts.MaxWarnings,
		EscalationTimeout: opts.EscalationTimeout,
		MaxEscalations:    opts.MaxEscalations,
	}
	base := NewMinion(id, managerID, launchKind, svcs, state)
	m := &launchMinion{
		Minion:     base,
		activities: launchactivities.NewActivities(svcs.CfgSvc, svcs.DataSvc),
	}
	base.OnLoad = m.load
	base.OnSave = m.save
	base.OnSignal = m.handleRequest
	base.OnStart = m.registerTimers
	base.OnRunInitialActivities = m.create
	base.OnRunFinalActivities = m.finish
	return m
}

func (m *launchMinion) load(ctx context.Context, state *LaunchMinionState) error {
	if state.ID == "" {
		return errors.New("load launch minion: launch ID is empty")
	}
	if state.AssetID == "" {
		return errors.New("load launch minion: asset ID is empty")
	}
	if m.Svcs().DataSvc == nil {
		return errors.New("load launch minion: data service is nil")
	}
	launch, err := m.Svcs().DataSvc.RetrieveLaunchByID(ctx, state.ID)
	if errors.Is(err, data.ErrNotFound) {
		return ErrNoState
	}
	if err != nil {
		return fmt.Errorf("load launch %s: %w", state.ID, err)
	}
	if launch.AssetID != state.AssetID {
		return fmt.Errorf("load launch %s: asset ID is %q, want %q", state.ID, launch.AssetID, state.AssetID)
	}
	startedAt, err := time.Parse(time.RFC3339Nano, launch.CreatedAt)
	if err != nil {
		return fmt.Errorf("load launch %s creation time %q: %w", state.ID, launch.CreatedAt, err)
	}
	state.StartTime = &startedAt
	state.LaunchState = launch.State
	state.Reminders = launch.Reminders
	state.Warnings = launch.Warnings
	state.Escalations = launch.Escalations
	if launch.TierID != nil {
		state.TierID = *launch.TierID
	}
	questions, err := m.Svcs().DataSvc.RetrieveQuestionsByCategoryType(ctx, model.LaunchQuestionCategory)
	if err != nil {
		return fmt.Errorf("load launch questions: %w", err)
	}
	state.Questions = len(questions)
	saved, err := m.Svcs().DataSvc.RetrieveUserAnswersByLaunchID(ctx, state.ID)
	if err != nil {
		return fmt.Errorf("load launch answers: %w", err)
	}
	state.Answered = answers.Answered(saved)
	predefined, err := m.Svcs().DataSvc.RetrievePredefinedAnswersByCategoryType(ctx, model.LaunchQuestionCategory)
	if err != nil {
		return fmt.Errorf("load predefined launch answers: %w", err)
	}
	state.Points = answers.Points(saved, predefined)
	m.persisted = m.snapshot(state)

	if state.LaunchState == model.WaitingForTierApprovalLaunchState {
		state.TierApprovalDeadline = time.Now().Add(launchTierApprovalTimeout)
	}
	return nil
}

func (m *launchMinion) create(ctx context.Context, state *LaunchMinionState) error {
	result, err := m.activities.Create(ctx, launchactivities.LaunchCreationRequest{
		AssetID:  state.AssetID,
		LaunchID: state.ID,
	})
	if err != nil {
		return fmt.Errorf("create launch %s: %w", state.ID, err)
	}
	state.Questions = result.Questions
	state.LaunchState = model.WaitingForAnswersLaunchState
	m.persisted = m.snapshot(state)
	m.registerOwnerTimers(state)
	return nil
}

func (m *launchMinion) save(ctx context.Context, state *LaunchMinionState) error {
	return m.persistIfChanged(ctx, state)
}

func (m *launchMinion) finish(ctx context.Context, state *LaunchMinionState) error {
	if state.LaunchState != model.TierValidatedLaunchState &&
		state.LaunchState != model.TimedOutLaunchState &&
		state.LaunchState != model.CancelledLaunchState {
		state.LaunchState = model.CancelledLaunchState
	}
	return m.persistIfChanged(ctx, state)
}

func (m *launchMinion) handleRequest(ctx context.Context, state *LaunchMinionState, req MinionRequest) error {
	switch req.Command {
	case "SubmitAnswers":
		if !m.expectState(state, model.WaitingForAnswersLaunchState, req.Command) {
			return nil
		}
		submissions, err := launchAnswersArg(req)
		if err != nil {
			return err
		}
		return m.saveAnswers(ctx, state, submissions, state.BusinessOwner)
	case "FinalizeAnswers":
		if !m.expectState(state, model.WaitingForAnswersLaunchState, req.Command) {
			return nil
		}
		submissions, err := launchAnswersArg(req)
		if err != nil {
			return err
		}
		if err := m.saveAnswers(ctx, state, submissions, state.BusinessOwner); err != nil {
			return err
		}
		calculation, err := m.activities.CalculateTier(ctx, launchactivities.LaunchTierCalculationRequest{LaunchID: state.ID})
		if err != nil {
			return fmt.Errorf("calculate launch tier: %w", err)
		}
		state.Points = calculation.Points
		state.TierID = calculation.TierID
		state.LaunchState = model.WaitingForTierApprovalLaunchState
		state.TierApprovalDeadline = time.Now().Add(launchTierApprovalTimeout)
		state.Reminders, state.Warnings, state.Escalations = 0, 0, 0
		m.registerApprovalTimers(state)
		return m.persistStep(ctx, state)
	case "ReviseAnswers":
		if !m.expectState(state, model.WaitingForTierApprovalLaunchState, req.Command) {
			return nil
		}
		submissions, err := launchAnswersArg(req)
		if err != nil {
			return err
		}
		if len(submissions) == 0 {
			return errors.New("no answers to revise")
		}
		if err := m.saveAnswers(ctx, state, submissions, state.BusinessManager); err != nil {
			return err
		}
		calculation, err := m.activities.CalculateTier(ctx, launchactivities.LaunchTierCalculationRequest{LaunchID: state.ID})
		if err != nil {
			return fmt.Errorf("calculate revised launch tier: %w", err)
		}
		state.Points = calculation.Points
		state.TierID = calculation.TierID
		return m.persistStep(ctx, state)
	case "ValidateTier":
		if !m.expectState(state, model.WaitingForTierApprovalLaunchState, req.Command) {
			return nil
		}
		tierID, err := stringArg(req)
		if err != nil {
			return err
		}
		if tierID != "" {
			if _, err := m.Svcs().DataSvc.RetrieveTierByID(ctx, tierID); err != nil {
				return fmt.Errorf("validate launch tier %q: %w", tierID, err)
			}
			state.TierID = tierID
		}
		state.LaunchState = model.TierValidatedLaunchState
		if err := m.persistStep(ctx, state); err != nil {
			return err
		}
		m.startShield(ctx, state)
		return nil
	default:
		return fmt.Errorf("unknown launch minion command %q", req.Command)
	}
}

func (m *launchMinion) saveAnswers(ctx context.Context, state *LaunchMinionState, submissions []model.AnswerSubmission, updatedBy string) error {
	if len(submissions) == 0 {
		return nil
	}
	result, err := m.activities.SaveAnswers(ctx, launchactivities.LaunchAnswersSaveRequest{
		LaunchID:  state.ID,
		Answers:   submissions,
		UpdatedBy: updatedBy,
	})
	if err != nil {
		return fmt.Errorf("save launch answers: %w", err)
	}
	state.Answered = result.Answered
	state.Questions = result.Questions
	return nil
}

func (m *launchMinion) expectState(state *LaunchMinionState, want model.LaunchState, command string) bool {
	if state.LaunchState == want {
		return true
	}
	lgr.Logger.Warn("Ignoring launch minion command in current state",
		slog.String("command", command),
		slog.String("state", string(state.LaunchState)),
		slog.String("want", string(want)),
		slog.String("assetID", state.AssetID),
		slog.String("launchID", state.ID))
	return false
}

func (m *launchMinion) registerTimers(_ *Minion[*LaunchMinionState]) {
	state := m.state
	switch state.LaunchState {
	case model.WaitingForAnswersLaunchState:
		m.registerOwnerTimers(state)
	case model.WaitingForTierApprovalLaunchState:
		if state.TierApprovalDeadline.IsZero() {
			state.TierApprovalDeadline = time.Now().Add(launchTierApprovalTimeout)
		}
		m.registerApprovalTimers(state)
	}
}

func (m *launchMinion) registerOwnerTimers(state *LaunchMinionState) {
	startedAt := time.Now()
	if state.StartTime != nil && !state.StartTime.IsZero() {
		startedAt = *state.StartTime
	}
	m.registerNudge(state, model.WaitingForAnswersLaunchState, "reminder", seconds(state.ReminderTimeout), state.MaxReminders, &state.Reminders, state.BusinessOwner, "submit your launch answers", startedAt)
	m.registerNudge(state, model.WaitingForAnswersLaunchState, "warning", seconds(state.WarningTimeout), state.MaxWarnings, &state.Warnings, state.BusinessOwner, "submit your launch answers", startedAt)
	m.registerNudge(state, model.WaitingForAnswersLaunchState, "escalation", seconds(state.EscalationTimeout), state.MaxEscalations, &state.Escalations, state.BusinessManager, "submit your launch answers", startedAt)
}

func (m *launchMinion) registerApprovalTimers(state *LaunchMinionState) {
	now := time.Now()
	if state.TierApprovalDeadline.IsZero() {
		state.TierApprovalDeadline = now.Add(launchTierApprovalTimeout)
	}
	phaseStarted := state.TierApprovalDeadline.Add(-launchTierApprovalTimeout)
	m.registerNudge(state, model.WaitingForTierApprovalLaunchState, "reminder", 24*time.Hour, state.MaxReminders, &state.Reminders, state.BusinessManager, "validate the launch tier", phaseStarted)
	m.registerNudge(state, model.WaitingForTierApprovalLaunchState, "warning", 5*24*time.Hour, state.MaxWarnings, &state.Warnings, state.BusinessManager, "validate the launch tier", phaseStarted)
	m.registerNudge(state, model.WaitingForTierApprovalLaunchState, "escalation", 6*24*time.Hour, state.MaxEscalations, &state.Escalations, state.BusinessManager, "validate the launch tier", phaseStarted)
	m.RegisterTimer(state.TierApprovalDeadline, func(ctx context.Context, _ time.Time) (bool, time.Duration) {
		if state.LaunchState != model.WaitingForTierApprovalLaunchState {
			return false, 0
		}
		state.LaunchState = model.TimedOutLaunchState
		if err := m.persistStep(ctx, state); err != nil {
			lgr.Logger.Error("Failed to persist launch approval timeout", slog.String("launchID", state.ID), slog.Any("error", err))
		}
		return false, 0
	})
}

func (m *launchMinion) registerNudge(state *LaunchMinionState, phase model.LaunchState, kind string, interval time.Duration, max int, count *int, recipient, task string, startedAt time.Time) {
	if interval <= 0 {
		return
	}
	next := startedAt.Add(time.Duration(*count+1) * interval)
	m.RegisterTimer(next, func(ctx context.Context, _ time.Time) (bool, time.Duration) {
		if state.LaunchState != phase {
			return false, 0
		}
		if *count >= max {
			if kind == "escalation" {
				state.LaunchState = model.TimedOutLaunchState
				if err := m.persistStep(ctx, state); err != nil {
					lgr.Logger.Error("Failed to persist timed-out launch", slog.String("launchID", state.ID), slog.Any("error", err))
				}
			}
			return false, 0
		}

		message := stringsForNudge(kind, state.AssetID, task)
		_, err := notificationactivities.NewActivities(m.Svcs().CfgSvc, m.Svcs().DataSvc).Notify(
			notificationactivities.NotificationRequest{Recipient: recipient, Message: message},
		)
		if err != nil {
			lgr.Logger.Error("Failed to send launch "+kind, slog.String("launchID", state.ID), slog.Any("error", err))
		}
		(*count)++
		if err := m.persistStep(ctx, state); err != nil {
			lgr.Logger.Error("Failed to persist launch "+kind, slog.String("launchID", state.ID), slog.Any("error", err))
		}
		return true, interval
	})
}

func stringsForNudge(kind, assetID, task string) string {
	switch kind {
	case "escalation":
		return "Escalation: launch of asset " + assetID + " is overdue; someone needs to " + task
	case "warning":
		return "Warning: please " + task
	default:
		return "Reminder: please " + task
	}
}

func (m *launchMinion) persistStep(ctx context.Context, state *LaunchMinionState) error {
	return m.persistIfChanged(ctx, state)
}

func (m *launchMinion) persistIfChanged(ctx context.Context, state *LaunchMinionState) error {
	current := m.snapshot(state)
	if current == m.persisted {
		return nil
	}
	tierID := current.tierID
	err := m.activities.Persist(ctx, launchactivities.LaunchPersistRequest{
		LaunchID:    state.ID,
		TierID:      tierID,
		State:       current.state,
		Reminders:   current.reminders,
		Warnings:    current.warnings,
		Escalations: current.escalations,
	})
	if err != nil {
		return fmt.Errorf("persist launch %s: %w", state.ID, err)
	}
	m.persisted = current
	return nil
}

func (m *launchMinion) snapshot(state *LaunchMinionState) launchPersisted {
	return launchPersisted{
		tierID:      state.TierID,
		state:       state.LaunchState,
		reminders:   state.Reminders,
		warnings:    state.Warnings,
		escalations: state.Escalations,
	}
}

func (m *launchMinion) startShield(ctx context.Context, state *LaunchMinionState) {
	if m.Svcs().FlowSvc == nil {
		lgr.Logger.Error("Unable to start shield for validated launch: flow service is nil", slog.String("launchID", state.ID))
		return
	}
	if err := m.Svcs().FlowSvc.InitiateShield(ctx, state.AssetID); err != nil {
		lgr.Logger.Error("Failed to start shield workflow", slog.String("assetID", state.AssetID), slog.Any("error", err))
	}
}

func (m *launchMinion) SubmitAnswers(submissions []model.AnswerSubmission) error {
	if err := m.preflight(model.WaitingForAnswersLaunchState, submissions); err != nil {
		return err
	}
	return m.send("SubmitAnswers", submissions)
}

func (m *launchMinion) FinalizeAnswers(submissions []model.AnswerSubmission) error {
	if err := m.preflight(model.WaitingForAnswersLaunchState, submissions); err != nil {
		return err
	}
	return m.send("FinalizeAnswers", submissions)
}

func (m *launchMinion) ReviseAnswers(submissions []model.AnswerSubmission) error {
	if len(submissions) == 0 {
		return errors.New("no answers to revise")
	}
	if err := m.preflight(model.WaitingForTierApprovalLaunchState, submissions); err != nil {
		return err
	}
	return m.send("ReviseAnswers", submissions)
}

func (m *launchMinion) ValidateTier(tierID string) error {
	state := m.GetState()
	if !state.Running {
		return ErrNotRunning
	}
	if state.LaunchState != model.WaitingForTierApprovalLaunchState {
		return fmt.Errorf("%w: launch is %s, want %s", ErrInvalidLaunchMinionState, state.LaunchState, model.WaitingForTierApprovalLaunchState)
	}
	if tierID != "" {
		if m.Svcs().DataSvc == nil {
			return errors.New("validate launch tier: data service is nil")
		}
		if _, err := m.Svcs().DataSvc.RetrieveTierByID(m.ctx, tierID); err != nil {
			return fmt.Errorf("validate launch tier %q: %w", tierID, err)
		}
	}
	return m.send("ValidateTier", tierID)
}

func (m *launchMinion) preflight(want model.LaunchState, submissions []model.AnswerSubmission) error {
	state := m.GetState()
	if !state.Running {
		return ErrNotRunning
	}
	if state.LaunchState != want {
		return fmt.Errorf("%w: launch is %s, want %s", ErrInvalidLaunchMinionState, state.LaunchState, want)
	}
	if len(submissions) == 0 {
		return nil
	}
	if m.Svcs().DataSvc == nil {
		return errors.New("validate launch answers: data service is nil")
	}
	questions, err := m.Svcs().DataSvc.RetrieveQuestionsByCategoryType(m.ctx, model.LaunchQuestionCategory)
	if err != nil {
		return fmt.Errorf("retrieve launch questions: %w", err)
	}
	predefined, err := m.Svcs().DataSvc.RetrievePredefinedAnswersByCategoryType(m.ctx, model.LaunchQuestionCategory)
	if err != nil {
		return fmt.Errorf("retrieve predefined launch answers: %w", err)
	}
	if err := answers.Validate(questions, predefined, submissions); err != nil {
		return err
	}
	return nil
}

func (m *launchMinion) send(command string, arg any) error {
	state := m.State()
	return m.Signal(MinionRequest{
		ID:        state.ID,
		ManagerID: state.ManagerID,
		Kind:      state.Kind,
		Command:   command,
		Args:      []any{arg},
	})
}

func (m *launchMinion) GetState() LaunchMinionState {
	var state LaunchMinionState
	m.View(func(s *LaunchMinionState) {
		state = *s
		state.MinionState = cloneBase(&s.MinionState)
	})
	return state
}

func launchAnswersArg(req MinionRequest) ([]model.AnswerSubmission, error) {
	if len(req.Args) != 1 {
		return nil, fmt.Errorf("%s needs answer submissions", req.Command)
	}
	submissions, ok := req.Args[0].([]model.AnswerSubmission)
	if !ok {
		return nil, fmt.Errorf("%s submissions have type %T", req.Command, req.Args[0])
	}
	return submissions, nil
}

func stringArg(req MinionRequest) (string, error) {
	if len(req.Args) != 1 {
		return "", fmt.Errorf("%s needs a tier ID", req.Command)
	}
	tierID, ok := req.Args[0].(string)
	if !ok {
		return "", fmt.Errorf("%s tier ID has type %T", req.Command, req.Args[0])
	}
	return tierID, nil
}

func seconds(n int) time.Duration {
	return time.Duration(n) * time.Second
}
