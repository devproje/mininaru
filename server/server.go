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
			writeError(w, http.StatusUnauthorized, "invalid_request_error", "invalid_api_key", "invalid or missing api key")
			return
		}

		next.ServeHTTP(w, r)
	})
}

func routes(key string, reg *core.Registry) http.Handler {
	var mux *http.ServeMux

	mux = http.NewServeMux()

	mux.HandleFunc(pathModels, func(w http.ResponseWriter, r *http.Request) {
		handleModels(w, r, reg)
	})

	mux.HandleFunc(pathCompletions, func(w http.ResponseWriter, r *http.Request) {
		handleCompletions(w, r, reg)
	})

	return authorize(key, mux)
}

func AnnounceAgents(reg *core.Registry) {
	var model Model

	for _, model = range modelList(reg).Data {
		fmt.Printf("  agent %s (provider %s)\n", model.Id, model.OwnedBy)
	}
}

func announce(addr net.Addr, reg *core.Registry) {
	fmt.Printf("mininaru api listening on http://%s/api/v1\n", addr)

	AnnounceAgents(reg)
}

func Serve(ctx context.Context, cfg Config, registry *core.Registry) error {
	var address string
	var listener net.Listener
	var srv *http.Server
	var notified context.Context
	var stop context.CancelFunc
	var shutdown context.Context
	var cancel context.CancelFunc
	var errs chan error

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

	srv = &http.Server{Handler: routes(cfg.ApiKey, registry)}
	errs = make(chan error, 1)

	announce(listener.Addr(), registry)

	go func() {
		errs <- srv.Serve(listener)
	}()

	select {
	case err = <-errs:
		if err == http.ErrServerClosed {
			return nil
		}

		return err
	case <-notified.Done():
	}

	shutdown, cancel = context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	return srv.Shutdown(shutdown)
}
