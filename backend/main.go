package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"os"
	"os/signal"
	"runtime"
	"sync"
	"syscall"
	"time"

	"go.opentelemetry.io/contrib/exporters/autoexport"
	"go.opentelemetry.io/contrib/propagators/autoprop"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	noopmeter "go.opentelemetry.io/otel/metric/noop"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/trace"
	nooptrace "go.opentelemetry.io/otel/trace/noop"
	"golang.org/x/xerrors"

	"go.opentelemetry.io/otel/metric"

	"github.com/khaledhikmat/psap/mode"
	"github.com/khaledhikmat/psap/service"
	"github.com/khaledhikmat/psap/service/config"
	"github.com/khaledhikmat/psap/service/data"
	"github.com/khaledhikmat/psap/service/lgr"
	"github.com/khaledhikmat/psap/utils"
	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/process"
)

const (
	// WARNING: this has to be bigger that the mode processor shutdown time
	waitOnShutdown   = 8 * time.Second
	profilerDuration = 10 * time.Second
	mainMetricPrefix = "grp.esg.soi.td.sp.main"
)

var (
	metricsOnce        sync.Once
	metricsInitialized bool

	mainmeter = otel.Meter(mainMetricPrefix)

	uptimeGauge             metric.Int64Gauge
	heapSize                metric.Int64Gauge
	totalHeapSize           metric.Int64Gauge
	totalSystemHeapSize     metric.Int64Gauge
	totalGarbageCollections metric.Int64Gauge
	cpuUsage                metric.Float64Gauge
	memUsage                metric.Float64Gauge
	processCount            metric.Int64Gauge
	goRoutineCount          metric.Int64Gauge
)

// Available mode processors
var modeProcessors = map[string]mode.Processor{
	"httpserver":      mode.HTTPServer,
	"flowworker":      mode.Worker,
	"multiflowworker": mode.MultiWorker,
	"cli":             mode.CLI,
}

var startTime = time.Now()

var otelAttrs = []attribute.KeyValue{
	attribute.String("instanceType", utils.GetInstanceType()),
}

// Initialize metrics lazingly
func ensureMetrics() {
	metricsOnce.Do(func() {

		_, _ = time.LoadLocation("America/Chicago")

		var err error
		uptimeGauge, err = mainmeter.Int64Gauge(
			fmt.Sprintf("%s.uptime", mainMetricPrefix),
			metric.WithDescription("Uptime in seconds"),
			metric.WithUnit("s"))
		if err != nil {
			lgr.Logger.Error(
				"main - unable to initialize metrics",
				slog.Any("error", xerrors.New(err.Error())),
			)
			return
		}

		heapSize, err = mainmeter.Int64Gauge(
			fmt.Sprintf("%s.heap_size", mainMetricPrefix),
			metric.WithDescription("Heap size in MB"),
			metric.WithUnit("MiBy"))
		if err != nil {
			lgr.Logger.Error(
				"main - unable to initialize metrics",
				slog.Any("error", xerrors.New(err.Error())),
			)
			return
		}

		totalHeapSize, err = mainmeter.Int64Gauge(
			fmt.Sprintf("%s.total_heap_size", mainMetricPrefix),
			metric.WithDescription("Total heap size in MB"),
			metric.WithUnit("MiBy"))
		if err != nil {
			lgr.Logger.Error(
				"main - unable to initialize metrics",
				slog.Any("error", xerrors.New(err.Error())),
			)
			return
		}

		totalSystemHeapSize, err = mainmeter.Int64Gauge(
			fmt.Sprintf("%s.total_sys_heap_size", mainMetricPrefix),
			metric.WithDescription("Total system heap size in MB"),
			metric.WithUnit("MiBy"))
		if err != nil {
			lgr.Logger.Error(
				"main - unable to initialize metrics",
				slog.Any("error", xerrors.New(err.Error())),
			)
			return
		}

		totalGarbageCollections, err = mainmeter.Int64Gauge(
			fmt.Sprintf("%s.total_garbage_collection_cycles", mainMetricPrefix),
			metric.WithDescription("Total number of garbage collection cycles"),
			metric.WithUnit("1"))
		if err != nil {
			lgr.Logger.Error(
				"main - unable to initialize metrics",
				slog.Any("error", xerrors.New(err.Error())),
			)
			return
		}

		cpuUsage, err = mainmeter.Float64Gauge(
			fmt.Sprintf("%s.system_cpu_usage", mainMetricPrefix),
			metric.WithDescription("CPU usage"),
			metric.WithUnit("%"))
		if err != nil {
			lgr.Logger.Error(
				"main - unable to initialize metrics",
				slog.Any("error", xerrors.New(err.Error())),
			)
			return
		}

		memUsage, err = mainmeter.Float64Gauge(
			fmt.Sprintf("%s.system_mem_usage", mainMetricPrefix),
			metric.WithDescription("MEM usage"),
			metric.WithUnit("%"))
		if err != nil {
			lgr.Logger.Error(
				"main - unable to initialize metrics",
				slog.Any("error", xerrors.New(err.Error())),
			)
			return
		}

		processCount, err = mainmeter.Int64Gauge(
			fmt.Sprintf("%s.system_process_count", mainMetricPrefix),
			metric.WithDescription("Number of processes"),
			metric.WithUnit("1"))
		if err != nil {
			lgr.Logger.Error(
				"main - unable to initialize metrics",
				slog.Any("error", xerrors.New(err.Error())),
			)
			return
		}

		goRoutineCount, err = mainmeter.Int64Gauge(
			fmt.Sprintf("%s.concurrent_routine_count", mainMetricPrefix),
			metric.WithDescription("Number of go routines"),
			metric.WithUnit("1"))
		if err != nil {
			lgr.Logger.Error(
				"main - unable to initialize metrics",
				slog.Any("error", xerrors.New(err.Error())),
			)
			return
		}

		metricsInitialized = true
	})
}

func main() {
	ensureMetrics()

	if !metricsInitialized {
		lgr.Logger.Error(
			"unable to initialize metrics",
			slog.Any("error", xerrors.New("no metrics")),
		)
		return
	}

	rootCtx := context.Background()
	canxCtx, canxFn := context.WithCancel(rootCtx)

	// Hook up a signal handler to cancel the context
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		lgr.Logger.Info(
			"received kill signal",
			slog.Any("signal", sig),
		)
		canxFn()
	}()

	// Setup OpenTelemetry
	shutdown, err := setupOpenTelemetry(rootCtx)
	if err != nil {
		lgr.Logger.Error(
			"setting up OpenTelemetry",
			slog.Any("error", xerrors.New(err.Error())),
		)
		return
	}
	defer func() {
		err := shutdown(rootCtx)
		if err != nil {
			lgr.Logger.Error(
				"shutting down OpenTelemetry",
				slog.Any("error", xerrors.New(err.Error())),
			)
		}
	}()

	// Create a prfiler reporter
	reportProfiling(canxCtx)

	modeType := "manager"
	if os.Getenv("MODE_PROC") != "" {
		modeType = os.Getenv("MODE_PROC")
	}

	modeProc, ok := modeProcessors[modeType]
	if !ok {
		lgr.Logger.Error("invalid mode", slog.String("mode", modeType))
		return
	}

	// Create the services needed for the mode processor
	// They can be overridden by the mode processor with different implementations
	cfgSvc := config.NewEnvVars()
	if cfgSvc == nil {
		lgr.Logger.Error(
			"unable to instantiate config service",
			slog.Any("error", xerrors.New(err.Error())),
		)
		return
	}
	// data service
	dataSvc := data.NewDynamoDB(canxCtx, cfgSvc)
	if dataSvc == nil {
		lgr.Logger.Error(
			"unable to instantiate data service",
			slog.Any("error", xerrors.New(err.Error())),
		)
		return
	}

	svcs := service.ServicesFactory{
		CanxCtx: canxCtx,
		CfgSvc:  cfgSvc,
		DataSvc: dataSvc,
	}

	// Run Timestamp as POD
	ts := fmt.Sprintf("%d", time.Now().UnixMilli())

	// Create mode processor result
	modeProcResult := make(chan error)
	defer close(modeProcResult)

	// Start the mode processor
	go func() {
		modeProcResult <- modeProc(canxCtx, ts, svcs)
	}()

	// Wait for cancellation, mode proc, stats or error
	for {
		select {
		case <-canxCtx.Done():
			lgr.Logger.Info(
				"stream processor pod context cancelled",
			)
			goto resume

		case err := <-modeProcResult:
			if err != nil {
				lgr.Logger.Info(
					"stream processor pod mode processor exited",
					slog.Any("error", xerrors.New(err.Error())),
				)
			}
			goto resume
		}
	}

	// Wait in a non-blocking way for `waitOnShutdown` for all the go routines to exit
	// This is needed because the go routines may need to report errors as they are existing
resume:
	// Cancel the context if not already cancelled
	if canxCtx.Err() == nil {
		// Force cancel the context
		canxFn()
	}

	lgr.Logger.Info(
		"stream processor pod is waiting for all go routines to exit",
	)

	// The only way to exit the main function is to wait for the shutdown
	// duration
	timer := time.NewTimer(waitOnShutdown)
	defer timer.Stop()

	for {
		select {
		case <-timer.C:
			// Timer expired, proceed with shutdown
			lgr.Logger.Info(
				"stream processor pod shutdown waiting period expired. Exiting now",
				slog.Duration("period", waitOnShutdown),
			)

			return

		case err := <-modeProcResult:
			if err != nil {
				lgr.Logger.Info(
					"stream processor pod mode processor exited",
					slog.Any("error", xerrors.New(err.Error())),
				)
			}
		}
	}
}

// Reference:
// https://cloud.google.com/stackdriver/docs/instrumentation/setup/go
// setupOpenTelemetry sets up the OpenTelemetry SDK and exporters for metrics and
// traces. If it does not return an error, call shutdown for proper cleanup.
func setupOpenTelemetry(ctx context.Context) (shutdown func(context.Context) error, err error) {
	if os.Getenv("ENABLE_TELEMETRY") != "true" {
		// Set Noop Tracer Provider
		otel.SetTracerProvider(nooptrace.NewTracerProvider())

		// Set Noop Meter Provider
		otel.SetMeterProvider(noopmeter.NewMeterProvider())

		// Return a no-op shutdown function
		return func(_ context.Context) error {
			return nil
		}, nil
	}

	var shutdownFuncs []func(context.Context) error

	// shutdown combines shutdown functions from multiple OpenTelemetry
	// components into a single function.
	shutdown = func(ctx context.Context) error {
		var err error
		for _, fn := range shutdownFuncs {
			err = errors.Join(err, fn(ctx))
		}
		shutdownFuncs = nil
		return err
	}

	// Configure Context Propagation to use the default W3C traceparent format
	otel.SetTextMapPropagator(autoprop.NewTextMapPropagator())

	// Configure Trace Export to send spans as OTLP
	texporter, err := autoexport.NewSpanExporter(ctx)
	if err != nil {
		err = errors.Join(err, shutdown(ctx))
		return
	}
	tp := trace.NewTracerProvider(trace.WithBatcher(texporter))
	shutdownFuncs = append(shutdownFuncs, tp.Shutdown)
	otel.SetTracerProvider(tp)

	// Configure Metric Export to send metrics as OTLP
	mreader, err := autoexport.NewMetricReader(ctx)
	if err != nil {
		err = errors.Join(err, shutdown(ctx))
		return
	}
	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(mreader),
	)
	shutdownFuncs = append(shutdownFuncs, mp.Shutdown)
	otel.SetMeterProvider(mp)

	return shutdown, nil
}

func reportProfiling(ctx context.Context) {
	go func() {
		ticket := time.NewTicker(profilerDuration)
		defer ticket.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticket.C:
				uptime := time.Since(startTime).Seconds()
				uptimeGauge.Record(ctx, int64(uptime), metric.WithAttributes(otelAttrs...))

				var memStats runtime.MemStats
				runtime.ReadMemStats(&memStats)

				heapSize.Record(ctx, bToMB(memStats.HeapAlloc), metric.WithAttributes(otelAttrs...))
				totalHeapSize.Record(ctx, bToMB(memStats.TotalAlloc), metric.WithAttributes(otelAttrs...))
				totalSystemHeapSize.Record(ctx, bToMB(memStats.Sys), metric.WithAttributes(otelAttrs...))
				totalGarbageCollections.Record(ctx, int64(memStats.NumGC), metric.WithAttributes(otelAttrs...))

				sysStats, err := getSystemStats(profilerDuration)
				if err != nil {
					lgr.Logger.Error(
						"failed to get system stats",
						slog.Any("error", xerrors.New(err.Error())),
					)
					continue
				}
				cpuUsage.Record(ctx, sysStats.CPUPercent, metric.WithAttributes(otelAttrs...))
				memUsage.Record(ctx, sysStats.MemoryPercent, metric.WithAttributes(otelAttrs...))
				processCount.Record(ctx, int64(sysStats.ProcessCount), metric.WithAttributes(otelAttrs...))
				goRoutineCount.Record(ctx, int64(runtime.NumGoroutine()), metric.WithAttributes(otelAttrs...))
			}
		}
	}()
}

func bToMB(b uint64) int64 {
	cb := b / 1024 / 1024
	if cb > uint64(math.MaxInt64) {
		return math.MaxInt64
	}

	return int64(cb)
}

// SystemStats holds system monitoring data
type systemStats struct {
	CPUPercent    float64
	MemoryPercent float64
	ProcessCount  int
	Timestamp     time.Time
}

// getSystemStats collects all system statistics
func getSystemStats(duration time.Duration) (*systemStats, error) {
	stats := &systemStats{
		Timestamp: time.Now(),
	}

	// Get CPU usage percentage
	cpuPercent, err := cpu.Percent(duration, false)
	if err != nil {
		return nil, fmt.Errorf("failed to get CPU usage: %w", err)
	}
	if len(cpuPercent) > 0 {
		stats.CPUPercent = cpuPercent[0]
	}

	// Get memory usage percentage
	memInfo, err := mem.VirtualMemory()
	if err != nil {
		return nil, fmt.Errorf("failed to get memory usage: %w", err)
	}
	stats.MemoryPercent = memInfo.UsedPercent

	// Get number of running processes (PIDs)
	processes, err := process.Processes()
	if err != nil {
		return nil, fmt.Errorf("failed to get process count: %w", err)
	}
	stats.ProcessCount = len(processes)

	return stats, nil
}
