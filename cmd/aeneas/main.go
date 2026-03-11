package aeneas

import (
	"context"
	"net"
	"os"
	"os/signal"
	"sync"

	"github.com/nojyerac/aeneas/config"
	"github.com/nojyerac/aeneas/data/db"
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

func main() { //nolint:unused // main is the entry point for the service.
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

	logger.Info("Service starting")
	<-ctx.Done()
	logger.Info("Service stopping")
	wg.Wait()
	logger.Info("Service stopped")
}
