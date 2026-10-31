package launch

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/khaledhikmat/psap/model"
	"github.com/khaledhikmat/psap/service/config"
	"github.com/khaledhikmat/psap/service/data"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/temporal"
)

const launchActor = "launch-workflow"

// ErrInvalidAnswer is the application error type of answers that cannot be saved. Retrying
// does not help, so activities return it as a non-retryable error.
const ErrInvalidAnswer = "InvalidAnswer"

// Activities holds the dependencies shared by the launch activities.
type Activities struct {
	CfgSvc  config.IService
	DataSvc data.IService
}

func NewActivities(cfgSvc config.IService, dataSvc data.IService) *Activities {
	return &Activities{CfgSvc: cfgSvc, DataSvc: dataSvc}
}

// Create stores a new launch with a blank answer for every launch question, and records it
// as the asset's last launch.
//
// The launch ID is the workflow's run ID and answer IDs derive from it, so both are stable
// across activity retries: a retry after a successful insert finds the rows and succeeds
// instead of creating a second launch.
func (a *Activities) Create(ctx context.Context, req LaunchCreationRequest) (LaunchCreationResult, error) {
	launchID := activity.GetInfo(ctx).WorkflowExecution.RunID

	asset, err := a.DataSvc.RetrieveAssetByID(ctx, req.AssetID)
	if err != nil {
		return LaunchCreationResult{}, err
	}

	err = a.DataSvc.NewLaunch(ctx, model.Launch{
		ID:        launchID,
		AssetID:   req.AssetID,
		State:     model.WaitingForAnswersLaunchState,
		CreatedBy: launchActor,
		UpdatedBy: launchActor,
	})
	if err != nil && !errors.Is(err, data.ErrAlreadyExists) {
		return LaunchCreationResult{}, err
	}

	questions, err := a.DataSvc.RetrieveQuestionsByCategoryType(ctx, model.LaunchQuestionCategory)
	if err != nil {
		return LaunchCreationResult{}, err
	}
	answers := make([]model.Answer, len(questions))
	for i, q := range questions {
		answers[i] = model.Answer{
			ID:         launchID + "-" + q.ID,
			QuestionID: q.ID,
			LaunchID:   launchID,
			Reply:      model.UnansweredReply,
			CreatedBy:  launchActor,
			UpdatedBy:  launchActor,
		}
	}
	err = a.DataSvc.NewAnswers(ctx, answers)
	if err != nil && !errors.Is(err, data.ErrAlreadyExists) {
		return LaunchCreationResult{}, err
	}

	asset.LastLaunchID = &launchID
	asset.UpdatedBy = launchActor
	if err := a.DataSvc.UpdateAsset(ctx, asset); err != nil {
		return LaunchCreationResult{}, err
	}

	return LaunchCreationResult{
		LaunchID:  launchID,
		Questions: len(questions),
	}, nil
}

// SaveAnswers records the given replies on the launch's answers. Partial submissions are
// accepted; questions not in the request keep their current reply.
func (a *Activities) SaveAnswers(ctx context.Context, req LaunchAnswersSaveRequest) (LaunchAnswersSaveResult, error) {
	existing, err := a.DataSvc.RetrieveAnswersByLaunchID(ctx, req.LaunchID)
	if err != nil {
		return LaunchAnswersSaveResult{}, err
	}
	byQuestion := make(map[string]int, len(existing))
	for i, ans := range existing {
		byQuestion[ans.QuestionID] = i
	}

	changed := map[int]bool{}
	for _, submitted := range req.Answers {
		i, ok := byQuestion[submitted.QuestionID]
		if !ok {
			return LaunchAnswersSaveResult{}, temporal.NewNonRetryableApplicationError(
				fmt.Sprintf("question %q is not part of launch %s", submitted.QuestionID, req.LaunchID), ErrInvalidAnswer, nil)
		}
		reply, err := NormalizeReply(submitted.Reply)
		if err != nil {
			return LaunchAnswersSaveResult{}, temporal.NewNonRetryableApplicationError(err.Error(), ErrInvalidAnswer, nil)
		}
		if existing[i].Reply != reply {
			existing[i].Reply = reply
			existing[i].UpdatedBy = req.UpdatedBy
			changed[i] = true
		}
	}

	updates := make([]model.Answer, 0, len(changed))
	for i := range existing {
		if changed[i] {
			updates = append(updates, existing[i])
		}
	}
	if err := a.DataSvc.UpdateAnswers(ctx, updates); err != nil {
		return LaunchAnswersSaveResult{}, err
	}

	return LaunchAnswersSaveResult{
		Answered:  countAnswered(existing),
		Questions: len(existing),
	}, nil
}

// CalculateTier adds up the weights of the launch's YES answers and picks the tier whose
// point range holds the total. Unanswered questions count as NO.
func (a *Activities) CalculateTier(ctx context.Context, req LaunchTierCalculationRequest) (LaunchTierCalculationResult, error) {
	answers, err := a.DataSvc.RetrieveAnswersByLaunchID(ctx, req.LaunchID)
	if err != nil {
		return LaunchTierCalculationResult{}, err
	}
	questions, err := a.DataSvc.RetrieveQuestionsByCategoryType(ctx, model.LaunchQuestionCategory)
	if err != nil {
		return LaunchTierCalculationResult{}, err
	}

	points := Points(questions, answers)
	tier, err := a.DataSvc.RetrieveTierByPoints(ctx, points)
	if errors.Is(err, data.ErrNotFound) {
		// The tier table has a gap; retrying will not fix it.
		return LaunchTierCalculationResult{}, temporal.NewNonRetryableApplicationError(
			fmt.Sprintf("no tier covers %d points", points), "NoTier", err)
	}
	if err != nil {
		return LaunchTierCalculationResult{}, err
	}

	return LaunchTierCalculationResult{
		Points: points,
		TierID: tier.ID,
	}, nil
}

// Persist stores the launch's current tier, state and nudge counts. Once the tier is
// validated, it also becomes the asset's tier and the asset moves on to pending shield.
func (a *Activities) Persist(ctx context.Context, req LaunchPersistRequest) error {
	launch, err := a.DataSvc.RetrieveLaunchByID(ctx, req.LaunchID)
	if errors.Is(err, data.ErrNotFound) {
		// E.g. the launch was cancelled before Create stored it; retrying will not help.
		return temporal.NewNonRetryableApplicationError(err.Error(), "LaunchNotFound", err)
	}
	if err != nil {
		return err
	}
	launch.TierID = &req.TierID
	launch.State = req.State
	launch.Reminders = req.Reminders
	launch.Warnings = req.Warnings
	launch.Escalations = req.Escalations
	launch.UpdatedBy = launchActor
	if err := a.DataSvc.UpdateLaunch(ctx, launch); err != nil {
		return err
	}

	if req.State != model.TierValidatedLaunchState {
		return nil
	}
	asset, err := a.DataSvc.RetrieveAssetByID(ctx, launch.AssetID)
	if err != nil {
		return err
	}
	asset.TierID = &req.TierID
	asset.State = model.PendingShieldAssetState
	asset.UpdatedBy = launchActor
	return a.DataSvc.UpdateAsset(ctx, asset)
}

// NormalizeReply accepts yes/no in any case (and "" for unanswered) and returns the stored form.
func NormalizeReply(reply string) (string, error) {
	switch r := strings.ToUpper(strings.TrimSpace(reply)); r {
	case model.UnansweredReply, model.YesReply, model.NoReply:
		return r, nil
	default:
		return "", fmt.Errorf("invalid reply %q, want %s or %s", reply, model.YesReply, model.NoReply)
	}
}

// Points is the sum of the weights of the questions answered YES.
func Points(questions []model.Question, answers []model.Answer) int {
	weights := make(map[string]int, len(questions))
	for _, q := range questions {
		weights[q.ID] = q.Weight
	}
	points := 0
	for _, ans := range answers {
		if ans.Reply == model.YesReply {
			points += weights[ans.QuestionID]
		}
	}
	return points
}

func countAnswered(answers []model.Answer) int {
	n := 0
	for _, ans := range answers {
		if ans.Reply != model.UnansweredReply {
			n++
		}
	}
	return n
}
