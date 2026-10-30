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
)

type IService interface {
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

	// NewAnswers and UpdateAnswers apply all answers in a single transaction.
	NewAnswers(ctx context.Context, answers []model.Answer) error
	RetrieveAnswersByLaunchID(ctx context.Context, launchID string) ([]model.Answer, error)
	RetrieveAnswerByID(ctx context.Context, id string) (model.Answer, error)
	UpdateAnswers(ctx context.Context, answers []model.Answer) error

	NewLaunch(ctx context.Context, launch model.Launch) error
	RetrieveLaunches(ctx context.Context) ([]model.Launch, error)
	RetrieveLaunchesByAssetID(ctx context.Context, assetID string) ([]model.Launch, error)
	RetrieveLaunchByID(ctx context.Context, id string) (model.Launch, error)
	UpdateLaunch(ctx context.Context, launch model.Launch) error

	NewShield(ctx context.Context, shield model.Shield) error
	RetrieveShields(ctx context.Context) ([]model.Shield, error)
	RetrieveShieldsByAssetID(ctx context.Context, assetID string) ([]model.Shield, error)
	RetrieveShieldByID(ctx context.Context, id string) (model.Shield, error)
	UpdateShield(ctx context.Context, shield model.Shield) error

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
