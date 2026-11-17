package integration

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"
)

const xlsxType = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"

type importIssue struct {
	Row     int    `json:"row"`
	Column  string `json:"column"`
	Message string `json:"message"`
}

type importResult struct {
	DryRun    bool          `json:"dry_run"`
	Applied   bool          `json:"applied"`
	TotalRows int           `json:"total_rows"`
	Created   int           `json:"created"`
	Updated   int           `json:"updated"`
	Issues    []importIssue `json:"issues"`
	Rows      []struct {
		Row             int     `json:"row"`
		Action          string  `json:"action"`
		Nip             *string `json:"nip"`
		FullName        string  `json:"full_name"`
		InitialPassword *string `json:"initial_password"`
		ExpiresAt       *string `json:"expires_at"`
	} `json:"rows"`
	CredentialsFile *string `json:"credentials_file"`
}

// fillTemplate mengisi template dari server berdasarkan judul kolom, seperti SDM mengisinya di Excel.
func fillTemplate(t *testing.T, template []byte, rows []map[string]string) []byte {
	t.Helper()
	f, err := excelize.OpenReader(bytes.NewReader(template))
	if err != nil {
		t.Fatal(err)
	}
	header, err := f.GetRows("Pejuang")
	if err != nil || len(header) == 0 {
		t.Fatalf("sheet Pejuang: %v", err)
	}
	cols := map[string]int{}
	for i, h := range header[0] {
		cols[strings.TrimSuffix(h, "*")] = i + 1
	}
	for r, row := range rows {
		for label, value := range row {
			col, ok := cols[label]
			if !ok {
				t.Fatalf("kolom %q tidak ada di template", label)
			}
			cell, _ := excelize.CoordinatesToCellName(col, r+2)
			if err := f.SetCellStr("Pejuang", cell, value); err != nil {
				t.Fatal(err)
			}
		}
	}
	buf, err := f.WriteToBuffer()
	if err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func runImport(t *testing.T, c *client, file []byte, dryRun bool) importResult {
	t.Helper()
	path := "/v1/admin/employees/import?dry_run=true"
	if !dryRun {
		path = "/v1/admin/employees/import?dry_run=false"
	}
	res := c.send(http.MethodPost, path, bytes.NewReader(file), xlsxType).want(t, http.StatusOK)
	var out struct {
		Data importResult `json:"data"`
	}
	if err := json.Unmarshal(res.body, &out); err != nil {
		t.Fatal(err)
	}
	return out.Data
}

// D53: template xlsx dan impor massal; semua baris tersimpan atau tidak sama sekali.
func TestEmployeeImport(t *testing.T) {
	srv := fresh(t, options{})
	sdm := loginSDM(t, srv)
	ctx := context.Background()

	var branch, division, sub string
	if err := ownerPool.QueryRow(ctx, `
		SELECT b.name, d.name, s.name FROM subdivisions s
		JOIN divisions d ON d.id = s.division_id JOIN branches b ON b.id = d.branch_id
		WHERE s.code = 'OPS-LAP'`).Scan(&branch, &division, &sub); err != nil {
		t.Fatal(err)
	}
	existing := activeAccount(t, "pejuang", "OPS-GDG")
	var existingNip string
	if err := ownerPool.QueryRow(ctx, `SELECT nip FROM employees WHERE id = $1`, existing.employeeID).Scan(&existingNip); err != nil {
		t.Fatal(err)
	}

	template := sdm.do(http.MethodGet, "/v1/admin/employees/import-template", nil).want(t, http.StatusOK)
	if got := template.header.Get("Content-Type"); got != xlsxType {
		t.Fatalf("content-type template = %q", got)
	}
	tf, err := excelize.OpenReader(bytes.NewReader(template.body))
	if err != nil {
		t.Fatal(err)
	}
	structure, _ := tf.GetRows("Struktur")
	if len(structure) < 2 {
		t.Fatal("sheet Struktur kosong")
	}

	place := map[string]string{"Cabang": branch, "Divisi": division, "Subdiv": sub}
	row := func(fields map[string]string) map[string]string {
		for k, v := range place {
			if _, set := fields[k]; !set {
				fields[k] = v
			}
		}
		return fields
	}
	good := [][]map[string]string{{
		row(map[string]string{
			"Nama lengkap": "Ahmad Fauzan", "Jenis kelamin": "Laki-laki", "No. WA": "0812-5555-0001",
			"Status pernikahan": "Menikah", "Memiliki anak": "Ya", "Jumlah anak": "1",
			"Nama anak 1": "Zahra Fauzan", "Jenis kelamin anak 1": "Perempuan", "Tanggal lahir anak 1": "02/04/2015",
			"Status anak 1": "Kandung", "Pendidikan anak 1": "SD/MI", "Nama sekolah anak 1": "MI Al Bahjah",
		}),
		row(map[string]string{"NIPY": existingNip, "Nama lengkap": "Nama Diperbarui Impor", "Jabatan": "Koordinator Gudang"}),
		row(map[string]string{"NIPY": "IMP-0001", "Nama lengkap": "Siti Rahmah", "Email": "siti.rahmah@pejuang.id"}),
	}}[0]
	file := fillTemplate(t, template.body, good)

	// Pratinjau: diperiksa lengkap tetapi tidak menyimpan apa pun.
	preview := runImport(t, sdm, file, true)
	if !preview.DryRun || preview.Applied || preview.Created != 2 || preview.Updated != 1 || len(preview.Issues) != 0 {
		t.Fatalf("pratinjau: %+v", preview)
	}
	var count int
	if err := ownerPool.QueryRow(ctx, `SELECT count(*) FROM employees WHERE full_name IN ('Ahmad Fauzan', 'Siti Rahmah')`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("pratinjau menyimpan data: %d %v", count, err)
	}

	// Baris bermasalah: subdiv salah dan nomor WA kembar di dalam berkas. Tidak ada yang tersimpan.
	bad := fillTemplate(t, template.body, []map[string]string{
		row(map[string]string{"Nama lengkap": "Budi Santosa", "No. WA": "081255550009"}),
		row(map[string]string{"Nama lengkap": "Rina Lestari", "No. WA": "+62 812 5555 0009"}),
		row(map[string]string{"Nama lengkap": "Tono Wijaya", "Subdiv": "Tidak Ada"}),
	})
	rejected := runImport(t, sdm, bad, false)
	if rejected.Applied || len(rejected.Issues) != 2 {
		t.Fatalf("berkas bermasalah: %+v", rejected)
	}
	cols := map[string]int{}
	for _, issue := range rejected.Issues {
		cols[issue.Column] = issue.Row
	}
	if cols["No. WA"] != 3 || cols["Subdiv"] != 4 {
		t.Errorf("letak masalah: %+v", rejected.Issues)
	}
	if err := ownerPool.QueryRow(ctx, `SELECT count(*) FROM employees WHERE full_name = 'Budi Santosa'`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("berkas bermasalah menyimpan sebagian: %d %v", count, err)
	}

	// Simpan: Pejuang baru mendapat kata sandi awal, Pejuang lama diperbarui tanpa mengubah kata sandi.
	saved := runImport(t, sdm, file, false)
	if !saved.Applied || saved.Created != 2 || saved.Updated != 1 {
		t.Fatalf("simpan: %+v", saved)
	}
	var created *struct {
		nip, password string
	}
	for _, r := range saved.Rows {
		if r.Action == "create" && r.FullName == "Ahmad Fauzan" {
			if r.Nip == nil || r.InitialPassword == nil || r.ExpiresAt == nil || !strings.HasPrefix(*r.InitialPassword, "@Ahmad") {
				t.Fatalf("baris baru tanpa kredensial: %+v", r)
			}
			created = &struct{ nip, password string }{*r.Nip, *r.InitialPassword}
		}
		if r.Action == "update" && r.InitialPassword != nil {
			t.Error("Pejuang lama tidak boleh mendapat kata sandi baru")
		}
	}
	if created == nil {
		t.Fatal("Ahmad Fauzan tidak dibuat")
	}
	// Daftar kata sandi untuk SDM berupa xlsx dengan kata sandi sebagai teks, bukan rumus.
	if saved.CredentialsFile == nil || preview.CredentialsFile != nil {
		t.Fatal("berkas kata sandi awal hanya ada saat impor disimpan")
	}
	raw, err := base64.StdEncoding.DecodeString(*saved.CredentialsFile)
	if err != nil {
		t.Fatal(err)
	}
	cf, err := excelize.OpenReader(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	listed, _ := cf.GetRows("Kata sandi awal")
	if len(listed) != 3 || listed[1][2] == "" || !strings.HasPrefix(listed[1][2], "@") {
		t.Errorf("daftar kata sandi: %v", listed)
	}
	login(t, srv, created.nip, created.password).do(http.MethodGet, "/v1/announcements/active", nil).
		wantError(t, http.StatusForbidden, "PASSWORD_CHANGE_REQUIRED")
	login(t, srv, "6281255550001", created.password)

	var children int
	var phone, expiresSet string
	if err := ownerPool.QueryRow(ctx, `
		SELECT (SELECT count(*) FROM employee_children c WHERE c.employee_id = e.id), e.phone,
		       (u.initial_password_expires_at IS NOT NULL)::text
		FROM employees e JOIN users u ON u.employee_id = e.id WHERE e.nip = $1`, created.nip).Scan(&children, &phone, &expiresSet); err != nil {
		t.Fatal(err)
	}
	if children != 1 || phone != "6281255550001" || expiresSet != "true" {
		t.Errorf("Pejuang baru: anak=%d wa=%s batas=%s", children, phone, expiresSet)
	}
	var position, name string
	if err := ownerPool.QueryRow(ctx, `SELECT position, full_name FROM employees WHERE id = $1`, existing.employeeID).Scan(&position, &name); err != nil {
		t.Fatal(err)
	}
	if position != "Koordinator Gudang" || name != "Nama Diperbarui Impor" {
		t.Errorf("Pejuang lama: %s / %s", name, position)
	}
	login(t, srv, existing.email, seedPassword)
	if got := countAudit(t, "employee.imported"); got != 1 {
		t.Errorf("audit employee.imported = %d", got)
	}

	// Berkas bukan xlsx ditolak dengan pesan di field file; Pejuang biasa tidak punya akses.
	if f := sdm.send(http.MethodPost, "/v1/admin/employees/import", strings.NewReader("nama,cabang"), "text/csv").
		wantError(t, http.StatusBadRequest, "VALIDATION_FAILED").fields(); f["file"] == nil {
		t.Errorf("berkas bukan xlsx: %v", f)
	}
	login(t, srv, activeAccount(t, "pejuang", "OPS-LAP").email, seedPassword).
		do(http.MethodGet, "/v1/admin/employees/import-template", nil).wantError(t, http.StatusForbidden, "FORBIDDEN_ROLE")
}
