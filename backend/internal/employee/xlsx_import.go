package employee

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/xuri/excelize/v2"

	"github.com/rzarxx/HRIS/backend/internal/audit"
	"github.com/rzarxx/HRIS/backend/internal/auth"
	"github.com/rzarxx/HRIS/backend/internal/permission"
	"github.com/rzarxx/HRIS/backend/internal/platform/database"
	"github.com/rzarxx/HRIS/backend/internal/platform/httpx"
	"github.com/rzarxx/HRIS/backend/internal/platform/pgconv"
	"github.com/rzarxx/HRIS/backend/internal/store"
)

// Impor massal data Pejuang dari template xlsx (D53, MASTER.md §7.5).
const (
	ImportMaxBytes   = 5 << 20
	importMaxRows    = 1000
	importMaxIssues  = 200
	importChildSlots = 3
	importSheet      = "Pejuang"
	importHashers    = 4
)

// ImportIssue menunjuk satu sel yang perlu diperbaiki. Row memakai nomor baris Excel.
type ImportIssue struct {
	Sheet   string `json:"sheet"`
	Row     int    `json:"row"`
	Column  string `json:"column"`
	Message string `json:"message"`
}

// ImportRow adalah hasil per baris. Kata sandi awal hanya ada untuk Pejuang baru saat impor disimpan.
type ImportRow struct {
	Row             int     `json:"row"`
	Action          string  `json:"action"`
	Nip             *string `json:"nip"`
	FullName        string  `json:"full_name"`
	InitialPassword *string `json:"initial_password"`
	ExpiresAt       *string `json:"expires_at"`
}

type ImportResult struct {
	DryRun    bool          `json:"dry_run"`
	Applied   bool          `json:"applied"`
	TotalRows int           `json:"total_rows"`
	Created   int           `json:"created"`
	Updated   int           `json:"updated"`
	Issues    []ImportIssue `json:"issues"`
	Rows      []ImportRow   `json:"rows"`
	// NewItems adalah struktur, lokasi, dan shift baru yang ikut dibuat (atau akan dibuat saat pratinjau).
	NewItems ImportNewItems `json:"new_items"`
	// CredentialsFile adalah xlsx (base64) berisi kata sandi awal Pejuang baru. Kata sandi disimpan sebagai
	// teks sehingga awalan @ tidak dibaca Excel sebagai rumus. Hanya ada saat impor disimpan.
	CredentialsFile *string `json:"credentials_file"`
}

type importColumn struct {
	key   string
	label string
	width float64
	list  string // nama daftar pilihan di sheet Daftar
	text  bool   // format teks agar nol di depan tidak hilang
	date  bool
}

var (
	genderLabels    = map[string]string{"L": "Laki-laki", "P": "Perempuan"}
	maritalLabels   = map[string]string{"belum_menikah": "Belum menikah", "menikah": "Menikah", "cerai_hidup": "Cerai hidup", "cerai_mati": "Cerai mati"}
	relationLabels  = map[string]string{"kandung": "Kandung", "angkat": "Angkat", "tiri": "Tiri"}
	educationLabels = map[string]string{
		"belum_sekolah": "Belum sekolah", "paud": "PAUD", "tk_ra": "TK/RA", "sd_mi": "SD/MI", "smp_mts": "SMP/MTs",
		"sma_sederajat": "SMA/Sederajat", "perguruan_tinggi": "Perguruan tinggi", "pondok_pesantren": "Pondok pesantren",
	}
	// Urutan tampil daftar pilihan di template.
	genderOrder    = []string{"L", "P"}
	maritalOrder   = MaritalStatuses
	relationOrder  = ChildRelations
	educationOrder = EducationLevels
)

// importColumns mengikuti urutan formulir data Pejuang yayasan: identitas, penempatan, lalu anak.
var importColumns = func() []importColumn {
	cols := []importColumn{
		{key: "nip", label: "NIPY", width: 16, text: true},
		{key: "full_name", label: "Nama lengkap*", width: 28},
		{key: "gender", label: "Jenis kelamin", width: 14, list: "gender"},
		{key: "branch", label: "Cabang*", width: 20, list: "branch"},
		{key: "division", label: "Divisi*", width: 20, list: "division"},
		{key: "subdivision", label: "Subdiv*", width: 20, list: "subdivision"},
		{key: "position", label: "Jabatan", width: 20},
		{key: "role", label: "Peran", width: 16, list: "role"},
		{key: "shift", label: "Shift", width: 18, list: "shift"},
		{key: "location1", label: "Lokasi 1", width: 22, list: "location"},
		{key: "location2", label: "Lokasi 2", width: 22, list: "location"},
		{key: "location3", label: "Lokasi 3", width: 22, list: "location"},
		{key: "phone", label: "No. WA", width: 16, text: true},
		{key: "email", label: "Email", width: 26},
		{key: "marital_status", label: "Status pernikahan", width: 18, list: "marital"},
		{key: "join_date", label: "Tanggal bergabung", width: 16, date: true},
		{key: "has_children", label: "Memiliki anak", width: 14, list: "yesno"},
		{key: "children_count", label: "Jumlah anak", width: 12},
		{key: "base_salary", label: "Ubar pokok", width: 14},
	}
	for k := 1; k <= importChildSlots; k++ {
		cols = append(cols,
			importColumn{key: fmt.Sprintf("child%d_name", k), label: fmt.Sprintf("Nama anak %d", k), width: 24},
			importColumn{key: fmt.Sprintf("child%d_gender", k), label: fmt.Sprintf("Jenis kelamin anak %d", k), width: 14, list: "gender"},
			importColumn{key: fmt.Sprintf("child%d_birth_place", k), label: fmt.Sprintf("Tempat lahir anak %d", k), width: 18},
			importColumn{key: fmt.Sprintf("child%d_birth_date", k), label: fmt.Sprintf("Tanggal lahir anak %d", k), width: 16, date: true},
			importColumn{key: fmt.Sprintf("child%d_order", k), label: fmt.Sprintf("Anak ke- (anak %d)", k), width: 12},
			importColumn{key: fmt.Sprintf("child%d_relation", k), label: fmt.Sprintf("Status anak %d", k), width: 14, list: "relation"},
			importColumn{key: fmt.Sprintf("child%d_education", k), label: fmt.Sprintf("Pendidikan anak %d", k), width: 18, list: "education"},
			importColumn{key: fmt.Sprintf("child%d_school", k), label: fmt.Sprintf("Nama sekolah anak %d", k), width: 24},
		)
	}
	return cols
}()

func columnLabel(key string) string {
	for _, c := range importColumns {
		if c.key == key {
			return strings.TrimSuffix(c.label, "*")
		}
	}
	return key
}

func normalizeHeader(s string) string {
	s = strings.ToLower(strings.TrimSpace(strings.ReplaceAll(s, "*", "")))
	return strings.Join(strings.Fields(s), " ")
}

func fileError(message string) error {
	return httpx.Validation(map[string]string{"file": message})
}

// parseLabel menerima kode (mis. "L") atau labelnya (mis. "Laki-laki") tanpa membedakan huruf besar.
func parseLabel(raw string, labels map[string]string) (string, bool) {
	v := strings.TrimSpace(raw)
	for code, label := range labels {
		if strings.EqualFold(v, code) || strings.EqualFold(v, label) {
			return code, true
		}
	}
	return "", false
}

func labelList(order []string, labels map[string]string) string {
	out := make([]string, 0, len(order))
	for _, code := range order {
		out = append(out, labels[code])
	}
	return strings.Join(out, ", ")
}

var (
	dateDMY   = regexp.MustCompile(`^(\d{1,2})[/.-](\d{1,2})[/.-](\d{4})$`)
	moneyFull = regexp.MustCompile(`^\d+(\.\d{1,2})?$`)
	nonDigit  = regexp.MustCompile(`\D`)
)

// parseImportDate menerima nomor seri tanggal Excel, 2026-01-31, atau 31/01/2026.
func parseImportDate(raw string) (string, bool) {
	v := strings.TrimSpace(raw)
	if serial, err := strconv.ParseFloat(v, 64); err == nil && serial > 0 && serial < 2958466 {
		t, err := excelize.ExcelDateToTime(serial, false)
		if err != nil {
			return "", false
		}
		return t.Format("2006-01-02"), true
	}
	if t, err := time.Parse("2006-01-02", v); err == nil {
		return t.Format("2006-01-02"), true
	}
	if m := dateDMY.FindStringSubmatch(v); m != nil {
		t, err := time.Parse("2/1/2006", m[1]+"/"+m[2]+"/"+m[3])
		if err == nil {
			return t.Format("2006-01-02"), true
		}
	}
	return "", false
}

// parseRupiah menerima 4500000, 4500000.00, atau 4.500.000.
func parseRupiah(raw string) string {
	v := strings.TrimSpace(raw)
	if moneyFull.MatchString(v) {
		if !strings.Contains(v, ".") {
			return v + ".00"
		}
		return v
	}
	digits := nonDigit.ReplaceAllString(v, "")
	if digits == "" {
		return v
	}
	return digits + ".00"
}

type importPlan struct {
	row      int
	create   bool
	id       uuid.UUID
	current  store.GetEmployeeScopedRow
	v        validated
	fullName string
	org      orgRef
	// shiftPicks adalah shift dari kolom Shift (bisa beberapa, dipisah koma, D61): id shift lama, atau
	// key shift baru dari sheet Shift yang baru punya id saat disimpan.
	shiftPicks   []shiftPick
	locations    []string
	setLocations bool
	role         string
}

type importRows struct {
	header    map[string]int
	rows      [][]string
	lines     []int
	locations [][]string
	shifts    [][]string
}

func (r importRows) get(i int, key string) string {
	col, ok := r.header[key]
	if !ok || col >= len(r.rows[i]) {
		return ""
	}
	return strings.TrimSpace(r.rows[i][col])
}

func readImportRows(body []byte) (importRows, error) {
	if !bytes.HasPrefix(body, []byte("PK\x03\x04")) {
		return importRows{}, fileError("Mohon gunakan berkas .xlsx. Silakan unduh template lalu isi datanya di sana.")
	}
	f, err := excelize.OpenReader(bytes.NewReader(body), excelize.Options{UnzipSizeLimit: 64 << 20, UnzipXMLSizeLimit: 32 << 20})
	if err != nil {
		return importRows{}, fileError("Berkas tidak dapat dibaca sebagai .xlsx. Simpan ulang dari Excel atau unduh template baru.")
	}
	defer f.Close()
	all, err := f.GetRows(importSheet, excelize.Options{RawCellValue: true})
	if err != nil || len(all) == 0 {
		return importRows{}, fileError("Sheet Pejuang belum ditemukan. Gunakan template impor terbaru.")
	}
	labels := map[string]string{}
	for _, c := range importColumns {
		labels[normalizeHeader(c.label)] = c.key
	}
	out := importRows{header: map[string]int{}}
	for i, h := range all[0] {
		if key, ok := labels[normalizeHeader(h)]; ok {
			out.header[key] = i
		}
	}
	missing := []string{}
	for _, key := range []string{"full_name", "branch", "division", "subdivision"} {
		if _, ok := out.header[key]; !ok {
			missing = append(missing, columnLabel(key))
		}
	}
	if len(missing) > 0 {
		return importRows{}, fileError("Kolom " + strings.Join(missing, ", ") + " belum ada. Gunakan template impor terbaru tanpa mengubah judul kolom.")
	}
	for i := 1; i < len(all); i++ {
		empty := true
		for _, cell := range all[i] {
			if strings.TrimSpace(cell) != "" {
				empty = false
				break
			}
		}
		if empty {
			continue
		}
		if len(out.rows) == importMaxRows {
			return importRows{}, fileError(fmt.Sprintf("Satu berkas maksimal %d Pejuang. Bagi data menjadi beberapa berkas.", importMaxRows))
		}
		out.rows = append(out.rows, all[i])
		out.lines = append(out.lines, i+1)
	}
	if len(out.rows) == 0 {
		return importRows{}, fileError("Belum ada data Pejuang di sheet Pejuang. Isi mulai baris 2.")
	}
	out.locations = sheetRows(f, locationSheet)
	out.shifts = sheetRows(f, shiftSheet)
	return out, nil
}

func orgKey(parts ...string) string {
	for i, p := range parts {
		parts[i] = strings.ToLower(strings.Join(strings.Fields(p), " "))
	}
	return strings.Join(parts, "|")
}

// Import memeriksa lalu (bila dryRun false) menyimpan seluruh baris dalam satu transaksi: semua baris
// tersimpan atau tidak sama sekali. Pratinjau juga dijalankan dalam transaksi yang dibatalkan, agar
// bentrok data (NIPY, nomor WA, email) sudah ketahuan sebelum disimpan.
func (s *Service) Import(ctx context.Context, p auth.Principal, ip audit.Entry, body []byte, dryRun bool, now time.Time) (ImportResult, error) {
	in, err := readImportRows(body)
	if err != nil {
		return ImportResult{}, err
	}
	ref, err := s.loadRefData(ctx)
	if err != nil {
		return ImportResult{}, err
	}
	nips := []string{}
	for i := range in.rows {
		if nip := strings.ToUpper(in.get(i, "nip")); nip != "" {
			nips = append(nips, nip)
		}
	}
	existingRows, err := s.q.ListEmployeesByNips(ctx, nips)
	if err != nil {
		return ImportResult{}, err
	}
	existing := map[string]uuid.UUID{}
	for _, r := range existingRows {
		existing[r.Nip] = r.ID
	}

	res := ImportResult{DryRun: dryRun, TotalRows: len(in.rows), Issues: []ImportIssue{}, Rows: []ImportRow{}}
	addSheetIssue := func(sheet string, row int, column, message string) {
		if len(res.Issues) < importMaxIssues {
			res.Issues = append(res.Issues, ImportIssue{Sheet: sheet, Row: row, Column: column, Message: message})
		}
	}
	addIssue := func(row int, key, message string) { addSheetIssue(importSheet, row, columnLabel(key), message) }

	ref.readRefSheets(in.locations, in.shifts, addSheetIssue)
	if len(ref.newLocations) > 0 && !permission.Allows(p.Role, permission.LocationManage) {
		addSheetIssue(locationSheet, 2, "Nama lokasi", "Lokasi baru hanya bisa ditambahkan oleh pengelola lokasi. Kosongkan baris lokasi baru.")
	}
	if len(ref.newShifts) > 0 && !permission.Allows(p.Role, permission.ShiftManage) {
		addSheetIssue(shiftSheet, 2, "Nama shift", "Shift baru hanya bisa ditambahkan oleh pengelola shift. Kosongkan baris shift baru.")
	}

	today := pgconv.WorkDate(now)
	seen := map[string]int{}
	heads := map[string]int{}
	plans := make([]importPlan, 0, len(in.rows))
	for i := range in.rows {
		line := in.lines[i]
		plan, ok := s.planImportRow(ctx, p, in, i, line, ref, existing, today, addIssue)
		if !ok {
			continue
		}
		// Baris dalam satu berkas tidak boleh saling bentrok.
		for key, label := range map[string]string{
			"nip:" + strings.ToUpper(plan.v.Nip):            "nip",
			"phone:" + plan.v.Phone.String:                  "phone",
			"email:" + strings.ToLower(plan.v.Email.String): "email",
		} {
			if strings.HasSuffix(key, ":") || (label == "phone" && !plan.v.Phone.Valid) || (label == "email" && !plan.v.Email.Valid) {
				continue
			}
			if first, dup := seen[key]; dup {
				addIssue(line, label, fmt.Sprintf("Sama dengan baris %d di berkas ini.", first))
				ok = false
				continue
			}
			seen[key] = line
		}
		// Satu unit hanya punya satu kepala.
		if unit := plan.headUnit(); unit != "" {
			if first, dup := heads[unit]; dup {
				addIssue(line, "role", fmt.Sprintf("Unit ini sudah diberi kepala di baris %d.", first))
				ok = false
			} else {
				heads[unit] = line
			}
		}
		if ok {
			plans = append(plans, plan)
		}
	}
	res.NewItems = ref.newItems()
	if len(res.Issues) > 0 {
		return res, nil
	}

	credentials, hashes, err := importCredentials(plans, dryRun, now)
	if err != nil {
		return ImportResult{}, err
	}
	errRollback := errors.New("impor dibatalkan")
	// Email yang diganti lewat impor diberi tahu setelah transaksi tersimpan (D59).
	type emailNotice struct {
		id       uuid.UUID
		old, new string
	}
	var notices []emailNotice
	err = database.InTx(ctx, s.pool, func(q *store.Queries) error {
		notices = notices[:0]
		res.Rows = res.Rows[:0]
		res.Created, res.Updated = 0, 0
		if err := ref.createRefs(ctx, q, ip); err != nil {
			return err
		}
		changes := []headChange{}
		moved := []uuid.UUID{}
		for _, plan := range plans {
			v := plan.v
			if plan.org.sub == uuid.Nil {
				v.SubdivisionID = ref.subs[plan.org.subKey]
			}
			if len(plan.shiftPicks) > 0 {
				ids := make([]uuid.UUID, 0, len(plan.shiftPicks))
				for _, pick := range plan.shiftPicks {
					id := pick.id
					if pick.key != "" {
						id = ref.shiftID(pick.key)
					}
					if !slices.Contains(ids, id) {
						ids = append(ids, id)
					}
				}
				v.ShiftIDs, v.ShiftIDsExplicit = ids, true
				v.DefaultShiftID = uuid.NullUUID{UUID: ids[0], Valid: true}
			}
			row := ImportRow{Row: plan.row, FullName: plan.fullName}
			id := plan.id
			if plan.create {
				credential := credentials[plan.row]
				if dryRun && v.Nip == "" {
					// NIPY otomatis tidak diambil dari urutan saat pratinjau agar nomornya tidak terlewat.
					v.Nip = fmt.Sprintf("PRATINJAU%05d", plan.row)
				}
				newID, nip, err := s.insert(ctx, q, ip, v, hashes[credential.InitialPassword], credential.Expires(), now)
				if err != nil {
					return importRowError(err, plan.row, addIssue, errRollback)
				}
				id = newID
				row.Action = "create"
				if !dryRun || plan.v.Nip != "" {
					row.Nip = &nip
				}
				if !dryRun {
					password, expires := credential.InitialPassword, credential.ExpiresAt
					row.InitialPassword, row.ExpiresAt = &password, &expires
				}
				res.Created++
			} else {
				if err := update(ctx, q, ip, plan.id, plan.current, v); err != nil {
					return importRowError(err, plan.row, addIssue, errRollback)
				}
				if v.Email != plan.current.Email {
					notices = append(notices, emailNotice{id: plan.id, old: plan.current.Email.String, new: v.Email.String})
				}
				if v.SubdivisionID != plan.current.SubdivisionID || v.Status == "inactive" {
					moved = append(moved, plan.id)
				}
				nip := v.Nip
				row.Action, row.Nip = "update", &nip
				res.Updated++
			}
			if plan.setLocations {
				if err := setImportLocations(ctx, q, ip, id, plan.locations, ref); err != nil {
					return err
				}
			}
			if plan.role != "" {
				changes = append(changes, headChange{row: plan.row, employee: id, role: plan.role, sub: v.SubdivisionID})
			}
			res.Rows = append(res.Rows, row)
		}
		if err := ref.applyHeads(ctx, q, ip, changes, moved); err != nil {
			return err
		}
		entry := ip
		entry.Action, entry.Entity = audit.EmployeeImported, "employee"
		entry.Detail = map[string]any{
			"created": res.Created, "updated": res.Updated, "rows": len(plans),
			"new_branches": len(ref.newBranches), "new_divisions": len(ref.newDivisions), "new_subdivisions": len(ref.newSubs),
			"new_locations": len(ref.newLocations), "new_shifts": len(ref.newShifts), "head_changes": len(changes),
		}
		if err := audit.Write(ctx, q, entry); err != nil {
			return err
		}
		if dryRun {
			return errRollback
		}
		return nil
	})
	switch {
	case errors.Is(err, errRollback):
		if len(res.Issues) > 0 {
			res.Rows, res.Created, res.Updated = []ImportRow{}, 0, 0
		}
		return res, nil
	case err != nil:
		return ImportResult{}, err
	}
	res.Applied = true
	if s.emails != nil {
		for _, n := range notices {
			s.emails.NotifyEmailChanged(ctx, n.id, n.old, n.new, true)
		}
	}
	if res.Created > 0 {
		file, err := credentialsWorkbook(res.Rows)
		if err != nil {
			return ImportResult{}, err
		}
		encoded := base64.StdEncoding.EncodeToString(file)
		res.CredentialsFile = &encoded
	}
	return res, nil
}

// headUnit adalah kunci unit yang dipimpin baris ini, untuk mencegah dua kepala di unit yang sama.
func (p importPlan) headUnit() string {
	sub := p.org.subKey
	if p.org.sub != uuid.Nil {
		sub = p.org.sub.String()
	}
	switch p.role {
	case roleSubdiv:
		return "subdiv:" + sub
	case roleDivision:
		return "division:" + p.org.divKey
	}
	return ""
}

// setImportLocations mengganti daftar lokasi presensi Pejuang dengan lokasi dari kolom Lokasi 1 sampai 3.
func setImportLocations(ctx context.Context, q *store.Queries, ip audit.Entry, employeeID uuid.UUID, keys []string, ref *refData) error {
	if err := q.ClearEmployeeLocations(ctx, employeeID); err != nil {
		return err
	}
	for _, key := range keys {
		if err := q.AddEmployeeLocation(ctx, store.AddEmployeeLocationParams{EmployeeID: employeeID, LocationID: ref.locationID(key)}); err != nil {
			return err
		}
	}
	entry := ip
	entry.Action, entry.Entity, entry.EntityID = audit.EmployeeUpdated, "employee", employeeID
	entry.Detail = map[string]any{"change": "locations", "count": len(keys), "source": "import"}
	return audit.Write(ctx, q, entry)
}

// credentialsWorkbook menyusun daftar kata sandi awal untuk disampaikan SDM ke Pejuang baru.
func credentialsWorkbook(rows []ImportRow) ([]byte, error) {
	f := excelize.NewFile()
	defer f.Close()
	const sheet = "Kata sandi awal"
	if err := f.SetSheetName("Sheet1", sheet); err != nil {
		return nil, err
	}
	if err := f.SetSheetRow(sheet, "A1", &[]any{"NIPY", "Nama lengkap", "Kata sandi awal", "Berlaku sampai (WIB)"}); err != nil {
		return nil, err
	}
	line := 2
	for _, r := range rows {
		if r.InitialPassword == nil || r.Nip == nil || r.ExpiresAt == nil {
			continue
		}
		expires := *r.ExpiresAt
		if t, err := time.Parse(time.RFC3339, expires); err == nil {
			expires = t.In(pgconv.WIB).Format("02-01-2006 15:04")
		}
		for col, v := range []string{*r.Nip, r.FullName, *r.InitialPassword, expires} {
			cell, _ := excelize.CoordinatesToCellName(col+1, line)
			if err := f.SetCellStr(sheet, cell, v); err != nil {
				return nil, err
			}
		}
		line++
	}
	for col, width := range map[string]float64{"A": 16, "B": 30, "C": 18, "D": 20} {
		if err := f.SetColWidth(sheet, col, col, width); err != nil {
			return nil, err
		}
	}
	bold, err := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true}})
	if err != nil {
		return nil, err
	}
	if err := f.SetCellStyle(sheet, "A1", "D1", bold); err != nil {
		return nil, err
	}
	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// importRowError mengubah galat validasi atau data kembar dari database menjadi catatan per baris.
func importRowError(err error, row int, addIssue func(int, string, string), rollback error) error {
	var apiErr *httpx.Error
	if errors.As(err, &apiErr) && (apiErr.Code == "VALIDATION_FAILED" || apiErr.Code == "CONFLICT_DUPLICATE") {
		fields, _ := apiErr.Details["fields"].(map[string]string)
		if len(fields) == 0 {
			addIssue(row, "", apiErr.Message)
		}
		for key, message := range fields {
			addIssue(row, key, message)
		}
		return rollback
	}
	return err
}

// importCredentials menyiapkan kata sandi awal Pejuang baru dan meng-hash-nya paralel sebelum
// transaksi dibuka. Pratinjau tidak meng-hash karena hasilnya dibatalkan.
func importCredentials(plans []importPlan, dryRun bool, now time.Time) (map[int]auth.InitialCredential, map[string]string, error) {
	credentials := map[int]auth.InitialCredential{}
	unique := map[string]bool{}
	for _, plan := range plans {
		if plan.create {
			c := auth.NewInitialCredential(plan.v.FullName, now)
			credentials[plan.row] = c
			unique[c.InitialPassword] = true
		}
	}
	hashes := map[string]string{}
	if dryRun {
		for password := range unique {
			hashes[password] = "pratinjau"
		}
		return credentials, hashes, nil
	}
	var (
		mu       sync.Mutex
		wg       sync.WaitGroup
		firstErr error
		work     = make(chan string)
	)
	for range importHashers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for password := range work {
				hash, err := auth.HashInitialPassword(password)
				mu.Lock()
				if err != nil && firstErr == nil {
					firstErr = err
				}
				hashes[password] = hash
				mu.Unlock()
			}
		}()
	}
	for password := range unique {
		work <- password
	}
	close(work)
	wg.Wait()
	return credentials, hashes, firstErr
}

// planImportRow menyusun Input dari satu baris lalu memakai validasi yang sama dengan form web.
func (s *Service) planImportRow(ctx context.Context, p auth.Principal, in importRows, i, line int, ref *refData, existing map[string]uuid.UUID, today time.Time, addIssue func(int, string, string)) (importPlan, bool) {
	ok := true
	issue := func(key, message string) {
		addIssue(line, key, message)
		ok = false
	}
	get := func(key string) string { return in.get(i, key) }

	plan := importPlan{row: line, fullName: get("full_name")}
	nip := strings.ToUpper(get("nip"))
	var input Input
	if id, found := existing[nip]; found && nip != "" {
		current, err := s.getScoped(ctx, p, id)
		if err != nil {
			issue("nip", "Pejuang dengan NIPY ini berada di luar akses Anda.")
			return plan, false
		}
		if err := auth.AssertEmployeeNotProtected(ctx, s.q, p, id); err != nil {
			issue("nip", "Data Pejuang ini hanya bisa diubah oleh super admin lewat form web.")
			return plan, false
		}
		plan.id, plan.current = id, current
		input = currentInput(current)
	} else {
		plan.create = true
		input = Input{Status: "active"}
		if nip != "" {
			input.Nip = &nip
		}
	}

	input.FullName = get("full_name")
	if input.FullName == "" {
		issue("full_name", "Nama lengkap wajib diisi.")
	}

	branch, division, sub := get("branch"), get("division"), get("subdivision")
	switch {
	case branch == "" || division == "" || sub == "":
		for key, v := range map[string]string{"branch": branch, "division": division, "subdivision": sub} {
			if v == "" {
				issue(key, columnLabel(key)+" wajib diisi.")
			}
		}
	default:
		// Struktur yang belum ada dibuat otomatis bila pengimpor berhak mengubah struktur.
		where, key, message := ref.resolveOrg(branch, division, sub, permission.Allows(p.Role, permission.OrgWrite))
		if message != "" {
			issue(key, message)
		} else {
			plan.org = where
			if where.sub != uuid.Nil {
				input.SubdivisionID = where.sub.String()
			}
		}
	}
	if input.SubdivisionID == "" {
		// Subdiv baru baru punya id di transaksi; galat struktur sudah dicatat di atas.
		input.SubdivisionID = uuid.Nil.String()
	}

	if v := get("role"); v != "" {
		code, found := parseLabel(v, roleLabels)
		switch {
		case !found:
			issue("role", "Pilih salah satu: "+labelList(roleOrder, roleLabels)+".")
		case !permission.Allows(p.Role, permission.OrgWrite):
			issue("role", "Kepala unit hanya diatur oleh pengelola struktur. Kosongkan kolom ini.")
		case code == roleMember && plan.create:
			// Pejuang baru belum memimpin unit apa pun.
		default:
			plan.role = code
		}
	}
	// Beberapa shift ditulis dipisah koma, misalnya "Reguler, Sabtu setengah hari" (D61).
	for _, name := range strings.Split(get("shift"), ",") {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		key := nameKey(name)
		switch ids := ref.shifts[key]; {
		case len(ids) > 1:
			issue("shift", "Ada beberapa shift bernama \""+name+"\". Bedakan namanya di web lebih dulu.")
		case len(ids) == 1:
			plan.shiftPicks = append(plan.shiftPicks, shiftPick{id: ids[0]})
		case ref.isNewShift(key):
			plan.shiftPicks = append(plan.shiftPicks, shiftPick{key: key})
		default:
			issue("shift", "Shift \""+name+"\" belum terdaftar. Tambahkan di sheet Shift atau pilih dari daftar.")
		}
	}
	for k := 1; k <= importLocationMax; k++ {
		col := fmt.Sprintf("location%d", k)
		v := get(col)
		if v == "" {
			continue
		}
		key := nameKey(v)
		switch ids := ref.locations[key]; {
		case len(ids) > 1:
			issue(col, "Ada beberapa lokasi bernama \""+v+"\". Bedakan namanya di web lebih dulu.")
			continue
		case len(ids) == 0 && !ref.isNewLocation(key):
			issue(col, "Lokasi \""+v+"\" belum terdaftar. Tambahkan di sheet Lokasi atau pilih dari daftar.")
			continue
		}
		plan.setLocations = true
		if !slices.Contains(plan.locations, key) {
			plan.locations = append(plan.locations, key)
		}
	}

	setText := func(key string, target **string) {
		if v := get(key); v != "" {
			*target = &v
		}
	}
	setText("position", &input.Position)
	setText("phone", &input.Phone)
	setText("email", &input.Email)
	setEnum := func(key string, labels map[string]string, order []string, target **string) {
		v := get(key)
		if v == "" {
			return
		}
		code, found := parseLabel(v, labels)
		if !found {
			issue(key, "Pilih salah satu: "+labelList(order, labels)+".")
			return
		}
		*target = &code
	}
	setEnum("gender", genderLabels, genderOrder, &input.Gender)
	setEnum("marital_status", maritalLabels, maritalOrder, &input.MaritalStatus)
	if v := get("join_date"); v != "" {
		if d, found := parseImportDate(v); found {
			input.JoinDate = &d
		} else {
			issue("join_date", "Tulis tanggal seperti 2026-01-31 atau 31/01/2026.")
		}
	}
	if v := get("has_children"); v != "" {
		switch strings.ToLower(v) {
		case "ya", "y", "true", "1":
			yes := true
			input.HasChildren = &yes
		case "tidak", "t", "false", "0":
			no := false
			input.HasChildren = &no
		default:
			issue("has_children", "Isi Ya atau Tidak.")
		}
	}
	if v := get("children_count"); v != "" {
		n, err := parseSmallInt(v)
		if err != nil {
			issue("children_count", "Isi dengan angka, contoh 2.")
		} else {
			input.ChildrenCount = &n
		}
	}
	if v := get("base_salary"); v != "" {
		if !permission.ManagesSalary(p.Role) {
			issue("base_salary", "Ubar pokok hanya diisi oleh SDM. Kosongkan kolom ini.")
		} else {
			input.BaseSalary = parseRupiah(v)
		}
	} else if plan.create {
		input.BaseSalary = "0.00"
	}

	// Data anak 1 sampai 3. Blok yang kosong seluruhnya dilewati.
	children := []Child{}
	slots := []int{}
	for k := 1; k <= importChildSlots; k++ {
		key := func(field string) string { return fmt.Sprintf("child%d_%s", k, field) }
		name := get(key("name"))
		filled := name != ""
		for _, field := range []string{"gender", "birth_place", "birth_date", "order", "relation", "education", "school"} {
			if get(key(field)) != "" {
				filled = true
			}
		}
		if !filled {
			continue
		}
		if name == "" {
			issue(key("name"), fmt.Sprintf("Nama anak %d wajib diisi bila data anak %d diisi.", k, k))
			continue
		}
		child := Child{ChildOrder: k, FullName: name}
		if v := get(key("order")); v != "" {
			n, err := parseSmallInt(v)
			if err != nil {
				issue(key("order"), "Isi dengan angka, contoh 1.")
			} else {
				child.ChildOrder = n
			}
		}
		setEnum(key("gender"), genderLabels, genderOrder, &child.Gender)
		setEnum(key("relation"), relationLabels, relationOrder, &child.Relation)
		setEnum(key("education"), educationLabels, educationOrder, &child.Education)
		if v := get(key("birth_place")); v != "" {
			child.BirthPlace = &v
		}
		if v := get(key("school")); v != "" {
			child.SchoolName = &v
		}
		if v := get(key("birth_date")); v != "" {
			if d, found := parseImportDate(v); found {
				child.BirthDate = &d
			} else {
				issue(key("birth_date"), "Tulis tanggal seperti 2015-04-02 atau 02/04/2015.")
			}
		}
		children = append(children, child)
		slots = append(slots, k)
	}
	// Pejuang lama: data anak hanya diganti bila berkas mengisinya atau menyatakan tidak memiliki anak.
	if plan.create || len(children) > 0 || (input.HasChildren != nil && !*input.HasChildren) {
		input.Children = &children
	}
	if !ok {
		return plan, false
	}

	v, err := input.validate(today)
	if err != nil {
		var apiErr *httpx.Error
		if errors.As(err, &apiErr) {
			fields, _ := apiErr.Details["fields"].(map[string]string)
			keys := make([]string, 0, len(fields))
			for key := range fields {
				keys = append(keys, key)
			}
			sort.Strings(keys)
			for _, key := range keys {
				addIssue(line, importFieldKey(key, slots), fields[key])
			}
			return plan, false
		}
		addIssue(line, "", err.Error())
		return plan, false
	}
	if !plan.create && v.Nip == "" {
		v.Nip = plan.current.Nip
	}
	if plan.create && !v.JoinDate.Valid {
		v.JoinDate = pgconv.Date(today)
	}
	plan.v = v
	return plan, true
}

// importFieldKey memetakan nama field validasi (mis. children.0.full_name) ke kolom template.
func importFieldKey(field string, slots []int) string {
	if rest, found := strings.CutPrefix(field, "children."); found {
		idx, sub, _ := strings.Cut(rest, ".")
		n, err := strconv.Atoi(idx)
		if err == nil && n < len(slots) {
			mapping := map[string]string{"full_name": "name", "child_order": "order", "school_name": "school"}
			if m, has := mapping[sub]; has {
				sub = m
			}
			return fmt.Sprintf("child%d_%s", slots[n], sub)
		}
		return "child1_name"
	}
	return field
}

// ImportTemplate membuat template xlsx: sheet Pejuang untuk diisi, Petunjuk, Struktur organisasi, Lokasi,
// Shift, dan daftar pilihan tersembunyi untuk dropdown.
func (s *Service) ImportTemplate(ctx context.Context) ([]byte, error) {
	subs, err := s.q.ListSubdivisions(ctx, uuid.NullUUID{})
	if err != nil {
		return nil, err
	}
	locations, err := s.q.ListOfficeLocations(ctx)
	if err != nil {
		return nil, err
	}
	shifts, err := s.q.ListShifts(ctx)
	if err != nil {
		return nil, err
	}
	f := excelize.NewFile()
	defer f.Close()
	if err := f.SetSheetName("Sheet1", importSheet); err != nil {
		return nil, err
	}
	for _, name := range []string{"Petunjuk", "Struktur", locationSheet, shiftSheet, "Daftar"} {
		if _, err := f.NewSheet(name); err != nil {
			return nil, err
		}
	}

	header, err := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Color: "FFFFFF"},
		Fill:      excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"15803D"}},
		Alignment: &excelize.Alignment{Vertical: "center", WrapText: true},
	})
	if err != nil {
		return nil, err
	}
	textFmt, err := f.NewStyle(&excelize.Style{NumFmt: 49})
	if err != nil {
		return nil, err
	}
	dateFormat := "yyyy-mm-dd"
	dateFmt, err := f.NewStyle(&excelize.Style{CustomNumFmt: &dateFormat})
	if err != nil {
		return nil, err
	}
	existingFill, err := f.NewStyle(&excelize.Style{NumFmt: 49, Fill: excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"F1F5F9"}}})
	if err != nil {
		return nil, err
	}

	// Daftar pilihan untuk dropdown.
	unique := func(values []string) []string {
		set := map[string]bool{}
		out := []string{}
		for _, v := range values {
			if !set[v] {
				set[v] = true
				out = append(out, v)
			}
		}
		sort.Strings(out)
		return out
	}
	var branchNames, divisionNames, subNames []string
	for _, r := range subs {
		branchNames = append(branchNames, r.BranchName)
		divisionNames = append(divisionNames, r.DivisionName)
		subNames = append(subNames, r.Name)
	}
	labelsOf := func(order []string, labels map[string]string) []string {
		out := []string{}
		for _, code := range order {
			out = append(out, labels[code])
		}
		return out
	}
	lists := []struct {
		name   string
		title  string
		values []string
	}{
		{"branch", "Cabang", unique(branchNames)},
		{"division", "Divisi", unique(divisionNames)},
		{"subdivision", "Subdiv", unique(subNames)},
		{"role", "Peran", labelsOf(roleOrder, roleLabels)},
		{"gender", "Jenis kelamin", labelsOf(genderOrder, genderLabels)},
		{"marital", "Status pernikahan", labelsOf(maritalOrder, maritalLabels)},
		{"yesno", "Memiliki anak", []string{"Ya", "Tidak"}},
		{"relation", "Status anak", labelsOf(relationOrder, relationLabels)},
		{"education", "Pendidikan", labelsOf(educationOrder, educationLabels)},
	}
	listRef := map[string]string{
		// Shift dan lokasi merujuk sheet-nya sendiri, sehingga baris baru langsung muncul di pilihan.
		"shift":    fmt.Sprintf("%s!$A$2:$A$%d", shiftSheet, refSheetMaxRows+1),
		"location": fmt.Sprintf("%s!$A$2:$A$%d", locationSheet, refSheetMaxRows+1),
	}
	for i, list := range lists {
		col, _ := excelize.ColumnNumberToName(i + 1)
		if err := f.SetCellValue("Daftar", col+"1", list.title); err != nil {
			return nil, err
		}
		for j, v := range list.values {
			if err := f.SetCellValue("Daftar", fmt.Sprintf("%s%d", col, j+2), v); err != nil {
				return nil, err
			}
		}
		if len(list.values) > 0 {
			listRef[list.name] = fmt.Sprintf("Daftar!$%s$2:$%s$%d", col, col, len(list.values)+1)
		}
	}
	if err := f.SetSheetVisible("Daftar", false); err != nil {
		return nil, err
	}
	// Cabang, divisi, dan subdiv boleh diisi nama baru: unit yang belum ada dibuat saat impor.
	openLists := map[string]bool{"branch": true, "division": true, "subdivision": true}

	// Sheet Pejuang.
	lastRow := importMaxRows + 1
	for i, c := range importColumns {
		col, _ := excelize.ColumnNumberToName(i + 1)
		if err := f.SetCellValue(importSheet, col+"1", c.label); err != nil {
			return nil, err
		}
		if err := f.SetColWidth(importSheet, col, col, c.width); err != nil {
			return nil, err
		}
		cells := fmt.Sprintf("%s2:%s%d", col, col, lastRow)
		switch {
		case c.text:
			if err := f.SetCellStyle(importSheet, col+"2", fmt.Sprintf("%s%d", col, lastRow), textFmt); err != nil {
				return nil, err
			}
		case c.date:
			if err := f.SetCellStyle(importSheet, col+"2", fmt.Sprintf("%s%d", col, lastRow), dateFmt); err != nil {
				return nil, err
			}
		}
		if ref, has := listRef[c.list]; has {
			dv := excelize.NewDataValidation(true)
			dv.SetSqref(cells)
			dv.SetSqrefDropList(ref)
			if openLists[c.list] {
				dv.SetError(excelize.DataValidationErrorStyleInformation, "Unit baru", "Nama yang belum ada di daftar akan dibuat sebagai unit baru saat impor.")
			} else {
				dv.SetError(excelize.DataValidationErrorStyleStop, "Pilihan belum sesuai", "Pilih salah satu nilai dari daftar.")
			}
			if err := f.AddDataValidation(importSheet, dv); err != nil {
				return nil, err
			}
		}
	}
	lastCol, _ := excelize.ColumnNumberToName(len(importColumns))
	if err := f.SetCellStyle(importSheet, "A1", lastCol+"1", header); err != nil {
		return nil, err
	}
	if err := f.SetRowHeight(importSheet, 1, 32); err != nil {
		return nil, err
	}
	if err := f.SetPanes(importSheet, &excelize.Panes{Freeze: true, YSplit: 1, TopLeftCell: "A2", ActivePane: "bottomLeft"}); err != nil {
		return nil, err
	}

	// refSheet menulis judul kolom dan baris yang sudah terdaftar (berlatar abu-abu).
	refSheet := func(sheet string, titles []string, widths []float64, rows [][]any, textCols []string) error {
		for i, title := range titles {
			col, _ := excelize.ColumnNumberToName(i + 1)
			if err := f.SetCellValue(sheet, col+"1", title); err != nil {
				return err
			}
			if err := f.SetColWidth(sheet, col, col, widths[i]); err != nil {
				return err
			}
		}
		lastCol, _ := excelize.ColumnNumberToName(len(titles))
		if err := f.SetCellStyle(sheet, "A1", lastCol+"1", header); err != nil {
			return err
		}
		for _, col := range textCols {
			if err := f.SetCellStyle(sheet, col+"2", fmt.Sprintf("%s%d", col, refSheetMaxRows+1), textFmt); err != nil {
				return err
			}
		}
		for i, r := range rows {
			if err := f.SetSheetRow(sheet, fmt.Sprintf("A%d", i+2), &r); err != nil {
				return err
			}
		}
		if len(rows) > 0 {
			if err := f.SetCellStyle(sheet, "A2", fmt.Sprintf("%s%d", lastCol, len(rows)+1), existingFill); err != nil {
				return err
			}
		}
		return f.SetPanes(sheet, &excelize.Panes{Freeze: true, YSplit: 1, TopLeftCell: "A2", ActivePane: "bottomLeft"})
	}

	// Sheet Struktur: pasangan Cabang, Divisi, dan Subdiv yang berlaku.
	structure := make([][]any, 0, len(subs))
	for _, r := range subs {
		structure = append(structure, []any{r.BranchName, r.DivisionName, r.Name, r.Code})
	}
	if err := refSheet("Struktur", []string{"Cabang", "Divisi", "Subdiv", "Kode subdiv"}, []float64{24, 24, 24, 16}, structure, nil); err != nil {
		return nil, err
	}

	// Sheet Lokasi: lokasi terdaftar, lalu baris kosong untuk lokasi baru.
	locRows := make([][]any, 0, len(locations))
	for _, l := range locations {
		locRows = append(locRows, []any{l.Name, l.Latitude.StringFixed(6), l.Longitude.StringFixed(6), strconv.Itoa(int(l.RadiusM))})
	}
	if err := refSheet(locationSheet, []string{"Nama lokasi*", "Lintang*", "Bujur*", "Radius (meter)"}, []float64{30, 14, 14, 14}, locRows, []string{"B", "C"}); err != nil {
		return nil, err
	}

	// Sheet Shift: shift terdaftar, lalu baris kosong untuk shift baru.
	shiftRows := make([][]any, 0, len(shifts))
	for _, sh := range shifts {
		shiftRows = append(shiftRows, []any{sh.Name, clockString(sh.StartTime), clockString(sh.EndTime), strconv.Itoa(int(sh.LateToleranceMinutes)), workDaysString(sh.WorkDays), shiftKindLabel(sh.Kind)})
	}
	if err := refSheet(shiftSheet, []string{"Nama shift*", "Jam masuk*", "Jam pulang*", "Toleransi telat (menit)", "Hari kerja", "Jenis"}, []float64{24, 12, 12, 14, 18, 16}, shiftRows, []string{"B", "C", "E"}); err != nil {
		return nil, err
	}

	// Sheet Petunjuk.
	notes := []string{
		"Petunjuk impor data Pejuang",
		"",
		"1. Isi satu Pejuang per baris di sheet Pejuang, mulai baris 2. Mohon judul kolom di baris 1 tidak diubah.",
		"2. Kolom bertanda * wajib diisi: Nama lengkap, Cabang, Divisi, dan Subdiv. Struktur yang sudah ada tercantum di sheet Struktur.",
		"3. Cabang, Divisi, atau Subdiv yang belum ada boleh ditulis langsung. Unit baru itu dibuat otomatis saat impor, dengan kode yang bisa diubah nanti di Master Data.",
		"4. Kolom Peran: Kepala Subdiv menjadikan Pejuang itu kepala subdiv tempatnya, Kepala Divisi menjadikannya kepala divisinya. Kepala lama diganti dan akun keduanya menyesuaikan. Pejuang berarti melepas jabatan kepala yang sedang dipegang. Kosong berarti tidak diubah.",
		"5. Kolom Shift dan Lokasi 1 sampai 3 dipilih dari sheet Shift dan Lokasi. Lokasi yang diisi menggantikan daftar lokasi presensi Pejuang tersebut. Kosong berarti tidak diubah.",
		"6. Shift atau lokasi baru ditambahkan di baris kosong sheet Shift atau Lokasi, lalu langsung bisa dipilih di sheet Pejuang. Baris berlatar abu-abu sudah terdaftar dan tidak diubah lewat impor; ubah lewat web.",
		"7. Lokasi baru: isi nama, lintang, dan bujur (contoh -6.200100 dan 106.816600), serta radius 10 sampai 5000 meter (kosong berarti 100).",
		"8. Shift baru: jam ditulis JJ:MM (contoh 07:30), toleransi telat 0 sampai 240 menit, hari kerja seperti 1-5 atau Senin-Jumat (kosong berarti Senin sampai Jumat).",
		"9. NIPY yang sudah terdaftar akan memperbarui data Pejuang tersebut, dan kata sandinya tetap. Kolom yang dibiarkan kosong tidak mengubah data lama.",
		"10. NIPY yang belum terdaftar, atau yang dikosongkan, akan dibuat sebagai Pejuang baru. NIPY kosong dibuat otomatis.",
		"11. Pejuang baru langsung mendapat kata sandi awal @NamaDepan+tahun yang berlaku 72 jam. Daftarnya tampil setelah impor disimpan dan bisa diunduh.",
		"12. No. WA boleh ditulis 08…, +62…, atau 62…. Tanggal ditulis 2026-01-31 atau 31/01/2026.",
		"13. Ubar pokok diisi oleh SDM berupa angka rupiah, contoh 4500000. Bila kosong, Pejuang baru tercatat 0 dan bisa dilengkapi nanti.",
		"14. Data anak 1 sampai 3 diisi bila ada. Anak berikutnya bisa dilengkapi lewat form Pejuang di web. Data anak hanya diubah oleh SDM.",
		fmt.Sprintf("15. Satu berkas memuat paling banyak %d Pejuang dan berukuran paling besar 5 MB.", importMaxRows),
		"16. Sistem memeriksa semua baris lebih dulu, termasuk unit, lokasi, dan shift baru. Bila ada yang perlu diperbaiki, belum ada data yang disimpan sampai semuanya sesuai.",
	}
	if err := f.SetColWidth("Petunjuk", "A", "A", 140); err != nil {
		return nil, err
	}
	for i, note := range notes {
		if err := f.SetCellValue("Petunjuk", fmt.Sprintf("A%d", i+1), note); err != nil {
			return nil, err
		}
	}
	bold, err := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true, Size: 13}})
	if err != nil {
		return nil, err
	}
	if err := f.SetCellStyle("Petunjuk", "A1", "A1", bold); err != nil {
		return nil, err
	}
	f.SetActiveSheet(0)

	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// clockString menulis jam shift sebagai JJ:MM.
func clockString(t pgtype.Time) string {
	minutes := t.Microseconds / 60_000_000
	return fmt.Sprintf("%02d:%02d", minutes/60, minutes%60)
}

// workDaysString meringkas hari kerja, misalnya [1 2 3 4 5 6] menjadi 1-6 dan [1 3 5] menjadi 1,3,5.
func workDaysString(days []int16) string {
	parts := []string{}
	for i := 0; i < len(days); {
		j := i
		for j+1 < len(days) && days[j+1] == days[j]+1 {
			j++
		}
		if j > i {
			parts = append(parts, fmt.Sprintf("%d-%d", days[i], days[j]))
		} else {
			parts = append(parts, strconv.Itoa(int(days[i])))
		}
		i = j + 1
	}
	return strings.Join(parts, ",")
}

// errImportTooLarge dipakai handler saat berkas melebihi ImportMaxBytes.
func errImportTooLarge() error {
	return httpx.NewError(http.StatusRequestEntityTooLarge, "PAYLOAD_TOO_LARGE", "Ukuran berkas paling besar 5 MB.")
}

// parseSmallInt membaca angka kecil dari sel (mis. "2" atau "2.0"). Hasilnya dibatasi 16 bit karena
// disimpan sebagai SMALLINT; nilai di luar jangkauan dianggap bukan angka yang sah.
func parseSmallInt(v string) (int, error) {
	n, err := strconv.ParseInt(strings.TrimSuffix(v, ".0"), 10, 16)
	return int(n), err
}
