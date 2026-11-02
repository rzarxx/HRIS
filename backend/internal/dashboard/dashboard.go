// Package dashboard melayani kartu ringkasan dan grafik organisasi (QUERY.sql #1 dan #2).
package dashboard

import (
	"net/http"
	"time"

	"github.com/rzarxx/HRIS/backend/internal/platform/httpx"
	"github.com/rzarxx/HRIS/backend/internal/platform/pgconv"
	"github.com/rzarxx/HRIS/backend/internal/store"
)

type Handler struct{ q *store.Queries }

func NewHandler(q *store.Queries) *Handler { return &Handler{q: q} }

func (h *Handler) Summary(w http.ResponseWriter, r *http.Request) error {
	workDate := pgconv.WorkDate(time.Now())
	row, err := h.q.DashboardSummary(r.Context(), pgconv.Date(workDate))
	if err != nil {
		return err
	}
	return httpx.Data(w, http.StatusOK, map[string]any{
		"employee_count":    row.EmployeeCount,
		"division_count":    row.DivisionCount,
		"subdivision_count": row.SubdivisionCount,
		"present_today":     row.PresentToday,
		"late_today":        row.LateToday,
		"work_date":         workDate.Format("2006-01-02"),
	})
}

type employeesPerSubdivision struct {
	Division      string `json:"division"`
	Subdivision   string `json:"subdivision"`
	EmployeeCount int64  `json:"employee_count"`
}

type subdivisionsPerDivision struct {
	Division         string `json:"division"`
	SubdivisionCount int64  `json:"subdivision_count"`
}

func (h *Handler) OrgChart(w http.ResponseWriter, r *http.Request) error {
	subs, err := h.q.EmployeesPerSubdivision(r.Context())
	if err != nil {
		return err
	}
	divs, err := h.q.SubdivisionsPerDivision(r.Context())
	if err != nil {
		return err
	}
	perSub := make([]employeesPerSubdivision, 0, len(subs))
	for _, s := range subs {
		perSub = append(perSub, employeesPerSubdivision(s))
	}
	perDiv := make([]subdivisionsPerDivision, 0, len(divs))
	for _, d := range divs {
		perDiv = append(perDiv, subdivisionsPerDivision(d))
	}
	return httpx.Data(w, http.StatusOK, map[string]any{
		"employees_per_subdivision": perSub,
		"subdivisions_per_division": perDiv,
	})
}
