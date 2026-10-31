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
	SubmitLaunchAnswers(ctx context.Context, assetID string, answers []model.Answer) error
	FinalizeLaunchAnswers(ctx context.Context, assetID string, answers []model.Answer) error
	ValidateTier(ctx context.Context, assetID string, tierID string) error
	QueryLaunch(ctx context.Context, assetID string) (model.Launch, error)

	InitiateShield(ctx context.Context, assetID string) error
	SubmitShieldAnswers(ctx context.Context, assetID string, answers map[string]string) error
	FinalizeShieldAnswers(ctx context.Context, assetID string, answers map[string]string) error
	SubmitShieldEvidences(ctx context.Context, assetID string, evidences map[string]model.Evidence) error
	FinalizeShieldEvidences(ctx context.Context, assetID string, evidences map[string]model.Evidence) error
	QueryShield(ctx context.Context, assetID string) (model.Shield, error)
}
