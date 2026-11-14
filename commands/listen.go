package commands

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
	"unicode/utf8"

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
path, and body, and replies with 200 OK and a JSON body describing the
request: method, URL, query, protocol, host, remote address, headers, content
length, body size, and the body itself (base64-encoded if it isn't valid
UTF-8). It is a debugging aid for checking that atc forwards traffic.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			addr, err := listenAddr(v.GetInt("port"))
			if err != nil {
				return err
			}

			logger, err := newLogger(cmd.OutOrStdout(), v.GetString("log-format"))
			if err != nil {
				return err
			}

			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()

			return runServer(ctx, addr, requestLogger(logger), logger)
		},
	}

	cmd.Flags().IntP("port", "p", 8080, "port to listen on")
	return cmd
}

// requestInfo is the JSON body listen sends back for each request.
type requestInfo struct {
	Method           string              `json:"method"`
	URL              string              `json:"url"`
	Path             string              `json:"path"`
	Query            map[string][]string `json:"query,omitempty"`
	Proto            string              `json:"proto"`
	Host             string              `json:"host"`
	RemoteAddr       string              `json:"remote_addr"`
	TLS              bool                `json:"tls"`
	Headers          map[string][]string `json:"headers"`
	ContentLength    int64               `json:"content_length"`
	TransferEncoding []string            `json:"transfer_encoding,omitempty"`
	BodySize         int64               `json:"body_size"`
	Body             string              `json:"body,omitempty"`
	BodyBase64       []byte              `json:"body_base64,omitempty"`
	BodyTruncated    bool                `json:"body_truncated,omitempty"`
	ReceivedAt       time.Time           `json:"received_at"`
}

// requestLogger returns a handler that logs each request and replies 200 OK
// with a JSON description of it.
func requestLogger(logger *slog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received := time.Now()
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

		size := int64(len(body))
		truncated := len(body) > maxLoggedBody
		if truncated {
			body = body[:maxLoggedBody]
			attrs = append(attrs, slog.Bool("truncated", true))
			// Drain the rest so the client isn't left hanging, counting it
			// so the reported size is the full body size.
			n, _ := io.Copy(io.Discard, r.Body)
			size += n
		}
		if len(body) > 0 {
			attrs = append(attrs, slog.String("body", string(body)))
		}

		logger.Info("request", attrs...)

		info := requestInfo{
			Method:           r.Method,
			URL:              r.URL.RequestURI(),
			Path:             r.URL.Path,
			Query:            r.URL.Query(),
			Proto:            r.Proto,
			Host:             r.Host,
			RemoteAddr:       r.RemoteAddr,
			TLS:              r.TLS != nil,
			Headers:          r.Header,
			ContentLength:    r.ContentLength,
			TransferEncoding: r.TransferEncoding,
			BodySize:         size,
			BodyTruncated:    truncated,
			ReceivedAt:       received,
		}
		if len(info.Query) == 0 {
			info.Query = nil
		}
		// Echo text bodies as-is; binary bodies as base64.
		if utf8.Valid(body) {
			info.Body = string(body)
		} else {
			info.BodyBase64 = body
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		if err := enc.Encode(info); err != nil {
			logger.Error("error writing response", append(attrs, slog.Any("error", err))...)
		}
	})
}
