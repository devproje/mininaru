package server

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/devproje/mininaru/util"
	"github.com/google/uuid"
)

type recorder struct {
	http.ResponseWriter

	status  int
	written int64
}

type requestKey struct{}

const requestIdHeader = "X-Request-Id"

func (r *recorder) WriteHeader(status int) {
	if r.status == 0 {
		r.status = status
	}

	r.ResponseWriter.WriteHeader(status)
}

func (r *recorder) Write(buf []byte) (int, error) {
	var written int

	var err error

	if r.status == 0 {
		r.status = http.StatusOK
	}

	written, err = r.ResponseWriter.Write(buf)
	r.written += int64(written)

	return written, err
}

func (r *recorder) Flush() {
	var flusher http.Flusher
	var ok bool

	flusher, ok = r.ResponseWriter.(http.Flusher)
	if !ok {
		return
	}

	if r.status == 0 {
		r.status = http.StatusOK
	}

	flusher.Flush()
}

func requestLogger(ctx context.Context) *slog.Logger {
	var found *slog.Logger
	var ok bool

	found, ok = ctx.Value(requestKey{}).(*slog.Logger)
	if !ok {
		return util.Log
	}

	return found
}

func requestId(r *http.Request) string {
	var given string

	given = r.Header.Get(requestIdHeader)
	if given != "" && len(given) <= 128 {
		return given
	}

	return uuid.NewString()
}

func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var id string
		var logger *slog.Logger
		var tracked *recorder
		var started time.Time
		var elapsed time.Duration

		id = requestId(r)
		logger = util.Log.With("request_id", id, "method", r.Method, "path", r.URL.Path)

		w.Header().Set(requestIdHeader, id)

		tracked = &recorder{ResponseWriter: w}
		started = time.Now()

		logger.Debug("request started", "remote", r.RemoteAddr, "user_agent", r.UserAgent())

		next.ServeHTTP(tracked, r.WithContext(context.WithValue(r.Context(), requestKey{}, logger)))

		elapsed = time.Since(started)
		if tracked.status == 0 {
			tracked.status = http.StatusOK
		}

		if tracked.status >= http.StatusInternalServerError {
			logger.Error("request failed", "status", tracked.status, "bytes", tracked.written, "duration_ms", elapsed.Milliseconds())
			return
		}

		if tracked.status >= http.StatusBadRequest {
			logger.Warn("request rejected", "status", tracked.status, "bytes", tracked.written, "duration_ms", elapsed.Milliseconds())
			return
		}

		logger.Info("request completed", "status", tracked.status, "bytes", tracked.written, "duration_ms", elapsed.Milliseconds())
	})
}
