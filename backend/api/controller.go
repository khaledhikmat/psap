package api

import (
	"fmt"
	"log/slog"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/khaledhikmat/psap/model"
	"github.com/khaledhikmat/psap/service"
	"github.com/khaledhikmat/psap/service/lgr"
	"github.com/khaledhikmat/psap/utils"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
	"golang.org/x/xerrors"
)

const (
	ContextRoot = "/v1/physec/accesscontrol/psap"
)

const (
	apiMetricPrefix = "grp.esg.soi.td.sp.api"
)

var (
	apiMetricsOnce        sync.Once
	apiMetricsInitialized bool

	apiAlertermeter = otel.Meter(apiMetricPrefix)

	apiAlertLatency metric.Int64Histogram
	instanceType    = utils.GetInstanceType()
)

// Initialize metrics lazingly
func apiEnsureMetrics() {
	apiMetricsOnce.Do(func() {

		var err error
		apiAlertLatency, err = apiAlertermeter.Int64Histogram(
			fmt.Sprintf("%s.alert_latency", apiMetricPrefix),
			metric.WithDescription("Measures alert latency"),
			metric.WithUnit("ms"))
		if err != nil {
			lgr.Logger.Error(
				"apiEnsureMetrics - unable to initialize metrics",
				slog.Any("error", xerrors.New(err.Error())),
			)
			return
		}

		apiMetricsInitialized = true
	})
}

type errorResult struct {
	Error string `json:"error"`
}

func newError(ctx *gin.Context, status int, err error) {
	ctx.JSON(status, errorResult{
		Error: err.Error(),
	})
}

type Controller struct {
	Svcs        service.ServicesFactory
	EventStream chan<- interface{}
}

func NewController(svcs service.ServicesFactory, eventStream chan<- interface{}) *Controller {
	apiEnsureMetrics()

	if !apiMetricsInitialized {
		eventStream <- model.GenErrEvent("api", "controller",
			model.HighEventSeverity,
			fmt.Errorf("unable to initialize metrics"),
			map[string]interface{}{},
			"error initializing metrics")

		return nil
	}

	// TODO: Create Temporal client

	return &Controller{
		Svcs:        svcs,
		EventStream: eventStream,
	}
}

func (c *Controller) Routes(rootRouter *gin.RouterGroup) {
	rootRouter.GET("/ping", c.ping)

	tiersRouter := rootRouter.Group("/tiers")
	{
		tiersRouter.GET("", c.listTiers)
	}

	assetsRouter := rootRouter.Group("/assets")
	{
		assetsRouter.GET("", c.listAssets)
		assetsRouter.GET("/:id", c.showAsset)
		assetsRouter.GET("/:id/launches", c.listAssetLaunches)
		assetsRouter.GET("/:id/shields", c.listAssetShields)
		assetsRouter.GET("/:id/baselines", c.listAssetBaselines)

		assetsRouter.POST("/:id/launches", c.initiateAssetLaunch)
	}

	launchesRouter := rootRouter.Group("/launches")
	{
		launchesRouter.GET("", c.listLaunches)
		launchesRouter.GET("/:id", c.showLaunch)
	}

	errorsRouter := rootRouter.Group("/errors")
	{
		errorsRouter.GET("", c.listErrors)
	}
}
