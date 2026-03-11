package aeneas

import (
	"context"
	"net"
	"os"
	"os/signal"
	"sync"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/nojyerac/aeneas/config"
	"github.com/nojyerac/aeneas/data/db"
	"github.com/nojyerac/aeneas/domain"
	"github.com/nojyerac/aeneas/engine"
	"github.com/nojyerac/aeneas/runner"
	"github.com/nojyerac/aeneas/runner/local"
	"github.com/nojyerac/aeneas/service"
	"github.com/nojyerac/aeneas/transport/http"
	"github.com/nojyerac/aeneas/transport/rpc"
	libdb "github.com/nojyerac/go-lib/db"
	"github.com/nojyerac/go-lib/health"
	"github.com/nojyerac/go-lib/log"
	"github.com/nojyerac/go-lib/metrics"
	"github.com/nojyerac/go-lib/tracing"
	"github.com/nojyerac/go-lib/transport"
	libgrpc "github.com/nojyerac/go-lib/transport/grpc"
	libhttp "github.com/nojyerac/go-lib/transport/http"
	"github.com/nojyerac/go-lib/version"
)

func main() { //nolint:unused,funlen // main is the entry point for the service.
	// init & config
	version.SetSemVer("0.0.0")
	version.SetServiceName("aeneas")
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	if err := config.InitAndValidate(); err != nil {
		panic(err)
	}

	// telemetry
	logger := log.NewLogger(config.LogConfig)
	ctx = log.WithLogger(ctx, logger)
	tracing.NewTracerProvider(config.TraceConfig)
	mp, metricHandler, err := metrics.NewMetricProvider()
	if err != nil {
		logger.WithError(err).Panic("failed to create metric provider")
	}
	metrics.SetGlobal(mp)
	hc := health.NewChecker(config.HealthConfig)

	// database
	database := libdb.NewDatabase(
		config.DBConfig,
		libdb.WithHealthChecker(hc),
		libdb.WithLogger(logger),
	)

	// repositories
	workflowRepo := db.NewWorkflowRepository(database, db.WithLogger(logger))
	executionRepo := db.NewExecutionRepository(database, db.WithLogger(logger))
	stepExecutionRepo := db.NewStepExecutionRepository(database, db.WithLogger(logger))

	// services
	workflowSvc := service.NewWorkflowService(workflowRepo)
	executionSvc := service.NewExecutionService(workflowRepo, executionRepo, stepExecutionRepo)

	// engine (workflow execution orchestrator)
	// Type assert logger to *logrus.Logger for runner and engine initialization
	logrusLogger, ok := logger.(*logrus.Logger)
	if !ok {
		logger.Panic("logger is not *logrus.Logger")
	}
	runnr, err := initializeRunner(logrusLogger)
	if err != nil {
		logger.WithError(err).Panic("failed to initialize runner")
	}
	eng := initializeEngine(workflowRepo, executionRepo, stepExecutionRepo, runnr, logrusLogger)

	// transports
	hSrv := libhttp.NewServer(
		config.HTTPConfig,
		libhttp.WithMetricsHandler(metricHandler),
		libhttp.WithHealthChecker(hc),
	)
	http.RegisterRoutes(hSrv, workflowSvc, executionSvc)

	reg := rpc.RegisterServices(rpc.WithLogger(logger))
	gSrv := libgrpc.NewServer(reg)

	srv, err := transport.NewServer(
		config.TransConfig,
		transport.WithHTTP(hSrv),
		transport.WithGRPC(gSrv),
	)
	if err != nil {
		logger.WithError(err).Panic("failed to create server")
	}

	// start service
	wg := &sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := database.Open(ctx); err != nil {
			logger.WithError(err).Panic("database error")
		}
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := srv.Start(ctx); err != nil && err != net.ErrClosed {
			logger.WithError(err).Panic("server error")
		}
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := hc.Start(ctx); err != nil {
			logger.WithError(err).Panic("health checker error")
		}
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := eng.Start(ctx); err != nil {
			logger.WithError(err).Panic("engine error")
		}
	}()

	logger.Info("Service starting")
	<-ctx.Done()
	logger.Info("Service stopping")

	// Graceful shutdown: stop engine before closing database
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := eng.Stop(shutdownCtx); err != nil {
		logger.WithError(err).Error("Engine shutdown error")
	}

	wg.Wait()
	logger.Info("Service stopped")
}

// initializeRunner creates a runner based on configuration
// Currently defaults to LocalRunner; can be extended to support K8sRunner based on config
//
//nolint:unused // Used by main
func initializeRunner(logger *logrus.Logger) (runner.Runner, error) {
	// TODO: Add config option to choose between LocalRunner and K8sRunner
	// For now, default to LocalRunner
	return local.NewLocalRunner(logger)
}

// initializeEngine creates and configures the workflow execution engine
//
//nolint:unused // Used by main
func initializeEngine(
	workflowRepo domain.WorkflowRepository,
	executionRepo domain.ExecutionRepository,
	stepExecutionRepo domain.StepExecutionRepository,
	runnr runner.Runner,
	logger *logrus.Logger,
) *engine.Engine {
	cfg := engine.Config{
		PollInterval: 2 * time.Second, // TODO: Make configurable
	}
	return engine.NewEngine(workflowRepo, executionRepo, stepExecutionRepo, runnr, logger, cfg)
}
