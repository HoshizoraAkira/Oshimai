package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/oshimai/twin/pkg/chaos"
	"github.com/oshimai/twin/pkg/dotenv"
	"github.com/oshimai/twin/pkg/generator"
	"github.com/oshimai/twin/pkg/server"
)

func main() {
	logBuf := server.GetDefaultLogBuffer()
	log.SetOutput(io.MultiWriter(os.Stderr, logBuf))

	// Auto-load .env from the working directory for development convenience.
	// Real environment variables always take precedence over .env values.
	// A missing .env file is silently ignored — safe for production.
	if err := dotenv.Load(".env"); err != nil {
		log.Printf("[Oshimai] Warning: failed to load .env: %v", err)
	}

	addr := flag.String("addr", ":8080", "Server listen address")
	maxRuns := flag.Int("max-runs", 2, "Maximum concurrent load test runs")
	enforceVerification := flag.Bool("enforce-target-verification", true, "Require DNS/well-known ownership proof before testing a public target (private/local targets are always exempt)")
	requireProdApproval := flag.Bool("require-production-approval", true, "Hold runs labeled environment=production for a second operator's approval before executing")
	chaosDriverMode := flag.String("chaos-driver", "mock", "Fault injection driver: mock (safe default, no side effects) | http_proxy (no-root userspace proxy, any OS) | netem (Linux + NET_ADMIN, most realistic) | resource_stress (CPU/memory/disk pressure on this host)")
	flag.Parse()

	log.Printf("[Oshimai] Initializing Control Plane Server on %s (Max Concurrent Runs: %d)...", *addr, *maxRuns)

	repo := server.NewMemoryRepository()
	eb := server.NewEventBus()
	defer eb.Close()

	underlyingDriver, err := chaos.NewDriverByMode(*chaosDriverMode)
	if err != nil {
		log.Fatalf("[Oshimai] Failed to initialize chaos driver %q: %v", *chaosDriverMode, err)
	}
	chaosDriver := chaos.NewManagedChaosDriver(underlyingDriver)
	defer chaosDriver.Close()
	log.Printf("[Oshimai] Chaos fault-injection driver: %s", *chaosDriverMode)

	coordinatorCfg := server.CoordinatorConfig{
		MaxConcurrentRuns:            *maxRuns,
		ChaosDriver:                  chaosDriver,
		EnforceTargetVerification:    *enforceVerification,
		RequireApprovalForProduction: *requireProdApproval,
	}
	// The HTTP chaos proxy is also the transport Virtual Users must send traffic through for its
	// fault injection to have any effect — wire that up only when this driver mode is active.
	if proxyDriver, ok := underlyingDriver.(*chaos.HTTPProxyDriver); ok {
		coordinatorCfg.HTTPClientFactory = proxyDriver.HTTPClient
		log.Printf("[Oshimai] HTTP chaos proxy listening at %s — all run traffic will route through it.", proxyDriver.ProxyURL())
	}

	coordinator := server.NewTestCoordinator(repo, eb, coordinatorCfg)
	if *enforceVerification {
		log.Printf("[Oshimai] Target-ownership verification ENFORCED — public targets must complete DNS TXT / well-known challenge before testing.")
	}
	if *requireProdApproval {
		log.Printf("[Oshimai] Guarded-production mode ENABLED — environment=production runs require a second operator's approval.")
	}

	handler := server.NewAPIHandler(repo, coordinator, eb)
	if llmClient := generator.NewBynamaClientFromEnv(os.Getenv); llmClient != nil {
		handler.SetLLMClient(llmClient)
		log.Printf("[Oshimai] Natural-language scenario generation ENABLED via Bynara AI (model: %s).", os.Getenv("BYNARA_MODEL"))
	} else {
		log.Printf("[Oshimai] Natural-language scenario generation disabled — set BYNARA_API_KEY to enable it.")
	}

	schedulerCtx, stopScheduler := context.WithCancel(context.Background())
	defer stopScheduler()
	handler.StartGameDayScheduler(schedulerCtx)

	httpServer := server.NewServer(server.ServerConfig{
		Address:      *addr,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 0, // Streaming SSE
	}, handler)

	// Graceful shutdown listener
	stopChan := make(chan os.Signal, 1)
	signal.Notify(stopChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		log.Printf("[Oshimai] Control Plane REST & SSE API listening on %s", *addr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("[Oshimai] Server error: %v", err)
		}
	}()

	<-stopChan
	log.Println("[Oshimai] Shutdown signal received, draining active runs...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(ctx); err != nil {
		log.Printf("[Oshimai] Graceful shutdown error: %v", err)
	}

	fmt.Println("[Oshimai] Server cleanly stopped.")
}
