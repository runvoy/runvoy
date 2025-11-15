// Package main implements the local development server for runvoy.
// It runs both the orchestrator and async event processor services locally for testing and development.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"runvoy/cmd/local/server"
	"runvoy/internal/app"
	"runvoy/internal/app/processor"
	"runvoy/internal/config"
	"runvoy/internal/constants"
	"runvoy/internal/logger"
	serverPkg "runvoy/internal/server"
)

const numServers = 2

func initializeServices(ctx context.Context, log *slog.Logger, oCfg *config.Config, eCfg *config.Config,
) (*app.Service, processor.Processor, error) {
	svc, err := app.Initialize(ctx, oCfg, log)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to initialize orchestrator service: %w", err)
	}

	proc, err := processor.Initialize(ctx, eCfg, log)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to initialize event processor: %w", err)
	}

	return svc, proc, nil
}

func startOrchestratorServer(log *slog.Logger, cfg *config.Config, svc *app.Service,
	serverErrors chan error, wg *sync.WaitGroup) *http.Server {
	wg.Go(func() {
		log.Info("starting orchestrator server",
			"port", cfg.Port,
			"version", *constants.GetVersion(),
			"log_level", cfg.LogLevel,
			"request_timeout", cfg.RequestTimeout,
		)
		log.Debug("orchestrator health check available",
			"url", fmt.Sprintf("http://localhost:%d/api/v1/health", cfg.Port),
		)

		router := serverPkg.NewRouter(svc, cfg.RequestTimeout)
		srv := &http.Server{
			Addr:         fmt.Sprintf(":%d", cfg.Port),
			Handler:      router.Handler(),
			ReadTimeout:  constants.ServerReadTimeout,
			WriteTimeout: constants.ServerWriteTimeout,
			IdleTimeout:  constants.ServerIdleTimeout,
		}
		if serveErr := srv.ListenAndServe(); serveErr != nil && serveErr != http.ErrServerClosed {
			serverErrors <- fmt.Errorf("orchestrator server failed: %w", serveErr)
		}
	})

	router := serverPkg.NewRouter(svc, cfg.RequestTimeout)
	return &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Port),
		Handler:      router.Handler(),
		ReadTimeout:  constants.ServerReadTimeout,
		WriteTimeout: constants.ServerWriteTimeout,
		IdleTimeout:  constants.ServerIdleTimeout,
	}
}

func startAsyncProcessorServer(log *slog.Logger, cfg *config.Config, proc processor.Processor,
	serverErrors chan error, wg *sync.WaitGroup) *http.Server {
	wg.Go(func() {
		port := cfg.Port + 1
		log.Info("starting async processor server",
			"port", port,
			"version", *constants.GetVersion(),
			"log_level", cfg.LogLevel,
		)
		log.Debug("async processor endpoint available",
			"url", fmt.Sprintf("http://localhost:%d/process", port),
		)

		router := server.NewRouter(proc, log)
		srv := &http.Server{
			Addr:         fmt.Sprintf(":%d", port),
			Handler:      router,
			ReadTimeout:  constants.ServerReadTimeout,
			WriteTimeout: constants.ServerWriteTimeout,
			IdleTimeout:  constants.ServerIdleTimeout,
		}
		if serveErr := srv.ListenAndServe(); serveErr != nil && serveErr != http.ErrServerClosed {
			serverErrors <- fmt.Errorf("async processor server failed: %w", serveErr)
		}
	})

	router := server.NewRouter(proc, log)
	return &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Port),
		Handler:      router,
		ReadTimeout:  constants.ServerReadTimeout,
		WriteTimeout: constants.ServerWriteTimeout,
		IdleTimeout:  constants.ServerIdleTimeout,
	}
}

func shutdownServers(log *slog.Logger, orchestratorServer, asyncServer *http.Server) bool {
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), constants.ServerShutdownTimeout)
	defer shutdownCancel()

	var shutdownErrors []error
	var mu sync.Mutex

	// Shutdown orchestrator server
	go func() {
		if shutdownErr := orchestratorServer.Shutdown(shutdownCtx); shutdownErr != nil {
			mu.Lock()
			shutdownErrors = append(shutdownErrors, fmt.Errorf("orchestrator shutdown error: %w", shutdownErr))
			mu.Unlock()
		}
	}()

	// Shutdown async processor server
	go func() {
		if shutdownErr := asyncServer.Shutdown(shutdownCtx); shutdownErr != nil {
			mu.Lock()
			shutdownErrors = append(shutdownErrors, fmt.Errorf("async processor shutdown error: %w", shutdownErr))
			mu.Unlock()
		}
	}()

	// Wait for both shutdowns to complete
	<-shutdownCtx.Done()

	if len(shutdownErrors) > 0 {
		for _, err := range shutdownErrors {
			log.Error("shutdown error", "error", err)
		}
		return false
	}

	log.Info("servers shutdown complete")
	return true
}

func main() {
	// Load configurations for both services
	orchestratorCfg := config.MustLoadOrchestrator()
	eventProcessorCfg := config.MustLoadEventProcessor()

	log := logger.Initialize(constants.Development, orchestratorCfg.GetLogLevel())

	ctx, cancel := context.WithTimeout(context.Background(), orchestratorCfg.InitTimeout)
	svc, proc, initErr := initializeServices(ctx, log, orchestratorCfg, eventProcessorCfg)
	cancel()
	if initErr != nil {
		log.Error("initialization failed", "error", initErr)
		os.Exit(1)
	}

	// Start both servers
	serverErrors := make(chan error, numServers)
	var wg sync.WaitGroup

	orchestratorServer := startOrchestratorServer(log, orchestratorCfg, svc, serverErrors, &wg)
	asyncServer := startAsyncProcessorServer(log, eventProcessorCfg, proc, serverErrors, &wg)

	// Wait for interrupt signal or server error
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case runErr := <-serverErrors:
		log.Error("server error", "error", runErr)
		os.Exit(1)
	case <-quit:
		log.Info("shutting down servers...")
	}

	// Gracefully shutdown both servers
	if !shutdownServers(log, orchestratorServer, asyncServer) {
		os.Exit(1)
	}
}
