// Package pgconv mengonversi tipe pgx ke nilai JSON sesuai format baku MASTER.md §2.3.
package pgconv

import (
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

// WIB adalah zona Asia/Jakarta. Indonesia tidak memakai DST sehingga offset tetap aman
// dan tidak bergantung pada tzdata di server.
var WIB = time.FixedZone("WIB", 7*3600)

const dateLayout = "2006-01-02"

// WorkDate menghitung tanggal kerja di sisi server (MASTER.md §2.3, D11).
func WorkDate(now time.Time) time.Time {
	y, m, d := now.In(WIB).Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func Date(t time.Time) pgtype.Date { return pgtype.Date{Time: t, Valid: true} }

func DateString(d pgtype.Date) *string {
	if !d.Valid {
		return nil
	}
	s := d.Time.Format(dateLayout)
	return &s
}

func ParseDate(s string) (pgtype.Date, error) {
	t, err := time.Parse(dateLayout, s)
	if err != nil {
		return pgtype.Date{}, err
	}
	return Date(t), nil
}

func Time(t time.Time) pgtype.Timestamptz { return pgtype.Timestamptz{Time: t, Valid: true} }

// TimeString memformat waktu sebagai RFC 3339 UTC.
func TimeString(t pgtype.Timestamptz) *string {
	if !t.Valid {
		return nil
	}
	s := t.Time.UTC().Format(time.RFC3339)
	return &s
}

func TextPtr(t pgtype.Text) *string {
	if !t.Valid {
		return nil
	}
	return &t.String
}

func Text(s *string) pgtype.Text {
	if s == nil || strings.TrimSpace(*s) == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: strings.TrimSpace(*s), Valid: true}
}

func UUIDPtr(u uuid.NullUUID) *string {
	if !u.Valid {
		return nil
	}
	s := u.UUID.String()
	return &s
}

// ParseOptionalUUID mengubah *string menjadi NullUUID; nil atau kosong berarti NULL.
func ParseOptionalUUID(s *string) (uuid.NullUUID, error) {
	if s == nil || *s == "" {
		return uuid.NullUUID{}, nil
	}
	id, err := uuid.Parse(*s)
	if err != nil {
		return uuid.NullUUID{}, err
	}
	return uuid.NullUUID{UUID: id, Valid: true}, nil
}

func IsNoRows(err error) bool { return errors.Is(err, pgx.ErrNoRows) }

// UniqueConstraint mengembalikan nama constraint bila err adalah pelanggaran UNIQUE.
func UniqueConstraint(err error) (string, bool) {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return pgErr.ConstraintName, true
	}
	return "", false
}

// IsForeignKeyViolation mendeteksi referensi yang masih dipakai atau tidak ada.
func IsForeignKeyViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23503"
}
