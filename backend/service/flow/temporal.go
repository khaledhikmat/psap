package flow

import (
	"context"
	"log/slog"

	"github.com/khaledhikmat/psap/model"
	"github.com/khaledhikmat/psap/service/config"
	"github.com/khaledhikmat/psap/service/data"
	"github.com/khaledhikmat/psap/service/flow/helpers"
	"github.com/khaledhikmat/psap/service/flow/workflows"
	"github.com/khaledhikmat/psap/service/lgr"
	"golang.org/x/xerrors"

	"go.temporal.io/sdk/client"
)

type service struct {
	cfgsvc  config.IService
	datasvc data.IService
	tClient client.Client
}

func NewTemporal(cfgsvc config.IService, datasvc data.IService) IService {
	// Create Temporal client here if needed
	t, err := client.Dial(client.Options{})
	if err != nil {
		lgr.Logger.Error(
			"httpServerEnsureMetrics - unable to initialize metrics",
			slog.Any("error", xerrors.New(err.Error())),
		)
		return nil
	}

	return &service{
		cfgsvc:  cfgsvc,
		datasvc: datasvc,
		tClient: t,
	}
}

func (s *service) InitiateLaunch(ctx context.Context, assetID string) error {
	//TODO: Do not allow another asset launch for the same asset if one is already in progress
	desc, err := s.tClient.DescribeWorkflow(ctx, "asset-launch-wf-"+assetID, "")
	if err != nil {
		lgr.Logger.Error(
			"Unable to describe workflow",
			slog.Any("error", xerrors.New(err.Error())),
		)
	}
	if desc.Status == client.WorkflowRunning {
		return xerrors.New("asset launch already in progress for this asset")
	}

	// Implement the logic for initiating an asset launch here
	workflowID := "asset-launch-wf-" + assetID
	options := client.StartWorkflowOptions{
		ID:        workflowID,
		TaskQueue: helpers.WorkerQueueName,
	}

	we, err := s.tClient.ExecuteWorkflow(ctx, options, workflows.Launch, workflows.LaunchRequest{
		ID:                   workflowID,
		Goal:                 100,
		Rules:                []string{"rule1", "rule2"},
		Awards:               []string{"award1", "award2", "award3"},
		ReviewTimeout:        60,
		FundingTimeout:       60,
		IdeasAcceptedTimeout: 60,
	})
	if err != nil {
		lgr.Logger.Error(
			"Unable to execute workflow",
			slog.Any("error", xerrors.New(err.Error())),
		)
		return err
	}

	lgr.Logger.Info(
		"Started workflow",
		slog.String("WorkflowID", we.GetID()),
		slog.String("RunID", we.GetRunID()),
	)

	return nil
}

func (s *service) QueryLaunch(ctx context.Context, assetID string) (model.Launch, error) {
	// Implement the logic for submitting asset launch answers here
	workflowID := "asset-launch-wf-" + assetID
	we := s.tClient.GetWorkflow(ctx, workflowID, "")
	var launch model.Launch
	err := we.Get(ctx, &launch)
	if err != nil {
		lgr.Logger.Error(
			"Unable to query workflow",
			slog.Any("error", xerrors.New(err.Error())),
		)
		return model.Launch{}, err
	}
	return launch, nil
}

func (s *service) SubmitLaunchAnswers(ctx context.Context, assetID string, answers []model.Answer) error {
	// Implement the logic for submitting asset launch answers here
	workflowID := "asset-launch-wf-" + assetID
	err := s.tClient.SignalWorkflow(ctx, workflowID, "", "submit-answers", answers)
	if err != nil {
		lgr.Logger.Error(
			"Unable to signal workflow",
			slog.Any("error", xerrors.New(err.Error())),
		)
		return err
	}
	return nil
}

func (s *service) FinalizeLaunchAnswers(ctx context.Context, assetID string, answers []model.Answer) error {
	// Implement the logic for submitting asset launch answers here
	workflowID := "asset-launch-wf-" + assetID
	err := s.tClient.SignalWorkflow(ctx, workflowID, "", "finalize-answers", answers)
	if err != nil {
		lgr.Logger.Error(
			"Unable to signal workflow",
			slog.Any("error", xerrors.New(err.Error())),
		)
		return err
	}
	return nil
}

func (s *service) ValidateTier(ctx context.Context, assetID string, tierID string) error {
	// Implement the logic for validating the tier here
	workflowID := "asset-launch-wf-" + assetID
	we := s.tClient.GetWorkflow(ctx, workflowID, "")
	err := we.Signal(ctx, "validate-tier", tierID)
	if err != nil {
		lgr.Logger.Error(
			"Unable to signal workflow",
			slog.Any("error", xerrors.New(err.Error())),
		)
		return err
	}
	return nil
}
