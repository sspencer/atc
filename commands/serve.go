package commands

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func newServeCmd(v *viper.Viper) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Run an HTTP proxy that forwards traffic to a remote endpoint",
		Long: `Serve starts an HTTP reverse proxy on the given port and forwards every
request to the remote endpoint. The request path and query are appended to the
endpoint URL, and X-Forwarded-For, X-Forwarded-Host, and X-Forwarded-Proto
headers are set. Each proxied request is logged with its status and duration.`,
		Example: `  atc serve -p 8080 -e http://localhost:9000
  atc serve -e https://api.example.com/v1`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			addr, err := listenAddr(v.GetInt("port"))
			if err != nil {
				return err
			}

			target, err := parseEndpoint(v.GetString("endpoint"))
			if err != nil {
				return err
			}

			logger, err := newLogger(cmd.OutOrStdout(), v.GetString("log-format"))
			if err != nil {
				return err
			}

			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()

			logger.Info("proxying", "endpoint", target.String())
			return runServer(ctx, addr, newProxy(target, logger), logger)
		},
	}

	cmd.Flags().IntP("port", "p", 8080, "port to listen on")
	cmd.Flags().StringP("endpoint", "e", "", "remote HTTP endpoint to forward traffic to (required)")
	return cmd
}

// parseEndpoint validates that raw is an absolute http or https URL.
func parseEndpoint(raw string) (*url.URL, error) {
	if raw == "" {
		return nil, errors.New("an endpoint is required: set --endpoint (-e) or ATC_ENDPOINT")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("invalid endpoint %q: %w", raw, err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("invalid endpoint %q: scheme must be http or https", raw)
	}
	if u.Host == "" {
		return nil, fmt.Errorf("invalid endpoint %q: missing host", raw)
	}
	return u, nil
}

// newProxy returns a handler that forwards requests to target and logs each
// one. Upstream failures are logged and answered with 502 Bad Gateway.
func newProxy(target *url.URL, logger *slog.Logger) http.Handler {
	proxy := &httputil.ReverseProxy{
		Rewrite: func(r *httputil.ProxyRequest) {
			r.SetURL(target)
			r.SetXForwarded()
		},
		ErrorLog: slog.NewLogLogger(logger.Handler(), slog.LevelError),
		// r here is the outbound request, so log the full upstream URL.
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			logger.Error("proxy error",
				slog.String("method", r.Method),
				slog.String("upstream", r.URL.String()),
				slog.Any("error", err),
			)
			w.WriteHeader(http.StatusBadGateway)
		},
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		proxy.ServeHTTP(rec, r)
		logger.Info("proxied",
			slog.String("method", r.Method),
			slog.String("path", r.URL.RequestURI()),
			slog.Int("status", rec.status),
			slog.Duration("duration", time.Since(start)),
		)
	})
}

// statusRecorder captures the response status code for logging.
type statusRecorder struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (s *statusRecorder) WriteHeader(code int) {
	if !s.wroteHeader {
		s.status = code
		s.wroteHeader = true
	}
	s.ResponseWriter.WriteHeader(code)
}

// Unwrap lets http.ResponseController reach the underlying writer, so
// flushing (used for streaming responses) still works through the recorder.
func (s *statusRecorder) Unwrap() http.ResponseWriter {
	return s.ResponseWriter
}
