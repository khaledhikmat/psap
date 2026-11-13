package launch

import (
	"context"
	"errors"
	"fmt"

	"github.com/khaledhikmat/psap/model"
	"github.com/khaledhikmat/psap/service/answers"
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

// Create stores a new launch and records it as the asset's last launch. Answers are created
// as the business owner submits them.
//
// Temporal workflows use the run ID as the launch ID, so retries find the row created by an
// earlier attempt. Minions supply their own stable launch ID in req.LaunchID.
func (a *Activities) Create(ctx context.Context, req LaunchCreationRequest) (LaunchCreationResult, error) {
	launchID := req.LaunchID
	if launchID == "" {
		launchID = activity.GetInfo(ctx).WorkflowExecution.RunID
	}

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

// SaveAnswers records the selected predefined answers of the submitted questions. Partial
// submissions are accepted; questions not in the request keep their current answers.
func (a *Activities) SaveAnswers(ctx context.Context, req LaunchAnswersSaveRequest) (LaunchAnswersSaveResult, error) {
	questions, err := a.DataSvc.RetrieveQuestionsByCategoryType(ctx, model.LaunchQuestionCategory)
	if err != nil {
		return LaunchAnswersSaveResult{}, err
	}
	predefined, err := a.DataSvc.RetrievePredefinedAnswersByCategoryType(ctx, model.LaunchQuestionCategory)
	if err != nil {
		return LaunchAnswersSaveResult{}, err
	}
	if err := answers.Validate(questions, predefined, req.Answers); err != nil {
		return LaunchAnswersSaveResult{}, temporal.NewNonRetryableApplicationError(err.Error(), ErrInvalidAnswer, err)
	}

	if err := a.DataSvc.SaveLaunchAnswers(ctx, req.LaunchID, req.Answers, req.UpdatedBy); err != nil {
		return LaunchAnswersSaveResult{}, err
	}
	saved, err := a.DataSvc.RetrieveUserAnswersByLaunchID(ctx, req.LaunchID)
	if err != nil {
		return LaunchAnswersSaveResult{}, err
	}

	return LaunchAnswersSaveResult{
		Answered:  answers.Answered(saved),
		Questions: len(questions),
	}, nil
}

// CalculateTier adds up the points of the launch's selected answers and picks the tier whose
// point range holds the total. Unanswered questions earn no points.
func (a *Activities) CalculateTier(ctx context.Context, req LaunchTierCalculationRequest) (LaunchTierCalculationResult, error) {
	saved, err := a.DataSvc.RetrieveUserAnswersByLaunchID(ctx, req.LaunchID)
	if err != nil {
		return LaunchTierCalculationResult{}, err
	}
	predefined, err := a.DataSvc.RetrievePredefinedAnswersByCategoryType(ctx, model.LaunchQuestionCategory)
	if err != nil {
		return LaunchTierCalculationResult{}, err
	}

	points := answers.Points(saved, predefined)
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
