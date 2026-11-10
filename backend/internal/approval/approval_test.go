package approval

import (
	"testing"

	"github.com/google/uuid"

	"github.com/rzarxx/HRIS/backend/internal/store"
)

func id() uuid.NullUUID { return uuid.NullUUID{UUID: uuid.New(), Valid: true} }

func TestPlanAssignsHeadsAndPool(t *testing.T) {
	requester := uuid.New()
	sub, div := id(), id()
	got := Plan([]store.UserRole{store.UserRoleSubdiv, store.UserRoleKadiv, store.UserRoleSdm}, requester, sub, div)
	if len(got) != 3 {
		t.Fatalf("langkah = %d, ingin 3", len(got))
	}
	if got[0].assigned != sub || got[1].assigned != div || got[2].assigned.Valid {
		t.Fatalf("approver salah: %+v", got)
	}
	for _, p := range got {
		if p.skipNote != "" {
			t.Fatalf("tidak ada langkah yang boleh dilewati: %+v", p)
		}
	}
}

func TestPlanSkipsSelfAndMissingHead(t *testing.T) {
	requester := uuid.New()
	self := uuid.NullUUID{UUID: requester, Valid: true}
	got := Plan([]store.UserRole{store.UserRoleSubdiv, store.UserRoleKadiv, store.UserRoleSdm}, requester, self, uuid.NullUUID{})
	if got[0].skipNote == "" || got[1].skipNote == "" {
		t.Fatalf("kepala = pemohon dan kepala kosong harus dilewati: %+v", got)
	}
	if got[2].skipNote != "" {
		t.Fatalf("pool sdm tidak boleh dilewati")
	}
}

func TestPlanAddsSDMWhenAllSkipped(t *testing.T) {
	requester := uuid.New()
	self := uuid.NullUUID{UUID: requester, Valid: true}
	got := Plan([]store.UserRole{store.UserRoleKadiv}, requester, uuid.NullUUID{}, self)
	if len(got) != 2 || got[1].role != store.UserRoleSdm || got[1].skipNote != "" {
		t.Fatalf("SDM harus ditambahkan bila semua langkah terlewati: %+v", got)
	}
}

func TestCleanNote(t *testing.T) {
	if _, err := cleanNote(nil, true); err == nil {
		t.Fatal("reject tanpa alasan harus ditolak")
	}
	short := " ok "
	if _, err := cleanNote(&short, true); err == nil {
		t.Fatal("alasan reject kurang dari 3 karakter harus ditolak")
	}
	empty := "   "
	if got, err := cleanNote(&empty, false); err != nil || got != nil {
		t.Fatalf("catatan kosong saat approve = %v, %v", got, err)
	}
}
