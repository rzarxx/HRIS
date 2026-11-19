package integration

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/rzarxx/HRIS/backend/internal/platform/filescan"
)

// Test-only scanner: deterministic failures exercise publication gates, not AV signatures.
type testFileScanner struct{ err error }

func (s testFileScanner) Scan(context.Context, []byte) error { return s.err }

type attachmentMemoryStore map[string][]byte

func (m attachmentMemoryStore) Put(_ context.Context, k string, b []byte) error {
	m[k] = bytes.Clone(b)
	return nil
}
func (m attachmentMemoryStore) Get(_ context.Context, k string) ([]byte, error) {
	return bytes.Clone(m[k]), nil
}
func (m attachmentMemoryStore) Delete(_ context.Context, k string) error { delete(m, k); return nil }

func TestAnnouncementCompressedAndLegacyFiles(t *testing.T) {
	files := attachmentMemoryStore{}
	scanner := &testFileScanner{}
	srv := fresh(t, options{announcementFiles: files, announcementScanner: scanner})
	sdm := loginSDM(t, srv)
	ops, opsAcc := pejuangClient(t, srv, "OPS-GDG")
	keu, _ := pejuangClient(t, srv, "KEU-AKT")
	a := data[announcementWithFiles](t, sdm.do(http.MethodPost, "/v1/admin/announcements", map[string]any{
		"title": "Dokumen", "body": "Pengumuman", "type": "popup", "starts_at": time.Now().Add(-time.Hour).UTC().Format(time.RFC3339), "division_id": opsAcc.division,
	}).want(t, http.StatusCreated))
	pdf := bytes.Repeat([]byte("%PDF-1.4 document\n"), 1000)
	got := data[announcementWithFiles](t, sdm.send(http.MethodPost, "/v1/admin/announcements/"+a.ID+"/attachments?name=dokumen.pdf", bytes.NewReader(pdf), "application/pdf").want(t, http.StatusCreated))
	att := got.Attachments[0]
	var key string
	if err := ownerPool.QueryRow(context.Background(), "SELECT storage_key FROM announcement_attachments WHERE id=$1", att.ID).Scan(&key); err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(key, ".safe-v1.gz") || len(files[key]) >= len(pdf) || att.SizeBytes != len(pdf) {
		t.Fatal("storage compression or metadata incorrect")
	}
	url := "/v1/announcements/" + a.ID + "/attachments/" + att.ID
	if res := ops.do(http.MethodGet, url, nil).want(t, http.StatusOK); !bytes.Equal(res.body, pdf) {
		t.Fatal("PDF round trip changed")
	}
	// Simulate a pre-upgrade key: it must be scanned before sending any bytes.
	files["announcements/legacy.pdf"] = pdf
	exec(t, "UPDATE announcement_attachments SET storage_key='announcements/legacy.pdf' WHERE id=$1", att.ID)
	scanner.err = filescan.ErrUnsafe
	ops.do(http.MethodGet, url, nil).wantError(t, 422, "FILE_UNSAFE")
	keu.do(http.MethodGet, url, nil).wantError(t, 404, "NOT_FOUND")
	scanner.err = errors.New("offline")
	ops.do(http.MethodGet, url, nil).wantError(t, 503, "FILE_SCAN_UNAVAILABLE")
	scanner.err = nil
	if res := ops.do(http.MethodGet, url, nil).want(t, http.StatusOK); !bytes.Equal(res.body, pdf) {
		t.Fatal("clean legacy file unavailable")
	}
}

func TestAnnouncementScanFailureDoesNotPublish(t *testing.T) {
	for _, tc := range []struct {
		name       string
		scannerErr error
		absent     bool
		status     int
		code       string
	}{
		{"unsafe", filescan.ErrUnsafe, false, 422, "FILE_UNSAFE"},
		{"offline", errors.New("scanner unavailable"), false, 503, "FILE_SCAN_UNAVAILABLE"},
		{"not configured", nil, true, 503, "FILE_SCAN_UNAVAILABLE"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			files := attachmentMemoryStore{}
			srv := fresh(t, options{announcementScanner: testFileScanner{tc.scannerErr}, noAnnouncementScanner: tc.absent, announcementFiles: files})
			sdm := loginSDM(t, srv)
			a := data[announcementWithFiles](t, sdm.do(http.MethodPost, "/v1/admin/announcements", map[string]any{
				"title": "Pemeriksaan lampiran", "body": "Tes keamanan", "type": "popup", "starts_at": time.Now().UTC().Format(time.RFC3339),
			}).want(t, http.StatusCreated))
			for _, file := range [][]byte{[]byte("%PDF-1.4 document"), samplePNG(t, 20, 20)} {
				sdm.send(http.MethodPost, "/v1/admin/announcements/"+a.ID+"/attachments?name=berkas.pdf", bytes.NewReader(file), "application/pdf").wantError(t, tc.status, tc.code)
			}
			var count int
			if err := ownerPool.QueryRow(context.Background(), "SELECT count(*) FROM announcement_attachments WHERE announcement_id=$1", a.ID).Scan(&count); err != nil || count != 0 {
				t.Fatalf("rejected upload published: %d %v", count, err)
			}
			if len(files) != 0 {
				t.Fatal("rejected upload written to storage")
			}
		})
	}
}

type attachmentInfo struct {
	ID          string `json:"id"`
	FileName    string `json:"file_name"`
	ContentType string `json:"content_type"`
	SizeBytes   int    `json:"size_bytes"`
}

type announcementWithFiles struct {
	ID          string           `json:"id"`
	Attachments []attachmentInfo `json:"attachments"`
}

func TestAnnouncementAttachments(t *testing.T) {
	srv := fresh(t, options{})
	exec(t, `DELETE FROM announcements`)
	sdm := loginSDM(t, srv)
	ops, opsAcc := pejuangClient(t, srv, "OPS-GDG")
	keu, _ := pejuangClient(t, srv, "KEU-AKT")

	start := time.Now().Add(-time.Hour).UTC().Format(time.RFC3339)
	a := data[announcementWithFiles](t, sdm.do(http.MethodPost, "/v1/admin/announcements", map[string]any{
		"title": "Jadwal kajian", "body": "Lihat lampiran", "type": "popup", "starts_at": start, "division_id": opsAcc.division,
	}).want(t, http.StatusCreated))
	if a.Attachments == nil || len(a.Attachments) != 0 {
		t.Fatalf("attachments awal = %v, ingin array kosong", a.Attachments)
	}
	base := "/v1/admin/announcements/" + a.ID + "/attachments"
	pdf := []byte("%PDF-1.4\nisi jadwal kajian")

	// Jenis berkas dicek dari isinya, bukan dari Content-Type.
	fake := sdm.send(http.MethodPost, base+"?name=jadwal.pdf", bytes.NewReader([]byte("<script>alert(1)</script>")), "application/pdf").want(t, http.StatusBadRequest)
	if fake.fields()["file"] == nil {
		t.Fatalf("galat file tidak ada: %s", fake.body)
	}
	// Pejuang tidak boleh menambah lampiran.
	ops.send(http.MethodPost, base+"?name=jadwal.pdf", bytes.NewReader(pdf), "application/pdf").want(t, http.StatusForbidden)

	got := data[announcementWithFiles](t, sdm.send(http.MethodPost, base+"?name=../Jadwal%20Kajian.exe", bytes.NewReader(pdf), "application/pdf").want(t, http.StatusCreated))
	if len(got.Attachments) != 1 {
		t.Fatalf("lampiran = %v", got.Attachments)
	}
	att := got.Attachments[0]
	if att.FileName != "Jadwal Kajian.pdf" || att.ContentType != "application/pdf" || att.SizeBytes != len(pdf) {
		t.Fatalf("metadata lampiran = %+v", att)
	}

	// Sasaran melihat lampiran di daftar aktif dan bisa mengunduhnya.
	var mine []announcementWithFiles
	mine = data[[]announcementWithFiles](t, ops.do(http.MethodGet, "/v1/announcements/active", nil).want(t, http.StatusOK))
	if len(mine) != 1 || len(mine[0].Attachments) != 1 {
		t.Fatalf("pengumuman aktif = %+v", mine)
	}
	opened := data[announcementWithFiles](t, ops.do(http.MethodPost, "/v1/announcements/"+a.ID+"/open", nil).want(t, http.StatusOK))
	if len(opened.Attachments) != 1 {
		t.Fatalf("respons /open tanpa lampiran: %+v", opened)
	}
	file := "/v1/announcements/" + a.ID + "/attachments/" + att.ID
	res := ops.do(http.MethodGet, file, nil).want(t, http.StatusOK)
	if !bytes.Equal(res.body, pdf) || res.header.Get("Content-Type") != "application/pdf" || res.header.Get("Cache-Control") != "private, no-store" {
		t.Fatalf("unduhan lampiran: %q %v", res.body, res.header)
	}
	// Bukan sasaran (divisi lain) tidak bisa mengunduh.
	keu.do(http.MethodGet, file, nil).wantError(t, http.StatusNotFound, "NOT_FOUND")
	// SDM tetap bisa mengunduh walau bukan sasaran.
	sdm.do(http.MethodGet, file, nil).want(t, http.StatusOK)

	// Batas 5 lampiran per pengumuman.
	for i := 0; i < 4; i++ {
		sdm.send(http.MethodPost, base+"?name=foto.png", bytes.NewReader(samplePNG(t, 20, 20)), "image/png").want(t, http.StatusCreated)
	}
	sdm.send(http.MethodPost, base+"?name=lagi.pdf", bytes.NewReader(pdf), "application/pdf").want(t, http.StatusBadRequest)

	// Hapus lampiran, lalu hapus pengumuman beserta semua berkasnya.
	sdm.do(http.MethodDelete, base+"/"+att.ID, nil).want(t, http.StatusNoContent)
	ops.do(http.MethodGet, file, nil).wantError(t, http.StatusNotFound, "NOT_FOUND")
	sdm.do(http.MethodDelete, "/v1/admin/announcements/"+a.ID, nil).want(t, http.StatusNoContent)
	var n int
	if err := ownerPool.QueryRow(context.Background(), `SELECT count(*) FROM announcement_attachments`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("baris lampiran tersisa: %d %v", n, err)
	}
}
