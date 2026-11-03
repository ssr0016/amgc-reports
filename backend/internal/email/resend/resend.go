package resend

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/ssr0016/pastoral-reports/backend/internal/email"
)

const endpoint = "https://api.resend.com/emails"

type Client struct {
	apiKey   string
	from     string
	replyTo  string
	http     *http.Client
	endpoint string
}

type HTTPError struct {
	StatusCode int
	RetryAfter *time.Duration
	Body       string
}

func (e *HTTPError) Error() string {
	if e.Body == "" {
		return fmt.Sprintf("Resend returned HTTP %d", e.StatusCode)
	}
	return fmt.Sprintf("Resend returned HTTP %d: %s", e.StatusCode, e.Body)
}

func (e *HTTPError) Retryable() bool {
	return e.StatusCode == http.StatusTooManyRequests || e.StatusCode >= http.StatusInternalServerError
}

type resendRequest struct {
	From    string   `json:"from"`
	To      []string `json:"to"`
	Subject string   `json:"subject"`
	Text    string   `json:"text"`
	HTML    string   `json:"html"`
	ReplyTo string   `json:"reply_to,omitempty"`
}

func New(apiKey, from, replyTo string, httpClient *http.Client) (email.Mailer, error) {
	return newClientWithEndpoint(apiKey, from, replyTo, httpClient, endpoint)
}

func newClientWithEndpoint(apiKey, from, replyTo string, httpClient *http.Client, url string) (*Client, error) {
	missing := make([]string, 0, 2)
	if apiKey == "" {
		missing = append(missing, "RESEND_API_KEY")
	}
	if from == "" {
		missing = append(missing, "EMAIL_FROM")
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("resend configuration incomplete: missing %s", strings.Join(missing, ", "))
	}
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &Client{apiKey: apiKey, from: from, replyTo: replyTo, http: httpClient, endpoint: url}, nil
}

func (c *Client) Send(ctx context.Context, message email.Message) error {
	body, err := json.Marshal(resendRequest{
		From: c.from, To: []string{message.To}, Subject: message.Subject,
		Text: message.PlainText, HTML: message.HTML, ReplyTo: c.replyTo,
	})
	if err != nil {
		return fmt.Errorf("encode Resend request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create Resend request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("send Resend request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices {
		return nil
	}
	responseBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	return &HTTPError{StatusCode: resp.StatusCode, RetryAfter: parseRetryAfter(resp.Header.Get("Retry-After")), Body: strings.TrimSpace(string(responseBody))}
}

func parseRetryAfter(value string) *time.Duration {
	if seconds, err := strconv.Atoi(strings.TrimSpace(value)); err == nil && seconds >= 0 {
		d := time.Duration(seconds) * time.Second
		return &d
	}
	if at, err := http.ParseTime(value); err == nil {
		d := time.Until(at)
		if d < 0 {
			d = 0
		}
		return &d
	}
	return nil
}
