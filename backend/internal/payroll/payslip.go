package payroll

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"mime"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/shopspring/decimal"

	"github.com/rzarxx/HRIS/backend/internal/audit"
	"github.com/rzarxx/HRIS/backend/internal/auth"
	"github.com/rzarxx/HRIS/backend/internal/platform/httpx"
	"github.com/rzarxx/HRIS/backend/internal/platform/pgconv"
	"github.com/rzarxx/HRIS/backend/internal/store"
)

// Mailer mengirim email pemberitahuan; mail.Service memenuhinya.
type Mailer interface {
	Enabled(ctx context.Context) bool
	Send(ctx context.Context, to, subject, body string) error
}

const mailTimeout = 10 * time.Minute

// SetMailer memasang pengirim email pemberitahuan slip (D68). Tanpa ini hanya notifikasi di aplikasi.
func (s *Service) SetMailer(m Mailer) { s.mailer = m }

func periodTitle(year, month int) string { return fmt.Sprintf("%s %d", MonthName(month), year) }

// publish membuat notifikasi payslip_ready untuk semua slip periode, dalam transaksi finalisasi.
func publish(ctx context.Context, q *store.Queries, p store.PayrollPeriod) error {
	_, err := q.InsertPayslipNotifications(ctx, store.InsertPayslipNotificationsParams{
		PeriodID: p.ID,
		Title:    "Slip ubar " + periodTitle(int(p.Year), int(p.Month)) + " sudah terbit",
		Body:     "Buka tab Slip untuk melihat dan mengunduh slip ubar Anda.",
	})
	return err
}

// emailPayslips mengirim pemberitahuan tanpa angka ubar ke email akun Pejuang, di latar setelah transaksi.
// Kegagalan per alamat dicatat sebagai email.delivery_failed tanpa membatalkan finalisasi.
func (s *Service) emailPayslips(periodID uuid.UUID, year, month int) {
	if s.mailer == nil {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), mailTimeout)
		defer cancel()
		if !s.mailer.Enabled(ctx) {
			return
		}
		rows, err := s.q.PayslipRecipients(ctx, periodID)
		if err != nil {
			slog.Warn("daftar penerima email slip gagal dibaca", "error", err.Error())
			return
		}
		label := periodTitle(year, month)
		subject := "Slip ubar " + label + " sudah terbit"
		body := "Assalamu'alaikum,\n\nSlip ubar periode " + label + " Anda sudah terbit.\n" +
			"Buka aplikasi HRIS Pejuang, tab Slip, untuk melihat rincian dan mengunduh PDF-nya.\n\n" +
			"Email ini hanya pemberitahuan dan tidak memuat rincian ubar. Bila ada yang tidak sesuai, hubungi SDM.\n\n" +
			"Wassalamu'alaikum,\nSDM " + OrgName
		for _, r := range rows {
			if !r.Email.Valid || r.Email.String == "" {
				continue
			}
			if err := s.mailer.Send(ctx, r.Email.String, subject, body); err != nil {
				slog.Warn("email slip belum terkirim", "error", err.Error())
				_ = audit.Write(ctx, s.q, audit.Entry{
					UserID: audit.Actor(r.UserID), Action: audit.EmailDeliveryFailed, Entity: "payroll_period", EntityID: periodID,
					Detail: map[string]any{"purpose": "payslip_ready"},
				})
			}
		}
	}()
}

// slipSource adalah kolom slip yang sama di GetMyPayslipRow dan GetPayrollForPDFRow.
type slipSource struct {
	ID                            uuid.UUID
	Year, Month                   int16
	StartDate, EndDate            pgtype.Date
	Status                        store.PayrollStatus
	FinalizedAt, PublishedAt      pgtype.Timestamptz
	Nip, FullName, Position       string
	Division, Subdivision         string
	Attendance                    []byte
	BaseSalary, Gross, Deductions decimal.Decimal
	Net                           decimal.Decimal
	NetClamped                    bool
}

// PayslipView adalah slip ubar untuk Pejuang (D68). Item yang tidak tampil di slip tidak dikirim.
type PayslipView struct {
	ID             string            `json:"id"`
	DocumentNumber string            `json:"document_number"`
	Year           int               `json:"year"`
	Month          int               `json:"month"`
	StartDate      string            `json:"start_date"`
	EndDate        string            `json:"end_date"`
	Status         string            `json:"status"`
	PublishedAt    *string           `json:"published_at"`
	Nip            string            `json:"nip"`
	FullName       string            `json:"full_name"`
	Position       string            `json:"position"`
	Division       string            `json:"division"`
	Subdivision    string            `json:"subdivision"`
	Attendance     AttendanceSummary `json:"attendance"`
	Items          []ItemView        `json:"items"`
	Gross          string            `json:"gross"`
	Deductions     string            `json:"deductions"`
	Net            string            `json:"net"`
	NetClamped     bool              `json:"net_clamped"`
}

// PayslipSummary adalah satu baris daftar slip.
type PayslipSummary struct {
	ID          string  `json:"id"`
	Year        int     `json:"year"`
	Month       int     `json:"month"`
	StartDate   string  `json:"start_date"`
	EndDate     string  `json:"end_date"`
	Status      string  `json:"status"`
	Net         string  `json:"net"`
	PublishedAt *string `json:"published_at"`
	Unread      bool    `json:"unread"`
}

func (s *Service) slipDoc(ctx context.Context, src slipSource) (SlipDoc, []ItemView, error) {
	items, err := s.q.ListPayrollItems(ctx, src.ID)
	if err != nil {
		return SlipDoc{}, nil, err
	}
	var att AttendanceSummary
	if err := json.Unmarshal(src.Attendance, &att); err != nil {
		return SlipDoc{}, nil, err
	}
	all := itemViews(items)
	issued := src.FinalizedAt.Time
	if !src.FinalizedAt.Valid {
		issued = s.now()
	}
	return SlipDoc{
		ID: src.ID.String(), Year: int(src.Year), Month: int(src.Month), Start: src.StartDate.Time, End: src.EndDate.Time,
		Status: string(src.Status), IssuedAt: issued, Nip: src.Nip, FullName: src.FullName, Position: src.Position,
		Division: src.Division, Subdivision: src.Subdivision, Attendance: att, Items: all,
		Gross: src.Gross, Deductions: src.Deductions, Net: src.Net,
	}, all, nil
}

func fromMine(r store.GetMyPayslipRow) slipSource {
	return slipSource{
		ID: r.ID, Year: r.Year, Month: r.Month, StartDate: r.StartDate, EndDate: r.EndDate, Status: r.PeriodStatus,
		FinalizedAt: r.FinalizedAt, PublishedAt: r.PublishedAt, Nip: r.Nip, FullName: r.FullName, Position: r.Position,
		Division: r.Division, Subdivision: r.Subdivision, Attendance: r.Attendance, BaseSalary: r.BaseSalary,
		Gross: r.Gross, Deductions: r.Deductions, Net: r.Net, NetClamped: r.NetClamped,
	}
}

func fromAdmin(r store.GetPayrollForPDFRow) slipSource {
	return slipSource{
		ID: r.ID, Year: r.Year, Month: r.Month, StartDate: r.StartDate, EndDate: r.EndDate, Status: r.PeriodStatus,
		FinalizedAt: r.FinalizedAt, PublishedAt: r.PublishedAt, Nip: r.Nip, FullName: r.FullName, Position: r.Position,
		Division: r.Division, Subdivision: r.Subdivision, Attendance: r.Attendance, BaseSalary: r.BaseSalary,
		Gross: r.Gross, Deductions: r.Deductions, Net: r.Net, NetClamped: r.NetClamped,
	}
}

// MyPayslips mengembalikan slip Pejuang dari periode final atau dibayar.
func (s *Service) MyPayslips(ctx context.Context, employeeID uuid.UUID) ([]PayslipSummary, error) {
	rows, err := s.q.ListMyPayslips(ctx, employeeID)
	if err != nil {
		return nil, err
	}
	out := make([]PayslipSummary, 0, len(rows))
	for _, r := range rows {
		out = append(out, PayslipSummary{
			ID: r.ID.String(), Year: int(r.Year), Month: int(r.Month),
			StartDate: r.StartDate.Time.Format(time.DateOnly), EndDate: r.EndDate.Time.Format(time.DateOnly),
			Status: string(r.Status), Net: r.Net.StringFixed(2), PublishedAt: pgconv.TimeString(r.PublishedAt), Unread: r.Unread,
		})
	}
	return out, nil
}

func (s *Service) myPayslip(ctx context.Context, employeeID, id uuid.UUID) (slipSource, error) {
	r, err := s.q.GetMyPayslip(ctx, store.GetMyPayslipParams{ID: id, EmployeeID: employeeID})
	if pgconv.IsNoRows(err) {
		// Slip orang lain dan slip yang belum terbit sama-sama 404, tanpa membedakan.
		return slipSource{}, httpx.ErrNotFound()
	}
	return fromMine(r), err
}

// MyPayslip mengembalikan rincian slip dan menandai notifikasinya sudah dibaca.
func (s *Service) MyPayslip(ctx context.Context, employeeID, id uuid.UUID) (PayslipView, error) {
	src, err := s.myPayslip(ctx, employeeID, id)
	if err != nil {
		return PayslipView{}, err
	}
	doc, items, err := s.slipDoc(ctx, src)
	if err != nil {
		return PayslipView{}, err
	}
	if err := s.q.MarkPayslipNotificationRead(ctx, store.MarkPayslipNotificationReadParams{
		EmployeeID: employeeID, RefID: uuid.NullUUID{UUID: id, Valid: true},
	}); err != nil {
		return PayslipView{}, err
	}
	shown := make([]ItemView, 0, len(items))
	for _, it := range items {
		if it.ShowOnSlip {
			shown = append(shown, it)
		}
	}
	return PayslipView{
		ID: doc.ID, DocumentNumber: DocumentNumber(doc), Year: doc.Year, Month: doc.Month,
		StartDate: doc.Start.Format(time.DateOnly), EndDate: doc.End.Format(time.DateOnly), Status: doc.Status,
		PublishedAt: pgconv.TimeString(src.PublishedAt), Nip: doc.Nip, FullName: doc.FullName, Position: doc.Position,
		Division: doc.Division, Subdivision: doc.Subdivision, Attendance: doc.Attendance, Items: shown,
		Gross: src.Gross.StringFixed(2), Deductions: src.Deductions.StringFixed(2), Net: src.Net.StringFixed(2), NetClamped: src.NetClamped,
	}, nil
}

// MyPayslipPDF menyusun PDF slip Pejuang sendiri.
func (s *Service) MyPayslipPDF(ctx context.Context, employeeID, id uuid.UUID) (SlipDoc, []byte, error) {
	src, err := s.myPayslip(ctx, employeeID, id)
	if err != nil {
		return SlipDoc{}, nil, err
	}
	return s.renderSource(ctx, src)
}

// AdminPayslipPDF menyusun PDF slip satu Pejuang untuk SDM; periode yang belum final diberi tanda DRAF.
func (s *Service) AdminPayslipPDF(ctx context.Context, periodID, employeeID uuid.UUID) (SlipDoc, []byte, error) {
	r, err := s.q.GetPayrollForPDF(ctx, store.GetPayrollForPDFParams{PeriodID: periodID, EmployeeID: employeeID})
	if pgconv.IsNoRows(err) {
		return SlipDoc{}, nil, httpx.ErrNotFound()
	}
	if err != nil {
		return SlipDoc{}, nil, err
	}
	return s.renderSource(ctx, fromAdmin(r))
}

func (s *Service) renderSource(ctx context.Context, src slipSource) (SlipDoc, []byte, error) {
	doc, _, err := s.slipDoc(ctx, src)
	if err != nil {
		return SlipDoc{}, nil, err
	}
	data, err := RenderPDF(doc)
	return doc, data, err
}

// NotificationView adalah notifikasi di aplikasi.
type NotificationView struct {
	ID        string  `json:"id"`
	Type      string  `json:"type"`
	RefID     *string `json:"ref_id"`
	Title     string  `json:"title"`
	Body      string  `json:"body"`
	ReadAt    *string `json:"read_at"`
	CreatedAt string  `json:"created_at"`
}

// Notifications mengembalikan 50 notifikasi terbaru dan jumlah yang belum dibaca.
func (s *Service) Notifications(ctx context.Context, employeeID uuid.UUID) ([]NotificationView, int64, error) {
	rows, err := s.q.ListNotifications(ctx, employeeID)
	if err != nil {
		return nil, 0, err
	}
	unread, err := s.q.CountUnreadNotifications(ctx, employeeID)
	if err != nil {
		return nil, 0, err
	}
	out := make([]NotificationView, 0, len(rows))
	for _, n := range rows {
		v := NotificationView{
			ID: n.ID.String(), Type: string(n.Type), Title: n.Title, Body: n.Body,
			ReadAt: pgconv.TimeString(n.ReadAt), CreatedAt: n.CreatedAt.Time.UTC().Format(time.RFC3339),
		}
		if n.RefID.Valid {
			ref := n.RefID.UUID.String()
			v.RefID = &ref
		}
		out = append(out, v)
	}
	return out, unread, nil
}

func (s *Service) ReadNotification(ctx context.Context, employeeID, id uuid.UUID) error {
	n, err := s.q.MarkNotificationRead(ctx, store.MarkNotificationReadParams{ID: id, EmployeeID: employeeID})
	if err != nil {
		return err
	}
	if n == 0 {
		return httpx.ErrNotFound()
	}
	return nil
}

func principal(r *http.Request) auth.Principal {
	p, _ := auth.PrincipalFrom(r.Context())
	return p
}

func writePDF(w http.ResponseWriter, doc SlipDoc, data []byte) error {
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": PDFFileName(doc)}))
	w.WriteHeader(http.StatusOK)
	_, err := w.Write(data)
	return err
}

// MyPayslips menangani GET /v1/payslips.
func (h *Handler) MyPayslips(w http.ResponseWriter, r *http.Request) error {
	out, err := h.svc.MyPayslips(r.Context(), principal(r).EmployeeID)
	if err != nil {
		return err
	}
	return httpx.Data(w, http.StatusOK, out)
}

// MyPayslip menangani GET /v1/payslips/{id}.
func (h *Handler) MyPayslip(w http.ResponseWriter, r *http.Request) error {
	id, err := httpx.PathID(r)
	if err != nil {
		return err
	}
	out, err := h.svc.MyPayslip(r.Context(), principal(r).EmployeeID, id)
	if err != nil {
		return err
	}
	return httpx.Data(w, http.StatusOK, out)
}

// MyPayslipPDF menangani GET /v1/payslips/{id}/pdf.
func (h *Handler) MyPayslipPDF(w http.ResponseWriter, r *http.Request) error {
	id, err := httpx.PathID(r)
	if err != nil {
		return err
	}
	doc, data, err := h.svc.MyPayslipPDF(r.Context(), principal(r).EmployeeID, id)
	if err != nil {
		return err
	}
	return writePDF(w, doc, data)
}

// AdminPayslipPDF menangani GET /v1/admin/payroll/periods/{id}/employees/{employee_id}/pdf.
func (h *Handler) AdminPayslipPDF(w http.ResponseWriter, r *http.Request) error {
	id, employeeID, err := employeePathIDs(r)
	if err != nil {
		return err
	}
	doc, data, err := h.svc.AdminPayslipPDF(r.Context(), id, employeeID)
	if err != nil {
		return err
	}
	return writePDF(w, doc, data)
}

// Notifications menangani GET /v1/notifications.
func (h *Handler) Notifications(w http.ResponseWriter, r *http.Request) error {
	out, unread, err := h.svc.Notifications(r.Context(), principal(r).EmployeeID)
	if err != nil {
		return err
	}
	return httpx.DataMeta(w, out, map[string]any{"unread": unread})
}

// ReadNotification menangani POST /v1/notifications/{id}/read.
func (h *Handler) ReadNotification(w http.ResponseWriter, r *http.Request) error {
	id, err := httpx.PathID(r)
	if err != nil {
		return err
	}
	if err := h.svc.ReadNotification(r.Context(), principal(r).EmployeeID, id); err != nil {
		return err
	}
	return httpx.NoContent(w)
}
