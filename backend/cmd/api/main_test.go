package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/getsentry/sentry-go"
)

func TestStripQuery_DropsToken(t *testing.T) {
	r := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/auth/verify-email?token=secret", nil)
	event := stripQuery(&sentry.Event{Request: sentry.NewRequest(r)}, nil)

	if event.Request.QueryString != "" {
		t.Errorf("QueryString = %q, want empty", event.Request.QueryString)
	}
	if got := stripQuery(&sentry.Event{}, nil); got == nil {
		t.Error("event without a request was dropped")
	}
}
