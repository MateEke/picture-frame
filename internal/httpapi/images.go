package httpapi

import (
	"bufio"
	"bytes"
	"context"
	crand "crypto/rand"
	"errors"
	"fmt"
	"image"
	_ "image/gif" // register GIF decoder
	stdjpeg "image/jpeg"
	_ "image/png" // register PNG decoder
	"io"
	"net/http"
	"os"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"

	"github.com/MateEke/picture-frame/internal/config"
	"github.com/MateEke/picture-frame/internal/library"
	"github.com/MateEke/picture-frame/internal/state"
)

const (
	maxUploadBytes    = 50 << 20
	headerPeekBytes   = 512
	uploadNameRetries = 5
	// maxDecodePixels caps the non-JPEG decode: a tiny PNG can declare
	// dimensions whose decode OOMs a 512MB Pi (30MP ≈ 120MB RGBA).
	maxDecodePixels = 30_000_000
)

type ImageItem struct {
	Name string `json:"name" doc:"Image filename"`
}

type ListImagesOutput struct {
	Body []ImageItem
}

type UploadImageForm struct {
	Image huma.FormFile `form:"image" required:"true"`
}

type UploadImageInput struct {
	RawBody huma.MultipartFormFiles[UploadImageForm]
}

type UploadImageOutput struct {
	Body ImageItem
}

type DeleteImageInput struct {
	Name string `path:"name" pattern:"^[a-zA-Z0-9_.~-]+\\.(jpe?g|png)$" doc:"Image filename"`
}

type ServeImageInput struct {
	Name string `path:"name" pattern:"^[a-zA-Z0-9_.~-]+\\.(jpe?g|png)$" doc:"Image filename"`
}

func (s *server) registerImageRoutes(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "list-images",
		Method:      http.MethodGet,
		Path:        "/api/images",
		Summary:     "List all images",
	}, func(_ context.Context, _ *struct{}) (*ListImagesOutput, error) {
		imgs := s.lib.List()
		out := make([]ImageItem, len(imgs))
		for i, img := range imgs {
			out[i] = ImageItem{Name: img.Name}
		}
		return &ListImagesOutput{Body: out}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "upload-image",
		Method:        http.MethodPost,
		Path:          "/api/images",
		Summary:       "Upload an image",
		DefaultStatus: http.StatusCreated,
		MaxBodyBytes:  maxUploadBytes + 4096, // headroom for multipart framing
	}, s.handleUploadImage)

	huma.Register(api, huma.Operation{
		OperationID:   "delete-image",
		Method:        http.MethodDelete,
		Path:          "/api/images/{name}",
		Summary:       "Delete an image",
		DefaultStatus: http.StatusNoContent,
	}, func(_ context.Context, input *DeleteImageInput) (*struct{}, error) {
		if s.backend != config.BackendFS {
			return nil, huma.Error409Conflict("deletes disabled: a remote backend is active")
		}
		// Membership first so a 404 can't delete an on-disk file outside the library.
		if !s.lib.Has(input.Name) {
			return nil, huma.Error404NotFound("image not found")
		}
		if err := s.imagesRoot.Remove(input.Name); err != nil && !os.IsNotExist(err) {
			s.log.Error("failed to delete image file", "name", input.Name, "err", err)
			return nil, huma.Error500InternalServerError("failed to delete image")
		}
		s.lib.Remove(input.Name)
		if s.aspect != nil {
			// In-memory only: the sidecar is a cache, a stale entry for a removed file is
			// never read (the planner queries live names only), and the next upload/sync
			// flush prunes it. Avoids a full-index rewrite per item during bulk delete.
			s.aspect.Delete(input.Name)
		}
		if s.lib.Len() == 0 {
			s.bus.Publish(state.Event{
				Kind:    state.KindImage,
				Payload: state.ImagePayload{},
			})
		}
		return nil, nil
	})

	s.kioskExemptPrefix("/img/")
	huma.Register(api, huma.Operation{
		OperationID: "serve-image",
		Method:      http.MethodGet,
		Path:        "/img/{name}",
		Summary:     "Serve an image file",
	}, func(_ context.Context, input *ServeImageInput) (*huma.StreamResponse, error) {
		f, err := s.imagesRoot.Open(input.Name)
		if err != nil {
			if os.IsNotExist(err) {
				return nil, huma.Error404NotFound("image not found")
			}
			s.log.Error("failed to open image", "name", input.Name, "err", err)
			return nil, huma.Error500InternalServerError("failed to open image")
		}
		info, err := f.Stat()
		if err != nil {
			f.Close()
			s.log.Error("failed to stat image", "name", input.Name, "err", err)
			return nil, huma.Error500InternalServerError("failed to open image")
		}
		return &huma.StreamResponse{
			Body: func(ctx huma.Context) {
				defer f.Close()
				r, w := humachi.Unwrap(ctx)
				w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
				http.ServeContent(w, r, input.Name, info.ModTime(), f)
			},
		}, nil
	})
}

func (s *server) registerSlideshowRoutes(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID:   "slideshow-next",
		Method:        http.MethodPost,
		Path:          "/api/slideshow/next",
		Summary:       "Advance slideshow to next image",
		DefaultStatus: http.StatusNoContent,
	}, func(_ context.Context, _ *struct{}) (*struct{}, error) {
		if s.slideshow != nil {
			s.slideshow.Next()
		}
		return nil, nil
	})
}

func (s *server) handleUploadImage(_ context.Context, input *UploadImageInput) (*UploadImageOutput, error) {
	if s.backend != config.BackendFS {
		return nil, huma.Error409Conflict("uploads disabled: a remote backend is active")
	}

	file := input.RawBody.Data().Image
	defer file.Close()

	if file.Size > maxUploadBytes {
		return nil, huma.Error413RequestEntityTooLarge("image too large")
	}

	br := bufio.NewReaderSize(file, headerPeekBytes)
	sniff, err := br.Peek(headerPeekBytes)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, huma.Error400BadRequest("invalid image")
	}
	format, ok := sniffImageFormat(sniff)
	if !ok {
		return nil, huma.Error400BadRequest("invalid image")
	}
	if format != "jpeg" {
		// PNG/GIF dimensions sit within the sniff bytes, so DecodeConfig
		// doesn't consume the stream.
		imgCfg, _, err := image.DecodeConfig(bytes.NewReader(sniff))
		if err != nil {
			return nil, huma.Error400BadRequest("invalid image")
		}
		// int64: the pixel product overflows 32-bit int on ARM.
		w, h := int64(imgCfg.Width), int64(imgCfg.Height)
		if w <= 0 || h <= 0 || w*h > maxDecodePixels {
			return nil, huma.Error413RequestEntityTooLarge("image dimensions too large")
		}
	}

	name, f, err := s.createImageFile()
	if err != nil {
		s.log.Error("failed to create image file", "err", err)
		return nil, huma.Error500InternalServerError("failed to save image")
	}

	var writeErr error
	var w, h int
	if format == "jpeg" {
		_, writeErr = io.Copy(f, br)
	} else {
		img, _, decodeErr := image.Decode(br)
		if decodeErr != nil {
			writeErr = decodeErr
		} else {
			b := img.Bounds()
			w, h = b.Dx(), b.Dy() // captured free; re-encoded JPEG keeps these dims
			writeErr = stdjpeg.Encode(f, img, &stdjpeg.Options{Quality: 85})
		}
	}
	closeErr := f.Close()
	if writeErr != nil || closeErr != nil {
		_ = s.imagesRoot.Remove(name)
		s.log.Error("failed to save image", "format", format, "writeErr", writeErr, "closeErr", closeErr)
		return nil, huma.Error500InternalServerError("failed to save image")
	}

	if format == "jpeg" {
		// JPEG was streamed, not decoded; read its header now.
		w, h = library.ImageDimensions(s.imagesRoot, name)
	}
	s.recordAspect(name, w, h)

	wasEmpty := s.lib.Len() == 0
	s.lib.Add(name)
	if wasEmpty && s.slideshow != nil {
		s.slideshow.Next()
	}

	return &UploadImageOutput{Body: ImageItem{Name: name}}, nil
}

func (s *server) recordAspect(name string, w, h int) {
	if s.aspect == nil || w <= 0 || h <= 0 {
		return
	}
	s.aspect.Set(name, w, h)
	if err := s.aspect.Flush(); err != nil {
		s.log.Warn("failed to persist aspect index", "err", err)
	}
}

// sniffImageFormat maps net/http content sniffing to a stdlib image format name.
func sniffImageFormat(sniff []byte) (string, bool) {
	switch http.DetectContentType(sniff) {
	case "image/jpeg":
		return "jpeg", true
	case "image/png":
		return "png", true
	case "image/gif":
		return "gif", true
	default:
		return "", false
	}
}

// createImageFile opens a fresh, uniquely-named file in the images root,
// retrying on the rare O_EXCL collision.
func (s *server) createImageFile() (string, *os.File, error) {
	for range uploadNameRetries {
		var buf [8]byte
		if _, err := crand.Read(buf[:]); err != nil {
			return "", nil, fmt.Errorf("random: %w", err)
		}
		name := fmt.Sprintf("%d-%x.jpg", time.Now().UnixMilli(), buf)
		f, err := s.imagesRoot.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
		if err == nil {
			return name, f, nil
		}
		if !errors.Is(err, os.ErrExist) {
			return "", nil, err
		}
	}
	return "", nil, errors.New("could not allocate unique image filename")
}
