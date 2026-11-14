package commands

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRequestLogger(t *testing.T) {
	bigBody := strings.Repeat("x", maxLoggedBody+10)

	tests := []struct {
		name          string
		method        string
		target        string
		body          string
		wantBody      string
		wantTruncated bool
	}{
		{
			name:   "no body",
			method: http.MethodGet,
			target: "/users?id=7",
		},
		{
			name:     "with body",
			method:   http.MethodPost,
			target:   "/orders",
			body:     `{"item":"widget"}`,
			wantBody: `{"item":"widget"}`,
		},
		{
			name:          "truncated body",
			method:        http.MethodPut,
			target:        "/big",
			body:          bigBody,
			wantBody:      bigBody[:maxLoggedBody],
			wantTruncated: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			h := requestLogger(slog.New(slog.NewJSONHandler(&buf, nil)))

			req := httptest.NewRequest(tt.method, tt.target, strings.NewReader(tt.body))
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
			}

			var entry struct {
				Msg       string `json:"msg"`
				Method    string `json:"method"`
				Path      string `json:"path"`
				Body      string `json:"body"`
				Truncated bool   `json:"truncated"`
			}
			if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
				t.Fatalf("decoding log entry: %v\n%s", err, buf.String())
			}

			if entry.Msg != "request" {
				t.Errorf("msg = %q, want %q", entry.Msg, "request")
			}
			if entry.Method != tt.method {
				t.Errorf("method = %q, want %q", entry.Method, tt.method)
			}
			if entry.Path != tt.target {
				t.Errorf("path = %q, want %q", entry.Path, tt.target)
			}
			if entry.Body != tt.wantBody {
				t.Errorf("body = %.40q (len %d), want %.40q (len %d)", entry.Body, len(entry.Body), tt.wantBody, len(tt.wantBody))
			}
			if entry.Truncated != tt.wantTruncated {
				t.Errorf("truncated = %v, want %v", entry.Truncated, tt.wantTruncated)
			}
		})
	}
}

func TestNewLogger(t *testing.T) {
	for _, format := range []string{"text", "json", "JSON"} {
		if _, err := newLogger(&bytes.Buffer{}, format); err != nil {
			t.Errorf("newLogger(%q) error = %v", format, err)
		}
	}
	if _, err := newLogger(&bytes.Buffer{}, "xml"); err == nil {
		t.Error("newLogger(\"xml\") error = nil, want error")
	}
}
