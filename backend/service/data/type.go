package data

import (
	"context"

	"github.com/khaledhikmat/psap/model"
)

type IService interface {
	NewWorker(ctx context.Context, worker model.Worker) error
	RetrieveWorkers(ctx context.Context) ([]model.Worker, error)
	RetrieveWorkerByID(ctx context.Context, id string) (model.Worker, error)
	UpdateWorker(ctx context.Context, worker model.Worker) error

	NewAsset(ctx context.Context, asset model.Asset) error
	RetrieveAssets(ctx context.Context) ([]model.Asset, error)
	RetrieveAssetByName(ctx context.Context, name string) (model.Asset, error)
	UpdateAsset(ctx context.Context, asset model.Asset) error

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
	UpdateTier(ctx context.Context, tier model.Tier) error
}
