package resend

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ssr0016/pastoral-reports/backend/internal/email"
)

func TestClient_Send_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/emails", r.URL.Path)
		assert.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		var got map[string]any
		require.NoError(t, json.Unmarshal(body, &got))
		assert.Equal(t, "AmgReports <reports@example.com>", got["from"])
		assert.Equal(t, []any{"owner@example.com"}, got["to"])
		assert.Equal(t, "subject", got["subject"])
		assert.Equal(t, "plain text", got["text"])
		assert.Equal(t, "<p>html</p>", got["html"])
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client, err := newClient("test-key", "AmgReports <reports@example.com>", "", server.Client(), server.URL+"/emails")
	require.NoError(t, err)
	err = client.Send(t.Context(), email.Message{To: "owner@example.com", Subject: "subject", PlainText: "plain text", HTML: "<p>html</p>"})
	assert.NoError(t, err)
}

func TestClient_Send_IncludesReplyTo(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		var got map[string]any
		require.NoError(t, json.Unmarshal(body, &got))
		assert.Equal(t, "admin@example.com", got["reply_to"])
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client, err := newClient("key", "from@example.com", "admin@example.com", server.Client(), server.URL)
	require.NoError(t, err)
	assert.NoError(t, client.Send(t.Context(), email.Message{To: "owner@example.com"}))
}

func TestClient_Send_OmitsEmptyReplyTo(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		assert.NotContains(t, string(body), "reply_to")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client, err := newClient("key", "from@example.com", "", server.Client(), server.URL)
	require.NoError(t, err)
	assert.NoError(t, client.Send(t.Context(), email.Message{To: "owner@example.com"}))
}

func TestClient_Send_400ReturnsPermanentHTTPError(t *testing.T) {
	client := testErrorClient(t, http.StatusBadRequest, "bad request", "")
	err := client.Send(t.Context(), email.Message{To: "owner@example.com"})

	var httpErr *HTTPError
	require.ErrorAs(t, err, &httpErr)
	assert.Equal(t, http.StatusBadRequest, httpErr.StatusCode)
	assert.False(t, httpErr.Retryable())
}

func TestClient_Send_422ReturnsPermanentHTTPError(t *testing.T) {
	client := testErrorClient(t, http.StatusUnprocessableEntity, "invalid", "")
	err := client.Send(t.Context(), email.Message{To: "owner@example.com"})

	var httpErr *HTTPError
	require.ErrorAs(t, err, &httpErr)
	assert.Equal(t, http.StatusUnprocessableEntity, httpErr.StatusCode)
	assert.False(t, httpErr.Retryable())
}

func TestClient_Send_429ReturnsRetryableHTTPError(t *testing.T) {
	client := testErrorClient(t, http.StatusTooManyRequests, "rate limited", "17")
	err := client.Send(t.Context(), email.Message{To: "owner@example.com"})

	var httpErr *HTTPError
	require.ErrorAs(t, err, &httpErr)
	assert.Equal(t, http.StatusTooManyRequests, httpErr.StatusCode)
	assert.True(t, httpErr.Retryable())
	require.NotNil(t, httpErr.RetryAfter)
	assert.Equal(t, 17*time.Second, *httpErr.RetryAfter)
}

func TestClient_Send_429WithZeroRetryAfter(t *testing.T) {
	client := testErrorClient(t, http.StatusTooManyRequests, "rate limited", "0")
	err := client.Send(t.Context(), email.Message{To: "owner@example.com"})

	var httpErr *HTTPError
	require.ErrorAs(t, err, &httpErr)
	require.NotNil(t, httpErr.RetryAfter)
	assert.Zero(t, *httpErr.RetryAfter)
}

func TestClient_Send_500ReturnsRetryableHTTPError(t *testing.T) {
	client := testErrorClient(t, http.StatusInternalServerError, "server error", "")
	err := client.Send(t.Context(), email.Message{To: "owner@example.com"})

	var httpErr *HTTPError
	require.ErrorAs(t, err, &httpErr)
	assert.Equal(t, http.StatusInternalServerError, httpErr.StatusCode)
	assert.True(t, httpErr.Retryable())
}

func newClient(apiKey, from, replyTo string, httpClient *http.Client, endpoint string) (*Client, error) {
	return newClientWithEndpoint(apiKey, from, replyTo, httpClient, endpoint)
}

func testErrorClient(t *testing.T, status int, body, retryAfter string) *Client {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if retryAfter != "" {
			w.Header().Set("Retry-After", retryAfter)
		}
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(server.Close)
	return &Client{
		apiKey: "key", from: "from@example.com", http: server.Client(), endpoint: server.URL,
	}
}

func TestClient_Send_RejectsEmptyConfiguration(t *testing.T) {
	_, err := New("", "from@example.com", "", http.DefaultClient)
	assert.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "RESEND_API_KEY"))
}
