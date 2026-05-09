package handler

import (
	"archive/zip"
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/autovideo/project-service/internal/model"
)

// fakeImageServer starts a test HTTP server that serves a 1×1 PNG for any path.
// Returns the server (caller must close) and a client configured to talk to it.
func fakeImageServer(t *testing.T) (*httptest.Server, *http.Client) {
	t.Helper()
	// Minimal valid 1×1 PNG (67 bytes).
	minPNG := []byte{
		0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A,
		0x00, 0x00, 0x00, 0x0D, 0x49, 0x48, 0x44, 0x52,
		0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		0x08, 0x02, 0x00, 0x00, 0x00, 0x90, 0x77, 0x53,
		0xDE, 0x00, 0x00, 0x00, 0x0C, 0x49, 0x44, 0x41,
		0x54, 0x08, 0xD7, 0x63, 0xF8, 0xCF, 0xC0, 0x00,
		0x00, 0x00, 0x02, 0x00, 0x01, 0xE2, 0x21, 0xBC,
		0x33, 0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4E,
		0x44, 0xAE, 0x42, 0x60, 0x82,
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(minPNG)
	}))
	client := &http.Client{}
	return srv, client
}

// readZipEntries opens a zip from bytes and returns all entry names.
func readZipEntries(t *testing.T, data []byte) []string {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("open zip: %v", err)
	}
	names := make([]string, 0, len(zr.File))
	for _, f := range zr.File {
		names = append(names, f.Name)
	}
	return names
}

func TestWriteStoryboardsZip_CreatesOneEntryPerImage(t *testing.T) {
	srv, client := fakeImageServer(t)
	defer srv.Close()

	sbs := []model.Storyboard{
		{ID: 1, SequenceNumber: 1, ImageURL: srv.URL + "/img1.png"},
		{ID: 2, SequenceNumber: 2, ImageURL: srv.URL + "/img2.png"},
		{ID: 3, SequenceNumber: 3, ImageURL: srv.URL + "/img3.png"},
	}

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	writeStoryboardsZip(sbs, client, zw, nil)
	_ = zw.Close()

	entries := readZipEntries(t, buf.Bytes())
	if len(entries) != 3 {
		t.Fatalf("expected 3 zip entries, got %d: %v", len(entries), entries)
	}
}

func TestWriteStoryboardsZip_EntryNamesUseSequenceNumber(t *testing.T) {
	srv, client := fakeImageServer(t)
	defer srv.Close()

	sbs := []model.Storyboard{
		{ID: 10, SequenceNumber: 5, ImageURL: srv.URL + "/img.png"},
	}

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	writeStoryboardsZip(sbs, client, zw, nil)
	_ = zw.Close()

	entries := readZipEntries(t, buf.Bytes())
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	// Entry should be "0001_scene5.png"
	want := "0001_scene5.png"
	if entries[0] != want {
		t.Errorf("entry name: got %q want %q", entries[0], want)
	}
}

func TestWriteStoryboardsZip_EntryContainsImageBytes(t *testing.T) {
	srv, client := fakeImageServer(t)
	defer srv.Close()

	sbs := []model.Storyboard{
		{ID: 1, SequenceNumber: 1, ImageURL: srv.URL + "/img.png"},
	}

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	writeStoryboardsZip(sbs, client, zw, nil)
	_ = zw.Close()

	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatalf("open zip: %v", err)
	}
	f, err := zr.File[0].Open()
	if err != nil {
		t.Fatalf("open entry: %v", err)
	}
	defer f.Close()
	data, _ := io.ReadAll(f)

	// PNG magic bytes: first 8 bytes
	pngHeader := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
	if len(data) < 8 || string(data[:8]) != string(pngHeader) {
		t.Errorf("expected PNG header in zip entry, got %x", data[:min(8, len(data))])
	}
}

func TestWriteStoryboardsZip_SkipsUnreachableImages(t *testing.T) {
	srv, client := fakeImageServer(t)
	defer srv.Close()

	sbs := []model.Storyboard{
		{ID: 1, SequenceNumber: 1, ImageURL: srv.URL + "/good.png"},
		{ID: 2, SequenceNumber: 2, ImageURL: "http://127.0.0.1:1/bad.png"}, // unreachable
		{ID: 3, SequenceNumber: 3, ImageURL: srv.URL + "/good2.png"},
	}

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	writeStoryboardsZip(sbs, client, zw, nil)
	_ = zw.Close()

	entries := readZipEntries(t, buf.Bytes())
	// Only 2 good images should be in ZIP; the bad one is skipped.
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries (bad image skipped), got %d: %v", len(entries), entries)
	}
}

func TestWriteStoryboardsZip_EmptySlice_ProducesEmptyZip(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	writeStoryboardsZip(nil, &http.Client{}, zw, nil)
	_ = zw.Close()

	entries := readZipEntries(t, buf.Bytes())
	if len(entries) != 0 {
		t.Fatalf("expected empty zip, got entries: %v", entries)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
