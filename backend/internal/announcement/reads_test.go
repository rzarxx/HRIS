package announcement

import (
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

func TestWaitSecondsCountsFromFirstOpen(t *testing.T) {
	opened := time.Date(2026, 11, 2, 1, 0, 0, 0, time.UTC)
	at := func(t time.Time) pgtype.Timestamptz { return pgtype.Timestamptz{Time: t, Valid: true} }
	none := pgtype.Timestamptz{}
	cases := []struct {
		name   string
		min    int16
		opened pgtype.Timestamptz
		read   pgtype.Timestamptz
		now    time.Time
		want   int
	}{
		{"belum dibuka memakai jeda penuh", 10, none, none, opened, 10},
		{"baru dibuka", 10, at(opened), none, opened, 10},
		{"sisa dibulatkan ke atas", 10, at(opened), none, opened.Add(2500 * time.Millisecond), 8},
		{"jeda tepat habis", 10, at(opened), none, opened.Add(10 * time.Second), 0},
		{"sudah lewat", 10, at(opened), none, opened.Add(time.Minute), 0},
		{"tanpa jeda", 0, at(opened), none, opened, 0},
		{"sudah dibaca", 10, at(opened), at(opened.Add(time.Second)), opened, 0},
	}
	for _, c := range cases {
		if got := waitSeconds(c.min, c.opened, c.read, c.now); got != c.want {
			t.Errorf("%s: %d, ingin %d", c.name, got, c.want)
		}
	}
}
