package mode

import (
	"context"
	"log/slog"

	"github.com/khaledhikmat/psap/service"
	"github.com/khaledhikmat/psap/service/lgr"
	"go.opentelemetry.io/otel"
)

const (
	tracerPrefix = "grp.esg.soi.td.sp.mode"
)

var (
	tracer = otel.Tracer(tracerPrefix)
)

type Processor func(canxCtx context.Context, runID string, svcs service.ServicesFactory) error

func procEvent(ctx context.Context, svcs service.ServicesFactory, evt interface{}) {
	// Send the event to the database
	err := svcs.DataSvc.NewEvent(ctx, evt)
	if err != nil {
		lgr.Logger.Error(
			"failed to store event",
			slog.Any("error", err),
		)
	}

	// Send the event to other destinations
}
