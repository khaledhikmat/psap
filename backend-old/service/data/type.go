package data

import (
	"context"
	"errors"

	"github.com/khaledhikmat/psap/model"
)

var (
	// ErrNotFound is returned when a retrieve or update targets an ID (or name) that does not exist.
	ErrNotFound = errors.New("not found")
	// ErrAlreadyExists is returned when a new entity's ID is already taken.
	ErrAlreadyExists = errors.New("already exists")
	// ErrInvalidState is returned when a state transition is attempted from the wrong state.
	ErrInvalidState = errors.New("invalid entity state")
)

type IService interface {
	// Finalize releases the service's resources (e.g. closes the database). The service must
	// not be used afterwards.
	Finalize()

	NewEvent(ctx context.Context, evt interface{}) error

	NewWorker(ctx context.Context, worker model.Worker) error
	RetrieveWorkers(ctx context.Context) ([]model.Worker, error)
	RetrieveWorkerByID(ctx context.Context, id string) (model.Worker, error)
	UpdateWorker(ctx context.Context, worker model.Worker) error

	NewAsset(ctx context.Context, asset model.Asset) error
	RetrieveAssets(ctx context.Context) ([]model.Asset, error)
	RetrieveAssetByID(ctx context.Context, id string) (model.Asset, error)
	RetrieveAssetByName(ctx context.Context, name string) (model.Asset, error)
	UpdateAsset(ctx context.Context, asset model.Asset) error

	NewQuestionCategory(ctx context.Context, category model.QuestionCategory) error
	RetrieveQuestionCategories(ctx context.Context) ([]model.QuestionCategory, error)
	RetrieveQuestionCategoriesByType(ctx context.Context, categoryType model.QuestionCategoryType) ([]model.QuestionCategory, error)
	RetrieveQuestionCategoryByID(ctx context.Context, id string) (model.QuestionCategory, error)
	UpdateQuestionCategory(ctx context.Context, category model.QuestionCategory) error

	NewQuestion(ctx context.Context, question model.Question) error
	RetrieveQuestions(ctx context.Context) ([]model.Question, error)
	RetrieveQuestionsByCategoryID(ctx context.Context, categoryID string) ([]model.Question, error)
	// RetrieveQuestionsByCategoryType returns the questions of every category of the given type.
	RetrieveQuestionsByCategoryType(ctx context.Context, categoryType model.QuestionCategoryType) ([]model.Question, error)
	RetrieveQuestionByID(ctx context.Context, id string) (model.Question, error)
	UpdateQuestion(ctx context.Context, question model.Question) error

	NewPredefinedAnswer(ctx context.Context, answer model.PredefinedAnswer) error
	RetrievePredefinedAnswers(ctx context.Context) ([]model.PredefinedAnswer, error)
	RetrievePredefinedAnswersByQuestionID(ctx context.Context, questionID string) ([]model.PredefinedAnswer, error)
	// RetrievePredefinedAnswersByCategoryType returns the predefined answers of every question of
	// every category of the given type, in question order, then answer order.
	RetrievePredefinedAnswersByCategoryType(ctx context.Context, categoryType model.QuestionCategoryType) ([]model.PredefinedAnswer, error)
	RetrievePredefinedAnswerByID(ctx context.Context, id string) (model.PredefinedAnswer, error)
	UpdatePredefinedAnswer(ctx context.Context, answer model.PredefinedAnswer) error

	// SaveLaunchAnswers and SaveShieldAnswers set the selected predefined answers of each
	// submitted question in a single transaction, creating the user answer on first submission.
	// Selections that change are recorded in the answer history. The submissions must already be
	// valid (see the answers package); questions not submitted are left unchanged.
	SaveLaunchAnswers(ctx context.Context, launchID string, submissions []model.AnswerSubmission, actor string) error
	SaveShieldAnswers(ctx context.Context, shieldID string, submissions []model.AnswerSubmission, actor string) error
	// SaveShieldEvidences atomically upserts evidence for questions already answered on a shield.
	SaveShieldEvidences(ctx context.Context, shieldID string, submissions []model.EvidenceSubmission, actor string) error
	RetrieveUserAnswersByLaunchID(ctx context.Context, launchID string) ([]model.UserAnswerDetail, error)
	RetrieveUserAnswersByShieldID(ctx context.Context, shieldID string) ([]model.UserAnswerDetail, error)
	RetrieveUserAnswerByID(ctx context.Context, id string) (model.UserAnswerDetail, error)

	// UserAnswerHistory is the audit trail of changes to user answers, newest first.
	RetrieveUserAnswerHistoryByUserAnswerID(ctx context.Context, userAnswerID string) ([]model.UserAnswerHistory, error)
	RetrieveUserAnswerHistoryByLaunchID(ctx context.Context, launchID string) ([]model.UserAnswerHistory, error)
	RetrieveUserAnswerHistoryByShieldID(ctx context.Context, shieldID string) ([]model.UserAnswerHistory, error)

	NewLaunch(ctx context.Context, launch model.Launch) error
	// InitiateLaunch atomically replaces any active launch for the asset, inserts launch, and updates LastLaunchID.
	InitiateLaunch(ctx context.Context, launch model.Launch) error
	RetrieveLaunches(ctx context.Context) ([]model.Launch, error)
	RetrieveLaunchesByAssetID(ctx context.Context, assetID string) ([]model.Launch, error)
	RetrieveLaunchByID(ctx context.Context, id string) (model.Launch, error)
	UpdateLaunch(ctx context.Context, launch model.Launch) error
	// ValidateLaunchTier atomically validates the launch and advances its asset to pending-shield.
	ValidateLaunchTier(ctx context.Context, launchID, tierID, actor string) error

	NewShield(ctx context.Context, shield model.Shield) error
	// InitiateShield atomically replaces any active shield for the asset, inserts shield, and updates LastShieldID.
	InitiateShield(ctx context.Context, shield model.Shield) error
	RetrieveShields(ctx context.Context) ([]model.Shield, error)
	RetrieveShieldsByAssetID(ctx context.Context, assetID string) ([]model.Shield, error)
	RetrieveShieldByID(ctx context.Context, id string) (model.Shield, error)
	UpdateShield(ctx context.Context, shield model.Shield) error
	// CompleteShield atomically completes a shield and advances its asset to shielded.
	CompleteShield(ctx context.Context, shieldID string, score int, actor string) error

	// NewEvidences and UpdateEvidences apply all evidences in a single transaction.
	NewEvidences(ctx context.Context, evidences []model.Evidence) error
	RetrieveEvidencesByLaunchID(ctx context.Context, launchID string) ([]model.Evidence, error)
	RetrieveEvidencesByShieldID(ctx context.Context, shieldID string) ([]model.Evidence, error)
	RetrieveEvidenceByID(ctx context.Context, id string) (model.Evidence, error)
	UpdateEvidences(ctx context.Context, evidences []model.Evidence) error

	NewBaseline(ctx context.Context, baseline model.Baseline) error
	RetrieveBaselines(ctx context.Context) ([]model.Baseline, error)
	RetrieveBaselinesByAssetID(ctx context.Context, assetID string) ([]model.Baseline, error)
	RetrieveBaselineByID(ctx context.Context, id string) (model.Baseline, error)
	UpdateBaseline(ctx context.Context, baseline model.Baseline) error

	NewActionItem(ctx context.Context, actionItem model.ActionItem) error
	RetrieveActionItems(ctx context.Context) ([]model.ActionItem, error)
	RetrieveActionItemsByBaselineID(ctx context.Context, baselineID string) ([]model.ActionItem, error)
	RetrieveActionItemByID(ctx context.Context, id string) (model.ActionItem, error)
	UpdateActionItem(ctx context.Context, actionItem model.ActionItem) error

	NewTier(ctx context.Context, tier model.Tier) error
	RetrieveTiers(ctx context.Context) ([]model.Tier, error)
	RetrieveTierByID(ctx context.Context, id string) (model.Tier, error)
	// RetrieveTierByPoints returns the tier whose [FromPoint, ToPoint] range contains points.
	RetrieveTierByPoints(ctx context.Context, points int) (model.Tier, error)
	UpdateTier(ctx context.Context, tier model.Tier) error
}
