package commands

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestParseEndpoint(t *testing.T) {
	valid := []string{
		"http://localhost:9000",
		"https://api.example.com/v1",
	}
	for _, raw := range valid {
		if _, err := parseEndpoint(raw); err != nil {
			t.Errorf("parseEndpoint(%q) error = %v", raw, err)
		}
	}

	invalid := []string{
		"",
		"localhost:9000",
		"ftp://example.com",
		"http://",
		"://bad",
	}
	for _, raw := range invalid {
		if _, err := parseEndpoint(raw); err == nil {
			t.Errorf("parseEndpoint(%q) error = nil, want error", raw)
		}
	}
}

func TestProxyForwards(t *testing.T) {
	type seen struct {
		method, path, body, fwdFor string
	}
	var got seen
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		got = seen{r.Method, r.URL.RequestURI(), string(body), r.Header.Get("X-Forwarded-For")}
		w.Header().Set("X-Upstream", "yes")
		w.WriteHeader(http.StatusCreated)
		io.WriteString(w, "created")
	}))
	defer upstream.Close()

	target, err := url.Parse(upstream.URL + "/base")
	if err != nil {
		t.Fatal(err)
	}

	var logs bytes.Buffer
	front := httptest.NewServer(newProxy(target, slog.New(slog.NewJSONHandler(&logs, nil))))
	defer front.Close()

	resp, err := http.Post(front.URL+"/items?id=7", "application/json", strings.NewReader(`{"a":1}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusCreated {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusCreated)
	}
	if string(respBody) != "created" {
		t.Errorf("response body = %q, want %q", respBody, "created")
	}
	if resp.Header.Get("X-Upstream") != "yes" {
		t.Error("upstream response header was not passed through")
	}

	want := seen{http.MethodPost, "/base/items?id=7", `{"a":1}`, "127.0.0.1"}
	if got != want {
		t.Errorf("upstream saw %+v, want %+v", got, want)
	}

	var entry struct {
		Msg    string `json:"msg"`
		Path   string `json:"path"`
		Status int    `json:"status"`
	}
	if err := json.Unmarshal(logs.Bytes(), &entry); err != nil {
		t.Fatalf("decoding log entry: %v\n%s", err, logs.String())
	}
	if entry.Msg != "proxied" || entry.Path != "/items?id=7" || entry.Status != http.StatusCreated {
		t.Errorf("log entry = %+v, want proxied /items?id=7 201", entry)
	}
}

func TestProxyUpstreamDown(t *testing.T) {
	upstream := httptest.NewServer(http.NotFoundHandler())
	target, _ := url.Parse(upstream.URL)
	upstream.Close() // nothing is listening now

	var logs bytes.Buffer
	h := newProxy(target, slog.New(slog.NewJSONHandler(&logs, nil)))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/x", nil))

	if rec.Code != http.StatusBadGateway {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadGateway)
	}
	if !strings.Contains(logs.String(), `"msg":"proxy error"`) {
		t.Errorf("expected a proxy error log entry, got:\n%s", logs.String())
	}
}
