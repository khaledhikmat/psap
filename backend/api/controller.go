package api

import (
	"fmt"
	"log/slog"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/khaledhikmat/psap/model"
	"github.com/khaledhikmat/psap/service/data"
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
	apiMetricPrefix = "grp.csa.soi.psap.api"
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
	DataSvc     data.IService
	EventStream chan<- interface{}
}

func NewController(dataSvc data.IService, eventStream chan<- interface{}) *Controller {
	apiEnsureMetrics()

	if !apiMetricsInitialized {
		eventStream <- model.GenErrEvent("api", "controller",
			"high",
			fmt.Errorf("unable to initialize metrics"),
			map[string]interface{}{},
			"error initializing metrics")

		return nil
	}

	return &Controller{
		DataSvc:     dataSvc,
		EventStream: eventStream,
	}
}

func (c *Controller) Routes(rootRouter *gin.RouterGroup) {
	rootRouter.GET("/ping", c.ping)

	// Launches
	launchesRouter := rootRouter.Group("/launches")
	{
		launchesRouter.GET("", c.listLaunches)
		launchesRouter.GET("/:id", c.showLaunch)
		launchesRouter.GET("/:id/answers", c.listLaunchAnswers)
		launchesRouter.GET("/:id/history", c.listLaunchAnswerHistory)
		launchesRouter.POST("", c.initiateLaunch)
		launchesRouter.POST("/:id/submit-answers", c.submitLaunchAnswers)
		launchesRouter.POST("/:id/finalize-answers", c.finalizeLaunchAnswers)
		launchesRouter.POST("/:id/revise-answers", c.reviseLaunchAnswers)
		launchesRouter.POST("/:id/validate-tier", c.validateLaunchTier)
	}

	// Shields
	shieldsRouter := rootRouter.Group("/shields")
	{
		shieldsRouter.GET("", c.listShields)
		shieldsRouter.GET("/:id", c.showShield)
		shieldsRouter.GET("/:id/answers", c.listShieldAnswers)
		shieldsRouter.GET("/:id/evidences", c.listShieldEvidences)
		shieldsRouter.GET("/:id/history", c.listShieldAnswerHistory)
		shieldsRouter.POST("", c.initiateShield)
		shieldsRouter.POST("/:id/submit-answers", c.submitShieldAnswers)
		shieldsRouter.POST("/:id/finalize-answers", c.finalizeShieldAnswers)
		shieldsRouter.POST("/:id/submit-evidences", c.submitShieldEvidences)
		shieldsRouter.POST("/:id/finalize-evidences", c.finalizeShieldEvidences)
	}
}
