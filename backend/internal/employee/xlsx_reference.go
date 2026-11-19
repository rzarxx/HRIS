package employee

import (
	"context"
	"fmt"
	"math"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/shopspring/decimal"
	"github.com/xuri/excelize/v2"

	"github.com/rzarxx/HRIS/backend/internal/audit"
	"github.com/rzarxx/HRIS/backend/internal/org"
	"github.com/rzarxx/HRIS/backend/internal/store"
)

// Data rujukan impor (D53): struktur, lokasi kantor, dan shift. Yang sudah terdaftar dipakai apa adanya;
// yang belum ada dibuat dalam transaksi impor yang sama, sehingga ikut batal bila impor batal.
const (
	locationSheet     = "Lokasi"
	shiftSheet        = "Shift"
	importLocationMax = 3
	refSheetMaxRows   = 300
)

// Peran di kolom Peran. Kosong berarti tidak mengubah kepala unit.
const (
	roleMember   = "pejuang"
	roleSubdiv   = "subdiv"
	roleDivision = "kadiv"
)

var roleLabels = map[string]string{roleMember: "Pejuang", roleSubdiv: "Kepala Subdiv", roleDivision: "Kepala Divisi"}
var roleOrder = []string{roleMember, roleSubdiv, roleDivision}

// ImportNewItems adalah daftar data rujukan baru yang dibuat impor, untuk ditampilkan di pratinjau.
type ImportNewItems struct {
	Branches     []string `json:"branches"`
	Divisions    []string `json:"divisions"`
	Subdivisions []string `json:"subdivisions"`
	Locations    []string `json:"locations"`
	Shifts       []string `json:"shifts"`
}

func (n ImportNewItems) empty() bool {
	return len(n.Branches)+len(n.Divisions)+len(n.Subdivisions)+len(n.Locations)+len(n.Shifts) == 0
}

type newLocation struct {
	key    string
	params store.CreateOfficeLocationParams
}

type newShift struct {
	key    string
	params store.InsertShiftParams
}

// refData memuat data rujukan yang sudah ada dan yang akan dibuat.
type refData struct {
	branches  map[string]uuid.UUID // orgKey(cabang)
	divisions map[string]uuid.UUID // orgKey(cabang, divisi)
	subs      map[string]uuid.UUID // orgKey(cabang, divisi, subdiv) dan kode subdiv
	subDiv    map[uuid.UUID]uuid.UUID

	branchCodes   map[string]bool
	branchNames   map[string]bool
	divisionCodes map[string]map[string]bool // branch key -> kode
	subCodes      map[string]map[string]bool // division key -> kode

	locations map[string][]uuid.UUID // nama (huruf kecil) -> id
	shifts    map[string][]uuid.UUID

	newLocations []newLocation
	newShifts    []newShift
	// Struktur baru, berurutan sesuai kemunculan di sheet Pejuang.
	newBranches  []string // orgKey
	newDivisions []string
	newSubs      []string
	labels       map[string]string // orgKey -> nama tampil

	// Diisi saat transaksi berjalan untuk data rujukan baru.
	createdLocations map[string]uuid.UUID
	createdShifts    map[string]uuid.UUID
}

func nameKey(s string) string { return strings.ToLower(strings.Join(strings.Fields(s), " ")) }

func (s *Service) loadRefData(ctx context.Context) (*refData, error) {
	d := &refData{
		branches: map[string]uuid.UUID{}, divisions: map[string]uuid.UUID{}, subs: map[string]uuid.UUID{}, subDiv: map[uuid.UUID]uuid.UUID{},
		branchCodes: map[string]bool{}, branchNames: map[string]bool{}, divisionCodes: map[string]map[string]bool{}, subCodes: map[string]map[string]bool{},
		locations: map[string][]uuid.UUID{}, shifts: map[string][]uuid.UUID{}, labels: map[string]string{},
		createdLocations: map[string]uuid.UUID{}, createdShifts: map[string]uuid.UUID{},
	}
	branches, err := s.q.ListBranches(ctx)
	if err != nil {
		return nil, err
	}
	branchKeyOf := map[uuid.UUID]string{}
	for _, b := range branches {
		key := orgKey(b.Name)
		d.branches[key] = b.ID
		d.branchCodes[b.Code] = true
		d.branchNames[key] = true
		d.labels[key] = b.Name
		branchKeyOf[b.ID] = key
	}
	divisions, err := s.q.ListDivisions(ctx, uuid.NullUUID{})
	if err != nil {
		return nil, err
	}
	divisionKeyOf := map[uuid.UUID]string{}
	for _, v := range divisions {
		bk := branchKeyOf[v.BranchID]
		key := bk + "|" + orgKey(v.Name)
		d.divisions[key] = v.ID
		d.labels[key] = v.Name
		divisionKeyOf[v.ID] = key
		if d.divisionCodes[bk] == nil {
			d.divisionCodes[bk] = map[string]bool{}
		}
		d.divisionCodes[bk][v.Code] = true
	}
	subs, err := s.q.ListSubdivisions(ctx, uuid.NullUUID{})
	if err != nil {
		return nil, err
	}
	for _, r := range subs {
		dk := divisionKeyOf[r.DivisionID]
		d.subs[dk+"|"+orgKey(r.Name)] = r.ID
		d.subs[dk+"|"+orgKey(r.Code)] = r.ID
		d.subDiv[r.ID] = r.DivisionID
		if d.subCodes[dk] == nil {
			d.subCodes[dk] = map[string]bool{}
		}
		d.subCodes[dk][r.Code] = true
	}
	locations, err := s.q.ListOfficeLocations(ctx)
	if err != nil {
		return nil, err
	}
	for _, l := range locations {
		d.locations[nameKey(l.Name)] = append(d.locations[nameKey(l.Name)], l.ID)
	}
	shifts, err := s.q.ListShifts(ctx)
	if err != nil {
		return nil, err
	}
	for _, sh := range shifts {
		d.shifts[nameKey(sh.Name)] = append(d.shifts[nameKey(sh.Name)], sh.ID)
	}
	return d, nil
}

// refIssue mencatat galat di sheet rujukan (Lokasi atau Shift).
type refIssue func(sheet string, row int, column, message string)

var (
	clockText = regexp.MustCompile(`^(\d{1,2})[:.](\d{2})$`)
	dayRange  = regexp.MustCompile(`^(\S+)\s*-\s*(\S+)$`)
	dayNames  = map[string]int16{"senin": 1, "selasa": 2, "rabu": 3, "kamis": 4, "jumat": 5, "jum'at": 5, "sabtu": 6, "ahad": 7, "minggu": 7}
)

// parseClockCell menerima 07:30, 7.30, atau nilai waktu Excel (pecahan hari).
func parseClockCell(raw string) (pgtype.Time, bool) {
	v := strings.TrimSpace(raw)
	minutes := -1
	if m := clockText.FindStringSubmatch(v); m != nil {
		h, _ := strconv.Atoi(m[1])
		mm, _ := strconv.Atoi(m[2])
		if h <= 23 && mm <= 59 {
			minutes = h*60 + mm
		}
	} else if f, err := strconv.ParseFloat(v, 64); err == nil && f >= 0 && f < 1 {
		minutes = int(math.Round(f * 1440))
		if minutes == 1440 {
			minutes = -1
		}
	}
	if minutes < 0 {
		return pgtype.Time{}, false
	}
	return pgtype.Time{Microseconds: int64(minutes) * 60_000_000, Valid: true}, true
}

func parseDay(s string) (int16, bool) {
	s = strings.ToLower(strings.TrimSpace(s))
	if n, err := strconv.Atoi(s); err == nil && n >= 1 && n <= 7 {
		return int16(n), true
	}
	d, ok := dayNames[s]
	return d, ok
}

// parseWorkDays menerima 1-5, 1,2,3,4,5, Senin-Jumat, atau Senin, Rabu. Kosong berarti Senin sampai Jumat.
func parseWorkDays(raw string) ([]int16, bool) {
	v := strings.TrimSpace(raw)
	if v == "" {
		return []int16{1, 2, 3, 4, 5}, true
	}
	days := []int16{}
	for _, part := range strings.FieldsFunc(v, func(r rune) bool { return r == ',' || r == ';' }) {
		part = strings.TrimSpace(part)
		if m := dayRange.FindStringSubmatch(part); m != nil {
			from, ok1 := parseDay(m[1])
			to, ok2 := parseDay(m[2])
			if !ok1 || !ok2 || from > to {
				return nil, false
			}
			for d := from; d <= to; d++ {
				days = append(days, d)
			}
			continue
		}
		d, ok := parseDay(part)
		if !ok {
			return nil, false
		}
		days = append(days, d)
	}
	slices.Sort(days)
	days = slices.Compact(days)
	return days, len(days) > 0
}

// sheetRows membaca sheet rujukan. Sheet yang tidak ada (template lama) dianggap kosong.
func sheetRows(f *excelize.File, sheet string) [][]string {
	rows, err := f.GetRows(sheet, excelize.Options{RawCellValue: true})
	if err != nil || len(rows) < 2 {
		return nil
	}
	if len(rows) > refSheetMaxRows+1 {
		rows = rows[:refSheetMaxRows+1]
	}
	return rows
}

func cell(row []string, i int) string {
	if i < len(row) {
		return strings.TrimSpace(row[i])
	}
	return ""
}

// readRefSheets membaca baris baru di sheet Lokasi dan Shift. Nama yang sudah terdaftar hanya menjadi
// rujukan dan datanya tidak diubah; perubahannya dilakukan lewat web.
func (d *refData) readRefSheets(locationRows, shiftRows [][]string, issue refIssue) {
	seen := map[string]int{}
	for i, row := range locationRows {
		if i == 0 {
			continue
		}
		line := i + 1
		name := cell(row, 0)
		if name == "" {
			if cell(row, 1) != "" || cell(row, 2) != "" {
				issue(locationSheet, line, "Nama lokasi", "Nama lokasi wajib diisi.")
			}
			continue
		}
		key := nameKey(name)
		if first, dup := seen[key]; dup {
			issue(locationSheet, line, "Nama lokasi", fmt.Sprintf("Sama dengan baris %d di sheet Lokasi.", first))
			continue
		}
		seen[key] = line
		if len(d.locations[key]) > 0 {
			continue
		}
		ok := true
		bad := func(column, message string) {
			issue(locationSheet, line, column, message)
			ok = false
		}
		if len([]rune(name)) > 120 {
			bad("Nama lokasi", "Nama maksimal 120 karakter.")
		}
		lat, err := strconv.ParseFloat(strings.ReplaceAll(cell(row, 1), ",", "."), 64)
		if err != nil || math.IsNaN(lat) || lat < -90 || lat > 90 {
			bad("Lintang", "Lintang diisi angka antara -90 dan 90, contoh -6.200100.")
		}
		lon, err := strconv.ParseFloat(strings.ReplaceAll(cell(row, 2), ",", "."), 64)
		if err != nil || math.IsNaN(lon) || lon < -180 || lon > 180 {
			bad("Bujur", "Bujur diisi angka antara -180 dan 180, contoh 106.816600.")
		}
		radius := int64(100)
		if v := cell(row, 3); v != "" {
			n, err := strconv.ParseInt(strings.TrimSuffix(v, ".0"), 10, 32)
			if err != nil || n < 10 || n > 5000 {
				bad("Radius (meter)", "Radius diisi 10 sampai 5000 meter.")
			} else {
				radius = n
			}
		}
		if !ok {
			continue
		}
		d.newLocations = append(d.newLocations, newLocation{key: key, params: store.CreateOfficeLocationParams{
			Name: strings.Join(strings.Fields(name), " "), Latitude: decimal.NewFromFloat(lat).Round(6), Longitude: decimal.NewFromFloat(lon).Round(6),
			RadiusM: int32(radius), IsActive: true,
		}})
	}

	seen = map[string]int{}
	for i, row := range shiftRows {
		if i == 0 {
			continue
		}
		line := i + 1
		name := cell(row, 0)
		if name == "" {
			if cell(row, 1) != "" || cell(row, 2) != "" {
				issue(shiftSheet, line, "Nama shift", "Nama shift wajib diisi.")
			}
			continue
		}
		key := nameKey(name)
		if first, dup := seen[key]; dup {
			issue(shiftSheet, line, "Nama shift", fmt.Sprintf("Sama dengan baris %d di sheet Shift.", first))
			continue
		}
		seen[key] = line
		if len(d.shifts[key]) > 0 {
			continue
		}
		ok := true
		bad := func(column, message string) {
			issue(shiftSheet, line, column, message)
			ok = false
		}
		if len([]rune(name)) > 80 {
			bad("Nama shift", "Nama maksimal 80 karakter.")
		}
		start, okStart := parseClockCell(cell(row, 1))
		if !okStart {
			bad("Jam masuk", "Tulis jam dengan format JJ:MM, contoh 07:30.")
		}
		end, okEnd := parseClockCell(cell(row, 2))
		if !okEnd {
			bad("Jam pulang", "Tulis jam dengan format JJ:MM, contoh 16:00.")
		}
		if okStart && okEnd && start == end {
			bad("Jam pulang", "Jam pulang perlu berbeda dari jam masuk.")
		}
		tolerance := int64(0)
		if v := cell(row, 3); v != "" {
			n, err := strconv.ParseInt(strings.TrimSuffix(v, ".0"), 10, 32)
			if err != nil || n < 0 || n > 240 {
				bad("Toleransi telat (menit)", "Toleransi telat diisi 0 sampai 240 menit.")
			} else {
				tolerance = n
			}
		}
		days, okDays := parseWorkDays(cell(row, 4))
		if !okDays {
			bad("Hari kerja", "Tulis hari kerja seperti 1-5, Senin-Jumat, atau Senin, Rabu, Jumat.")
		}
		if !ok {
			continue
		}
		d.newShifts = append(d.newShifts, newShift{key: key, params: store.InsertShiftParams{
			Name: strings.Join(strings.Fields(name), " "), StartTime: start, EndTime: end, LateToleranceMinutes: int32(tolerance), WorkDays: days,
		}})
	}
}

func (d *refData) isNewLocation(key string) bool {
	return slices.ContainsFunc(d.newLocations, func(l newLocation) bool { return l.key == key })
}

func (d *refData) isNewShift(key string) bool {
	return slices.ContainsFunc(d.newShifts, func(s newShift) bool { return s.key == key })
}

// orgRef adalah penempatan satu baris. sub terisi bila subdiv sudah terdaftar; bila tidak, subKey
// menunjuk subdiv baru yang dibuat dalam transaksi.
type orgRef struct {
	sub    uuid.UUID
	subKey string
	divKey string
}

// resolveOrg mencari pasangan cabang, divisi, dan subdiv. Yang belum ada dicatat untuk dibuat bila
// pengimpor berhak mengubah struktur.
func (d *refData) resolveOrg(branch, division, sub string, canCreate bool) (orgRef, string, string) {
	bk := orgKey(branch)
	dk := bk + "|" + orgKey(division)
	sk := dk + "|" + orgKey(sub)
	if id, ok := d.subs[sk]; ok {
		return orgRef{sub: id, divKey: dk}, "", ""
	}
	if !canCreate {
		switch {
		case d.branches[bk] == uuid.Nil:
			return orgRef{}, "branch", "Cabang \"" + branch + "\" belum terdaftar. Pilih dari daftar di template."
		case d.divisions[dk] == uuid.Nil:
			return orgRef{}, "division", "Divisi \"" + division + "\" belum ada di cabang " + branch + ". Lihat sheet Struktur."
		default:
			return orgRef{}, "subdivision", "Subdiv \"" + sub + "\" belum ada di divisi " + division + ". Lihat sheet Struktur."
		}
	}
	for field, v := range map[string]string{"branch": branch, "division": division, "subdivision": sub} {
		if len([]rune(strings.TrimSpace(v))) > 80 {
			return orgRef{}, field, columnLabel(field) + " maksimal 80 karakter."
		}
	}
	if _, ok := d.branches[bk]; !ok && !slices.Contains(d.newBranches, bk) {
		d.newBranches = append(d.newBranches, bk)
		d.labels[bk] = strings.Join(strings.Fields(branch), " ")
	}
	if _, ok := d.divisions[dk]; !ok && !slices.Contains(d.newDivisions, dk) {
		d.newDivisions = append(d.newDivisions, dk)
		d.labels[dk] = strings.Join(strings.Fields(division), " ")
	}
	if !slices.Contains(d.newSubs, sk) {
		d.newSubs = append(d.newSubs, sk)
		d.labels[sk] = strings.Join(strings.Fields(sub), " ")
	}
	return orgRef{subKey: sk, divKey: dk}, "", ""
}

var nonCode = regexp.MustCompile(`[^A-Z0-9]+`)

// makeCode menyusun kode dari nama (huruf besar, angka, dan tanda minus) yang belum dipakai.
func makeCode(name string, maxLen int, taken map[string]bool) string {
	base := strings.Trim(nonCode.ReplaceAllString(strings.ToUpper(name), "-"), "-")
	if len(base) < 2 {
		base = "UNIT"
	}
	trim := func(s string, n int) string {
		if len(s) > n {
			s = s[:n]
		}
		return strings.TrimRight(s, "-")
	}
	code := trim(base, maxLen)
	for n := 2; taken[code] && n < 1000; n++ {
		suffix := "-" + strconv.Itoa(n)
		code = trim(base, maxLen-len(suffix)) + suffix
	}
	taken[code] = true
	return code
}

// newItems merangkum data rujukan baru untuk respons.
func (d *refData) newItems() ImportNewItems {
	out := ImportNewItems{Branches: []string{}, Divisions: []string{}, Subdivisions: []string{}, Locations: []string{}, Shifts: []string{}}
	for _, k := range d.newBranches {
		out.Branches = append(out.Branches, d.labels[k])
	}
	parent := func(key string) string {
		i := strings.LastIndex(key, "|")
		return key[:i]
	}
	for _, k := range d.newDivisions {
		out.Divisions = append(out.Divisions, d.branchLabel(parent(k))+" / "+d.labels[k])
	}
	for _, k := range d.newSubs {
		dk := parent(k)
		out.Subdivisions = append(out.Subdivisions, d.branchLabel(parent(dk))+" / "+d.divisionLabel(dk)+" / "+d.labels[k])
	}
	for _, l := range d.newLocations {
		out.Locations = append(out.Locations, l.params.Name)
	}
	for _, sh := range d.newShifts {
		out.Shifts = append(out.Shifts, sh.params.Name)
	}
	return out
}

func (d *refData) branchLabel(key string) string   { return d.labels[key] }
func (d *refData) divisionLabel(key string) string { return d.labels[key] }

// createRefs membuat lokasi, shift, dan struktur baru di transaksi impor, lalu mencatat audit.
func (d *refData) createRefs(ctx context.Context, q *store.Queries, entry audit.Entry) error {
	for _, l := range d.newLocations {
		id, err := q.CreateOfficeLocation(ctx, l.params)
		if err != nil {
			return err
		}
		d.createdLocations[l.key] = id
		e := entry
		e.Action, e.Entity, e.EntityID = audit.SettingsChanged, "office_location", id
		e.Detail = map[string]any{"change": "created", "source": "import"}
		if err := audit.Write(ctx, q, e); err != nil {
			return err
		}
	}
	for _, sh := range d.newShifts {
		row, err := q.InsertShift(ctx, sh.params)
		if err != nil {
			return err
		}
		d.createdShifts[sh.key] = row.ID
		e := entry
		e.Action, e.Entity, e.EntityID = audit.SettingsChanged, "shift", row.ID
		e.Detail = map[string]any{"change": "created", "source": "import"}
		if err := audit.Write(ctx, q, e); err != nil {
			return err
		}
	}
	orgAudit := func(entity string, id uuid.UUID) error {
		e := entry
		e.Action, e.Entity, e.EntityID = audit.OrgUpdated, entity, id
		e.Detail = map[string]any{"change": "created", "source": "import"}
		return audit.Write(ctx, q, e)
	}
	for _, bk := range d.newBranches {
		name := d.labels[bk]
		row, err := q.CreateBranch(ctx, store.CreateBranchParams{Code: makeCode(name, 20, d.branchCodes), Name: name})
		if err != nil {
			return err
		}
		d.branches[bk] = row.ID
		if err := orgAudit("branch", row.ID); err != nil {
			return err
		}
	}
	for _, dk := range d.newDivisions {
		bk := dk[:strings.LastIndex(dk, "|")]
		if d.divisionCodes[bk] == nil {
			d.divisionCodes[bk] = map[string]bool{}
		}
		name := d.labels[dk]
		row, err := q.CreateDivision(ctx, store.CreateDivisionParams{BranchID: d.branches[bk], Code: makeCode(name, 10, d.divisionCodes[bk]), Name: name})
		if err != nil {
			return err
		}
		d.divisions[dk] = row.ID
		if err := orgAudit("division", row.ID); err != nil {
			return err
		}
	}
	for _, sk := range d.newSubs {
		dk := sk[:strings.LastIndex(sk, "|")]
		if d.subCodes[dk] == nil {
			d.subCodes[dk] = map[string]bool{}
		}
		name := d.labels[sk]
		row, err := q.CreateSubdivision(ctx, store.CreateSubdivisionParams{DivisionID: d.divisions[dk], Code: makeCode(name, 12, d.subCodes[dk]), Name: name})
		if err != nil {
			return err
		}
		d.subs[sk] = row.ID
		d.subDiv[row.ID] = d.divisions[dk]
		if err := orgAudit("subdivision", row.ID); err != nil {
			return err
		}
	}
	return nil
}

// locationID mengembalikan id lokasi lama atau lokasi yang baru dibuat di transaksi ini.
func (d *refData) locationID(key string) uuid.UUID {
	if id, ok := d.createdLocations[key]; ok {
		return id
	}
	return d.locations[key][0]
}

func (d *refData) shiftID(key string) uuid.UUID {
	if id, ok := d.createdShifts[key]; ok {
		return id
	}
	return d.shifts[key][0]
}

// headChange adalah penetapan kepala unit dari kolom Peran.
type headChange struct {
	row      int
	employee uuid.UUID
	role     string
	sub      uuid.UUID
}

// applyHeads memasang kepala subdiv atau divisi dari kolom Peran lalu menyesuaikan role semua Pejuang
// yang terdampak, termasuk kepala lama yang digantikan (D49).
func (d *refData) applyHeads(ctx context.Context, q *store.Queries, entry audit.Entry, changes []headChange, moved []uuid.UUID) error {
	touched := map[uuid.UUID]bool{}
	for _, id := range moved {
		touched[id] = true
	}
	for _, c := range changes {
		touched[c.employee] = true
		switch c.role {
		case roleMember:
			if err := q.ClearHeadOfEmployee(ctx, uuid.NullUUID{UUID: c.employee, Valid: true}); err != nil {
				return err
			}
		case roleSubdiv:
			current, err := q.GetSubdivision(ctx, c.sub)
			if err != nil {
				return err
			}
			if current.HeadEmployeeID.Valid {
				touched[current.HeadEmployeeID.UUID] = true
			}
			if err := q.SetSubdivisionHead(ctx, store.SetSubdivisionHeadParams{ID: c.sub, HeadEmployeeID: uuid.NullUUID{UUID: c.employee, Valid: true}}); err != nil {
				return err
			}
		case roleDivision:
			division := d.subDiv[c.sub]
			current, err := q.GetDivision(ctx, division)
			if err != nil {
				return err
			}
			if current.HeadEmployeeID.Valid {
				touched[current.HeadEmployeeID.UUID] = true
			}
			if err := q.SetDivisionHead(ctx, store.SetDivisionHeadParams{ID: division, HeadEmployeeID: uuid.NullUUID{UUID: c.employee, Valid: true}}); err != nil {
				return err
			}
		}
	}
	ids := make([]uuid.UUID, 0, len(touched))
	for id := range touched {
		ids = append(ids, id)
	}
	slices.SortFunc(ids, func(a, b uuid.UUID) int { return strings.Compare(a.String(), b.String()) })
	for _, id := range ids {
		if err := org.SyncHeadRole(ctx, q, entry, id); err != nil {
			return err
		}
	}
	return nil
}
