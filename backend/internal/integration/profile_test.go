package integration

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"net/http"
	"testing"
)

// samplePNG membuat gambar PNG polos berukuran w x h untuk uji unggah foto profil.
func samplePNG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x % 256), G: 120, B: uint8(y % 256), A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestProfileContact(t *testing.T) {
	srv := fresh(t, options{})
	me := activeAccount(t, "pejuang", "OPS-GDG")
	c := login(t, srv, me.email, seedPassword)
	before := countAudit(t, "employee.updated")

	got := data[map[string]any](t, c.do(http.MethodPatch, "/v1/me", map[string]any{"phone": "0812 3456 7890"}).want(t, http.StatusOK))
	if got["phone"] != "6281234567890" {
		t.Fatalf("phone = %v", got["phone"])
	}
	if got["email"] != me.email {
		t.Fatalf("email berubah padahal tidak dikirim: %v", got["email"])
	}

	bad := c.do(http.MethodPatch, "/v1/me", map[string]any{"phone": "12"}).want(t, http.StatusBadRequest)
	if bad.fields()["phone"] == nil {
		t.Fatalf("galat phone tidak ada: %s", bad.body)
	}

	// Ganti email wajib kata sandi saat ini yang benar.
	noPass := c.do(http.MethodPatch, "/v1/me", map[string]any{"email": "baru@pejuang.test"}).want(t, http.StatusBadRequest)
	if noPass.fields()["current_password"] == nil {
		t.Fatalf("current_password tidak diminta: %s", noPass.body)
	}
	c.do(http.MethodPatch, "/v1/me", map[string]any{"email": "baru@pejuang.test", "current_password": "salah-sekali"}).want(t, http.StatusBadRequest)
	c.do(http.MethodPatch, "/v1/me", map[string]any{"email": sdmEmail, "current_password": seedPassword}).wantError(t, http.StatusConflict, "CONFLICT_DUPLICATE")

	got = data[map[string]any](t, c.do(http.MethodPatch, "/v1/me", map[string]any{"email": "baru@pejuang.test", "current_password": seedPassword}).want(t, http.StatusOK))
	if got["email"] != "baru@pejuang.test" {
		t.Fatalf("email = %v", got["email"])
	}
	// Email login ikut berubah.
	login(t, srv, "baru@pejuang.test", seedPassword)

	if n := countAudit(t, "employee.updated") - before; n != 2 {
		t.Fatalf("audit employee.updated bertambah %d, ingin 2", n)
	}
	var detail string
	if err := ownerPool.QueryRow(context.Background(),
		`SELECT detail::text FROM audit_logs WHERE action = 'employee.updated' ORDER BY created_at DESC LIMIT 1`).Scan(&detail); err != nil {
		t.Fatal(err)
	}
	if bytes.Contains([]byte(detail), []byte("baru@pejuang.test")) {
		t.Fatalf("audit memuat nilai email: %s", detail)
	}
}

func TestProfileChildren(t *testing.T) {
	srv := fresh(t, options{})
	me := activeAccount(t, "pejuang", "OPS-GDG")
	c := login(t, srv, me.email, seedPassword)

	body := map[string]any{
		"has_children": true, "children_count": 2,
		"children": []map[string]any{{"child_order": 1, "full_name": "Aisyah", "gender": "P", "relation": "kandung", "education": "sd_mi"}},
	}
	got := data[map[string]any](t, c.do(http.MethodPut, "/v1/me/children", body).want(t, http.StatusOK))
	if got["children_count"] != float64(2) || len(got["children"].([]any)) != 1 {
		t.Fatalf("data anak = %v", got)
	}
	again := data[map[string]any](t, c.do(http.MethodGet, "/v1/me/children", nil).want(t, http.StatusOK))
	if len(again["children"].([]any)) != 1 {
		t.Fatalf("GET data anak = %v", again)
	}

	bad := c.do(http.MethodPut, "/v1/me/children", map[string]any{
		"has_children": false, "children_count": 0,
		"children": []map[string]any{{"child_order": 1, "full_name": "Aisyah"}},
	}).want(t, http.StatusBadRequest)
	if bad.fields()["has_children"] == nil {
		t.Fatalf("galat has_children tidak ada: %s", bad.body)
	}
	c.do(http.MethodPut, "/v1/me/children", map[string]any{
		"has_children": true, "children_count": 1,
		"children": []map[string]any{{"child_order": 1, "full_name": "A"}, {"child_order": 1, "full_name": "Budi"}},
	}).want(t, http.StatusBadRequest)

	// Kosongkan data anak.
	cleared := data[map[string]any](t, c.do(http.MethodPut, "/v1/me/children", map[string]any{"has_children": false, "children_count": 0, "children": []any{}}).want(t, http.StatusOK))
	if len(cleared["children"].([]any)) != 0 {
		t.Fatalf("data anak tidak terhapus: %v", cleared)
	}
}

func TestProfilePhoto(t *testing.T) {
	srv := fresh(t, options{})
	me := activeAccount(t, "pejuang", "OPS-GDG")
	other := activeAccount(t, "pejuang", "OPS-LAP")
	c := login(t, srv, me.email, seedPassword)
	path := "/v1/employees/" + me.employeeID + "/photo"

	c.do(http.MethodGet, path, nil).wantError(t, http.StatusNotFound, "NOT_FOUND")

	// Bukan gambar, walau Content-Type mengaku PNG.
	fake := c.send(http.MethodPut, "/v1/me/photo", bytes.NewReader([]byte("bukan gambar sama sekali")), "image/png").want(t, http.StatusBadRequest)
	if fake.fields()["photo"] == nil {
		t.Fatalf("galat photo tidak ada: %s", fake.body)
	}
	c.send(http.MethodPut, "/v1/me/photo", bytes.NewReader(samplePNG(t, 40, 40)), "image/png").want(t, http.StatusBadRequest)

	got := data[map[string]any](t, c.send(http.MethodPut, "/v1/me/photo", bytes.NewReader(samplePNG(t, 900, 600)), "image/png").want(t, http.StatusOK))
	if got["has_photo"] != true || got["photo_updated_at"] == nil {
		t.Fatalf("Me setelah unggah = %v", got)
	}

	photo := c.do(http.MethodGet, path, nil).want(t, http.StatusOK)
	if ct := photo.header.Get("Content-Type"); ct != "image/jpeg" {
		t.Fatalf("content-type = %q", ct)
	}
	if cc := photo.header.Get("Cache-Control"); cc != "private, no-store" {
		t.Fatalf("cache-control = %q", cc)
	}
	img, err := jpeg.Decode(bytes.NewReader(photo.body))
	if err != nil {
		t.Fatal(err)
	}
	if b := img.Bounds(); b.Dx() != 512 || b.Dy() != 512 {
		t.Fatalf("ukuran foto %dx%d, ingin 512x512", b.Dx(), b.Dy())
	}

	// Pejuang lain tidak boleh melihat; SDM boleh.
	login(t, srv, other.email, seedPassword).do(http.MethodGet, path, nil).wantError(t, http.StatusNotFound, "NOT_FOUND")
	loginSDM(t, srv).do(http.MethodGet, path, nil).want(t, http.StatusOK)

	// Ganti foto meninggalkan satu berkas saja.
	c.send(http.MethodPut, "/v1/me/photo", bytes.NewReader(samplePNG(t, 300, 300)), "image/png").want(t, http.StatusOK)
	var keys int
	if err := ownerPool.QueryRow(context.Background(),
		`SELECT count(*) FROM employees WHERE id = $1 AND photo_key LIKE 'profile/%'`, me.employeeID).Scan(&keys); err != nil || keys != 1 {
		t.Fatalf("photo_key: %d %v", keys, err)
	}

	c.do(http.MethodDelete, "/v1/me/photo", nil).want(t, http.StatusNoContent)
	c.do(http.MethodGet, path, nil).wantError(t, http.StatusNotFound, "NOT_FOUND")
	after := data[map[string]any](t, c.do(http.MethodGet, "/v1/me", nil).want(t, http.StatusOK))
	if after["has_photo"] != false || after["photo_updated_at"] != nil {
		t.Fatalf("Me setelah hapus = %v", after)
	}
}

func TestProfilePhotoWithoutStorage(t *testing.T) {
	srv := fresh(t, options{noFaceEngine: true})
	me := activeAccount(t, "pejuang", "OPS-GDG")
	c := login(t, srv, me.email, seedPassword)
	c.send(http.MethodPut, "/v1/me/photo", bytes.NewReader(samplePNG(t, 200, 200)), "image/png").wantError(t, http.StatusServiceUnavailable, "STORAGE_UNAVAILABLE")
}
