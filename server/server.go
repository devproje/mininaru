package server

import (
	"context"
	"crypto/subtle"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/devproje/mininaru/core"
	"github.com/devproje/mininaru/util"
)

type Config struct {
	Host   string
	Port   int
	ApiKey string
}

const (
	DefaultHost = "127.0.0.1"
	DefaultPort = 8080
)

const (
	pathModels      = "/api/v1/models"
	pathCompletions = "/api/v1/chat/completions"
)

const bearerPrefix = "Bearer "

const shutdownTimeout = 5 * time.Second

const (
	readHeaderTimeout        = 5 * time.Second
	readTimeout              = 30 * time.Second
	idleTimeout              = 60 * time.Second
	maxHeaderBytes           = 1 << 20
	maxConcurrentCompletions = 16
)

func bearerToken(r *http.Request) string {
	var header string

	header = r.Header.Get("Authorization")
	if !strings.HasPrefix(header, bearerPrefix) {
		return ""
	}

	return strings.TrimSpace(strings.TrimPrefix(header, bearerPrefix))
}

func authorize(key string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var token string

		token = bearerToken(r)
		if subtle.ConstantTimeCompare([]byte(token), []byte(key)) != 1 {
			requestLogger(r.Context()).Warn("rejected an unauthorized request", "presented_key", token != "")
			writeError(w, http.StatusUnauthorized, "invalid_request_error", "invalid_api_key", "invalid or missing api key")
			return
		}

		next.ServeHTTP(w, r)
	})
}

func routes(key string, reg *core.Registry) http.Handler {
	var mux *http.ServeMux
	var completions http.Handler

	mux = http.NewServeMux()

	mux.HandleFunc(pathModels, func(w http.ResponseWriter, r *http.Request) {
		handleModels(w, r, reg)
	})

	completions = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleCompletions(w, r, reg)
	})
	mux.Handle(pathCompletions, limitConcurrent(maxConcurrentCompletions, completions))

	return logRequests(authorize(key, mux))
}

func limitConcurrent(max int, next http.Handler) http.Handler {
	var slots chan struct{}

	slots = make(chan struct{}, max)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case slots <- struct{}{}:
			defer func() { <-slots }()
			next.ServeHTTP(w, r)
		default:
			requestLogger(r.Context()).Warn("shed a request over the concurrency limit", "limit", max)
			w.Header().Set("Retry-After", "1")
			writeError(w, http.StatusTooManyRequests, "rate_limit_error", "server_busy", "too many concurrent requests")
		}
	})
}

func newHTTPServer(handler http.Handler) *http.Server {
	return &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		IdleTimeout:       idleTimeout,
		MaxHeaderBytes:    maxHeaderBytes,
	}
}

func AnnounceAgents(reg *core.Registry) {
	var model Model

	for _, model = range modelList(reg).Data {
		util.Log.Info("agent available", "agent", model.Id, "provider", model.OwnedBy)
	}
}

func Serve(ctx context.Context, cfg Config, registry *core.Registry) error {
	var address string
	var listener net.Listener
	var notified context.Context
	var stop context.CancelFunc
	var srv *http.Server
	var errs chan error
	var shutdown context.Context
	var cancel context.CancelFunc

	var err error

	if cfg.ApiKey == "" {
		return fmt.Errorf("api key is required to serve")
	}

	if cfg.Host == "" {
		cfg.Host = DefaultHost
	}

	if cfg.Port == 0 {
		cfg.Port = DefaultPort
	}

	if registry == nil {
		return fmt.Errorf("registry is required to serve")
	}

	address = net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port))

	listener, err = net.Listen("tcp", address)
	if err != nil {
		return err
	}

	notified, stop = signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	srv = newHTTPServer(routes(cfg.ApiKey, registry))
	errs = make(chan error, 1)

	util.Log.Info("api server listening",
		"url", "http://"+listener.Addr().String()+"/api/v1",
		"agents", len(registry.List()),
		"max_concurrent_completions", maxConcurrentCompletions)

	AnnounceAgents(registry)

	go func() {
		errs <- srv.Serve(listener)
	}()

	select {
	case err = <-errs:
		if err == http.ErrServerClosed {
			return nil
		}

		util.Log.Error("api server stopped unexpectedly", "error", err)

		return err
	case <-notified.Done():
	}

	util.Log.Info("api server shutting down", "timeout", shutdownTimeout.String())

	shutdown, cancel = context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	err = srv.Shutdown(shutdown)
	if err != nil {
		util.Log.Error("api server shutdown did not finish cleanly", "error", err)
		return err
	}

	util.Log.Info("api server stopped")

	return nil
}
