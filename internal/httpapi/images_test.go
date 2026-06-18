package httpapi_test

// Coverage note: three error branches in images.go are intentionally left
// uncovered because they guard against failures that cannot be triggered
// through inputs, only by injecting fakes for rand/fs into production code,
// which isn't worth the test-only surface:
//   - createImageFile: crypto/rand.Read returning an error
//   - createImageFile: uploadNameRetries exhausted (same-ms name collisions)
//   - handleServeImage: Stat failing on a just-opened descriptor

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"hash/crc32"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/MateEke/picture-frame/internal/httpapi"
	"github.com/MateEke/picture-frame/internal/library"
	"github.com/MateEke/picture-frame/internal/state"
	"github.com/MateEke/picture-frame/internal/testutil"
)

type fakeSlideshow struct{ calls atomic.Int32 }

func (f *fakeSlideshow) Next() { f.calls.Add(1) }

type imageHarness struct {
	handler   http.Handler
	lib       *library.Library
	bus       *state.Bus
	root      *os.Root
	slideshow *fakeSlideshow
}

func newImageServer(t *testing.T) *imageHarness {
	return newImageServerWithBackend(t, "")
}

func newImageServerWithBackend(t *testing.T, backend string) *imageHarness {
	t.Helper()
	dir := t.TempDir()
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatalf("OpenRoot: %v", err)
	}
	t.Cleanup(func() { _ = root.Close() })
	lib := library.New(nil, false)
	bus := state.NewBus()
	ss := &fakeSlideshow{}
	h := httpapi.NewServer(httpapi.Config{
		Log:         testutil.NopLogger(),
		Bus:         bus,
		Library:     lib,
		ImagesRoot:  root,
		Slideshow:   ss,
		KioskBeater: &fakeBeater{},
		Backend:     backend,
	})
	return &imageHarness{handler: h, lib: lib, bus: bus, root: root, slideshow: ss}
}

func newImageServerWithAspect(t *testing.T) (*imageHarness, *library.AspectStore) {
	t.Helper()
	dir := t.TempDir()
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatalf("OpenRoot: %v", err)
	}
	t.Cleanup(func() { _ = root.Close() })
	lib := library.New(nil, false)
	bus := state.NewBus()
	ss := &fakeSlideshow{}
	aspect, err := library.LoadAspectStore(testutil.NopLogger(), root)
	if err != nil {
		t.Fatalf("LoadAspectStore: %v", err)
	}
	h := httpapi.NewServer(httpapi.Config{
		Log: testutil.NopLogger(), Bus: bus, Library: lib, ImagesRoot: root,
		Slideshow: ss, KioskBeater: &fakeBeater{}, Aspect: aspect,
	})
	return &imageHarness{handler: h, lib: lib, bus: bus, root: root, slideshow: ss}, aspect
}

func uploadedName(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var item struct{ Name string }
	if err := json.NewDecoder(rec.Body).Decode(&item); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return item.Name
}

func TestUploadStoresAspectJPEG(t *testing.T) {
	h, aspect := newImageServerWithAspect(t)
	rec := httptest.NewRecorder()
	h.handler.ServeHTTP(rec, uploadRequest(t, "image", "x.jpg", makeJPEG(t)))
	if rec.Code != http.StatusCreated {
		t.Fatalf("status %d, body %s", rec.Code, rec.Body)
	}
	ratio, ok := aspect.Ratio(uploadedName(t, rec))
	if !ok || ratio != 1.0 {
		t.Errorf("aspect = %v ok=%v, want 1.0 (8x8)", ratio, ok)
	}
}

func TestUploadStoresAspectPNG(t *testing.T) {
	h, aspect := newImageServerWithAspect(t)
	rec := httptest.NewRecorder()
	h.handler.ServeHTTP(rec, uploadRequest(t, "image", "x.png", makePNG(t)))
	if rec.Code != http.StatusCreated {
		t.Fatalf("status %d, body %s", rec.Code, rec.Body)
	}
	ratio, ok := aspect.Ratio(uploadedName(t, rec))
	if !ok || ratio != 1.0 {
		t.Errorf("aspect = %v ok=%v, want 1.0 (8x8)", ratio, ok)
	}
}

func TestDeleteRemovesAspect(t *testing.T) {
	h, aspect := newImageServerWithAspect(t)
	rec := httptest.NewRecorder()
	h.handler.ServeHTTP(rec, uploadRequest(t, "image", "x.jpg", makeJPEG(t)))
	name := uploadedName(t, rec)
	if _, ok := aspect.Ratio(name); !ok {
		t.Fatal("expected aspect stored after upload")
	}

	rec2 := httptest.NewRecorder()
	h.handler.ServeHTTP(rec2, httptest.NewRequest(http.MethodDelete, "/api/images/"+name, nil))
	if rec2.Code != http.StatusNoContent {
		t.Fatalf("delete status %d", rec2.Code)
	}
	if _, ok := aspect.Ratio(name); ok {
		t.Error("aspect should be removed after delete")
	}
}

func makeJPEG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 8, 8))
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, nil); err != nil {
		t.Fatalf("jpeg encode: %v", err)
	}
	return buf.Bytes()
}

func makePNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 8, 8))
	img.Set(0, 0, color.RGBA{255, 0, 0, 255})
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("png encode: %v", err)
	}
	return buf.Bytes()
}

func makeGIF(t *testing.T) []byte {
	t.Helper()
	img := image.NewPaletted(image.Rect(0, 0, 8, 8), color.Palette{color.Black, color.White})
	var buf bytes.Buffer
	if err := gif.Encode(&buf, img, nil); err != nil {
		t.Fatalf("gif encode: %v", err)
	}
	return buf.Bytes()
}

func uploadRequest(t *testing.T, fieldName, filename string, data []byte) *http.Request {
	t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	fw, err := mw.CreateFormFile(fieldName, filename)
	if err != nil {
		t.Fatalf("CreateFormFile: %v", err)
	}
	if _, err := fw.Write(data); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if err := mw.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/images", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	return req
}

func TestListImagesEmpty(t *testing.T) {
	h := newImageServer(t)
	rec := httptest.NewRecorder()
	h.handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/images", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("got %d, want 200", rec.Code)
	}
	if got := strings.TrimSpace(rec.Body.String()); got != "[]" {
		t.Errorf("body: got %q, want []", got)
	}
}

func TestListImagesNonEmpty(t *testing.T) {
	h := newImageServer(t)
	h.lib.Add("a.jpg")
	h.lib.Add("b.jpg")
	rec := httptest.NewRecorder()
	h.handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/images", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("got %d, want 200", rec.Code)
	}
	var items []struct{ Name string }
	if err := json.NewDecoder(rec.Body).Decode(&items); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("got %d items, want 2", len(items))
	}
}

func TestUploadJPEGHappyPath(t *testing.T) {
	h := newImageServer(t)
	rec := httptest.NewRecorder()
	h.handler.ServeHTTP(rec, uploadRequest(t, "image", "x.jpg", makeJPEG(t)))
	if rec.Code != http.StatusCreated {
		t.Fatalf("got %d, want 201; body=%s", rec.Code, rec.Body.String())
	}
	var item struct{ Name string }
	if err := json.NewDecoder(rec.Body).Decode(&item); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !strings.HasSuffix(item.Name, ".jpg") {
		t.Errorf("name: got %q, want .jpg suffix", item.Name)
	}
	if !h.lib.Has(item.Name) {
		t.Error("library missing uploaded image")
	}
	if _, err := h.root.Stat(item.Name); err != nil {
		t.Errorf("file missing on disk: %v", err)
	}
	if got := h.slideshow.calls.Load(); got != 1 {
		t.Errorf("slideshow.Next: got %d calls, want 1", got)
	}
}

// Non-JPEG formats take the decode + re-encode path and must land as JPEG.
func TestUploadReEncodesToJPEG(t *testing.T) {
	cases := []struct {
		name     string
		filename string
		data     func(t *testing.T) []byte
	}{
		{"png", "x.png", makePNG},
		{"gif", "x.gif", makeGIF},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := newImageServer(t)
			rec := httptest.NewRecorder()
			h.handler.ServeHTTP(rec, uploadRequest(t, "image", tc.filename, tc.data(t)))
			if rec.Code != http.StatusCreated {
				t.Fatalf("got %d, want 201; body=%s", rec.Code, rec.Body.String())
			}
			var item struct{ Name string }
			_ = json.NewDecoder(rec.Body).Decode(&item)
			f, err := h.root.Open(item.Name)
			if err != nil {
				t.Fatalf("open saved file: %v", err)
			}
			defer f.Close()
			header := make([]byte, 3)
			if _, err := io.ReadFull(f, header); err != nil {
				t.Fatalf("read header: %v", err)
			}
			if header[0] != 0xFF || header[1] != 0xD8 || header[2] != 0xFF {
				t.Errorf("saved file is not JPEG; header=% x", header)
			}
		})
	}
}

// Closing the root makes OpenFile return ErrClosed (a non-ErrExist error), so
// createImageFile bails and the handler reports 500 rather than retrying.
func TestUploadCreateFileError(t *testing.T) {
	h := newImageServer(t)
	_ = h.root.Close()
	rec := httptest.NewRecorder()
	h.handler.ServeHTTP(rec, uploadRequest(t, "image", "x.jpg", makeJPEG(t)))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("got %d, want 500; body=%s", rec.Code, rec.Body.String())
	}
}

func TestUploadSecondImageDoesNotAdvanceSlideshow(t *testing.T) {
	h := newImageServer(t)
	rec := httptest.NewRecorder()
	h.handler.ServeHTTP(rec, uploadRequest(t, "image", "1.jpg", makeJPEG(t)))
	h.handler.ServeHTTP(httptest.NewRecorder(), uploadRequest(t, "image", "2.jpg", makeJPEG(t)))
	if got := h.slideshow.calls.Load(); got != 1 {
		t.Errorf("slideshow.Next: got %d calls, want 1 (only on first upload)", got)
	}
}

// Every rejected upload must return the right status + message and leave no
// library entry behind. The 500 case shares the same post-conditions, so it
// rides along here rather than in its own test.
func TestUploadRejected(t *testing.T) {
	cases := []struct {
		name       string
		newReq     func(t *testing.T) *http.Request
		wantStatus int
		wantBody   string
	}{
		{
			name:       "missing image field",
			newReq:     func(t *testing.T) *http.Request { return uploadRequest(t, "wrong", "x.jpg", makeJPEG(t)) },
			wantStatus: http.StatusUnprocessableEntity,
			wantBody:   "File required",
		},
		{
			name: "non-multipart body",
			newReq: func(_ *testing.T) *http.Request {
				req := httptest.NewRequest(http.MethodPost, "/api/images", strings.NewReader("hello"))
				req.Header.Set("Content-Type", "text/plain")
				return req
			},
			wantStatus: http.StatusUnprocessableEntity,
			wantBody:   "multipart",
		},
		{
			// A part header line with no colon is a malformed MIME header, so
			// Huma's multipart parser rejects the form.
			name: "malformed part header",
			newReq: func(_ *testing.T) *http.Request {
				boundary := "abc123"
				body := "--" + boundary + "\r\nthis-is-not-a-valid-header-line\r\n\r\ndata\r\n--" + boundary + "--\r\n"
				req := httptest.NewRequest(http.MethodPost, "/api/images", strings.NewReader(body))
				req.Header.Set("Content-Type", "multipart/form-data; boundary="+boundary)
				return req
			},
			wantStatus: http.StatusUnprocessableEntity,
			wantBody:   "multipart",
		},
		{
			name: "unrecognized bytes",
			newReq: func(t *testing.T) *http.Request {
				return uploadRequest(t, "image", "x.jpg", []byte("definitely not an image, just plain text content here"))
			},
			wantStatus: http.StatusBadRequest,
			wantBody:   "invalid image",
		},
		{
			// Valid PNG signature + garbage sniffs as image/png but fails the
			// DecodeConfig dimension probe → rejected as a client error.
			name: "corrupt image header",
			newReq: func(t *testing.T) *http.Request {
				pngSig := []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}
				return uploadRequest(t, "image", "x.png", append(pngSig, []byte("not actually a valid PNG body at all")...))
			},
			wantStatus: http.StatusBadRequest,
			wantBody:   "invalid image",
		},
		{
			// Intact IHDR (passes the dimension probe) but truncated pixel data
			// fails image.Decode, exercising the decode-error branch → 500.
			name: "corrupt image body",
			newReq: func(t *testing.T) *http.Request {
				return uploadRequest(t, "image", "x.png", makePNG(t)[:40])
			},
			wantStatus: http.StatusInternalServerError,
			wantBody:   "failed to save image",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := newImageServer(t)
			rec := httptest.NewRecorder()
			h.handler.ServeHTTP(rec, tc.newReq(t))
			if rec.Code != tc.wantStatus {
				t.Fatalf("status: got %d, want %d; body=%s", rec.Code, tc.wantStatus, rec.Body.String())
			}
			if !strings.Contains(rec.Body.String(), tc.wantBody) {
				t.Errorf("body: got %q, want substring %q", rec.Body.String(), tc.wantBody)
			}
			if got := len(h.lib.List()); got != 0 {
				t.Errorf("library has %d entries after rejected upload, want 0", got)
			}
		})
	}
}

// zeroReader yields n bytes of 0xFF then EOF without allocating that much memory.
type zeroReader struct{ remaining int64 }

func (z *zeroReader) Read(p []byte) (int, error) {
	if z.remaining <= 0 {
		return 0, io.EOF
	}
	n := min(int64(len(p)), z.remaining)
	for i := range p[:n] {
		p[i] = 0xFF
	}
	z.remaining -= n
	return int(n), nil
}

func TestUploadOversizedReturns413(t *testing.T) {
	h := newImageServer(t)

	// Build the multipart prefix containing a valid JPEG sniff prefix, then
	// stream ~51MB of padding so the handler reads past the 50MB cap without
	// allocating the payload up front.
	var prefix bytes.Buffer
	mw := multipart.NewWriter(&prefix)
	fw, err := mw.CreateFormFile("image", "big.jpg")
	if err != nil {
		t.Fatalf("CreateFormFile: %v", err)
	}
	if _, err := fw.Write(makeJPEG(t)); err != nil {
		t.Fatalf("write jpeg prefix: %v", err)
	}
	boundary := mw.Boundary()
	// We can't .Close() the writer here because closing writes the trailing
	// boundary into prefix; we want padding before that. Write the trailer
	// manually after the padding stream below.
	trailer := []byte("\r\n--" + boundary + "--\r\n")
	body := io.MultiReader(&prefix, &zeroReader{remaining: 51 << 20}, bytes.NewReader(trailer))

	req := httptest.NewRequest(http.MethodPost, "/api/images", body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	rec := httptest.NewRecorder()
	h.handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("got %d, want 413", rec.Code)
	}
}

func TestDeleteInvalidName(t *testing.T) {
	h := newImageServer(t)
	rec := httptest.NewRecorder()
	h.handler.ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, "/api/images/..%2F..%2Fetc%2Fpasswd", nil))
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("got %d, want 422", rec.Code)
	}
}

func TestDeleteNotFound(t *testing.T) {
	h := newImageServer(t)
	rec := httptest.NewRecorder()
	h.handler.ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, "/api/images/nope.jpg", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("got %d, want 404", rec.Code)
	}
}

func TestDeleteHappyPath(t *testing.T) {
	h := newImageServer(t)
	// Upload first so the library + file exist.
	up := httptest.NewRecorder()
	h.handler.ServeHTTP(up, uploadRequest(t, "image", "x.jpg", makeJPEG(t)))
	var item struct{ Name string }
	_ = json.NewDecoder(up.Body).Decode(&item)

	// Subscribe before deleting so we observe the cleared-image event.
	events, unsub := h.bus.Subscribe()
	defer unsub()
	// Drain any backlog from the upload path.
	drained := false
	for !drained {
		select {
		case <-events:
		default:
			drained = true
		}
	}

	rec := httptest.NewRecorder()
	h.handler.ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, "/api/images/"+item.Name, nil))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("got %d, want 204", rec.Code)
	}
	if h.lib.Has(item.Name) {
		t.Error("library still has image after delete")
	}
	if _, err := h.root.Stat(item.Name); !os.IsNotExist(err) {
		t.Errorf("file still on disk: err=%v", err)
	}
	select {
	case ev := <-events:
		if ev.Kind != state.KindImage {
			t.Errorf("expected KindImage event, got %v", ev.Kind)
		}
	default:
		t.Error("expected cleared-image event after deleting last image")
	}
}

func TestDeleteFileAlreadyGone(t *testing.T) {
	h := newImageServer(t)
	// Library entry present, no file on disk.
	h.lib.Add("ghost.jpg")
	rec := httptest.NewRecorder()
	h.handler.ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, "/api/images/ghost.jpg", nil))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("got %d, want 204", rec.Code)
	}
	if h.lib.Has("ghost.jpg") {
		t.Error("library still has image after delete")
	}
}

// A non-empty directory named like an image makes os.Root.Remove fail with a
// real (non-ENOENT) error, exercising the failure branch. The library entry
// must survive so the request stays retryable.
func TestDeleteFileRemoveError(t *testing.T) {
	h := newImageServer(t)
	if err := h.root.Mkdir("stuck.jpg", 0o755); err != nil {
		t.Fatalf("Mkdir: %v", err)
	}
	f, err := h.root.OpenFile("stuck.jpg/child", os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatalf("OpenFile child: %v", err)
	}
	_ = f.Close()
	h.lib.Add("stuck.jpg")

	rec := httptest.NewRecorder()
	h.handler.ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, "/api/images/stuck.jpg", nil))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("got %d, want 500", rec.Code)
	}
	if !h.lib.Has("stuck.jpg") {
		t.Error("library dropped entry despite file-remove failure")
	}
}

func TestServeInvalidName(t *testing.T) {
	h := newImageServer(t)
	rec := httptest.NewRecorder()
	h.handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/img/..%2Fetc", nil))
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("got %d, want 422", rec.Code)
	}
}

func TestServeNotFound(t *testing.T) {
	h := newImageServer(t)
	rec := httptest.NewRecorder()
	h.handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/img/nope.jpg", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("got %d, want 404", rec.Code)
	}
}

// A closed root makes Open fail with ErrClosed (not NotExist), so serving falls
// through to the 500 branch rather than 404.
func TestServeOpenError(t *testing.T) {
	h := newImageServer(t)
	_ = h.root.Close()
	rec := httptest.NewRecorder()
	h.handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/img/x.jpg", nil))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("got %d, want 500", rec.Code)
	}
}

func TestServeHappyPath(t *testing.T) {
	h := newImageServer(t)
	up := httptest.NewRecorder()
	h.handler.ServeHTTP(up, uploadRequest(t, "image", "x.jpg", makeJPEG(t)))
	var item struct{ Name string }
	_ = json.NewDecoder(up.Body).Decode(&item)

	rec := httptest.NewRecorder()
	h.handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/img/"+item.Name, nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("got %d, want 200", rec.Code)
	}
	if cc := rec.Header().Get("Cache-Control"); !strings.Contains(cc, "immutable") {
		t.Errorf("Cache-Control: got %q, want immutable", cc)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "image/jpeg" {
		t.Errorf("Content-Type: got %q, want image/jpeg", ct)
	}
	if rec.Body.Len() < 10 {
		t.Errorf("body too short: %d bytes", rec.Body.Len())
	}
}

func TestSlideshowNextEndpoint(t *testing.T) {
	h := newImageServer(t)
	rec := httptest.NewRecorder()
	h.handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/slideshow/next", nil))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("got %d, want 204", rec.Code)
	}
	if got := h.slideshow.calls.Load(); got != 1 {
		t.Errorf("slideshow.Next: got %d, want 1", got)
	}
}

func TestUploadDisabledOnRemoteBackend(t *testing.T) {
	h := newImageServerWithBackend(t, "immich")
	req := uploadRequest(t, "image", "x.jpg", makeJPEG(t))
	rec := httptest.NewRecorder()
	h.handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusConflict {
		t.Errorf("upload status = %d, want 409", rec.Code)
	}
}

func TestDeleteDisabledOnRemoteBackend(t *testing.T) {
	h := newImageServerWithBackend(t, "immich")
	rec := httptest.NewRecorder()
	h.handler.ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, "/api/images/foo.jpg", nil))
	if rec.Code != http.StatusConflict {
		t.Errorf("delete status = %d, want 409", rec.Code)
	}
}

// pngHeader is a valid PNG signature + IHDR declaring the given dimensions,
// with no pixel data: enough for the DecodeConfig guard, nothing more.
func pngHeader(t *testing.T, w, h uint32) []byte {
	t.Helper()
	const ihdrLen = 13
	var buf bytes.Buffer
	buf.Write([]byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'})
	ihdr := make([]byte, ihdrLen)
	binary.BigEndian.PutUint32(ihdr[0:4], w)
	binary.BigEndian.PutUint32(ihdr[4:8], h)
	ihdr[8] = 8 // bit depth
	ihdr[9] = 6 // color type RGBA
	chunk := append([]byte("IHDR"), ihdr...)
	_ = binary.Write(&buf, binary.BigEndian, uint32(ihdrLen))
	buf.Write(chunk)
	_ = binary.Write(&buf, binary.BigEndian, crc32.ChecksumIEEE(chunk))
	return buf.Bytes()
}

// gifHeader is a GIF89a header + logical screen descriptor with the given
// dimensions (GIF allows zero, unlike PNG).
func gifHeader(t *testing.T, w, h uint16) []byte {
	t.Helper()
	var buf bytes.Buffer
	buf.WriteString("GIF89a")
	_ = binary.Write(&buf, binary.LittleEndian, w)
	_ = binary.Write(&buf, binary.LittleEndian, h)
	buf.Write([]byte{0, 0, 0}) // packed fields, bg color, aspect ratio
	return buf.Bytes()
}

func TestUploadRejectsOversizedDimensions(t *testing.T) {
	cases := []struct {
		name string
		data []byte
	}{
		{"decode bomb", pngHeader(t, 20000, 20000)},
		{"zero width", gifHeader(t, 0, 5)},
		{"zero height", gifHeader(t, 5, 0)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := newImageServer(t)
			rec := httptest.NewRecorder()
			h.handler.ServeHTTP(rec, uploadRequest(t, "image", "bomb.png", tc.data))
			if rec.Code != http.StatusRequestEntityTooLarge {
				t.Fatalf("got %d, want 413: %s", rec.Code, rec.Body.String())
			}
			if h.lib.Len() != 0 {
				t.Error("rejected upload must not enter the library")
			}
		})
	}
}

// Exactly at the pixel cap the guard must let the image through (it then fails
// later in decode, since the header carries no pixel data).
func TestUploadAcceptsDimensionsAtCap(t *testing.T) {
	h := newImageServer(t)
	rec := httptest.NewRecorder()
	h.handler.ServeHTTP(rec, uploadRequest(t, "image", "max.png", pngHeader(t, 6000, 5000)))
	if rec.Code == http.StatusRequestEntityTooLarge {
		t.Fatalf("exact-cap dimensions must pass the guard, got 413: %s", rec.Body.String())
	}
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("truncated body should fail decode with 500, got %d", rec.Code)
	}
}

// The huma `pattern` tags must stay in lockstep with library.ImageNamePattern,
// or the loader admits names the serve route then rejects (kiosk freeze).
func TestImageNameTagsMatchLibraryPattern(t *testing.T) {
	for _, typ := range []reflect.Type{
		reflect.TypeFor[httpapi.DeleteImageInput](),
		reflect.TypeFor[httpapi.ServeImageInput](),
	} {
		field, ok := typ.FieldByName("Name")
		if !ok {
			t.Fatalf("%s has no Name field", typ.Name())
		}
		if got := field.Tag.Get("pattern"); got != library.ImageNamePattern {
			t.Errorf("%s pattern tag %q != library.ImageNamePattern %q", typ.Name(), got, library.ImageNamePattern)
		}
	}
}

// A file on disk that isn't in the library must 404 without being deleted.
func TestDeleteFileNotInLibraryKeepsFile(t *testing.T) {
	h := newImageServer(t)
	f, err := h.root.OpenFile("orphan.jpg", os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatalf("OpenFile: %v", err)
	}
	_ = f.Close()

	rec := httptest.NewRecorder()
	h.handler.ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, "/api/images/orphan.jpg", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("got %d, want 404", rec.Code)
	}
	if _, err := h.root.Stat("orphan.jpg"); err != nil {
		t.Errorf("orphan file should survive the 404: %v", err)
	}
}
