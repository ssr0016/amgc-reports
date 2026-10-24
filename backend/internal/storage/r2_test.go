package storage

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/stretchr/testify/require"
)

type avatarHTTPClient func(*http.Request) (*http.Response, error)

func (f avatarHTTPClient) Do(req *http.Request) (*http.Response, error) { return f(req) }

func TestR2_Delete(t *testing.T) {
	for _, tt := range []struct {
		name, url, wantPath string
		wantError           bool
	}{
		{"versioned", "https://cdn.example/avatars/7/new", "/bucket/avatars/7/new", false},
		{"legacy cache token", "https://cdn.example/avatars/7?v=1", "/bucket/avatars/7", false},
		{"other host", "https://cdn.example.evil/avatars/7", "", true},
		{"other object", "https://cdn.example/reports/7", "", true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			calls := 0
			r := &R2{bucket: "bucket", publicURL: "https://cdn.example"}
			r.client = s3.New(s3.Options{
				Region:       "auto",
				BaseEndpoint: aws.String("https://r2.example"),
				Credentials:  credentials.NewStaticCredentialsProvider("test-key", "test-secret", ""),
				UsePathStyle: true,
				HTTPClient: avatarHTTPClient(func(req *http.Request) (*http.Response, error) {
					calls++
					require.Equal(t, http.MethodDelete, req.Method)
					require.Equal(t, tt.wantPath, req.URL.Path)
					require.Empty(t, req.URL.Query().Get("v"))
					return &http.Response{StatusCode: http.StatusNoContent, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(""))}, nil
				}),
			})
			err := r.Delete(context.Background(), tt.url)
			if tt.wantError {
				require.Error(t, err)
				require.Zero(t, calls)
			} else {
				require.NoError(t, err)
				require.Equal(t, 1, calls)
			}
		})
	}
}

func TestR2_PutPreservesPublicURL(t *testing.T) {
	const publicURL = "https://pub-34d82063b0224b23bf6873227edb3e55.r2.dev"
	r := &R2{bucket: "bucket", publicURL: publicURL}
	r.client = s3.New(s3.Options{
		Region:       "auto",
		BaseEndpoint: aws.String("https://r2.example"),
		Credentials:  credentials.NewStaticCredentialsProvider("test-key", "test-secret", ""),
		UsePathStyle: true,
		HTTPClient: avatarHTTPClient(func(req *http.Request) (*http.Response, error) {
			require.Equal(t, http.MethodPut, req.Method)
			require.Equal(t, "/bucket/avatars/4/test", req.URL.Path)
			return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(""))}, nil
		}),
	})

	url, err := r.Put(context.Background(), "avatars/4/test", "image/png", []byte("image"))
	require.NoError(t, err)
	require.Equal(t, publicURL+"/avatars/4/test", url)
}
