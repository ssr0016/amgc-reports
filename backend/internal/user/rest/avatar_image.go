package rest

import (
	"bytes"
	"errors"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"

	"golang.org/x/image/riff"
	"golang.org/x/image/vp8"
	"golang.org/x/image/vp8l"
	_ "golang.org/x/image/webp"
)

const maxAvatarDimension = 512

var (
	avatarFormats = map[string]string{"jpeg": "image/jpeg", "png": "image/png", "webp": "image/webp"}
	avatarWebP    = riff.FourCC{'W', 'E', 'B', 'P'}
	avatarVP8     = riff.FourCC{'V', 'P', '8', ' '}
	avatarVP8L    = riff.FourCC{'V', 'P', '8', 'L'}
)

func validateAvatarImage(data []byte) (string, error) {
	invalid := func() (string, error) { return "", fieldError("avatar", "must be a valid JPEG, PNG or WebP image") }
	config, format, err := image.DecodeConfig(bytes.NewReader(data))
	contentType, allowed := avatarFormats[format]
	if err != nil || !allowed {
		return invalid()
	}
	if format == "webp" {
		frame, err := webPFrameConfig(data)
		if err != nil || frame.Width != config.Width || frame.Height != config.Height {
			return invalid()
		}
	}
	if !validAvatarDimensions(config.Width, config.Height) {
		return "", fieldError("avatar", "must be at most 512 pixels per side")
	}
	if _, _, err := image.Decode(bytes.NewReader(data)); err != nil {
		return invalid()
	}
	return contentType, nil
}

func validAvatarDimensions(width, height int) bool {
	return width > 0 && height > 0 && width <= maxAvatarDimension && height <= maxAvatarDimension
}

// VP8X can report a canvas smaller than the underlying VP8/VP8L frame.
func webPFrameConfig(data []byte) (image.Config, error) {
	form, chunks, err := riff.NewReader(bytes.NewReader(data))
	if err != nil || form != avatarWebP {
		return image.Config{}, errors.New("invalid WebP container")
	}
	for {
		kind, length, chunk, err := chunks.Next()
		if err != nil {
			return image.Config{}, err
		}
		switch kind {
		case avatarVP8:
			decoder := vp8.NewDecoder()
			decoder.Init(chunk, int(length))
			header, err := decoder.DecodeFrameHeader()
			return image.Config{Width: header.Width, Height: header.Height}, err
		case avatarVP8L:
			return vp8l.DecodeConfig(chunk)
		}
		if _, err := io.Copy(io.Discard, chunk); err != nil {
			return image.Config{}, err
		}
	}
}
