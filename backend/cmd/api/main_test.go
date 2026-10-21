package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/getsentry/sentry-go"
)

func TestScrubRequest_DropsSecrets(t *testing.T) {
	r := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/verify-email?token=secret", nil)
	req := sentry.NewRequest(r)
	req.Data = `{"email":"a@b.c","password":"hunter2hunter2"}`
	event := scrubRequest(&sentry.Event{Request: req}, nil)

	if event.Request.QueryString != "" {
		t.Errorf("QueryString = %q, want empty", event.Request.QueryString)
	}
	if event.Request.Data != "" {
		t.Errorf("Data = %q, want empty", event.Request.Data)
	}
	if got := scrubRequest(&sentry.Event{}, nil); got == nil {
		t.Error("event without a request was dropped")
	}
}
