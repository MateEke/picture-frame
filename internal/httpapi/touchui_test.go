package httpapi_test

import (
	"bytes"
	"encoding/json"
	"image/jpeg"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/MateEke/picture-frame/internal/files"
	"github.com/MateEke/picture-frame/internal/httpapi"
	"github.com/MateEke/picture-frame/internal/library"
	"github.com/MateEke/picture-frame/internal/state"
	"github.com/MateEke/picture-frame/internal/testutil"
)

type touchHarness struct {
	*imageHarness
	exclude *library.ExcludeStore
	files   *files.Store
	fileDir string
}

func newTouchServer(t *testing.T) *touchHarness {
	t.Helper()
	root, err := os.OpenRoot(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = root.Close() })
	lib := library.New(nil, false)
	bus := state.NewBus()
	ss := &fakeSlideshow{}
	exclude, _, err := library.LoadExcludeStore(testutil.NopLogger(), root)
	if err != nil {
		t.Fatal(err)
	}
	thumbs, err := library.NewThumbStore(testutil.NopLogger(), root)
	if err != nil {
		t.Fatal(err)
	}
	fileDir := t.TempDir()
	fs, err := files.Open(testutil.NopLogger(), fileDir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = fs.Close() })
	h := httpapi.NewServer(httpapi.Config{
		Log: testutil.NopLogger(), Bus: bus, Library: lib, ImagesRoot: root,
		Slideshow: ss, KioskBeater: &fakeBeater{}, Exclude: exclude, Thumbs: thumbs, Files: fs,
	})
	return &touchHarness{
		imageHarness: &imageHarness{handler: h, lib: lib, bus: bus, root: root, slideshow: ss},
		exclude:      exclude, files: fs, fileDir: fileDir,
	}
}

func (h *touchHarness) do(req *http.Request) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.handler.ServeHTTP(rec, req)
	return rec
}

func (h *touchHarness) addJPEG(t *testing.T, name string) {
	t.Helper()
	f, err := h.root.Create(name)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.Write(makeJPEG(t)); err != nil {
		t.Fatal(err)
	}
	f.Close()
	h.lib.Add(name)
}

func TestSlideshowSelectionHidesAndPersists(t *testing.T) {
	h := newTouchServer(t)
	h.addJPEG(t, "a.jpg")
	h.addJPEG(t, "b.jpg")

	rec := h.put(t, "/api/images/slideshow", `{"names":["b.jpg"],"included":false}`)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	if h.slideshow.restarts.Load() != 1 {
		t.Fatal("cycle not restarted")
	}
	var items []httpapi.ImageItem
	list := h.do(httptest.NewRequest(http.MethodGet, "/api/images", nil))
	if err := json.NewDecoder(list.Body).Decode(&items); err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 || !items[0].Included || items[1].Included {
		t.Fatalf("items = %+v", items)
	}
	_, saved, _ := library.LoadExcludeStore(testutil.NopLogger(), h.root)
	if !slices.Equal(saved, []string{"b.jpg"}) {
		t.Fatalf("persisted = %v", saved)
	}
	var sawLibrary bool
	for _, e := range h.bus.Snapshot() {
		if e.Kind == state.KindLibrary {
			sawLibrary = true
		}
	}
	if !sawLibrary {
		t.Fatal("no library event published")
	}

	// No-op change doesn't restart the cycle again.
	h.put(t, "/api/images/slideshow", `{"names":["b.jpg"],"included":false}`)
	if h.slideshow.restarts.Load() != 1 {
		t.Fatal("no-op restarted the cycle")
	}

	// Deleting a hidden photo drops it from the persisted selection.
	del := h.do(httptest.NewRequest(http.MethodDelete, "/api/images/b.jpg", nil))
	if del.Code != http.StatusNoContent {
		t.Fatalf("delete %d", del.Code)
	}
	_, saved, _ = library.LoadExcludeStore(testutil.NopLogger(), h.root)
	if len(saved) != 0 {
		t.Fatalf("persisted after delete = %v", saved)
	}
}

func TestServeThumb(t *testing.T) {
	h := newTouchServer(t)
	h.addJPEG(t, "a.jpg")
	rec := h.do(httptest.NewRequest(http.MethodGet, "/thumb/a.jpg", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	if _, err := jpeg.DecodeConfig(rec.Body); err != nil {
		t.Fatalf("not a jpeg: %v", err)
	}
	if !strings.Contains(rec.Header().Get("Cache-Control"), "immutable") {
		t.Fatal("thumb not cacheable")
	}
	if _, err := h.root.Stat(".thumbs/a.jpg"); err != nil {
		t.Fatal("thumb not stored")
	}
	if rec := h.do(httptest.NewRequest(http.MethodGet, "/thumb/missing.jpg", nil)); rec.Code != http.StatusNotFound {
		t.Fatalf("missing = %d", rec.Code)
	}
}

func TestServeThumbWithoutStoreFallsBackToImage(t *testing.T) {
	h := newImageServer(t)
	f, _ := h.root.Create("a.jpg")
	_, _ = f.Write(makeJPEG(t))
	f.Close()
	h.lib.Add("a.jpg")
	rec := httptest.NewRecorder()
	h.handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/thumb/a.jpg", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
}

func fileUpload(t *testing.T, parts map[string]string) *http.Request {
	t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	for name, content := range parts {
		fw, err := mw.CreateFormFile("file", name)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = fw.Write([]byte(content))
	}
	_ = mw.WriteField("note", "ignored")
	_ = mw.Close()
	req := httptest.NewRequest(http.MethodPost, "/api/files", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	return req
}

func TestFilesUploadListServeDelete(t *testing.T) {
	h := newTouchServer(t)
	rec := h.do(fileUpload(t, map[string]string{"Kesä video.mp4": "vid", "ohje.pdf": "%PDF-1.4"}))
	if rec.Code != http.StatusCreated {
		t.Fatalf("upload %d: %s", rec.Code, rec.Body)
	}
	var saved []httpapi.FileItem
	if err := json.NewDecoder(rec.Body).Decode(&saved); err != nil || len(saved) != 2 {
		t.Fatalf("saved = %+v %v", saved, err)
	}

	var list struct {
		Files     []httpapi.FileItem `json:"files"`
		FreeBytes *uint64            `json:"free_bytes"`
	}
	lrec := h.do(httptest.NewRequest(http.MethodGet, "/api/files", nil))
	if err := json.NewDecoder(lrec.Body).Decode(&list); err != nil {
		t.Fatal(err)
	}
	if len(list.Files) != 2 || list.FreeBytes == nil {
		t.Fatalf("list = %+v", list)
	}

	srec := h.do(httptest.NewRequest(http.MethodGet, "/files/Kesa_video.mp4", nil))
	if srec.Code != http.StatusOK || srec.Body.String() != "vid" {
		t.Fatalf("serve %d %q", srec.Code, srec.Body)
	}
	if srec.Header().Get("Content-Type") != "video/mp4" ||
		srec.Header().Get("Content-Security-Policy") != "sandbox" ||
		!strings.HasPrefix(srec.Header().Get("Content-Disposition"), "inline") {
		t.Fatalf("headers = %v", srec.Header())
	}
	rng := httptest.NewRequest(http.MethodGet, "/files/Kesa_video.mp4", nil)
	rng.Header.Set("Range", "bytes=1-")
	if r := h.do(rng); r.Code != http.StatusPartialContent || r.Body.String() != "id" {
		t.Fatalf("range %d %q", r.Code, r.Body)
	}
	dl := h.do(httptest.NewRequest(http.MethodGet, "/files/ohje.pdf?download=true", nil))
	if !strings.HasPrefix(dl.Header().Get("Content-Disposition"), "attachment") {
		t.Fatalf("download disposition = %q", dl.Header().Get("Content-Disposition"))
	}

	if d := h.do(httptest.NewRequest(http.MethodDelete, "/api/files/ohje.pdf", nil)); d.Code != http.StatusNoContent {
		t.Fatalf("delete %d", d.Code)
	}
	if d := h.do(httptest.NewRequest(http.MethodDelete, "/api/files/ohje.pdf", nil)); d.Code != http.StatusNotFound {
		t.Fatalf("second delete %d", d.Code)
	}
	if _, err := os.Stat(filepath.Join(h.fileDir, "ohje.pdf")); !os.IsNotExist(err) {
		t.Fatal("file still on disk")
	}
	if s := h.do(httptest.NewRequest(http.MethodGet, "/files/ohje.pdf", nil)); s.Code != http.StatusNotFound {
		t.Fatalf("serve deleted %d", s.Code)
	}
	if s := h.do(httptest.NewRequest(http.MethodGet, "/files/.files-index.json", nil)); s.Code == http.StatusOK {
		t.Fatal("sidecar must not be servable")
	}
}

func TestFilesUploadRejectsBadRequests(t *testing.T) {
	h := newTouchServer(t)
	notMultipart := httptest.NewRequest(http.MethodPost, "/api/files", strings.NewReader("x"))
	if r := h.do(notMultipart); r.Code != http.StatusBadRequest {
		t.Fatalf("non-multipart %d", r.Code)
	}
	if r := h.do(fileUpload(t, nil)); r.Code != http.StatusBadRequest {
		t.Fatalf("no file %d", r.Code)
	}
	huge := fileUpload(t, map[string]string{"a.bin": "x"})
	huge.ContentLength = 1 << 62
	if r := h.do(huge); r.Code != http.StatusInsufficientStorage {
		t.Fatalf("huge %d", r.Code)
	}
}

func TestFilesRoutesWithoutStore(t *testing.T) {
	h := newImageServer(t)
	do := func(req *http.Request) int {
		rec := httptest.NewRecorder()
		h.handler.ServeHTTP(rec, req)
		return rec.Code
	}
	if c := do(httptest.NewRequest(http.MethodGet, "/api/files", nil)); c != http.StatusOK {
		t.Fatalf("list %d", c)
	}
	if c := do(fileUpload(t, map[string]string{"a.txt": "x"})); c != http.StatusServiceUnavailable {
		t.Fatalf("upload %d", c)
	}
	if c := do(httptest.NewRequest(http.MethodDelete, "/api/files/a.txt", nil)); c != http.StatusServiceUnavailable {
		t.Fatalf("delete %d", c)
	}
	if c := do(httptest.NewRequest(http.MethodGet, "/files/a.txt", nil)); c != http.StatusNotFound {
		t.Fatalf("serve %d", c)
	}
}

func TestFileNameTagMatchesFilesPattern(t *testing.T) {
	field, _ := reflect.TypeFor[httpapi.FileNameInput]().FieldByName("Name")
	if got := field.Tag.Get("pattern"); got != files.NamePattern {
		t.Errorf("pattern tag %q != files.NamePattern %q", got, files.NamePattern)
	}
}
