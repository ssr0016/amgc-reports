package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"time"

	"github.com/ssr0016/pastoral-reports/backend/internal/email"
	"github.com/ssr0016/pastoral-reports/backend/internal/email/resend"
	"github.com/ssr0016/pastoral-reports/backend/internal/report/outbox"
)

type Options struct {
	PollInterval    time.Duration
	LeaseTimeout    time.Duration
	MaxAttempts     int
	BackoffBase     time.Duration
	BackoffMax      time.Duration
	InFlightTimeout time.Duration
	Now             func() time.Time
	Logger          *slog.Logger
	DigestScheduler outbox.DigestScheduler
	ReportsURL      string
}

type Worker struct {
	queue           outbox.Queue
	mailer          email.Mailer
	pollInterval    time.Duration
	leaseTimeout    time.Duration
	maxAttempts     int
	backoffBase     time.Duration
	backoffMax      time.Duration
	inFlightTimeout time.Duration
	now             func() time.Time
	logger          *slog.Logger
	digestScheduler outbox.DigestScheduler
	reportsURL      string
	manila          *time.Location
}

func New(queue outbox.Queue, mailer email.Mailer, options Options) *Worker {
	if options.PollInterval <= 0 {
		options.PollInterval = 10 * time.Second
	}
	if options.LeaseTimeout <= 0 {
		options.LeaseTimeout = 5 * time.Minute
	}
	if options.MaxAttempts <= 0 {
		options.MaxAttempts = 5
	}
	if options.BackoffBase <= 0 {
		options.BackoffBase = time.Minute
	}
	if options.BackoffMax <= 0 {
		options.BackoffMax = time.Hour
	}
	if options.InFlightTimeout <= 0 {
		options.InFlightTimeout = 10 * time.Second
	}
	if options.Now == nil {
		options.Now = time.Now
	}
	if options.Logger == nil {
		options.Logger = slog.Default()
	}
	manila, err := time.LoadLocation("Asia/Manila")
	if err != nil {
		panic(err)
	}
	return &Worker{queue: queue, mailer: mailer, pollInterval: options.PollInterval,
		leaseTimeout: options.LeaseTimeout, maxAttempts: options.MaxAttempts,
		backoffBase: options.BackoffBase, backoffMax: options.BackoffMax,
		inFlightTimeout: options.InFlightTimeout, now: options.Now, logger: options.Logger,
		digestScheduler: options.DigestScheduler, reportsURL: options.ReportsURL, manila: manila}
}

func (w *Worker) Run(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return nil
	}
	if err := w.RunOnce(ctx); err != nil && !errors.Is(err, context.Canceled) {
		w.logger.Error("notification worker poll failed", "error", err)
	}
	ticker := time.NewTicker(w.pollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if err := w.RunOnce(ctx); err != nil && !errors.Is(err, context.Canceled) {
				w.logger.Error("notification worker poll failed", "error", err)
			}
		}
	}
}

func (w *Worker) RunOnce(ctx context.Context) error {
	if w.digestScheduler != nil {
		now := w.now().In(w.manila)
		if err := w.digestScheduler.ScheduleDailyDigest(ctx, now, w.reportsURL); err != nil {
			return err
		}
	}
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		job, err := w.queue.ClaimNext(ctx, w.now(), w.leaseTimeout)
		if err != nil {
			return err
		}
		if job == nil {
			return nil
		}
		if err := w.process(ctx, job); err != nil {
			w.logger.Error("notification worker row failed", "outbox_id", job.ID, "report_id", job.ReportID, "event_type", job.EventType, "error", err)
		}
	}
}

func (w *Worker) process(_ context.Context, job *outbox.Job) error {
	var message email.Message
	var err error
	if job.EventType == "daily_digest" {
		var data email.DailyDigestData
		if err = json.Unmarshal(job.Payload, &data); err == nil {
			message, err = email.RenderDailyDigest(data)
		}
	} else {
		data := email.ReportNotificationData{ReportMonth: job.ReportMonth, ReportYear: job.ReportYear, EventType: job.EventType, AdminRemark: job.AdminRemark}
		message, err = email.RenderReportNotification(data)
	}
	if err != nil {
		return w.skip(job, fmt.Sprintf("render notification: %v", err))
	}
	message.To = job.RecipientEmail
	jobCtx, cancel := context.WithTimeout(context.Background(), w.inFlightTimeout)
	defer cancel()
	err = w.mailer.Send(jobCtx, message)
	if err == nil {
		return w.queue.MarkSent(context.Background(), job.ID, job.ClaimedAt, w.now())
	}
	if httpErr, ok := err.(*resend.HTTPError); ok && !httpErr.Retryable() {
		return w.skip(job, httpErr.Error())
	}
	if job.Attempts >= w.maxAttempts {
		return w.skip(job, "maximum delivery attempts exceeded")
	}
	delay := w.backoff(job.Attempts)
	if httpErr, ok := err.(*resend.HTTPError); ok && httpErr.RetryAfter != nil {
		delay = *httpErr.RetryAfter
	}
	return w.queue.MarkFailed(context.Background(), job.ID, job.ClaimedAt, w.now().Add(delay), err.Error())
}

func (w *Worker) skip(job *outbox.Job, reason string) error {
	return w.queue.MarkSkipped(context.Background(), job.ID, job.ClaimedAt, reason)
}

func (w *Worker) backoff(attempt int) time.Duration {
	power := math.Pow(2, float64(max(attempt-1, 0)))
	delay := time.Duration(float64(w.backoffBase) * power)
	if delay > w.backoffMax {
		return w.backoffMax
	}
	return delay
}
