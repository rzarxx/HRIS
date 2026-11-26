package integration

import (
	"bytes"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/rzarxx/HRIS/backend/internal/mail"
)

type payslipSummaryRow struct {
	ID     string `json:"id"`
	Year   int    `json:"year"`
	Month  int    `json:"month"`
	Net    string `json:"net"`
	Unread bool   `json:"unread"`
}

type payslipResp struct {
	ID             string `json:"id"`
	DocumentNumber string `json:"document_number"`
	FullName       string `json:"full_name"`
	Net            string `json:"net"`
	Items          []struct {
		Code string `json:"code"`
	} `json:"items"`
}

type notificationRow struct {
	ID     string  `json:"id"`
	Type   string  `json:"type"`
	RefID  *string `json:"ref_id"`
	Title  string  `json:"title"`
	ReadAt *string `json:"read_at"`
}

// TestPayslipPublished menguji terbitnya slip (D68): slip baru terlihat setelah final, hanya milik sendiri,
// komponen tersembunyi tidak dikirim, PDF dapat diunduh, notifikasi dibuat, dan email tanpa angka ubar.
func TestPayslipPublished(t *testing.T) {
	box := &mailbox{}
	srv := fresh(t, options{mailbox: box})
	exec(t, `INSERT INTO app_settings (key, value) VALUES ('smtp', '{"host":"smtp.example.test","port":587,"from_email":"hris@albahjah.or.id"}')`)
	sdm := loginSDM(t, srv)
	checker := login(t, srv, secondSDM(t), seedPassword)
	acc := activeAccount(t, "pejuang", "OPS-GDG")
	other := activeAccount(t, "pejuang", "OPS-LAP")
	pejuang := login(t, srv, acc.email, seedPassword)
	otherClient := login(t, srv, other.email, seedPassword)
	exec(t, `DELETE FROM payroll_components`)
	exec(t, `UPDATE employees SET base_salary = 3000000 WHERE id = $1`, acc.employeeID)
	sdm.do(http.MethodPost, "/v1/admin/payroll/components", map[string]any{"code": "UANG_MAKAN", "name": "Uang makan", "kind": "earning", "method": "per_present_day", "rate": "10000"}).
		want(t, http.StatusCreated)
	sdm.do(http.MethodPost, "/v1/admin/payroll/components", map[string]any{"code": "INTERNAL", "name": "Catatan internal", "kind": "earning", "method": "fixed", "rate": "1", "show_on_slip": false}).
		want(t, http.StatusCreated)
	exec(t, `INSERT INTO attendances (employee_id, work_date, status, late_minutes) VALUES ($1, '2020-01-06', 'present', 0), ($1, '2020-01-07', 'present', 0)`, acc.employeeID)

	period := data[periodRow](t, sdm.do(http.MethodPost, "/v1/admin/payroll/periods", map[string]any{"year": 2020, "month": 1}).want(t, http.StatusCreated))
	base := "/v1/admin/payroll/periods/" + period.ID
	sdm.do(http.MethodPost, base+"/generate", nil).want(t, http.StatusOK)

	// SDM bisa mengunduh PDF draf; Pejuang belum melihat apa pun.
	draft := sdm.do(http.MethodGet, base+"/employees/"+acc.employeeID+"/pdf", nil).want(t, http.StatusOK)
	if !bytes.HasPrefix(draft.body, []byte("%PDF-")) {
		t.Fatalf("PDF draf tidak valid")
	}
	if rows := data[[]payslipSummaryRow](t, pejuang.do(http.MethodGet, "/v1/payslips", nil).want(t, http.StatusOK)); len(rows) != 0 {
		t.Fatalf("slip terlihat sebelum final: %+v", rows)
	}
	sdm.do(http.MethodPost, base+"/submit", nil).want(t, http.StatusOK)
	if rows := data[[]payslipSummaryRow](t, pejuang.do(http.MethodGet, "/v1/payslips", nil).want(t, http.StatusOK)); len(rows) != 0 {
		t.Fatalf("slip terlihat sebelum final: %+v", rows)
	}
	checker.do(http.MethodPost, base+"/finalize", nil).want(t, http.StatusOK)

	// Notifikasi dan daftar slip.
	notifs := pejuang.do(http.MethodGet, "/v1/notifications", nil).want(t, http.StatusOK)
	list := data[[]notificationRow](t, notifs)
	if len(list) != 1 || list[0].Type != "payslip_ready" || list[0].RefID == nil || !strings.Contains(list[0].Title, "Januari 2020") {
		t.Fatalf("notifikasi = %+v", list)
	}
	if !strings.Contains(string(notifs.body), `"unread":1`) {
		t.Errorf("unread: %s", notifs.body)
	}
	slips := data[[]payslipSummaryRow](t, pejuang.do(http.MethodGet, "/v1/payslips", nil).want(t, http.StatusOK))
	// 3.000.000 + 2 x 10.000 + 1 (komponen tersembunyi tetap dihitung).
	if len(slips) != 1 || slips[0].ID != *list[0].RefID || slips[0].Net != "3020001.00" || !slips[0].Unread {
		t.Fatalf("slip = %+v", slips)
	}
	slip := data[payslipResp](t, pejuang.do(http.MethodGet, "/v1/payslips/"+slips[0].ID, nil).want(t, http.StatusOK))
	if slip.DocumentNumber == "" || len(slip.Items) != 2 {
		t.Errorf("rincian = %+v", slip)
	}
	for _, it := range slip.Items {
		if it.Code == "INTERNAL" {
			t.Error("komponen yang tidak tampil di slip ikut terkirim")
		}
	}
	if after := data[[]payslipSummaryRow](t, pejuang.do(http.MethodGet, "/v1/payslips", nil).want(t, http.StatusOK)); after[0].Unread {
		t.Error("membuka slip harus menandai notifikasinya dibaca")
	}

	pdf := pejuang.do(http.MethodGet, "/v1/payslips/"+slips[0].ID+"/pdf", nil).want(t, http.StatusOK)
	if !bytes.HasPrefix(pdf.body, []byte("%PDF-")) || pdf.header.Get("Cache-Control") != "private, no-store" ||
		!strings.Contains(pdf.header.Get("Content-Disposition"), "Slip-Ubar-2020-01-") {
		t.Errorf("PDF: %v", pdf.header)
	}
	again := pejuang.do(http.MethodGet, "/v1/payslips/"+slips[0].ID+"/pdf", nil).want(t, http.StatusOK)
	if !bytes.Equal(pdf.body, again.body) {
		t.Error("PDF slip final harus identik setiap diunduh")
	}

	// Slip orang lain tidak bisa dibuka; notifikasi orang lain tidak bisa ditandai.
	otherClient.do(http.MethodGet, "/v1/payslips/"+slips[0].ID, nil).wantError(t, http.StatusNotFound, "NOT_FOUND")
	otherClient.do(http.MethodGet, "/v1/payslips/"+slips[0].ID+"/pdf", nil).wantError(t, http.StatusNotFound, "NOT_FOUND")
	otherClient.do(http.MethodPost, "/v1/notifications/"+list[0].ID+"/read", nil).wantError(t, http.StatusNotFound, "NOT_FOUND")
	pejuang.do(http.MethodPost, "/v1/notifications/"+list[0].ID+"/read", nil).want(t, http.StatusNoContent)
	pejuang.do(http.MethodGet, "/v1/admin/payroll/periods/"+period.ID+"/employees/"+acc.employeeID+"/pdf", nil).
		wantError(t, http.StatusForbidden, "FORBIDDEN_ROLE")

	// Email pemberitahuan tanpa angka ubar ke Pejuang berakun aktif (dikirim di latar).
	var msg *mail.Message
	for i := 0; i < 200 && msg == nil; i++ {
		box.mu.Lock()
		for _, m := range box.sent {
			if m.To == acc.email {
				m := m
				msg = &m
			}
		}
		box.mu.Unlock()
		if msg == nil {
			time.Sleep(20 * time.Millisecond)
		}
	}
	if msg == nil {
		t.Fatal("email pemberitahuan slip tidak terkirim ke Pejuang")
	}
	if !strings.Contains(msg.Subject, "Januari 2020") || strings.Contains(msg.Body, "Rp") || strings.Contains(msg.Body, "3.020.001") {
		t.Errorf("email: %q %q", msg.Subject, msg.Body)
	}
}
