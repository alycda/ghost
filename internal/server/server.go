// Package server implements the Ghost API for a single Postgres cluster.
//
// The hosted service gave every Ghost database an instance of its own. This
// server keeps them all in one cluster instead: a Ghost database is a Postgres
// database named after its ID, a fork is CREATE DATABASE ... TEMPLATE, pausing
// is ALLOW_CONNECTIONS false, and the one role the CLI knows, tsdbadmin, owns
// them all with one shared password. Bookkeeping lives in the "ghost" schema of
// the maintenance database. There is one space, one API key and no billing.
package server

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/timescale/ghost/internal/api"
)

// Server implements api.StrictServerInterface.
type Server struct {
	cfg  Config
	pool *pgxpool.Pool
	log  *slog.Logger

	// createdAt is when the ghost schema was first bootstrapped; the API key is
	// reported as created then.
	createdAt time.Time
	// timescale is whether the cluster can CREATE EXTENSION timescaledb.
	timescale bool
}

// New connects to the cluster and makes sure the role, schema and settings the
// server relies on exist.
func New(ctx context.Context, cfg Config, logger *slog.Logger) (*Server, error) {
	pool, err := pgxpool.New(ctx, cfg.PostgresURL)
	if err != nil {
		return nil, fmt.Errorf("connecting to postgres: %w", err)
	}
	s := &Server{cfg: cfg, pool: pool, log: logger}
	if err := s.bootstrap(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return s, nil
}

// Close releases the connection pool.
func (s *Server) Close() {
	s.pool.Close()
}

// Handler serves the API under /v0, the same path the hosted service used, so
// a client's api_url is "http://host:port/v0".
func (s *Server) Handler() http.Handler {
	strict := api.NewStrictHandlerWithOptions(s, nil, api.StrictHTTPServerOptions{
		RequestErrorHandlerFunc: func(w http.ResponseWriter, _ *http.Request, err error) {
			writeError(w, http.StatusBadRequest, err.Error())
		},
		ResponseErrorHandlerFunc: func(w http.ResponseWriter, r *http.Request, err error) {
			s.log.Error("request failed", slog.String("method", r.Method), slog.String("path", r.URL.Path), slog.String("error", err.Error()))
			writeError(w, http.StatusInternalServerError, "internal error")
		},
	})
	return api.HandlerWithOptions(strict, api.StdHTTPServerOptions{
		BaseURL:     "/v0",
		Middlewares: []api.MiddlewareFunc{s.requireAPIKey},
		ErrorHandlerFunc: func(w http.ResponseWriter, _ *http.Request, err error) {
			writeError(w, http.StatusBadRequest, err.Error())
		},
	})
}

// requireAPIKey admits a request only with the configured bearer token. The
// health check is the one exception, so a probe needs no secret.
func (s *Server) requireAPIKey(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/health") {
			next.ServeHTTP(w, r)
			return
		}
		const scheme = "Bearer "
		header := r.Header.Get("Authorization")
		if !strings.HasPrefix(header, scheme) ||
			subtle.ConstantTimeCompare([]byte(header[len(scheme):]), []byte(s.cfg.APIKey)) != 1 {
			writeError(w, http.StatusUnauthorized, "missing or invalid API key")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(api.Error{Message: message})
}

// httpError is a failure the client should hear about with a specific status,
// as opposed to an unexpected error, which becomes a 500.
type httpError struct {
	status  int
	message string
}

func (e *httpError) Error() string { return e.message }

func notFound(format string, args ...any) error {
	return &httpError{status: http.StatusNotFound, message: fmt.Sprintf(format, args...)}
}

func conflict(format string, args ...any) error {
	return &httpError{status: http.StatusConflict, message: fmt.Sprintf(format, args...)}
}

func badRequest(format string, args ...any) error {
	return &httpError{status: http.StatusBadRequest, message: fmt.Sprintf(format, args...)}
}

// failure splits an error into what a default error response needs. An
// httpError becomes its status and message with a nil error; anything else
// comes back as the error so the strict handler answers 500 and logs it.
func failure(err error) (api.Error, int, error) {
	var he *httpError
	if errors.As(err, &he) {
		return api.Error{Message: he.message}, he.status, nil
	}
	return api.Error{}, 0, err
}

func unsupported(operation string) api.Error {
	return api.Error{Message: fmt.Sprintf("%s is not supported by this ghost server", operation)}
}
