package middleware

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"

	"github.com/ssr0016/pastoral-reports/backend/internal/apperror"
)

// probeChunk is the probe's read size. A reader may hand over the chunk that
// crosses the limit together with the error, so the handler can see at most
// MaxRequestBodyBytes+probeChunk bytes.
const probeChunk = 4096

// bodyProbe is a POST handler behind BodyLimit that reads the whole body, as
// Bind does, and records how much of it arrived.
type bodyProbe struct {
	e       *echo.Echo
	gotLen  int
	gotErr  error
	reached bool
}

func newBodyProbe() *bodyProbe {
	p := &bodyProbe{e: echo.New()}
	p.e.HTTPErrorHandler = apperror.ErrorHandler
	p.e.Use(BodyLimit())
	p.e.POST("/probe", func(c echo.Context) error {
		p.reached = true
		buf := make([]byte, probeChunk)
		for {
			n, err := c.Request().Body.Read(buf)
			p.gotLen += n
			if err != nil {
				if err != io.EOF {
					p.gotErr = err
				}
				break
			}
		}
		if p.gotErr != nil {
			err := p.gotErr
			return apperror.BadRequest("invalid request body").WithError(err)
		}
		return c.NoContent(http.StatusOK)
	})
	return p
}

func (p *bodyProbe) post(body []byte, contentLength int64) int {
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/probe", bytes.NewReader(body))
	req.ContentLength = contentLength
	rec := httptest.NewRecorder()
	p.e.ServeHTTP(rec, req)
	return rec.Code
}

func TestBodyLimit_RejectsOversizeContentLength_413(t *testing.T) {
	p := newBodyProbe()
	body := make([]byte, MaxRequestBodyBytes+1)

	got := p.post(body, int64(len(body)))

	if got != http.StatusRequestEntityTooLarge {
		t.Errorf("status = %d, want %d", got, http.StatusRequestEntityTooLarge)
	}
	if p.reached {
		t.Error("handler ran although Content-Length exceeds the limit")
	}
}

// With no declared length the limit trips while the handler reads, and the
// handlers turn that read error into 400, so the status is only "not 2xx";
// what must hold is that the body is bounded, not just rejected.
func TestBodyLimit_RejectsOversizeChunkedBody(t *testing.T) {
	p := newBodyProbe()
	body := make([]byte, 2*MaxRequestBodyBytes)

	got := p.post(body, -1)

	if got < 400 {
		t.Errorf("status = %d, want a 4xx", got)
	}
	if p.gotErr == nil {
		t.Error("read of an oversize chunked body succeeded")
	}
	if max := MaxRequestBodyBytes + probeChunk; p.gotLen > max {
		t.Errorf("handler received %d bytes, want at most %d", p.gotLen, max)
	}
}

func TestBodyLimit_AllowsBodyAtLimit(t *testing.T) {
	p := newBodyProbe()
	body := make([]byte, MaxRequestBodyBytes)

	got := p.post(body, int64(len(body)))

	if got != http.StatusOK {
		t.Errorf("status = %d, want %d", got, http.StatusOK)
	}
	if p.gotLen != MaxRequestBodyBytes {
		t.Errorf("handler read %d bytes, want %d", p.gotLen, MaxRequestBodyBytes)
	}
}
