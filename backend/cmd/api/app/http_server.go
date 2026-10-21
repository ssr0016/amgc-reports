package app

import (
	"net/http"
	"time"

	"github.com/ssr0016/pastoral-reports/backend/internal/config"
)

// maxHeaderBytes caps the request line and headers, cookies included. Go's
// default is 1 MiB. net/http answers 431 itself, so an oversize request never
// reaches Echo, its logger or its metrics.
const maxHeaderBytes = 16 * 1024

// newHTTPServer builds the server without starting it; run owns
// ListenAndServe and Shutdown.
func newHTTPServer(cfg *config.Config, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:         ":" + cfg.App.Port,
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,

		MaxHeaderBytes: maxHeaderBytes,
	}
}
