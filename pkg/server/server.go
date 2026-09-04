// Package server is Oshimai's control plane: the REST/SSE API that the dashboard, the CLI
// (cmd/cli), CI gates, and every other client all speak against identically (see the README's
// architecture diagram). Its central pieces are TestCoordinator (coordinator.go), which owns a
// run's whole lifecycle — resolving/validating its scenario, running pkg/loadengine against it,
// scheduling pkg/chaos injection, enforcing pkg/verify target-ownership checks and guarded-
// production approval, and persisting results — and APIHandler (handler.go), which exposes that
// (plus everything else: presets, benchmarking, GameDay scheduling, multi-region agent fan-out,
// public run sharing) as ~65 HTTP routes. EventBus (eventbus.go) fans a run's live telemetry out
// to SSE subscribers, and StorageRepository (repository.go, sqlite_repository.go) is the
// swappable persistence layer behind it all — in-memory by default in tests, SQLite by default at
// runtime so run history survives a restart.
package server

import (
	"log"
	"net/http"
	"runtime/debug"
	"time"
)

// ServerConfig configures the HTTP control plane server.
type ServerConfig struct {
	Address      string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}

// NewServer configures and returns an http.Server ready to listen.
func NewServer(cfg ServerConfig, handler *APIHandler) *http.Server {
	addr := cfg.Address
	if addr == "" {
		addr = ":8080"
	}

	readTimeout := cfg.ReadTimeout
	if readTimeout <= 0 {
		readTimeout = 30 * time.Second
	}

	writeTimeout := cfg.WriteTimeout
	if writeTimeout <= 0 {
		// Note: WriteTimeout should be 0 or long for streaming endpoints (SSE)
		writeTimeout = 0
	}

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	// Wrap with recovery & CORS middleware
	rootHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		defer func() {
			if rec := recover(); rec != nil {
				// Without this, every panic in request handling (nil-pointer bugs, bad type
				// assertions, etc.) becomes an indistinguishable generic 500 with no forensic
				// trail — logged here via the same log/logBuffer path cmd/server wires up, so it
				// shows up both in stderr and in the dashboard's live backend-log stream.
				log.Printf("[Oshimai] panic recovered in %s %s: %v\n%s", r.Method, r.URL.Path, rec, debug.Stack())
				writeJSONError(w, http.StatusInternalServerError, "internal server error")
			}
		}()

		mux.ServeHTTP(w, r)
	})

	return &http.Server{
		Addr:         addr,
		Handler:      rootHandler,
		ReadTimeout:  readTimeout,
		WriteTimeout: writeTimeout,
	}
}
