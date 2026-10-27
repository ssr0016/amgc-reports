package rest

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/image/webp"

	"github.com/ssr0016/pastoral-reports/backend/internal/apperror"
	"github.com/ssr0016/pastoral-reports/backend/internal/middleware"
	"github.com/ssr0016/pastoral-reports/backend/internal/request"
	"github.com/ssr0016/pastoral-reports/backend/internal/user"
	"github.com/ssr0016/pastoral-reports/backend/internal/user/userimpl"
)

type fakeAvatarStorage struct {
	key, contentType  string
	calls             int
	deleted           []string
	putErr, deleteErr error
	onDelete          func(string)
}

func (f *fakeAvatarStorage) Put(_ context.Context, key, contentType string, _ []byte) (string, error) {
	f.key, f.contentType = key, contentType
	f.calls++
	return "https://cdn.example/" + key, f.putErr
}

func (f *fakeAvatarStorage) Delete(_ context.Context, url string) error {
	f.deleted = append(f.deleted, url)
	if f.onDelete != nil {
		f.onDelete(url)
	}
	return f.deleteErr
}

func avatarPNG(t *testing.T, width, height int) []byte {
	t.Helper()
	var b bytes.Buffer
	require.NoError(t, png.Encode(&b, image.NewRGBA(image.Rect(0, 0, width, height))))
	return b.Bytes()
}

func serveAvatar(t *testing.T, storage AvatarStorage, field string, file []byte) *httptest.ResponseRecorder {
	t.Helper()
	users := userimpl.NewMockStore()
	_, err := users.CreateWithPassword(context.Background(), "a@example.com", "Ann", "hash")
	require.NoError(t, err)

	return serveAvatarRequest(users, storage, avatarRequest(t, field, file))
}

func avatarRequest(t *testing.T, field string, file []byte) *http.Request {
	t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	if field != "" {
		fw, err := mw.CreateFormFile(field, "pic.bin")
		require.NoError(t, err)
		_, err = fw.Write(file)
		require.NoError(t, err)
	}
	require.NoError(t, mw.Close())

	req := httptest.NewRequestWithContext(request.WithUser(context.Background(), request.User{ID: 1}),
		http.MethodPost, "/users/me/avatar", &body)
	req.Header.Set(echo.HeaderContentType, mw.FormDataContentType())
	return req
}

func serveAvatarRequest(users user.Store, storage AvatarStorage, req *http.Request) *httptest.ResponseRecorder {
	e := echo.New()
	e.HTTPErrorHandler = apperror.ErrorHandler
	e.Use(middleware.BodyLimit())
	e.POST("/users/me/avatar", NewAvatarHandler(users, storage).UploadAvatar)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func TestAvatarHandler_UploadAvatar(t *testing.T) {
	t.Run("replaces and cleans up a previously saved avatar", func(t *testing.T) {
		users := userimpl.NewMockStore()
		u, err := users.CreateWithPassword(context.Background(), "a@example.com", "Ann", "hash")
		require.NoError(t, err)
		const oldURL = "https://pub-34d82063b022"
		_, _, err = users.UpdateAvatar(context.Background(), u.ID, oldURL)
		require.NoError(t, err)

		st := &fakeAvatarStorage{}
		rec := serveAvatarRequest(users, st, avatarRequest(t, "avatar", avatarPNG(t, 2, 3)))
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		require.Contains(t, st.deleted, oldURL)
		updated, err := users.GetByID(context.Background(), u.ID)
		require.NoError(t, err)
		require.Equal(t, "https://cdn.example/"+st.key, *updated.AvatarURL)
	})

	t.Run("200 stores a real PNG under a unique caller-owned key", func(t *testing.T) {
		st := &fakeAvatarStorage{}
		rec := serveAvatar(t, st, "avatar", avatarPNG(t, 2, 3))
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		assert.Regexp(t, `^avatars/1/[A-Za-z0-9_-]+\.png$`, st.key)
		assert.Equal(t, "image/png", st.contentType)
		assert.Contains(t, rec.Body.String(), `"avatar_url":"https://cdn.example/`+st.key+`"`)
	})

	t.Run("a file that is not an image is 422 and is not stored", func(t *testing.T) {
		st := &fakeAvatarStorage{}
		rec := serveAvatar(t, st, "avatar", []byte("<svg onload=alert(1)></svg>"))
		assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
		assert.Contains(t, rec.Body.String(), "avatar")
		assert.Zero(t, st.calls)
	})

	t.Run("no file is 422", func(t *testing.T) {
		rec := serveAvatar(t, &fakeAvatarStorage{}, "", nil)
		assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
	})

	t.Run("no storage configured is 503", func(t *testing.T) {
		rec := serveAvatar(t, nil, "avatar", avatarPNG(t, 2, 3))
		assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
	})
}

func webpWithCanvas(t *testing.T, webp []byte, width, height uint32) []byte {
	t.Helper()
	out := append([]byte(nil), webp[:12]...)
	out = append(out, 'V', 'P', '8', 'X', 10, 0, 0, 0, 0, 0, 0, 0, byte(width-1), byte((width-1)>>8), byte((width-1)>>16), byte(height-1), byte((height-1)>>8), byte((height-1)>>16))
	out = append(out, webp[12:]...)
	binary.LittleEndian.PutUint32(out[4:8], uint32(len(out)-8))
	return out
}

func TestAvatarHandler_ImageValidation(t *testing.T) {
	var jpegData bytes.Buffer
	require.NoError(t, jpeg.Encode(&jpegData, image.NewRGBA(image.Rect(0, 0, 3, 2)), nil))
	webpData, err := base64.StdEncoding.DecodeString("UklGRjgAAABXRUJQVlA4ICwAAACQAQCdASoCAAIAAgA0JaACdLoAA5gA/vmTb/+QH/+QH/+QH/8gP+IXeyAwAA==")
	require.NoError(t, err)
	// VP8X lies about the frame size; DecodeConfig alone trusts this canvas header.
	extendedWebP := webpWithCanvas(t, webpData, 1, 1)
	wideWebP, err := base64.StdEncoding.DecodeString("UklGRlYAAABXRUJQVlA4IEoAAABwBACdASoCAgIAPpFIoUylpCMiIQgAsBIJaQAACNBae41Eq81tfTp06dOm+AD+9Vi///ucD//ZwP/9nA/jr/9eqWuTo/gAAAAAAA==")
	require.NoError(t, err)
	wideWebPWithSmallCanvas := webpWithCanvas(t, wideWebP, 1, 1)
	canvasConfig, err := webp.DecodeConfig(bytes.NewReader(extendedWebP))
	require.NoError(t, err)
	decoded, err := webp.Decode(bytes.NewReader(extendedWebP))
	require.NoError(t, err)
	require.Equal(t, image.Rect(0, 0, 1, 1), image.Rect(0, 0, canvasConfig.Width, canvasConfig.Height))
	require.Equal(t, 2, decoded.Bounds().Dx())
	pngData := avatarPNG(t, 2, 3)
	for _, tt := range []struct {
		name   string
		data   []byte
		status int
	}{
		{"jpeg", jpegData.Bytes(), 200},
		{"webp", webpData, 200},
		{"PNG signature without image", append([]byte("\x89PNG\r\n\x1a\n"), make([]byte, 32)...), 422},
		{"truncated PNG after valid config", pngData[:len(pngData)-16], 422},
		{"truncated JPEG", jpegData.Bytes()[:len(jpegData.Bytes())-16], 422},
		{"truncated WebP", webpData[:len(webpData)-8], 422},
		{"WebP canvas and frame differ", extendedWebP, 422},
		{"WebP frame over limit behind small canvas", wideWebPWithSmallCanvas, 422},
		{"too wide", avatarPNG(t, 513, 1), 422},
		{"too tall", avatarPNG(t, 1, 513), 422},
		{"maximum dimensions", avatarPNG(t, 512, 512), 200},
	} {
		t.Run(tt.name, func(t *testing.T) {
			st := &fakeAvatarStorage{}
			rec := serveAvatar(t, st, "avatar", tt.data)
			require.Equal(t, tt.status, rec.Code, rec.Body.String())
			if tt.status != 200 {
				assert.Zero(t, st.calls)
			}
		})
	}
}

func TestAvatarHandler_BodyLimit(t *testing.T) {
	for _, tt := range []struct {
		name     string
		declared bool
		size     int
		status   int
	}{
		{"chunked exactly 1 MiB", false, 1 << 20, 200},
		{"declared above 1 MiB", true, (1 << 20) + 1, 413},
		{"chunked above 1 MiB after final boundary", false, (1 << 20) + 1, 413},
	} {
		t.Run(tt.name, func(t *testing.T) {
			users := userimpl.NewMockStore()
			_, err := users.CreateWithPassword(context.Background(), "a@example.com", "Ann", "hash")
			require.NoError(t, err)
			req := avatarRequest(t, "avatar", avatarPNG(t, 2, 3))
			body, err := io.ReadAll(req.Body)
			require.NoError(t, err)
			body = append(body, bytes.Repeat([]byte("x"), tt.size-len(body))...)
			req.Body = io.NopCloser(bytes.NewReader(body))
			req.ContentLength = -1
			if tt.declared {
				req.ContentLength = int64(len(body))
			}
			st := &fakeAvatarStorage{}
			rec := serveAvatarRequest(users, st, req)
			require.Equal(t, tt.status, rec.Code, rec.Body.String())
			if tt.status == 413 {
				assert.Zero(t, st.calls)
			}
		})
	}
	t.Run("chunked oversized file before final boundary", func(t *testing.T) {
		users := userimpl.NewMockStore()
		_, err := users.CreateWithPassword(context.Background(), "a@example.com", "Ann", "hash")
		require.NoError(t, err)
		data := append(avatarPNG(t, 2, 3), make([]byte, 1<<20)...)
		req := avatarRequest(t, "avatar", data)
		req.ContentLength = -1
		st := &fakeAvatarStorage{}
		rec := serveAvatarRequest(users, st, req)
		require.Equal(t, 413, rec.Code, rec.Body.String())
		assert.Zero(t, st.calls)
	})
}

type failingAvatarStore struct{ user.Store }

func (s failingAvatarStore) UpdateAvatar(context.Context, int64, string) (*user.User, string, error) {
	return nil, "", errors.New("database unavailable")
}

func TestAvatarHandler_Replacement(t *testing.T) {
	for _, mode := range []string{"success", "database failure", "storage failure", "cleanup failure"} {
		t.Run(mode, func(t *testing.T) {
			ctx := context.Background()
			users := userimpl.NewMockStore()
			u, err := users.CreateWithPassword(ctx, "a@example.com", "Ann", "hash")
			require.NoError(t, err)
			const oldURL = "https://cdn.example/avatars/1?v=1"
			_, _, err = users.UpdateAvatar(ctx, u.ID, oldURL)
			require.NoError(t, err)
			st := &fakeAvatarStorage{}
			var store user.Store = users
			if mode == "database failure" {
				store = failingAvatarStore{users}
			}
			if mode == "storage failure" {
				st.putErr = errors.New("storage unavailable")
			}
			if mode == "cleanup failure" {
				st.deleteErr = errors.New("delete unavailable")
			}
			st.onDelete = func(deletedURL string) {
				current, err := users.GetByID(ctx, u.ID)
				require.NoError(t, err)
				require.NotEqual(t, *current.AvatarURL, deletedURL, "must never delete the image referenced by the DB")
			}
			rec := serveAvatarRequest(store, st, avatarRequest(t, "avatar", avatarPNG(t, 2, 3)))
			current, err := users.GetByID(ctx, u.ID)
			require.NoError(t, err)
			if mode == "database failure" || mode == "storage failure" {
				require.Equal(t, 500, rec.Code, rec.Body.String())
				assert.Equal(t, oldURL, *current.AvatarURL)
				assert.NotContains(t, st.deleted, oldURL)
				if mode == "database failure" {
					assert.Equal(t, []string{"https://cdn.example/" + st.key}, st.deleted)
				}
				return
			}
			require.Equal(t, 200, rec.Code, rec.Body.String())
			assert.NotEqual(t, oldURL, *current.AvatarURL)
			require.Equal(t, []string{oldURL}, st.deleted)
			firstURL := *current.AvatarURL
			rec = serveAvatarRequest(users, st, avatarRequest(t, "avatar", avatarPNG(t, 2, 3)))
			require.Equal(t, 200, rec.Code, rec.Body.String())
			current, err = users.GetByID(ctx, u.ID)
			require.NoError(t, err)
			assert.NotEqual(t, firstURL, *current.AvatarURL)
			assert.Equal(t, firstURL, st.deleted[1])
		})
	}
}

type uncertainAvatarStore struct {
	user.Store
	committed  bool
	readFailed bool
}

func (s uncertainAvatarStore) UpdateAvatar(ctx context.Context, id int64, url string) (*user.User, string, error) {
	if s.committed {
		if _, _, err := s.Store.UpdateAvatar(ctx, id, url); err != nil {
			return nil, "", err
		}
	}
	return nil, "", errors.New("lost database connection")
}

func (s uncertainAvatarStore) GetByID(ctx context.Context, id int64) (*user.User, error) {
	if s.readFailed {
		return nil, errors.New("database still unavailable")
	}
	return s.Store.GetByID(ctx, id)
}

func TestAvatarHandler_UncertainCommitRetainsUploadedImage(t *testing.T) {
	for _, committed := range []bool{false, true} {
		for _, readFailed := range []bool{false, true} {
			t.Run(fmt.Sprintf("committed=%v/readFailed=%v", committed, readFailed), func(t *testing.T) {
				users := userimpl.NewMockStore()
				_, err := users.CreateWithPassword(context.Background(), "a@example.com", "Ann", "hash")
				require.NoError(t, err)
				st := &fakeAvatarStorage{}
				rec := serveAvatarRequest(uncertainAvatarStore{users, committed, readFailed}, st, avatarRequest(t, "avatar", avatarPNG(t, 2, 3)))
				require.Equal(t, 500, rec.Code)
				if committed || readFailed {
					require.Empty(t, st.deleted, "an uncertain or committed URL must remain available")
				} else {
					require.Equal(t, []string{"https://cdn.example/" + st.key}, st.deleted)
				}
			})
		}
	}
}
