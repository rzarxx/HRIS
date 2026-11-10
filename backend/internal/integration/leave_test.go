package integration

import (
	"context"
	"net/http"
	"testing"
)

type leaveStep struct {
	StepOrder    int     `json:"step_order"`
	ApproverRole string  `json:"approver_role"`
	Status       string  `json:"status"`
	Note         *string `json:"note"`
}

type leaveRow struct {
	ID        string      `json:"id"`
	Days      int         `json:"days"`
	Status    string      `json:"status"`
	Approvals []leaveStep `json:"approvals"`
}

type leaveTypeRow struct {
	ID      string `json:"id"`
	Code    string `json:"code"`
	Balance *struct {
		EntitledDays  int `json:"entitled_days"`
		UsedDays      int `json:"used_days"`
		RemainingDays int `json:"remaining_days"`
	} `json:"balance"`
}

type approvalRow struct {
	ID          string         `json:"id"`
	RequestID   string         `json:"request_id"`
	StepOrder   int            `json:"step_order"`
	Status      string         `json:"status"`
	Summary     map[string]any `json:"summary"`
	RequestType string         `json:"request_type"`
}

func leaveTypeID(t *testing.T, c *client, code string) leaveTypeRow {
	t.Helper()
	for _, lt := range data[[]leaveTypeRow](t, c.do(http.MethodGet, "/v1/leave-types", nil).want(t, http.StatusOK)) {
		if lt.Code == code {
			return lt
		}
	}
	t.Fatalf("jenis cuti %s tidak ada", code)
	return leaveTypeRow{}
}

func submitLeave(c *client, typeID, start, end string) response {
	return c.do(http.MethodPost, "/v1/leaves", map[string]any{
		"leave_type_id": typeID, "start_date": start, "end_date": end, "reason": "Keperluan keluarga",
	})
}

func approvalID(t *testing.T, requestID string, step int) string {
	t.Helper()
	var id string
	if err := ownerPool.QueryRow(context.Background(),
		`SELECT id::text FROM approvals WHERE request_id = $1 AND step_order = $2`, requestID, step).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

func pendingFor(t *testing.T, c *client, requestID string) approvalRow {
	t.Helper()
	for _, a := range data[[]approvalRow](t, c.do(http.MethodGet, "/v1/approvals/pending", nil).want(t, http.StatusOK)) {
		if a.RequestID == requestID {
			return a
		}
	}
	t.Fatalf("pengajuan %s tidak ada di kotak masuk", requestID)
	return approvalRow{}
}

// TestLeaveApprovalFlow menguji alur default D46 (kadiv lalu SDM) sampai presensi tercatat cuti.
func TestLeaveApprovalFlow(t *testing.T) {
	f := newClockFixture(t, options{}, "08:00", "17:00") // Senin 2 Nov 2026
	exec(t, `UPDATE employees SET join_date = '2020-01-06' WHERE id = $1`, f.account.employeeID)
	// Kepala divisi harus aktif saat pengajuan dibuat; kepala nonaktif dilewati.
	kadivAcc := activeAccount(t, "kadiv", "OPS-LAP")
	kadiv := login(t, f.srv, kadivAcc.email, seedPassword)
	annual := leaveTypeID(t, f.hp, "annual")
	if annual.Balance == nil || annual.Balance.EntitledDays != 12 || annual.Balance.RemainingDays != 12 {
		t.Fatalf("saldo awal = %+v, ingin 12 hari", annual.Balance)
	}

	lr := data[leaveRow](t, submitLeave(f.hp, annual.ID, "2026-11-04", "2026-11-06").want(t, http.StatusCreated))
	if lr.Days != 3 || lr.Status != "pending" || len(lr.Approvals) != 2 {
		t.Fatalf("pengajuan = %+v", lr)
	}
	if lr.Approvals[0].ApproverRole != "kadiv" || lr.Approvals[0].Status != "pending" ||
		lr.Approvals[1].ApproverRole != "sdm" || lr.Approvals[1].Status != "waiting" {
		t.Fatalf("timeline = %+v", lr.Approvals)
	}
	submitLeave(f.hp, annual.ID, "2026-11-05", "2026-11-05").wantError(t, http.StatusConflict, "DUPLICATE_REQUEST")
	if b := leaveTypeID(t, f.hp, "annual").Balance; b.UsedDays != 3 || b.RemainingDays != 9 {
		t.Fatalf("saldo setelah mengajukan = %+v", b)
	}

	step1 := approvalID(t, lr.ID, 1)
	f.hp.do(http.MethodPost, "/v1/approvals/"+step1+"/approve", nil).wantError(t, http.StatusForbidden, "FORBIDDEN_ROLE")
	f.sdm.do(http.MethodPost, "/v1/approvals/"+step1+"/approve", nil).wantError(t, http.StatusForbidden, "APPROVAL_NOT_YOUR_TURN")
	step2 := approvalID(t, lr.ID, 2)
	f.sdm.do(http.MethodPost, "/v1/approvals/"+step2+"/approve", nil).wantError(t, http.StatusForbidden, "APPROVAL_NOT_YOUR_TURN")

	item := pendingFor(t, kadiv, lr.ID)
	if item.ID != step1 || item.RequestType != "leave" || item.Summary["days"] != float64(3) {
		t.Fatalf("item kotak masuk = %+v", item)
	}
	kadiv.do(http.MethodPost, "/v1/approvals/"+step1+"/approve", map[string]any{"note": "Silakan"}).want(t, http.StatusOK)
	kadiv.do(http.MethodPost, "/v1/approvals/"+step1+"/approve", nil).wantError(t, http.StatusConflict, "REQUEST_NOT_PENDING")

	pendingFor(t, f.sdm, lr.ID)
	f.sdm.do(http.MethodPost, "/v1/approvals/"+step2+"/reject", map[string]any{"note": "x"}).wantError(t, http.StatusBadRequest, "VALIDATION_FAILED")
	f.sdm.do(http.MethodPost, "/v1/approvals/"+step2+"/approve", nil).want(t, http.StatusOK)

	got := data[leaveRow](t, f.hp.do(http.MethodGet, "/v1/leaves/"+lr.ID, nil).want(t, http.StatusOK))
	if got.Status != "approved" || got.Approvals[0].Status != "approved" || got.Approvals[1].Status != "approved" {
		t.Fatalf("setelah disetujui = %+v", got)
	}
	var marked int
	if err := ownerPool.QueryRow(context.Background(), `SELECT count(*) FROM attendances
		WHERE employee_id = $1 AND status = 'leave' AND work_date BETWEEN '2026-11-04' AND '2026-11-06'`, f.account.employeeID).Scan(&marked); err != nil {
		t.Fatal(err)
	}
	if marked != 3 {
		t.Fatalf("hari cuti di presensi = %d, ingin 3", marked)
	}
	if n := countAudit(t, "approval.approved"); n != 2 {
		t.Fatalf("audit approval.approved = %d, ingin 2", n)
	}

	f.clock.Set(wibTime(4, 8, 0))
	f.clockIn(t).wantError(t, http.StatusConflict, "LEAVE_ACTIVE")
}

func TestLeaveRejectCancelAndQuota(t *testing.T) {
	f := newClockFixture(t, options{}, "08:00", "17:00")
	exec(t, `UPDATE employees SET join_date = '2020-01-06' WHERE id = $1`, f.account.employeeID)
	permit := leaveTypeID(t, f.hp, "permit")
	if permit.Balance != nil {
		t.Fatalf("izin tidak bersaldo, dapat %+v", permit.Balance)
	}
	kadivAcc := activeAccount(t, "kadiv", "OPS-LAP")
	kadiv := login(t, f.srv, kadivAcc.email, seedPassword)

	rejected := data[leaveRow](t, submitLeave(f.hp, permit.ID, "2026-11-09", "2026-11-09").want(t, http.StatusCreated))
	kadiv.do(http.MethodPost, "/v1/approvals/"+approvalID(t, rejected.ID, 1)+"/reject", map[string]any{"note": "Jadwal padat"}).want(t, http.StatusOK)
	got := data[leaveRow](t, f.hp.do(http.MethodGet, "/v1/leaves/"+rejected.ID, nil).want(t, http.StatusOK))
	if got.Status != "rejected" || got.Approvals[1].Status != "skipped" {
		t.Fatalf("setelah ditolak = %+v", got)
	}

	cancelled := data[leaveRow](t, submitLeave(f.hp, permit.ID, "2026-11-10", "2026-11-10").want(t, http.StatusCreated))
	got = data[leaveRow](t, f.hp.do(http.MethodPost, "/v1/leaves/"+cancelled.ID+"/cancel", nil).want(t, http.StatusOK))
	if got.Status != "cancelled" {
		t.Fatalf("status = %s, ingin cancelled", got.Status)
	}
	kadiv.do(http.MethodPost, "/v1/approvals/"+approvalID(t, cancelled.ID, 1)+"/approve", nil).wantError(t, http.StatusConflict, "REQUEST_NOT_PENDING")
	f.hp.do(http.MethodPost, "/v1/leaves/"+cancelled.ID+"/cancel", nil).wantError(t, http.StatusConflict, "REQUEST_NOT_PENDING")

	// Akhir pekan saja: tidak ada hari kerja.
	submitLeave(f.hp, permit.ID, "2026-11-07", "2026-11-08").wantError(t, http.StatusBadRequest, "VALIDATION_FAILED")

	// Hak cuti tahunan baru berlaku setelah 12 bulan bekerja (D47).
	exec(t, `UPDATE employees SET join_date = '2026-06-01' WHERE id = $1`, f.account.employeeID)
	annual := leaveTypeID(t, f.hp, "annual")
	res := submitLeave(f.hp, annual.ID, "2026-11-11", "2026-11-11").wantError(t, http.StatusBadRequest, "VALIDATION_FAILED")
	if res.fields()["leave_type_id"] == nil {
		t.Fatalf("pesan saldo harus di field leave_type_id: %v", res.fields())
	}
}

// TestLeaveSelfApproverIsSkipped: kepala divisi yang mengajukan tidak menyetujui dirinya sendiri.
func TestLeaveSelfApproverIsSkipped(t *testing.T) {
	clock := &fakeClock{now: wibTime(2, 8, 0)}
	srv := fresh(t, options{clock: clock.Now})
	kadivAcc := activeAccount(t, "kadiv", "OPS-LAP")
	kadiv := login(t, srv, kadivAcc.email, seedPassword)
	shift := data[shiftRow](t, loginSDM(t, srv).do(http.MethodPost, "/v1/admin/shifts", map[string]any{
		"name": "Uji", "start_time": "08:00", "end_time": "17:00", "late_tolerance_minutes": 10, "work_days": []int{1, 2, 3, 4, 5, 6, 7},
	}).want(t, http.StatusCreated))
	exec(t, `UPDATE employees SET default_shift_id = $1 WHERE id = $2`, shift.ID, kadivAcc.employeeID)
	permit := leaveTypeID(t, kadiv, "permit")
	lr := data[leaveRow](t, submitLeave(kadiv, permit.ID, "2026-11-12", "2026-11-12").want(t, http.StatusCreated))
	if lr.Approvals[0].Status != "skipped" || lr.Approvals[1].Status != "pending" {
		t.Fatalf("timeline = %+v", lr.Approvals)
	}
	kadiv.do(http.MethodPost, "/v1/approvals/"+approvalID(t, lr.ID, 2)+"/approve", nil).wantError(t, http.StatusForbidden, "APPROVAL_SELF_FORBIDDEN")
	for _, a := range data[[]approvalRow](t, kadiv.do(http.MethodGet, "/v1/approvals/pending", nil).want(t, http.StatusOK)) {
		if a.RequestID == lr.ID {
			t.Fatal("pengajuan sendiri tidak boleh tampil di kotak masuk")
		}
	}
}

// TestHeadAssignmentSetsRole menguji D49: role mengikuti penunjukan kepala subdiv.
func TestHeadAssignmentSetsRole(t *testing.T) {
	srv := fresh(t, options{})
	sdm := loginSDM(t, srv)
	head := activeAccount(t, "subdiv", "OPS-GDG")
	member := activeAccount(t, "pejuang", "OPS-GDG")
	role := func(userID string) string {
		var r string
		if err := ownerPool.QueryRow(context.Background(), `SELECT role::text FROM users WHERE id = $1`, userID).Scan(&r); err != nil {
			t.Fatal(err)
		}
		return r
	}
	path := "/v1/admin/subdivisions/" + head.subdivision
	sdm.do(http.MethodPatch, path, map[string]any{"head_employee_id": member.employeeID}).want(t, http.StatusOK)
	if role(member.userID) != "subdiv" || role(head.userID) != "pejuang" {
		t.Fatalf("role setelah ganti kepala: baru=%s lama=%s", role(member.userID), role(head.userID))
	}
	sdm.do(http.MethodPatch, path, map[string]any{"head_employee_id": head.employeeID}).want(t, http.StatusOK)
	if role(member.userID) != "pejuang" || role(head.userID) != "subdiv" {
		t.Fatalf("role setelah dikembalikan: anggota=%s kepala=%s", role(member.userID), role(head.userID))
	}
	if n := countAudit(t, "user.role_changed"); n != 4 {
		t.Fatalf("audit user.role_changed = %d, ingin 4", n)
	}
}

func TestApprovalFlowSettings(t *testing.T) {
	srv := fresh(t, options{})
	sdm := loginSDM(t, srv)
	t.Cleanup(func() {
		exec(t, `DELETE FROM approval_flow_steps WHERE request_type = 'leave'`)
		exec(t, `INSERT INTO approval_flow_steps VALUES ('leave', 1, 'kadiv'), ('leave', 2, 'sdm')`)
	})
	sdm.do(http.MethodPut, "/v1/admin/approval-flows/leave", map[string]any{"steps": []map[string]any{{"approver_role": "pejuang"}}}).
		wantError(t, http.StatusBadRequest, "VALIDATION_FAILED")
	sdm.do(http.MethodPut, "/v1/admin/approval-flows/leave", map[string]any{"steps": []map[string]any{{"approver_role": "sdm"}, {"approver_role": "sdm"}}}).
		wantError(t, http.StatusBadRequest, "VALIDATION_FAILED")
	sdm.do(http.MethodPut, "/v1/admin/approval-flows/unknown", map[string]any{"steps": []map[string]any{{"approver_role": "sdm"}}}).
		wantError(t, http.StatusNotFound, "NOT_FOUND")
	type flow struct {
		Steps []struct {
			ApproverRole string `json:"approver_role"`
		} `json:"steps"`
	}
	got := data[flow](t, sdm.do(http.MethodPut, "/v1/admin/approval-flows/leave", map[string]any{
		"steps": []map[string]any{{"approver_role": "subdiv"}, {"approver_role": "sdm"}},
	}).want(t, http.StatusOK))
	if len(got.Steps) != 2 || got.Steps[0].ApproverRole != "subdiv" {
		t.Fatalf("alur = %+v", got)
	}
}
