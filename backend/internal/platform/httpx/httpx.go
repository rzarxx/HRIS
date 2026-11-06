// Package httpx berisi envelope respons, error registry, dan helper input (MASTER.md §7.3-7.4).
package httpx

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// Error adalah error yang boleh dilihat klien. Selain tipe ini, klien hanya menerima INTERNAL_ERROR.
type Error struct {
	Status  int
	Code    string
	Message string
	Details map[string]any
}

func (e *Error) Error() string { return e.Code + ": " + e.Message }

func NewError(status int, code, message string) *Error {
	return &Error{Status: status, Code: code, Message: message}
}

func ErrNotFound() *Error {
	return NewError(http.StatusNotFound, "NOT_FOUND", "Data tidak ditemukan.")
}

func ErrForbidden() *Error {
	return NewError(http.StatusForbidden, "FORBIDDEN_ROLE", "Anda tidak memiliki akses ke fitur ini.")
}

func ErrUnauthenticated() *Error {
	return NewError(http.StatusUnauthorized, "AUTH_TOKEN_INVALID", "Sesi berakhir, silakan masuk kembali.")
}

// Validation mengembalikan VALIDATION_FAILED dengan pesan per field.
func Validation(fields map[string]string) *Error {
	return &Error{
		Status:  http.StatusBadRequest,
		Code:    "VALIDATION_FAILED",
		Message: "Periksa kembali isian form.",
		Details: map[string]any{"fields": fields},
	}
}

// Duplicate mengembalikan CONFLICT_DUPLICATE untuk nilai unik yang sudah dipakai.
func Duplicate(field, message string) *Error {
	return &Error{
		Status:  http.StatusConflict,
		Code:    "CONFLICT_DUPLICATE",
		Message: message,
		Details: map[string]any{"fields": map[string]string{field: message}},
	}
}

type ctxKey int

const requestIDKey ctxKey = iota

func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey, id)
}

func RequestID(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey).(string)
	return id
}

// Handler adalah handler yang mengembalikan error; lihat H.
type Handler func(w http.ResponseWriter, r *http.Request) error

// H mengubah Handler menjadi http.HandlerFunc dengan format error seragam.
func H(fn Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := fn(w, r); err != nil {
			WriteError(w, r, err)
		}
	}
}

func WriteError(w http.ResponseWriter, r *http.Request, err error) {
	var apiErr *Error
	if !errors.As(err, &apiErr) {
		slog.ErrorContext(r.Context(), "internal error",
			"request_id", RequestID(r.Context()), "path", r.URL.Path, "error", err.Error())
		apiErr = NewError(http.StatusInternalServerError, "INTERNAL_ERROR", "Terjadi kesalahan pada server.")
	}
	details := apiErr.Details
	if details == nil {
		details = map[string]any{}
	}
	writeJSON(w, apiErr.Status, map[string]any{"error": map[string]any{
		"code":       apiErr.Code,
		"message":    apiErr.Message,
		"details":    details,
		"request_id": RequestID(r.Context()),
	}})
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		slog.Error("encode response", "error", err.Error())
	}
}

// Data menulis { "data": ... }.
func Data(w http.ResponseWriter, status int, data any) error {
	writeJSON(w, status, map[string]any{"data": data})
	return nil
}

// DataMeta menulis { "data": ..., "meta": ... }.
func DataMeta(w http.ResponseWriter, data, meta any) error {
	writeJSON(w, http.StatusOK, map[string]any{"data": data, "meta": meta})
	return nil
}

func NoContent(w http.ResponseWriter) error {
	w.WriteHeader(http.StatusNoContent)
	return nil
}

// Decode membaca JSON ke dst dan menolak key tak dikenal. Field yang tidak dikirim
// membiarkan nilai dst apa adanya, sehingga dipakai juga untuk PATCH (ubah sebagian).
func Decode(r *http.Request, dst any) error {
	return DecodeFrom(r.Body, dst)
}

// DecodeFrom sama dengan Decode untuk sumber lain, misalnya bagian payload multipart.
func DecodeFrom(src io.Reader, dst any) error {
	dec := json.NewDecoder(src)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			return NewError(http.StatusRequestEntityTooLarge, "PAYLOAD_TOO_LARGE", "Ukuran data terlalu besar.")
		}
		return decodeError(err)
	}
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return NewError(http.StatusBadRequest, "VALIDATION_FAILED", "Body harus berisi satu objek JSON.")
	}
	return nil
}

func decodeError(err error) error {
	var typeErr *json.UnmarshalTypeError
	if errors.As(err, &typeErr) && typeErr.Field != "" {
		return Validation(map[string]string{typeErr.Field: "Tipe data tidak sesuai."})
	}
	if msg := err.Error(); strings.HasPrefix(msg, "json: unknown field ") {
		field := strings.Trim(strings.TrimPrefix(msg, "json: unknown field "), `"`)
		return Validation(map[string]string{field: "Field tidak dikenal."})
	}
	return NewError(http.StatusBadRequest, "VALIDATION_FAILED", "Body bukan JSON yang valid.")
}

// PathID membaca {id} sebagai UUID. ID tidak valid dijawab 404 agar sama dengan objek yang tidak ada.
func PathID(r *http.Request) (uuid.UUID, error) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		return uuid.Nil, ErrNotFound()
	}
	return id, nil
}

// Page adalah paginasi baku: page mulai 1, per_page default 20, maksimum 100.
type Page struct {
	Number  int
	PerPage int
}

func (p Page) Limit() int32  { return int32(p.PerPage) }
func (p Page) Offset() int32 { return int32((p.Number - 1) * p.PerPage) }

// Meta membentuk objek meta untuk respons daftar.
func (p Page) Meta(total int64) map[string]any {
	return map[string]any{"page": p.Number, "per_page": p.PerPage, "total": total}
}

func ReadPage(r *http.Request) (Page, error) {
	page := Page{Number: 1, PerPage: 20}
	fields := map[string]string{}
	if raw := r.URL.Query().Get("page"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > 1_000_000 {
			fields["page"] = "Halaman tidak valid."
		} else {
			page.Number = n
		}
	}
	if raw := r.URL.Query().Get("per_page"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > 100 {
			fields["per_page"] = "per_page harus 1 sampai 100."
		} else {
			page.PerPage = n
		}
	}
	if len(fields) > 0 {
		return page, Validation(fields)
	}
	return page, nil
}

// QueryUUID membaca query param UUID opsional.
func QueryUUID(r *http.Request, key string) (uuid.NullUUID, error) {
	raw := r.URL.Query().Get(key)
	if raw == "" {
		return uuid.NullUUID{}, nil
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		return uuid.NullUUID{}, Validation(map[string]string{key: "ID tidak valid."})
	}
	return uuid.NullUUID{UUID: id, Valid: true}, nil
}
