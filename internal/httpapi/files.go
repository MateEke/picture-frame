package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"

	"github.com/MateEke/picture-frame/internal/files"
)

const (
	// maxFileUploadBytes caps one non-image upload (a long phone video fits).
	maxFileUploadBytes = 4 << 30
	// diskReserveBytes is kept free so uploads can't starve the OS, logs or images.
	diskReserveBytes = 256 << 20
)

type FileItem struct {
	Name     string    `json:"name" doc:"Stored filename"`
	Original string    `json:"original" doc:"Filename as uploaded"`
	Size     int64     `json:"size" doc:"Size in bytes"`
	MIME     string    `json:"mime" doc:"Media type"`
	Added    time.Time `json:"added" doc:"Upload time"`
}

func fileItem(m files.Meta) FileItem {
	return FileItem{Name: m.Name, Original: m.Original, Size: m.Size, MIME: m.MIME, Added: m.Added}
}

type ListFilesOutput struct {
	Body struct {
		Files     []FileItem `json:"files"`
		FreeBytes *uint64    `json:"free_bytes,omitempty" doc:"Space left on the storage, when known"`
	}
}

type FileNameInput struct {
	Name string `path:"name" pattern:"^[a-zA-Z0-9_~-][a-zA-Z0-9_.~-]*$" maxLength:"120" doc:"Stored filename"`
}

type ServeFileInput struct {
	FileNameInput
	Download bool `query:"download" doc:"Send as an attachment instead of inline"`
}

func (s *server) registerFileRoutes(api huma.API) {
	// The touch UI lists, previews and deletes files over loopback.
	s.kioskExempt("/api/files")
	s.kioskExemptPrefix("/api/files/")
	s.kioskExemptPrefix("/files/")

	huma.Register(api, huma.Operation{
		OperationID: "list-files",
		Method:      http.MethodGet,
		Path:        "/api/files",
		Summary:     "List uploaded non-image files",
	}, func(_ context.Context, _ *struct{}) (*ListFilesOutput, error) {
		out := &ListFilesOutput{}
		out.Body.Files = []FileItem{}
		if s.files == nil {
			return out, nil
		}
		for _, m := range s.files.List() {
			out.Body.Files = append(out.Body.Files, fileItem(m))
		}
		if free, ok := s.files.FreeBytes(); ok {
			out.Body.FreeBytes = &free
		}
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "delete-file",
		Method:        http.MethodDelete,
		Path:          "/api/files/{name}",
		Summary:       "Delete an uploaded file",
		DefaultStatus: http.StatusNoContent,
	}, func(_ context.Context, input *FileNameInput) (*struct{}, error) {
		if s.files == nil {
			return nil, huma.Error503ServiceUnavailable("file storage unavailable")
		}
		if err := s.files.Delete(input.Name); err != nil {
			if errors.Is(err, files.ErrNotFound) {
				return nil, huma.Error404NotFound("file not found")
			}
			s.log.Error("failed to delete file", "name", input.Name, "err", err)
			return nil, huma.Error500InternalServerError("failed to delete file")
		}
		s.publishLibraryChanged()
		return nil, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "serve-file",
		Method:      http.MethodGet,
		Path:        "/files/{name}",
		Summary:     "Serve an uploaded file (supports Range for media seeking)",
	}, s.handleServeFile)
}

func (s *server) handleServeFile(_ context.Context, input *ServeFileInput) (*huma.StreamResponse, error) {
	if s.files == nil {
		return nil, huma.Error404NotFound("file not found")
	}
	f, meta, err := s.files.OpenFile(input.Name)
	if err != nil {
		if errors.Is(err, files.ErrNotFound) {
			return nil, huma.Error404NotFound("file not found")
		}
		s.log.Error("failed to open file", "name", input.Name, "err", err)
		return nil, huma.Error500InternalServerError("failed to open file")
	}
	disposition := "inline"
	if input.Download {
		disposition = "attachment"
	}
	return &huma.StreamResponse{
		Body: func(ctx huma.Context) {
			defer f.Close()
			r, w := humachi.Unwrap(ctx)
			h := w.Header()
			h.Set("Content-Type", meta.MIME)
			h.Set("Content-Disposition", mime.FormatMediaType(disposition, map[string]string{"filename": meta.Original}))
			// Uploaded HTML/SVG opened directly must not run as our origin.
			h.Set("Content-Security-Policy", "sandbox")
			h.Set("X-Content-Type-Options", "nosniff")
			h.Set("Cache-Control", "private, max-age=3600")
			http.ServeContent(w, r, meta.Name, meta.Added, f)
		},
	}, nil
}

// handleUploadFile streams multipart parts straight to disk. It's a plain chi
// handler (not huma) because huma's multipart input buffers the whole body to
// a temp file first, doubling SD-card writes for multi-GB videos.
func (s *server) handleUploadFile(w http.ResponseWriter, r *http.Request) {
	writeErr := func(status int, msg string) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(map[string]string{"detail": msg})
	}
	if s.files == nil {
		writeErr(http.StatusServiceUnavailable, "file storage unavailable")
		return
	}
	if free, ok := s.files.FreeBytes(); ok && r.ContentLength > 0 &&
		uint64(r.ContentLength)+diskReserveBytes > free { //nolint:gosec // ContentLength > 0 checked
		writeErr(http.StatusInsufficientStorage, "not enough free space")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxFileUploadBytes+1<<20)
	mr, err := r.MultipartReader()
	if err != nil {
		writeErr(http.StatusBadRequest, "expected multipart/form-data")
		return
	}
	saved := []FileItem{}
	for {
		part, err := mr.NextPart()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			writeErr(http.StatusBadRequest, "malformed upload")
			return
		}
		if part.FileName() == "" {
			part.Close()
			continue
		}
		m, err := s.files.Save(part.FileName(), part, maxFileUploadBytes)
		part.Close()
		if err != nil {
			var maxErr *http.MaxBytesError
			if errors.Is(err, files.ErrTooLarge) || errors.As(err, &maxErr) {
				writeErr(http.StatusRequestEntityTooLarge, "file too large")
				return
			}
			s.log.Error("failed to save file", "name", part.FileName(), "err", err)
			writeErr(http.StatusInternalServerError, "failed to save file")
			return
		}
		saved = append(saved, fileItem(m))
	}
	if len(saved) == 0 {
		writeErr(http.StatusBadRequest, "no file in upload")
		return
	}
	s.publishLibraryChanged()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(saved)
}
