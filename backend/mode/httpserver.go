package mode

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"runtime"
	"sync"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/khaledhikmat/psap/api"
	"github.com/khaledhikmat/psap/service"
	"github.com/khaledhikmat/psap/service/lgr"
	"github.com/khaledhikmat/psap/utils"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"golang.org/x/xerrors"
)

const (
	httpServerMetricPrefix = "grp.csa.soi.psap.http.server"
)

var (
	httpServerMetricsOnce        sync.Once
	httpServerMetricsInitialized bool

	httpServerMeter = otel.Meter(httpServerMetricPrefix)

	httpServerUptimeGauge             metric.Int64Gauge
	httpServerHeapSize                metric.Int64Gauge
	httpServerTotalHeapSize           metric.Int64Gauge
	httpServerTotalSystemHeapSize     metric.Int64Gauge
	httpServerTotalGarbageCollections metric.Int64Gauge
	httpServerStartTime               = time.Now()
)

type ginWithContext func(ctx context.Context) error

// Initialize metrics lazingly
func httpServerEnsureMetrics() {
	httpServerMetricsOnce.Do(func() {

		var err error
		httpServerUptimeGauge, err = httpServerMeter.Int64Gauge(
			fmt.Sprintf("%s.uptime", httpServerMetricPrefix),
			metric.WithDescription("Engine uptime in seconds"),
			metric.WithUnit("s"))
		if err != nil {
			lgr.Logger.Error(
				"httpServerEnsureMetrics - unable to initialize metrics",
				slog.Any("error", xerrors.New(err.Error())),
			)
			return
		}

		httpServerHeapSize, err = httpServerMeter.Int64Gauge(
			fmt.Sprintf("%s.heap_size", httpServerMetricPrefix),
			metric.WithDescription("Current heap size"),
			metric.WithUnit("MiBy"))
		if err != nil {
			lgr.Logger.Error(
				"httpServerEnsureMetrics - unable to initialize metrics",
				slog.Any("error", xerrors.New(err.Error())),
			)
			return
		}

		httpServerTotalHeapSize, err = httpServerMeter.Int64Gauge(
			fmt.Sprintf("%s.total_heap_size", httpServerMetricPrefix),
			metric.WithDescription("Total heap size"),
			metric.WithUnit("MiBy"))
		if err != nil {
			lgr.Logger.Error(
				"httpServerEnsureMetrics - unable to initialize metrics",
				slog.Any("error", xerrors.New(err.Error())),
			)
			return
		}

		httpServerTotalSystemHeapSize, err = httpServerMeter.Int64Gauge(
			fmt.Sprintf("%s.total_system_heap_size", httpServerMetricPrefix),
			metric.WithDescription("Total system-requeted heap size"),
			metric.WithUnit("MiBy"))
		if err != nil {
			lgr.Logger.Error(
				"httpServerEnsureMetrics - unable to initialize metrics",
				slog.Any("error", xerrors.New(err.Error())),
			)
			return
		}

		httpServerTotalGarbageCollections, err = httpServerMeter.Int64Gauge(
			fmt.Sprintf("%s.total_garbage_collection_cycles", httpServerMetricPrefix),
			metric.WithDescription("Total garbage collection cycles"),
			metric.WithUnit("1"))
		if err != nil {
			lgr.Logger.Error(
				"httpServerEnsureMetrics - unable to initialize metrics",
				slog.Any("error", xerrors.New(err.Error())),
			)
			return
		}

		httpServerMetricsInitialized = true
	})
}

// The HTTP server is responsible for running HTTP endpoints
func HTTPServer(canxCtx context.Context, pod string, svcs service.ServicesFactory) error {
	port := os.Getenv("HTTP_PORT")
	if port == "" {
		port = "8080"
	}

	httpServerEnsureMetrics()

	if !httpServerMetricsInitialized {
		return fmt.Errorf("unable to initialize http server %s metrics", pod)
	}

	lgr.Logger.Info(
		"HTTP Server starting...",
		slog.String("pod", pod),
		slog.String("port", port),
	)

	// Create an error stream
	eventStream := make(chan interface{})
	defer close(eventStream)

	// Setup a router
	engine, err := setupRouter(canxCtx, svcs, eventStream)
	if err != nil {
		return err
	}

	// Report profile metrics
	reportHTTPProfiling(canxCtx, svcs, eventStream)

	// Call gin run with cancellable context so we can abort it
	f := cancellableGin(canxCtx, engine, svcs, pod, port, eventStream)
	// This blocks until we return from the gin run
	err = f(canxCtx)
	if err != nil {
		return err
	}

	// Wait in a non-blocking way for `waitOnShutdown` seconds for all the go routines to exit
	// This is needed because the go routines may need to report errors as they are existing
	lgr.Logger.Info(
		"http server is waiting for all go routines to exit",
	)

	// The only way to exit the main function is to wait for the shutdown
	// duration
	timer := time.NewTimer(time.Duration(5) * time.Second)
	defer timer.Stop()

	for {
		select {
		case <-timer.C:
			// Timer expired, proceed with shutdown
			lgr.Logger.Info(
				"http server shutdown waiting period expired. Exiting now",
				slog.Duration("period", time.Duration(5)*time.Second),
			)

			return nil

		case e := <-eventStream:
			procEvent(canxCtx, svcs, e)
		}
	}
}

func setupRouter(canxCtx context.Context, svcs service.ServicesFactory, eventStream chan<- interface{}) (*gin.Engine, error) {
	engine := gin.Default()
	config := cors.DefaultConfig()
	config.AllowAllOrigins = true
	config.AllowCredentials = true
	config.AllowMethods = []string{"OPTIONS", "HEAD", "GET", "POST", "PUT", "PATCH", "OPTIONS", "DELETE"}
	config.AllowHeaders = []string{"Origin", "Content-Length", "Content-Type", "Authorization", "api-key", "Cookie"}
	engine.Use(cors.New(config))

	// Add more middleware
	//....

	// Setup controller to handler routes
	mainRouter := engine.Group(api.ContextRoot)
	ctrlr := api.NewController(svcs, eventStream)
	ctrlr.Routes(mainRouter)

	return engine, nil
}

func reportHTTPProfiling(canxCtx context.Context, svcs service.ServicesFactory, eventStream chan<- interface{}) {
	go func() {
		attrs := []attribute.KeyValue{
			attribute.String("id", utils.GetPodName()),
			attribute.String("name", utils.GetPodName()),
		}

		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-canxCtx.Done():
				lgr.Logger.Info(
					"http server profile reporter context cancelled...exiting!!!",
				)
				return
			case <-ticker.C:
				uptime := time.Since(httpServerStartTime).Seconds()
				httpServerUptimeGauge.Record(canxCtx, int64(uptime), metric.WithAttributes(attrs...))

				var memStats runtime.MemStats
				runtime.ReadMemStats(&memStats)
				httpServerHeapSize.Record(canxCtx, int64(memStats.HeapAlloc/1024/1024), metric.WithAttributes(attrs...))
				httpServerTotalHeapSize.Record(canxCtx, int64(memStats.TotalAlloc/1024/1024), metric.WithAttributes(attrs...))
				httpServerTotalSystemHeapSize.Record(canxCtx, int64(memStats.Sys/1024/1024), metric.WithAttributes(attrs...))
				httpServerTotalGarbageCollections.Record(canxCtx, int64(memStats.NumGC), metric.WithAttributes(attrs...))
			}
		}
	}()
}

func cancellableGin(ctx context.Context, r *gin.Engine, svcs service.ServicesFactory, pod, port string, eventStream chan interface{}) ginWithContext {
	return func(ctx context.Context) error {
		go func() {
			lgr.Logger.Info(
				"http gin server starting...",
				slog.String("pod", pod),
				slog.String("port", port),
			)

			err := r.Run(":" + port)
			if err != nil {
				lgr.Logger.Error(
					"HTTP Server gin start error...exiting",
					slog.Any("error", xerrors.New(err.Error())),
				)

				return
			}
		}()

		// Wait
		for {
			select {
			case <-ctx.Done():
				lgr.Logger.Info("HTTP Server gin context cancelled...exiting!!!")
				return nil
			case e := <-eventStream:
				procEvent(ctx, svcs, e)
			}
		}
	}
}
