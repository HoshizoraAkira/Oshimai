package server

import (
	"net/http"
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
