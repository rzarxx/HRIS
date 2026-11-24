package employee

import (
	"context"
	"slices"
	"strings"

	"github.com/google/uuid"

	"github.com/rzarxx/HRIS/backend/internal/platform/httpx"
	"github.com/rzarxx/HRIS/backend/internal/store"
)

// Shift ganda (D61): satu Pejuang bisa punya beberapa shift dengan hari kerja yang tidak bertabrakan.
// Shift pertama adalah shift utama (employees.default_shift_id).

// ShiftRef adalah shift milik Pejuang di respons.
type ShiftRef struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Kind string `json:"kind"`
}

var shiftDayNames = map[int16]string{1: "Senin", 2: "Selasa", 3: "Rabu", 4: "Kamis", 5: "Jumat", 6: "Sabtu", 7: "Ahad"}

// parseShiftIDs membaca shift_ids: urutan dipertahankan, duplikat dibuang.
func parseShiftIDs(raw []string) ([]uuid.UUID, bool) {
	out := make([]uuid.UUID, 0, len(raw))
	for _, s := range raw {
		id, err := uuid.Parse(strings.TrimSpace(s))
		if err != nil {
			return nil, false
		}
		if !slices.Contains(out, id) {
			out = append(out, id)
		}
	}
	return out, true
}

// setShifts mengganti daftar shift Pejuang dalam transaksi pemanggil, setelah memastikan semua shift
// terdaftar dan hari kerjanya tidak bertabrakan.
func setShifts(ctx context.Context, q *store.Queries, employeeID uuid.UUID, ids []uuid.UUID) error {
	if len(ids) > 7 {
		return httpx.Validation(map[string]string{"shift_ids": "Maksimal 7 shift per Pejuang."})
	}
	if len(ids) > 0 {
		rows, err := q.GetShiftsByIDs(ctx, ids)
		if err != nil {
			return err
		}
		if len(rows) != len(ids) {
			return httpx.Validation(map[string]string{"shift_ids": "Shift belum terdaftar."})
		}
		owner := map[int16]string{}
		for _, sh := range rows {
			for _, d := range sh.WorkDays {
				if other, taken := owner[d]; taken {
					return httpx.Validation(map[string]string{
						"shift_ids": "Hari " + shiftDayNames[d] + " dipakai shift " + other + " dan " + sh.Name + ". Satu hari hanya boleh satu shift.",
					})
				}
				owner[d] = sh.Name
			}
		}
	}
	if err := q.DeleteEmployeeShifts(ctx, employeeID); err != nil {
		return err
	}
	for _, id := range ids {
		if err := q.AddEmployeeShift(ctx, store.AddEmployeeShiftParams{EmployeeID: employeeID, ShiftID: id}); err != nil {
			return err
		}
	}
	return nil
}

// attachShifts mengisi daftar shift setiap Pejuang di respons.
func (s *Service) attachShifts(ctx context.Context, employees []Employee, ids []uuid.UUID) error {
	if len(ids) == 0 {
		return nil
	}
	rows, err := s.q.ListShiftsForEmployees(ctx, ids)
	if err != nil {
		return err
	}
	byEmployee := map[string][]ShiftRef{}
	for _, r := range rows {
		key := r.EmployeeID.String()
		byEmployee[key] = append(byEmployee[key], ShiftRef{ID: r.ID.String(), Name: r.Name, Kind: r.Kind})
	}
	for i := range employees {
		employees[i].Shifts = byEmployee[employees[i].ID]
		if employees[i].Shifts == nil {
			employees[i].Shifts = []ShiftRef{}
		}
	}
	return nil
}

// shiftPick adalah satu shift dari kolom Shift impor: id shift lama atau key shift baru.
type shiftPick struct {
	id  uuid.UUID
	key string
}

// parseShiftKind membaca kolom Jenis di sheet Shift. Kosong berarti reguler.
func parseShiftKind(raw string) (string, bool) {
	switch strings.ToLower(strings.Join(strings.Fields(raw), " ")) {
	case "", "reguler", "regular":
		return "regular", true
	case "setengah hari", "setengah", "half_day", "half day", "1/2 hari":
		return "half_day", true
	}
	return "", false
}

// shiftKindLabel menulis jenis shift untuk template impor.
func shiftKindLabel(kind string) string {
	if kind == "half_day" {
		return "Setengah hari"
	}
	return "Reguler"
}
