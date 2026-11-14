package httpapi

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

// The board cancels a refresh when a card moves; a request the client gave up on is not a
// server error, but a cancel the client didn't ask for still is.
func TestCancelledRequestIsNotAnError(t *testing.T) {
	var logs bytes.Buffer
	log := slog.New(slog.NewTextHandler(&logs, nil))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	rec := httptest.NewRecorder()
	writeProblem(rec, httptest.NewRequest(http.MethodGet, "/", nil).WithContext(ctx), log, fmt.Errorf("board: %w", context.Canceled))
	if rec.Code != statusClientClosedRequest || logs.Len() != 0 {
		t.Fatalf("client cancel: status %d, logged %q", rec.Code, logs.String())
	}

	rec = httptest.NewRecorder()
	writeProblem(rec, httptest.NewRequest(http.MethodGet, "/", nil), log, context.Canceled)
	if rec.Code != http.StatusInternalServerError || logs.Len() == 0 {
		t.Fatalf("server-side cancel: status %d, logged %q", rec.Code, logs.String())
	}
}
