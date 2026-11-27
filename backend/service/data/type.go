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

	RetrieveQuestionsByType(ctx context.Context, questionType model.QuestionType) ([]model.Question, error)

	SaveLaunchAnswers(ctx context.Context, launchID string, submissions []model.AnswerSubmission, actor string) error
	SaveThreatAnswers(ctx context.Context, threatID string, submissions []model.AnswerSubmission, actor string) error
	SaveShieldAnswers(ctx context.Context, shieldID string, submissions []model.AnswerSubmission, actor string) error
	SaveThreatEvidences(ctx context.Context, threatID string, submissions []model.EvidenceSubmission, actor string) error
	SaveShieldEvidences(ctx context.Context, shieldID string, submissions []model.EvidenceSubmission, actor string) error
	RetrieveAnswersByLaunchID(ctx context.Context, launchID string) ([]model.UserAnswerDetail, error)
	RetrieveAnswersByShieldID(ctx context.Context, shieldID string) ([]model.UserAnswerDetail, error)
	RetrieveAnswersByThreatID(ctx context.Context, threatID string) ([]model.UserAnswerDetail, error)

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
	RetrieveEvidencesByThreatID(ctx context.Context, threatID string) ([]model.Evidence, error)
	RetrieveEvidenceByID(ctx context.Context, id string) (model.Evidence, error)
	UpdateEvidences(ctx context.Context, evidences []model.Evidence) error

	NewActionItem(ctx context.Context, actionItem model.ActionItem) error
	RetrieveActionItems(ctx context.Context) ([]model.ActionItem, error)
	RetrieveActionItemsByBaselineID(ctx context.Context, baselineID string) ([]model.ActionItem, error)
	RetrieveActionItemByID(ctx context.Context, id string) (model.ActionItem, error)
	UpdateActionItem(ctx context.Context, actionItem model.ActionItem) error
}
