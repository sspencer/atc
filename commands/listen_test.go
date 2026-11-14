package commands

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"reflect"
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

func TestRequestLoggerResponse(t *testing.T) {
	bigBody := strings.Repeat("x", maxLoggedBody+10)

	tests := []struct {
		name          string
		method        string
		target        string
		body          string
		wantQuery     map[string][]string
		wantSize      int64
		wantBody      string
		wantBase64    []byte
		wantTruncated bool
	}{
		{
			name:      "get with query",
			method:    http.MethodGet,
			target:    "/users?id=7&tag=a&tag=b",
			wantQuery: map[string][]string{"id": {"7"}, "tag": {"a", "b"}},
		},
		{
			name:     "post with body",
			method:   http.MethodPost,
			target:   "/orders",
			body:     `{"item":"widget"}`,
			wantSize: 17,
			wantBody: `{"item":"widget"}`,
		},
		{
			name:       "binary body",
			method:     http.MethodPut,
			target:     "/blob",
			body:       "\xff\xfe\x00",
			wantSize:   3,
			wantBase64: []byte{0xff, 0xfe, 0x00},
		},
		{
			name:          "truncated body reports full size",
			method:        http.MethodPut,
			target:        "/big",
			body:          bigBody,
			wantSize:      int64(len(bigBody)),
			wantBody:      bigBody[:maxLoggedBody],
			wantTruncated: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := requestLogger(slog.New(slog.NewJSONHandler(io.Discard, nil)))

			req := httptest.NewRequest(tt.method, tt.target, strings.NewReader(tt.body))
			req.Header.Set("X-Test", "yes")
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
			}
			if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
				t.Errorf("Content-Type = %q, want application/json", ct)
			}

			var got requestInfo
			if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
				t.Fatalf("decoding response: %v\n%.200s", err, rec.Body.String())
			}

			if got.Method != tt.method {
				t.Errorf("method = %q, want %q", got.Method, tt.method)
			}
			if got.URL != tt.target {
				t.Errorf("url = %q, want %q", got.URL, tt.target)
			}
			if got.Host != "example.com" {
				t.Errorf("host = %q, want example.com", got.Host)
			}
			if got.Headers["X-Test"] == nil || got.Headers["X-Test"][0] != "yes" {
				t.Errorf("headers = %v, want X-Test: yes", got.Headers)
			}
			if !reflect.DeepEqual(got.Query, tt.wantQuery) {
				t.Errorf("query = %v, want %v", got.Query, tt.wantQuery)
			}
			if got.BodySize != tt.wantSize {
				t.Errorf("body_size = %d, want %d", got.BodySize, tt.wantSize)
			}
			if got.Body != tt.wantBody {
				t.Errorf("body = %.40q (len %d), want %.40q (len %d)", got.Body, len(got.Body), tt.wantBody, len(tt.wantBody))
			}
			if !bytes.Equal(got.BodyBase64, tt.wantBase64) {
				t.Errorf("body_base64 = %v, want %v", got.BodyBase64, tt.wantBase64)
			}
			if got.BodyTruncated != tt.wantTruncated {
				t.Errorf("body_truncated = %v, want %v", got.BodyTruncated, tt.wantTruncated)
			}
			if got.ReceivedAt.IsZero() {
				t.Error("received_at is zero")
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
