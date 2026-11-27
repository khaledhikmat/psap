package flow

import (
	"context"

	"github.com/khaledhikmat/psap/model"
)

type IService interface {
	// Finalize releases the service's resources (e.g. closes the Temporal client). The service
	// must not be used afterwards.
	Finalize()

	InitiateLaunch(ctx context.Context, assetID string) error
	SubmitLaunchAnswers(ctx context.Context, assetID string, submissions []model.AnswerSubmission) error
	FinalizeLaunchAnswers(ctx context.Context, assetID string, submissions []model.AnswerSubmission) error
	// ReviseLaunchAnswers lets the business manager change the owner's answers while the tier
	// awaits approval. It returns the tier recalculated from the revised answers.
	ReviseLaunchAnswers(ctx context.Context, assetID string, submissions []model.AnswerSubmission) (string, error)
	ValidateTier(ctx context.Context, assetID string, tierID string) error
	QueryLaunch(ctx context.Context, assetID string) (model.Launch, error)

	InitiateShield(ctx context.Context, assetID string) error
	SubmitShieldAnswers(ctx context.Context, assetID string, submissions []model.AnswerSubmission) error
	FinalizeShieldAnswers(ctx context.Context, assetID string, submissions []model.AnswerSubmission) error
	SubmitShieldEvidences(ctx context.Context, assetID string, evidences []model.EvidenceSubmission) error
	FinalizeShieldEvidences(ctx context.Context, assetID string, evidences []model.EvidenceSubmission) error
	QueryShield(ctx context.Context, assetID string) (model.Shield, error)
}
