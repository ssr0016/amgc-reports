package main

// @title           Pastoral Reports API
// @version         1.0
// @description     Pastoral Reports backend API: JWT cookie auth and RBAC

// @license.name  MIT
// @license.url   https://opensource.org/licenses/MIT

// @BasePath  /api/v1

// @securityDefinitions.apikey CookieAuth
// @in cookie
// @name access_token

import (
	"fmt"
	"os"
	"time"

	"github.com/getsentry/sentry-go"

	"github.com/ssr0016/pastoral-reports/backend/cmd/api/app"
)

func main() {
	// An empty SENTRY_DSN (local, tests) leaves Sentry off; a failed init must not stop the API.
	if err := sentry.Init(sentry.ClientOptions{
		Dsn:         os.Getenv("SENTRY_DSN"),
		Environment: os.Getenv("APP_ENV"),
		// Render sets the deployed commit; when empty the SDK falls back to SENTRY_RELEASE or git.
		Release:       os.Getenv("RENDER_GIT_COMMIT"),
		EnableTracing: true,
		TracesSampler: func(ctx sentry.SamplingContext) float64 {
			// Health probes and Prometheus scrapes would use up the span quota.
			switch ctx.Span.Name {
			case "GET /health", "GET /ready", "GET /live", "GET /metrics":
				return 0
			}
			return 1
		},
		BeforeSend:            scrubRequest,
		BeforeSendTransaction: scrubRequest,
	}); err != nil {
		fmt.Fprintf(os.Stderr, "sentry init: %v\n", err)
	}
	code := app.Execute(os.Args[1:], os.Stderr)
	// os.Exit skips defers, so queued events are sent here.
	sentry.Flush(2 * time.Second)
	os.Exit(code)
}

// scrubRequest keeps request secrets out of Sentry. The SDK copies the body
// (passwords on login and password changes) whatever SendDefaultPII says, and
// the email verification link carries its token in the query string.
func scrubRequest(event *sentry.Event, _ *sentry.EventHint) *sentry.Event {
	if event.Request != nil {
		event.Request.QueryString = ""
		event.Request.Data = ""
	}
	return event
}
