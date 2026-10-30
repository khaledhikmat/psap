package flow

import (
	"context"

	"github.com/khaledhikmat/psap/model"
)

type IService interface {
	InitiateLaunch(ctx context.Context, assetID string) error
	SubmitLaunchAnswers(ctx context.Context, assetID string, answers []model.Answer) error
	FinalizeLaunchAnswers(ctx context.Context, assetID string, answers []model.Answer) error
	ValidateTier(ctx context.Context, assetID string, tierID string) error
	QueryLaunch(ctx context.Context, assetID string) (model.Launch, error)
}
