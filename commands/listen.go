package commands

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// maxLoggedBody caps how much of a request body is logged.
const maxLoggedBody = 64 << 10 // 64 KiB

func newListenCmd(v *viper.Viper) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "listen",
		Short: "Listen for HTTP traffic and log each request",
		Long: `Listen starts an HTTP server that accepts any request, logs its method,
path, and body, and replies with 200 OK. It is a debugging aid for checking
that atc forwards traffic.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			port := v.GetInt("port")
			if port < 1 || port > 65535 {
				return fmt.Errorf("invalid port %d: must be between 1 and 65535", port)
			}

			logger, err := newLogger(cmd.OutOrStdout(), v.GetString("log-format"))
			if err != nil {
				return err
			}

			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()

			return listen(ctx, net.JoinHostPort("", strconv.Itoa(port)), logger)
		},
	}

	cmd.Flags().IntP("port", "p", 8080, "port to listen on")
	return cmd
}

// listen serves requestLogger on addr until ctx is cancelled.
func listen(ctx context.Context, addr string, logger *slog.Logger) error {
	srv := &http.Server{
		Addr:              addr,
		Handler:           requestLogger(logger),
		ReadHeaderTimeout: 10 * time.Second,
		ErrorLog:          slog.NewLogLogger(logger.Handler(), slog.LevelError),
	}

	errCh := make(chan error, 1)
	go func() {
		logger.Info("listening", "addr", addr)
		errCh <- srv.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		logger.Info("shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			return err
		}
		if err := <-errCh; !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	}
}

// requestLogger returns a handler that logs each request and replies 200 OK.
func requestLogger(logger *slog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attrs := []any{
			slog.String("method", r.Method),
			slog.String("path", r.URL.RequestURI()),
		}

		body, err := io.ReadAll(io.LimitReader(r.Body, maxLoggedBody+1))
		if err != nil {
			logger.Error("error reading body", append(attrs, slog.Any("error", err))...)
			http.Error(w, "error reading body", http.StatusBadRequest)
			return
		}

		if len(body) > maxLoggedBody {
			body = body[:maxLoggedBody]
			attrs = append(attrs, slog.Bool("truncated", true))
			// Drain the rest so the client isn't left hanging.
			_, _ = io.Copy(io.Discard, r.Body)
		}
		if len(body) > 0 {
			attrs = append(attrs, slog.String("body", string(body)))
		}

		logger.Info("request", attrs...)
		w.WriteHeader(http.StatusOK)
	})
}
