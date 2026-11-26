package payroll

import (
	"context"
	"encoding/json"
	"net/http"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"

	"github.com/rzarxx/HRIS/backend/internal/audit"
	"github.com/rzarxx/HRIS/backend/internal/platform/database"
	"github.com/rzarxx/HRIS/backend/internal/platform/httpx"
	"github.com/rzarxx/HRIS/backend/internal/platform/pgconv"
	"github.com/rzarxx/HRIS/backend/internal/store"
)

const (
	settingsKey   = "payroll"
	maxAmount     = 1_000_000_000
	maxComponents = 100
)

var codePattern = regexp.MustCompile(`^[A-Z][A-Z0-9_]{1,19}$`)

// Service mengelola komponen ubar, setelan payroll, dan nilai komponen per Pejuang.
type Service struct {
	pool *pgxpool.Pool
	q    *store.Queries
	now  func() time.Time
}

func NewService(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool, q: store.New(pool), now: time.Now}
}

// SetClock mengganti jam untuk tes.
func (s *Service) SetClock(now func() time.Time) { s.now = now }

// ComponentView adalah respons API komponen ubar.
type ComponentView struct {
	ID         string  `json:"id"`
	Code       string  `json:"code"`
	Name       string  `json:"name"`
	Kind       string  `json:"kind"`
	Method     string  `json:"method"`
	Rate       *string `json:"rate"`
	Percent    *string `json:"percent"`
	Divisor    *string `json:"divisor"`
	Active     bool    `json:"active"`
	ShowOnSlip bool    `json:"show_on_slip"`
	SortOrder  int32   `json:"sort_order"`
	Note       string  `json:"note"`
	UpdatedAt  *string `json:"updated_at"`
}

// ComponentInput dipakai POST dan PATCH; pada PATCH field yang tidak dikirim tetap nilai lama. Kode tidak
// bisa diubah setelah dibuat agar salinan di periode lama tetap merujuk komponen yang sama.
type ComponentInput struct {
	Code       string  `json:"code"`
	Name       string  `json:"name"`
	Kind       string  `json:"kind"`
	Method     string  `json:"method"`
	Rate       *string `json:"rate"`
	Percent    *string `json:"percent"`
	Divisor    *string `json:"divisor"`
	Active     *bool   `json:"active"`
	ShowOnSlip *bool   `json:"show_on_slip"`
	SortOrder  *int32  `json:"sort_order"`
	Note       string  `json:"note"`
}

func nullString(v decimal.NullDecimal, places int32) *string {
	if !v.Valid {
		return nil
	}
	s := v.Decimal.StringFixed(places)
	return &s
}

func toView(c store.PayrollComponent) ComponentView {
	return ComponentView{
		ID: c.ID.String(), Code: c.Code, Name: c.Name, Kind: string(c.Kind), Method: string(c.Method),
		Rate: nullString(c.Rate, 2), Percent: nullString(c.Percent, 3), Divisor: nullString(c.Divisor, 2),
		Active: c.Active, ShowOnSlip: c.ShowOnSlip, SortOrder: c.SortOrder, Note: c.Note, UpdatedAt: pgconv.TimeString(c.UpdatedAt),
	}
}

// ToComponent menyalin baris komponen ke bentuk yang dipakai mesin hitung.
func ToComponent(c store.PayrollComponent) Component {
	return Component{
		ID: c.ID.String(), Code: c.Code, Name: c.Name, Kind: string(c.Kind), Method: string(c.Method),
		Rate: c.Rate, Percent: c.Percent, Divisor: c.Divisor, ShowOnSlip: c.ShowOnSlip, SortOrder: c.SortOrder,
	}
}

// cleanText membuang karakter kontrol dan spasi berlebih.
func cleanText(s string) string {
	s = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, s)
	return strings.Join(strings.Fields(s), " ")
}

func parseMoney(raw *string, field, label string, fields map[string]string) decimal.NullDecimal {
	if raw == nil || strings.TrimSpace(*raw) == "" {
		return decimal.NullDecimal{}
	}
	v, err := decimal.NewFromString(strings.TrimSpace(*raw))
	if err != nil || v.IsNegative() || v.GreaterThan(decimal.NewFromInt(maxAmount)) {
		fields[field] = label + " diisi angka 0 sampai 1.000.000.000."
		return decimal.NullDecimal{}
	}
	return decimal.NewNullDecimal(v.Round(2))
}

type validComponent struct {
	Code, Name, Kind, Method, Note string
	Rate, Percent, Divisor         decimal.NullDecimal
	Active, ShowOnSlip             bool
	SortOrder                      int32
}

func (in ComponentInput) validate() (validComponent, error) {
	fields := map[string]string{}
	v := validComponent{Code: strings.ToUpper(strings.TrimSpace(in.Code)), Name: cleanText(in.Name), Kind: in.Kind, Method: in.Method, Note: cleanText(in.Note)}
	if !codePattern.MatchString(v.Code) {
		fields["code"] = "Kode diisi 2-20 huruf kapital, angka, atau garis bawah, diawali huruf. Contoh: TUNJ_MAKAN."
	} else if v.Code == BaseCode {
		fields["code"] = "Kode UBAR_POKOK dipakai sistem untuk ubar pokok."
	}
	if n := utf8.RuneCountInString(v.Name); n < 2 || n > 80 {
		fields["name"] = "Nama diisi 2 sampai 80 karakter."
	}
	if v.Kind != KindEarning && v.Kind != KindDeduction {
		fields["kind"] = "Pilih Pendapatan atau Potongan."
	}
	if !slices.Contains(Methods, v.Method) {
		fields["method"] = "Pilih cara hitung dari daftar."
	}
	if utf8.RuneCountInString(v.Note) > 300 {
		fields["note"] = "Catatan maksimal 300 karakter."
	}
	v.Rate = parseMoney(in.Rate, "rate", "Nominal", fields)
	if in.Percent != nil && strings.TrimSpace(*in.Percent) != "" {
		p, err := decimal.NewFromString(strings.TrimSpace(*in.Percent))
		if err != nil || p.IsNegative() || p.GreaterThan(decimal.NewFromInt(100)) {
			fields["percent"] = "Persen diisi 0 sampai 100."
		} else {
			v.Percent = decimal.NewNullDecimal(p.Round(3))
		}
	}
	if in.Divisor != nil && strings.TrimSpace(*in.Divisor) != "" {
		dv, err := decimal.NewFromString(strings.TrimSpace(*in.Divisor))
		if err != nil || !dv.IsPositive() || dv.GreaterThan(decimal.NewFromInt(31)) {
			fields["divisor"] = "Pembagi diisi 1 sampai 31 hari."
		} else {
			v.Divisor = decimal.NewNullDecimal(dv.Round(2))
		}
	}
	// Isian yang wajib dan yang tidak dipakai per cara hitung; yang tidak dipakai dikosongkan agar tidak menyesatkan.
	switch v.Method {
	case MethodFixed, MethodPerPresentDay, MethodPerLate, MethodPerLateMinute, MethodPerUnit:
		if !v.Rate.Valid && fields["rate"] == "" {
			fields["rate"] = "Nominal wajib diisi untuk cara hitung ini."
		}
		v.Percent, v.Divisor = decimal.NullDecimal{}, decimal.NullDecimal{}
	case MethodPerAbsentDay, MethodPerUnpaidLeaveDay:
		v.Percent = decimal.NullDecimal{}
		if v.Rate.Valid {
			v.Divisor = decimal.NullDecimal{}
		}
	case MethodPercentOfBase, MethodPercentOfGross:
		if !v.Percent.Valid && fields["percent"] == "" {
			fields["percent"] = "Persen wajib diisi untuk cara hitung ini."
		}
		v.Rate, v.Divisor = decimal.NullDecimal{}, decimal.NullDecimal{}
	case MethodPerEmployee, MethodManual:
		v.Rate, v.Percent, v.Divisor = decimal.NullDecimal{}, decimal.NullDecimal{}, decimal.NullDecimal{}
	}
	v.Active = in.Active == nil || *in.Active
	v.ShowOnSlip = in.ShowOnSlip == nil || *in.ShowOnSlip
	if in.SortOrder != nil {
		if *in.SortOrder < 0 || *in.SortOrder > 999 {
			fields["sort_order"] = "Urutan diisi 0 sampai 999."
		}
		v.SortOrder = *in.SortOrder
	}
	if len(fields) > 0 {
		return v, httpx.Validation(fields)
	}
	return v, nil
}

func writeRulesAudit(ctx context.Context, q *store.Queries, entry audit.Entry, entity string, id uuid.UUID, detail map[string]any) error {
	entry.Action, entry.Entity, entry.EntityID, entry.Detail = audit.PayrollRulesChanged, entity, id, detail
	return audit.Write(ctx, q, entry)
}

func (s *Service) ListComponents(ctx context.Context) ([]ComponentView, error) {
	rows, err := s.q.ListPayrollComponents(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]ComponentView, 0, len(rows))
	for _, r := range rows {
		out = append(out, toView(r))
	}
	return out, nil
}

func (s *Service) CreateComponent(ctx context.Context, entry audit.Entry, in ComponentInput) (ComponentView, error) {
	v, err := in.validate()
	if err != nil {
		return ComponentView{}, err
	}
	var row store.PayrollComponent
	err = database.InTx(ctx, s.pool, func(q *store.Queries) error {
		existing, err := q.ListPayrollComponents(ctx)
		if err != nil {
			return err
		}
		if len(existing) >= maxComponents {
			return httpx.Validation(map[string]string{"code": "Komponen ubar maksimal 100."})
		}
		row, err = q.InsertPayrollComponent(ctx, store.InsertPayrollComponentParams{
			Code: v.Code, Name: v.Name, Kind: store.PayrollComponentKind(v.Kind), Method: store.PayrollCalcMethod(v.Method),
			Rate: v.Rate, Percent: v.Percent, Divisor: v.Divisor, Active: v.Active, ShowOnSlip: v.ShowOnSlip,
			SortOrder: v.SortOrder, Note: v.Note, UpdatedBy: entry.UserID,
		})
		if _, dup := pgconv.UniqueConstraint(err); dup {
			return httpx.Validation(map[string]string{"code": "Kode komponen sudah dipakai."})
		}
		if err != nil {
			return err
		}
		return writeRulesAudit(ctx, q, entry, "payroll_component", row.ID, map[string]any{"change": "created", "code": v.Code, "method": v.Method})
	})
	if err != nil {
		return ComponentView{}, err
	}
	return toView(row), nil
}

// UpdateComponent menimpa field yang dikirim. Kode tetap; komponen dinonaktifkan lewat active=false.
func (s *Service) UpdateComponent(ctx context.Context, entry audit.Entry, id uuid.UUID, decode func(*ComponentInput) error) (ComponentView, error) {
	cur, err := s.q.GetPayrollComponent(ctx, id)
	if pgconv.IsNoRows(err) {
		return ComponentView{}, httpx.ErrNotFound()
	}
	if err != nil {
		return ComponentView{}, err
	}
	view := toView(cur)
	in := ComponentInput{
		Name: cur.Name, Kind: view.Kind, Method: view.Method, Rate: view.Rate, Percent: view.Percent, Divisor: view.Divisor,
		Active: &cur.Active, ShowOnSlip: &cur.ShowOnSlip, SortOrder: &cur.SortOrder, Note: cur.Note,
	}
	if err := decode(&in); err != nil {
		return ComponentView{}, err
	}
	if in.Code != "" && strings.ToUpper(strings.TrimSpace(in.Code)) != cur.Code {
		return ComponentView{}, httpx.Validation(map[string]string{"code": "Kode komponen tidak bisa diubah. Buat komponen baru bila perlu."})
	}
	in.Code = cur.Code
	v, err := in.validate()
	if err != nil {
		return ComponentView{}, err
	}
	var row store.PayrollComponent
	err = database.InTx(ctx, s.pool, func(q *store.Queries) error {
		var err error
		row, err = q.UpdatePayrollComponent(ctx, store.UpdatePayrollComponentParams{
			ID: id, Name: v.Name, Kind: store.PayrollComponentKind(v.Kind), Method: store.PayrollCalcMethod(v.Method),
			Rate: v.Rate, Percent: v.Percent, Divisor: v.Divisor, Active: v.Active, ShowOnSlip: v.ShowOnSlip,
			SortOrder: v.SortOrder, Note: v.Note, UpdatedBy: entry.UserID,
		})
		if pgconv.IsNoRows(err) {
			return httpx.ErrNotFound()
		}
		if err != nil {
			return err
		}
		return writeRulesAudit(ctx, q, entry, "payroll_component", id, map[string]any{"change": "updated", "code": cur.Code, "active": v.Active})
	})
	if err != nil {
		return ComponentView{}, err
	}
	return toView(row), nil
}

// Settings adalah setelan payroll di app_settings kunci "payroll".
type Settings struct {
	// PeriodCutoffDay: periode berakhir di tanggal ini (1-28), 0 berarti akhir bulan (D31, bawaan 25).
	PeriodCutoffDay int `json:"period_cutoff_day"`
	// MinNetZero: ubar bersih tidak pernah negatif; kelebihan potongan dicatat sebagai peringatan.
	MinNetZero bool    `json:"min_net_zero"`
	UpdatedAt  *string `json:"updated_at"`
}

// DefaultSettings dipakai sampai SDM menyimpan setelan.
var DefaultSettings = Settings{PeriodCutoffDay: 25, MinNetZero: true}

func (s *Service) Settings(ctx context.Context) (Settings, error) {
	row, err := s.q.GetAppSetting(ctx, settingsKey)
	if pgconv.IsNoRows(err) {
		return DefaultSettings, nil
	}
	if err != nil {
		return Settings{}, err
	}
	out := DefaultSettings
	if err := json.Unmarshal([]byte(row.Value), &out); err != nil {
		return Settings{}, err
	}
	out.UpdatedAt = pgconv.TimeString(row.UpdatedAt)
	return out, nil
}

// SettingsInput mengganti seluruh setelan payroll.
type SettingsInput struct {
	PeriodCutoffDay *int  `json:"period_cutoff_day"`
	MinNetZero      *bool `json:"min_net_zero"`
}

func (s *Service) SetSettings(ctx context.Context, entry audit.Entry, in SettingsInput) (Settings, error) {
	if in.PeriodCutoffDay == nil || *in.PeriodCutoffDay < 0 || *in.PeriodCutoffDay > 28 {
		return Settings{}, httpx.Validation(map[string]string{"period_cutoff_day": "Tanggal cutoff diisi 1 sampai 28, atau 0 untuk akhir bulan."})
	}
	v := Settings{PeriodCutoffDay: *in.PeriodCutoffDay, MinNetZero: in.MinNetZero == nil || *in.MinNetZero}
	raw, err := json.Marshal(struct {
		PeriodCutoffDay int  `json:"period_cutoff_day"`
		MinNetZero      bool `json:"min_net_zero"`
	}{v.PeriodCutoffDay, v.MinNetZero})
	if err != nil {
		return Settings{}, err
	}
	err = database.InTx(ctx, s.pool, func(q *store.Queries) error {
		if err := q.UpsertAppSetting(ctx, store.UpsertAppSettingParams{Key: settingsKey, Value: string(raw), UpdatedBy: entry.UserID}); err != nil {
			return err
		}
		return writeRulesAudit(ctx, q, entry, "settings", uuid.Nil, map[string]any{"key": settingsKey, "period_cutoff_day": v.PeriodCutoffDay, "min_net_zero": v.MinNetZero})
	})
	if err != nil {
		return Settings{}, err
	}
	return s.Settings(ctx)
}

// EmployeeComponent adalah nilai satu komponen per_employee untuk satu Pejuang.
type EmployeeComponent struct {
	ComponentID string  `json:"component_id"`
	Code        string  `json:"code"`
	Name        string  `json:"name"`
	Kind        string  `json:"kind"`
	Active      bool    `json:"active"`
	Amount      *string `json:"amount"`
}

// EmployeeComponents mengembalikan semua komponen per_employee beserta nilainya untuk Pejuang ini (null bila
// belum diisi, artinya komponen tidak berlaku untuknya).
func (s *Service) EmployeeComponents(ctx context.Context, employeeID uuid.UUID) ([]EmployeeComponent, error) {
	exists, err := s.q.EmployeeExists(ctx, employeeID)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, httpx.ErrNotFound()
	}
	comps, err := s.q.ListPayrollComponents(ctx)
	if err != nil {
		return nil, err
	}
	values, err := s.q.ListEmployeePayrollComponents(ctx, employeeID)
	if err != nil {
		return nil, err
	}
	byID := map[uuid.UUID]decimal.Decimal{}
	for _, v := range values {
		byID[v.ComponentID] = v.Amount
	}
	out := []EmployeeComponent{}
	for _, c := range comps {
		if c.Method != store.PayrollCalcMethodPerEmployee {
			continue
		}
		ec := EmployeeComponent{ComponentID: c.ID.String(), Code: c.Code, Name: c.Name, Kind: string(c.Kind), Active: c.Active}
		if v, ok := byID[c.ID]; ok {
			s := v.StringFixed(2)
			ec.Amount = &s
		}
		out = append(out, ec)
	}
	return out, nil
}

// EmployeeComponentInput adalah satu nilai yang disimpan; amount null menghapus komponen dari Pejuang.
type EmployeeComponentInput struct {
	ComponentID string  `json:"component_id"`
	Amount      *string `json:"amount"`
}

// EmployeeComponentsInput mengganti seluruh nilai komponen per_employee Pejuang.
type EmployeeComponentsInput struct {
	Components []EmployeeComponentInput `json:"components"`
}

func (s *Service) SetEmployeeComponents(ctx context.Context, entry audit.Entry, employeeID uuid.UUID, in EmployeeComponentsInput) ([]EmployeeComponent, error) {
	exists, err := s.q.EmployeeExists(ctx, employeeID)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, httpx.ErrNotFound()
	}
	comps, err := s.q.ListPayrollComponents(ctx)
	if err != nil {
		return nil, err
	}
	perEmployee := map[string]store.PayrollComponent{}
	for _, c := range comps {
		if c.Method == store.PayrollCalcMethodPerEmployee {
			perEmployee[c.ID.String()] = c
		}
	}
	fields := map[string]string{}
	type value struct {
		id     uuid.UUID
		amount decimal.Decimal
	}
	var values []value
	seen := map[string]bool{}
	for i, item := range in.Components {
		c, ok := perEmployee[item.ComponentID]
		if !ok || seen[item.ComponentID] {
			fields["components"] = "Komponen tidak dikenal, bukan cara hitung per Pejuang, atau tercantum dua kali."
			break
		}
		seen[item.ComponentID] = true
		amount := parseMoney(item.Amount, "components", "Nominal baris "+itoa(i+1), fields)
		if amount.Valid {
			values = append(values, value{c.ID, amount.Decimal})
		}
	}
	if len(fields) > 0 {
		return nil, httpx.Validation(fields)
	}
	err = database.InTx(ctx, s.pool, func(q *store.Queries) error {
		if err := q.DeleteEmployeePayrollComponents(ctx, employeeID); err != nil {
			return err
		}
		for _, v := range values {
			if err := q.InsertEmployeePayrollComponent(ctx, store.InsertEmployeePayrollComponentParams{
				EmployeeID: employeeID, ComponentID: v.id, Amount: v.amount, UpdatedBy: entry.UserID,
			}); err != nil {
				return err
			}
		}
		return writeRulesAudit(ctx, q, entry, "employee", employeeID, map[string]any{"change": "employee_components", "count": len(values)})
	})
	if err != nil {
		return nil, err
	}
	return s.EmployeeComponents(ctx, employeeID)
}

func itoa(n int) string {
	if n < 10 {
		return string(rune('0' + n))
	}
	return itoa(n/10) + string(rune('0'+n%10))
}

// Handler melayani /v1/admin/payroll/* dan /v1/admin/employees/{id}/payroll-components.
type Handler struct{ svc *Service }

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) ListComponents(w http.ResponseWriter, r *http.Request) error {
	rows, err := h.svc.ListComponents(r.Context())
	if err != nil {
		return err
	}
	return httpx.Data(w, http.StatusOK, rows)
}

func (h *Handler) CreateComponent(w http.ResponseWriter, r *http.Request) error {
	var in ComponentInput
	if err := httpx.Decode(r, &in); err != nil {
		return err
	}
	out, err := h.svc.CreateComponent(r.Context(), actor(r), in)
	if err != nil {
		return err
	}
	return httpx.Data(w, http.StatusCreated, out)
}

func (h *Handler) UpdateComponent(w http.ResponseWriter, r *http.Request) error {
	id, err := httpx.PathID(r)
	if err != nil {
		return err
	}
	out, err := h.svc.UpdateComponent(r.Context(), actor(r), id, func(in *ComponentInput) error { return httpx.Decode(r, in) })
	if err != nil {
		return err
	}
	return httpx.Data(w, http.StatusOK, out)
}

func (h *Handler) Settings(w http.ResponseWriter, r *http.Request) error {
	out, err := h.svc.Settings(r.Context())
	if err != nil {
		return err
	}
	return httpx.Data(w, http.StatusOK, out)
}

func (h *Handler) SetSettings(w http.ResponseWriter, r *http.Request) error {
	var in SettingsInput
	if err := httpx.Decode(r, &in); err != nil {
		return err
	}
	out, err := h.svc.SetSettings(r.Context(), actor(r), in)
	if err != nil {
		return err
	}
	return httpx.Data(w, http.StatusOK, out)
}

func (h *Handler) EmployeeComponents(w http.ResponseWriter, r *http.Request) error {
	id, err := httpx.PathID(r)
	if err != nil {
		return err
	}
	out, err := h.svc.EmployeeComponents(r.Context(), id)
	if err != nil {
		return err
	}
	return httpx.Data(w, http.StatusOK, out)
}

func (h *Handler) SetEmployeeComponents(w http.ResponseWriter, r *http.Request) error {
	id, err := httpx.PathID(r)
	if err != nil {
		return err
	}
	var in EmployeeComponentsInput
	if err := httpx.Decode(r, &in); err != nil {
		return err
	}
	out, err := h.svc.SetEmployeeComponents(r.Context(), actor(r), id, in)
	if err != nil {
		return err
	}
	return httpx.Data(w, http.StatusOK, out)
}
