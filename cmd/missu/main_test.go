package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRenderPrefersServerSummary(t *testing.T) {
	st := &status{Summary: "You and Casey are Connected 💛 — you've sent 7, they've sent 5."}
	if got := render(st, false); got != st.Summary {
		t.Fatalf("render() = %q, want the server's summary verbatim", got)
	}
}

func TestRenderLeadsWithConfirmationAfterTap(t *testing.T) {
	st := &status{Summary: "You and Casey are Connected 💛"}
	got := render(st, true)
	if !strings.HasPrefix(got, "Miss U sent 💛\n\n") {
		t.Fatalf("render(tapped) = %q, want a confirmation lead", got)
	}
	if !strings.Contains(got, st.Summary) {
		t.Fatalf("render(tapped) dropped the summary: %q", got)
	}
}

// The fallback only matters against a server older than the summary field, but
// it is the one place this client restates the prose, so it gets a test.
func TestRenderFallsBackWhenSummaryMissing(t *testing.T) {
	var st status
	st.Miss.Connected = true
	st.Miss.Connection.Name = "Casey"
	st.Miss.Stats.SentCount = 7
	st.Miss.Stats.ReceivedCount = 5

	got := render(&st, false)
	for _, want := range []string{"You and Casey are Connected", "you've sent 7, they've sent 5"} {
		if !strings.Contains(got, want) {
			t.Fatalf("render() = %q, want it to contain %q", got, want)
		}
	}
}

func TestRenderExplainsOneSidedMiss(t *testing.T) {
	var st status
	st.Miss.Email = "them@x.test"
	got := render(&st, false)
	if !strings.Contains(got, "haven't named you back") || !strings.Contains(got, "them@x.test") {
		t.Fatalf("render() = %q, want a one-sided-miss explanation", got)
	}
}

func TestRequestSendsBearerTokenAndSurfacesAPIError(t *testing.T) {
	var gotAuth, gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth, gotPath = r.Header.Get("Authorization"), r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "connect to someone first"})
	}))
	defer srv.Close()

	_, err := request(context.Background(), srv.URL, "tok_abc", http.MethodGet, "/v1/me/status", nil)
	if err == nil || !strings.Contains(err.Error(), "connect to someone first") {
		t.Fatalf("request() error = %v, want the API's own message", err)
	}
	if gotAuth != "Bearer tok_abc" {
		t.Fatalf("Authorization header = %q", gotAuth)
	}
	if gotPath != "/v1/me/status" {
		t.Fatalf("path = %q", gotPath)
	}
}

func TestRequestReportsRetryAfterOnCooldown(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "42")
		w.WriteHeader(http.StatusTooManyRequests)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "slow down"})
	}))
	defer srv.Close()

	_, err := request(context.Background(), srv.URL, "tok", http.MethodPost, "/v1/missu", map[string]string{"source": "cli"})
	if err == nil || !strings.Contains(err.Error(), "retry in 42s") {
		t.Fatalf("request() error = %v, want the cooldown surfaced", err)
	}
}
