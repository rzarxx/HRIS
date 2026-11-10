// Package approval adalah mesin persetujuan generik (MASTER.md F12). Setiap jenis pengajuan
// (cuti, pinjaman, mutasi) menyimpan datanya sendiri dan mendaftarkan Kind agar mesin ini bisa
// mengunci, merangkum, dan menyelesaikan pengajuan tanpa mengenal tabelnya.
package approval

import (
	"context"
	"net/http"
	"net/netip"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rzarxx/HRIS/backend/internal/audit"
	"github.com/rzarxx/HRIS/backend/internal/auth"
	"github.com/rzarxx/HRIS/backend/internal/permission"
	"github.com/rzarxx/HRIS/backend/internal/platform/database"
	"github.com/rzarxx/HRIS/backend/internal/platform/httpx"
	"github.com/rzarxx/HRIS/backend/internal/platform/pgconv"
	"github.com/rzarxx/HRIS/backend/internal/store"
)

const (
	maxNoteLen   = 500
	minRejectLen = 3
	maxFlowSteps = 5
)

// Kind dipasang oleh modul pengajuan. Semua metode berjalan di dalam transaksi pemanggil.
type Kind interface {
	// Lock mengunci baris pengajuan dan mengembalikan statusnya.
	Lock(ctx context.Context, q *store.Queries, requestID uuid.UUID) (store.RequestStatus, error)
	// Decided dipanggil sekali saat pengajuan selesai: approved atau rejected.
	Decided(ctx context.Context, q *store.Queries, requestID uuid.UUID, status store.RequestStatus, at time.Time) error
	// Summaries memberi ringkasan pengajuan untuk kotak masuk approver.
	Summaries(ctx context.Context, q *store.Queries, ids []uuid.UUID) (map[uuid.UUID]map[string]any, error)
}

// Step adalah satu langkah timeline persetujuan (MASTER.md §7.6).
type Step struct {
	StepOrder    int     `json:"step_order"`
	ApproverRole string  `json:"approver_role"`
	ApproverName *string `json:"approver_name"`
	Status       string  `json:"status"`
	Note         *string `json:"note"`
	ActedAt      *string `json:"acted_at"`
}

// Requester adalah pemohon dalam item persetujuan.
type Requester struct {
	EmployeeID      string `json:"employee_id"`
	FullName        string `json:"full_name"`
	SubdivisionName string `json:"subdivision_name"`
}

// Item adalah satu langkah yang menunggu keputusan seorang approver.
type Item struct {
	ID           string         `json:"id"`
	RequestType  string         `json:"request_type"`
	RequestID    string         `json:"request_id"`
	StepOrder    int            `json:"step_order"`
	ApproverRole string         `json:"approver_role"`
	Status       string         `json:"status"`
	Requester    Requester      `json:"requester"`
	Summary      map[string]any `json:"summary"`
	Note         *string        `json:"note"`
	ActedAt      *string        `json:"acted_at"`
	CreatedAt    string         `json:"created_at"`
	Steps        []Step         `json:"steps,omitempty"`
}

// FlowStep adalah satu langkah alur yang diatur SDM.
type FlowStep struct {
	StepOrder    int    `json:"step_order"`
	ApproverRole string `json:"approver_role"`
}

// Flow adalah alur satu jenis pengajuan.
type Flow struct {
	RequestType string     `json:"request_type"`
	Steps       []FlowStep `json:"steps"`
}

// FlowInput mengganti seluruh alur. Berlaku untuk pengajuan baru; pengajuan berjalan memakai snapshot.
type FlowInput struct {
	Steps []struct {
		ApproverRole string `json:"approver_role"`
	} `json:"steps"`
}

type Service struct {
	pool  *pgxpool.Pool
	q     *store.Queries
	kinds map[store.RequestKind]Kind
	now   func() time.Time
}

func NewService(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool, q: store.New(pool), kinds: map[store.RequestKind]Kind{}, now: time.Now}
}

// Register memasang jenis pengajuan. Dipanggil sekali saat aplikasi dirakit.
func (s *Service) Register(kind store.RequestKind, k Kind) { s.kinds[kind] = k }

// SetClock dipakai tes.
func (s *Service) SetClock(now func() time.Time) { s.now = now }

func validKind(kind string) (store.RequestKind, bool) {
	k := store.RequestKind(kind)
	switch k {
	case store.RequestKindLeave, store.RequestKindCashAdvance, store.RequestKindTransfer:
		return k, true
	}
	return "", false
}

// planned adalah rencana satu langkah sebelum disimpan.
type planned struct {
	role     store.UserRole
	assigned uuid.NullUUID
	skipNote string
}

// Plan menentukan approver tiap langkah (F12): subdiv dan kadiv adalah kepala unit pemohon, sdm
// adalah pool. Langkah yang approvernya pemohon sendiri atau kepalanya belum ditetapkan dilewati;
// bila semua langkah terlewati, SDM tetap memutuskan.
func Plan(flow []store.UserRole, requester uuid.UUID, subdivHead, divHead uuid.NullUUID) []planned {
	out := make([]planned, 0, len(flow)+1)
	for _, role := range flow {
		p := planned{role: role}
		switch role {
		case store.UserRoleSubdiv:
			p.assigned = subdivHead
		case store.UserRoleKadiv:
			p.assigned = divHead
		}
		if role != store.UserRoleSdm {
			switch {
			case !p.assigned.Valid:
				p.skipNote = "Kepala unit belum ditetapkan."
			case p.assigned.UUID == requester:
				p.skipNote = "Pemohon adalah approver langkah ini."
			}
		}
		out = append(out, p)
	}
	if !slices.ContainsFunc(out, func(p planned) bool { return p.skipNote == "" }) {
		out = append(out, planned{role: store.UserRoleSdm})
	}
	return out
}

// Start membuat snapshot langkah untuk pengajuan baru, di dalam transaksi pemanggil.
func Start(ctx context.Context, q *store.Queries, kind store.RequestKind, requestID, requester uuid.UUID) error {
	steps, err := q.ListFlowSteps(ctx, kind)
	if err != nil {
		return err
	}
	heads, err := q.GetRequesterHeads(ctx, requester)
	if err != nil {
		return err
	}
	roles := make([]store.UserRole, 0, len(steps))
	for _, st := range steps {
		roles = append(roles, st.ApproverRole)
	}
	first := true
	for i, p := range Plan(roles, requester, heads.SubdivisionHeadID, heads.DivisionHeadID) {
		status := store.ApprovalStepStatusWaiting
		note := pgconv.Text(nil)
		switch {
		case p.skipNote != "":
			status = store.ApprovalStepStatusSkipped
			note = pgconv.Text(&p.skipNote)
		case first:
			status = store.ApprovalStepStatusPending
			first = false
		}
		if _, err := q.InsertApproval(ctx, store.InsertApprovalParams{
			RequestType: kind, RequestID: requestID, RequesterEmployeeID: requester, StepOrder: int16(i + 1),
			ApproverRole: p.role, AssignedEmployeeID: p.assigned, Status: status, Note: note,
		}); err != nil {
			return err
		}
	}
	return nil
}

// Close melewati langkah yang belum diputuskan, misalnya saat pemohon membatalkan.
func Close(ctx context.Context, q *store.Queries, kind store.RequestKind, requestID uuid.UUID) error {
	return q.SkipOpenApprovals(ctx, store.SkipOpenApprovalsParams{RequestType: kind, RequestID: requestID})
}

// Timeline mengembalikan langkah persetujuan satu pengajuan.
func Timeline(ctx context.Context, q *store.Queries, kind store.RequestKind, requestID uuid.UUID) ([]Step, error) {
	rows, err := q.ListApprovalsForRequest(ctx, store.ListApprovalsForRequestParams{RequestType: kind, RequestID: requestID})
	if err != nil {
		return nil, err
	}
	out := make([]Step, 0, len(rows))
	for _, r := range rows {
		out = append(out, step(r.StepOrder, r.ApproverRole, r.ApproverName.Valid, r.ApproverName.String, r.Status, r.Note, r.ActedAt))
	}
	return out, nil
}

// Timelines mengembalikan timeline banyak pengajuan sekaligus, dikelompokkan per pengajuan.
func Timelines(ctx context.Context, q *store.Queries, kind store.RequestKind, ids []uuid.UUID) (map[uuid.UUID][]Step, error) {
	out := map[uuid.UUID][]Step{}
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := q.ListApprovalsForRequests(ctx, store.ListApprovalsForRequestsParams{RequestType: kind, RequestIds: ids})
	if err != nil {
		return nil, err
	}
	for _, r := range rows {
		out[r.RequestID] = append(out[r.RequestID], step(r.StepOrder, r.ApproverRole, r.ApproverName.Valid, r.ApproverName.String, r.Status, r.Note, r.ActedAt))
	}
	return out, nil
}

func step(order int16, role store.UserRole, hasName bool, name string, status store.ApprovalStepStatus, note pgtype.Text, acted pgtype.Timestamptz) Step {
	st := Step{StepOrder: int(order), ApproverRole: string(role), Status: string(status), Note: pgconv.TextPtr(note), ActedAt: pgconv.TimeString(acted)}
	if hasName {
		st.ApproverName = &name
	}
	return st
}

// Pending mengembalikan langkah yang menunggu keputusan orang ini.
func (s *Service) Pending(ctx context.Context, p auth.Principal) ([]Item, error) {
	rows, err := s.q.ListPendingApprovals(ctx, store.ListPendingApprovalsParams{
		EmployeeID: p.EmployeeID, IsSdm: p.Role == permission.RoleSDM,
	})
	if err != nil {
		return nil, err
	}
	byKind := map[store.RequestKind][]uuid.UUID{}
	for _, r := range rows {
		byKind[r.RequestType] = append(byKind[r.RequestType], r.RequestID)
	}
	summaries := map[uuid.UUID]map[string]any{}
	for kind, ids := range byKind {
		k, ok := s.kinds[kind]
		if !ok {
			continue
		}
		got, err := k.Summaries(ctx, s.q, ids)
		if err != nil {
			return nil, err
		}
		for id, sum := range got {
			summaries[id] = sum
		}
	}
	out := make([]Item, 0, len(rows))
	for _, r := range rows {
		out = append(out, Item{
			ID: r.ID.String(), RequestType: string(r.RequestType), RequestID: r.RequestID.String(), StepOrder: int(r.StepOrder),
			ApproverRole: string(r.ApproverRole), Status: string(r.Status),
			Requester: Requester{EmployeeID: r.RequesterEmployeeID.String(), FullName: r.RequesterName, SubdivisionName: r.RequesterSubdivision},
			Summary:   summaryOrEmpty(summaries[r.RequestID]), Note: pgconv.TextPtr(r.Note), ActedAt: pgconv.TimeString(r.ActedAt),
			CreatedAt: r.CreatedAt.Time.UTC().Format(time.RFC3339),
		})
	}
	return out, nil
}

func summaryOrEmpty(m map[string]any) map[string]any {
	if m == nil {
		return map[string]any{}
	}
	return m
}

// mayAct menjawab apakah orang ini approver langkah tersebut: orang yang ditugaskan, atau SDM untuk pool.
func mayAct(p auth.Principal, assigned uuid.NullUUID) bool {
	if assigned.Valid {
		return assigned.UUID == p.EmployeeID
	}
	return p.Role == permission.RoleSDM
}

// Get mengembalikan satu langkah beserta timeline pengajuannya, hanya untuk approvernya.
func (s *Service) Get(ctx context.Context, p auth.Principal, id uuid.UUID) (Item, error) {
	r, err := s.q.GetApprovalDetail(ctx, id)
	if pgconv.IsNoRows(err) {
		return Item{}, httpx.ErrNotFound()
	}
	if err != nil {
		return Item{}, err
	}
	if r.RequesterEmployeeID == p.EmployeeID || !mayAct(p, r.AssignedEmployeeID) {
		return Item{}, httpx.ErrNotFound()
	}
	item := Item{
		ID: r.ID.String(), RequestType: string(r.RequestType), RequestID: r.RequestID.String(), StepOrder: int(r.StepOrder),
		ApproverRole: string(r.ApproverRole), Status: string(r.Status),
		Requester: Requester{EmployeeID: r.RequesterEmployeeID.String(), FullName: r.RequesterName, SubdivisionName: r.RequesterSubdivision},
		Summary:   map[string]any{}, Note: pgconv.TextPtr(r.Note), ActedAt: pgconv.TimeString(r.ActedAt),
		CreatedAt: r.CreatedAt.Time.UTC().Format(time.RFC3339),
	}
	if k, ok := s.kinds[r.RequestType]; ok {
		got, err := k.Summaries(ctx, s.q, []uuid.UUID{r.RequestID})
		if err != nil {
			return Item{}, err
		}
		item.Summary = summaryOrEmpty(got[r.RequestID])
	}
	if item.Steps, err = Timeline(ctx, s.q, r.RequestType, r.RequestID); err != nil {
		return Item{}, err
	}
	return item, nil
}

// Decision adalah isi approve atau reject.
type Decision struct {
	Note *string `json:"note"`
}

// Approve dan Reject memutuskan satu langkah. Pengajuan dikunci lebih dulu, lalu langkahnya, sama
// dengan urutan pembatalan, agar tidak terjadi deadlock. Klik ganda ditolak REQUEST_NOT_PENDING.
func (s *Service) Approve(ctx context.Context, p auth.Principal, ip *netip.Addr, id uuid.UUID, in Decision) (Item, error) {
	note, err := cleanNote(in.Note, false)
	if err != nil {
		return Item{}, err
	}
	return s.decide(ctx, p, ip, id, note, true)
}

func (s *Service) Reject(ctx context.Context, p auth.Principal, ip *netip.Addr, id uuid.UUID, in Decision) (Item, error) {
	note, err := cleanNote(in.Note, true)
	if err != nil {
		return Item{}, err
	}
	return s.decide(ctx, p, ip, id, note, false)
}

func cleanNote(in *string, required bool) (*string, error) {
	if in == nil {
		if required {
			return nil, httpx.Validation(map[string]string{"note": "Alasan penolakan wajib diisi."})
		}
		return nil, nil
	}
	note := strings.TrimSpace(*in)
	switch {
	case required && len([]rune(note)) < minRejectLen:
		return nil, httpx.Validation(map[string]string{"note": "Alasan penolakan minimal 3 karakter."})
	case len([]rune(note)) > maxNoteLen:
		return nil, httpx.Validation(map[string]string{"note": "Catatan maksimal 500 karakter."})
	case note == "":
		return nil, nil
	}
	return &note, nil
}

func (s *Service) decide(ctx context.Context, p auth.Principal, ip *netip.Addr, id uuid.UUID, note *string, approve bool) (Item, error) {
	head, err := s.q.GetApprovalDetail(ctx, id)
	if pgconv.IsNoRows(err) {
		return Item{}, httpx.ErrNotFound()
	}
	if err != nil {
		return Item{}, err
	}
	if head.RequesterEmployeeID == p.EmployeeID {
		return Item{}, httpx.NewError(http.StatusForbidden, "APPROVAL_SELF_FORBIDDEN", "Anda tidak boleh memutuskan pengajuan sendiri.")
	}
	if !mayAct(p, head.AssignedEmployeeID) {
		return Item{}, httpx.NewError(http.StatusForbidden, "APPROVAL_NOT_YOUR_TURN", "Langkah ini bukan milik Anda.")
	}
	kind, ok := s.kinds[head.RequestType]
	if !ok {
		return Item{}, httpx.ErrNotFound()
	}
	now := s.now()
	err = database.InTx(ctx, s.pool, func(q *store.Queries) error {
		status, err := kind.Lock(ctx, q, head.RequestID)
		if err != nil {
			return err
		}
		if status != store.RequestStatusPending {
			return notPending()
		}
		a, err := q.LockApproval(ctx, id)
		if err != nil {
			return err
		}
		switch a.Status {
		case store.ApprovalStepStatusPending:
		case store.ApprovalStepStatusWaiting:
			return httpx.NewError(http.StatusForbidden, "APPROVAL_NOT_YOUR_TURN", "Langkah sebelumnya belum diputuskan.")
		default:
			return notPending()
		}
		decision := store.ApprovalStepStatusApproved
		action := audit.ApprovalApproved
		if !approve {
			decision = store.ApprovalStepStatusRejected
			action = audit.ApprovalRejected
		}
		if err := q.SetApprovalDecision(ctx, store.SetApprovalDecisionParams{
			ID: a.ID, Status: decision, Note: pgconv.Text(note), ActedBy: audit.Actor(p.UserID), ActedAt: pgconv.Time(now),
		}); err != nil {
			return err
		}
		final := store.RequestStatusRejected
		if approve {
			steps, err := q.LockApprovalsForRequest(ctx, store.LockApprovalsForRequestParams{RequestType: a.RequestType, RequestID: a.RequestID})
			if err != nil {
				return err
			}
			final = store.RequestStatusApproved
			for _, st := range steps {
				if st.StepOrder > a.StepOrder && st.Status == store.ApprovalStepStatusWaiting {
					if err := q.ActivateApproval(ctx, st.ID); err != nil {
						return err
					}
					final = store.RequestStatusPending
					break
				}
			}
		} else if err := Close(ctx, q, a.RequestType, a.RequestID); err != nil {
			return err
		}
		if final != store.RequestStatusPending {
			if err := kind.Decided(ctx, q, a.RequestID, final, now); err != nil {
				return err
			}
		}
		return audit.Write(ctx, q, audit.Entry{
			UserID: audit.Actor(p.UserID), Action: action, Entity: string(a.RequestType), EntityID: a.RequestID, IP: ip,
			Detail: map[string]any{"approval_id": a.ID, "step_order": a.StepOrder, "approver_role": a.ApproverRole, "request_status": final},
		})
	})
	if err != nil {
		return Item{}, err
	}
	return s.Get(ctx, p, id)
}

func notPending() error {
	return httpx.NewError(http.StatusConflict, "REQUEST_NOT_PENDING", "Pengajuan ini sudah diputuskan atau dibatalkan.")
}

// Flow mengembalikan alur satu jenis pengajuan.
func (s *Service) Flow(ctx context.Context, kind string) (Flow, error) {
	k, ok := validKind(kind)
	if !ok {
		return Flow{}, httpx.ErrNotFound()
	}
	rows, err := s.q.ListFlowSteps(ctx, k)
	if err != nil {
		return Flow{}, err
	}
	out := Flow{RequestType: kind, Steps: make([]FlowStep, 0, len(rows))}
	for _, r := range rows {
		out.Steps = append(out.Steps, FlowStep{StepOrder: int(r.StepOrder), ApproverRole: string(r.ApproverRole)})
	}
	return out, nil
}

// SetFlow mengganti alur. Role tidak boleh berulang dan paling banyak lima langkah.
func (s *Service) SetFlow(ctx context.Context, p auth.Principal, ip *netip.Addr, kind string, in FlowInput) (Flow, error) {
	k, ok := validKind(kind)
	if !ok {
		return Flow{}, httpx.ErrNotFound()
	}
	if len(in.Steps) == 0 || len(in.Steps) > maxFlowSteps {
		return Flow{}, httpx.Validation(map[string]string{"steps": "Alur berisi 1 sampai 5 langkah."})
	}
	roles := make([]store.UserRole, 0, len(in.Steps))
	for _, st := range in.Steps {
		role := store.UserRole(st.ApproverRole)
		if role != store.UserRoleSubdiv && role != store.UserRoleKadiv && role != store.UserRoleSdm {
			return Flow{}, httpx.Validation(map[string]string{"steps": "Approver harus subdiv, kadiv, atau sdm."})
		}
		if slices.Contains(roles, role) {
			return Flow{}, httpx.Validation(map[string]string{"steps": "Setiap approver hanya boleh muncul sekali."})
		}
		roles = append(roles, role)
	}
	err := database.InTx(ctx, s.pool, func(q *store.Queries) error {
		if err := q.DeleteFlowSteps(ctx, k); err != nil {
			return err
		}
		names := make([]string, 0, len(roles))
		for i, role := range roles {
			if err := q.InsertFlowStep(ctx, store.InsertFlowStepParams{RequestType: k, StepOrder: int16(i + 1), ApproverRole: role}); err != nil {
				return err
			}
			names = append(names, string(role))
		}
		return audit.Write(ctx, q, audit.Entry{
			UserID: audit.Actor(p.UserID), Action: audit.SettingsChanged, Entity: "approval_flow", EntityID: uuid.Nil, IP: ip,
			Detail: map[string]any{"request_type": kind, "steps": names},
		})
	})
	if err != nil {
		return Flow{}, err
	}
	return s.Flow(ctx, kind)
}
