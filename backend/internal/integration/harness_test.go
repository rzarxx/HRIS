// Package integration menguji API lengkap terhadap PostgreSQL asli: router, middleware,
// service, query, dan hak akses role hris_app. Tes dilewati bila HRIS_TEST_DATABASE_URL kosong.
package integration

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rzarxx/HRIS/backend/internal/announcement"
	"github.com/rzarxx/HRIS/backend/internal/auth"
	"github.com/rzarxx/HRIS/backend/internal/dashboard"
	"github.com/rzarxx/HRIS/backend/internal/device"
	"github.com/rzarxx/HRIS/backend/internal/devsim"
	"github.com/rzarxx/HRIS/backend/internal/employee"
	"github.com/rzarxx/HRIS/backend/internal/location"
	"github.com/rzarxx/HRIS/backend/internal/org"
	"github.com/rzarxx/HRIS/backend/internal/platform/database"
	"github.com/rzarxx/HRIS/backend/internal/platform/fieldcrypt"
	"github.com/rzarxx/HRIS/backend/internal/report"
	"github.com/rzarxx/HRIS/backend/internal/server"
	"github.com/rzarxx/HRIS/backend/internal/shift"
	"github.com/rzarxx/HRIS/backend/internal/store"
)

const (
	seedPassword = "kata-sandi-seed-integrasi"
	appRole      = "hris_app"
	appPassword  = "hris-app-khusus-tes"
)

var (
	ownerPool *pgxpool.Pool // superuser: seed dan penyiapan data tes
	appPool   *pgxpool.Pool // role hris_app: dipakai API, sama seperti produksi
	testHash  string
)

func TestMain(m *testing.M) {
	dsn := os.Getenv("HRIS_TEST_DATABASE_URL")
	if dsn == "" {
		os.Exit(m.Run()) // setiap tes memanggil t.Skip
	}
	if err := setup(dsn); err != nil {
		fmt.Fprintln(os.Stderr, "penyiapan database tes gagal:", err)
		os.Exit(1)
	}
	code := m.Run()
	ownerPool.Close()
	appPool.Close()
	os.Exit(code)
}

// setup mengosongkan database lalu menjalankan migrasi. Karena merusak isi database,
// nama database wajib mengandung kata "test".
func setup(dsn string) error {
	ctx := context.Background()
	var err error
	if ownerPool, err = database.Open(ctx, dsn); err != nil {
		return err
	}
	var name string
	if err := ownerPool.QueryRow(ctx, "SELECT current_database()").Scan(&name); err != nil {
		return err
	}
	if !strings.Contains(name, "test") {
		return fmt.Errorf("menolak mengosongkan database %q: nama harus mengandung \"test\"", name)
	}
	// Hak akses mengikuti backend/scripts/dev-db-setup.sql, termasuk urutannya: default
	// privileges berlaku lebih dulu, lalu migrasi mencabut UPDATE/DELETE pada audit_logs.
	statements := []string{
		"DROP SCHEMA public CASCADE",
		"CREATE SCHEMA public",
		fmt.Sprintf(`DO $$ BEGIN
			IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = '%[1]s') THEN
				CREATE ROLE %[1]s LOGIN PASSWORD '%[2]s';
			END IF;
		END $$`, appRole, appPassword),
		"GRANT USAGE ON SCHEMA public TO " + appRole,
		"ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO " + appRole,
		"ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT USAGE, SELECT ON SEQUENCES TO " + appRole,
	}
	for _, stmt := range statements {
		if _, err := ownerPool.Exec(ctx, stmt); err != nil {
			return fmt.Errorf("%s: %w", stmt, err)
		}
	}

	mig, err := migrate.New("file://../../migrations", strings.Replace(dsn, "postgres://", "pgx5://", 1))
	if err != nil {
		return err
	}
	if err := mig.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("migrasi: %w", err)
	}
	if srcErr, dbErr := mig.Close(); srcErr != nil || dbErr != nil {
		return errors.Join(srcErr, dbErr)
	}

	cfg := ownerPool.Config().ConnConfig
	appDSN := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable", appRole, appPassword, cfg.Host, cfg.Port, cfg.Database)
	if appPool, err = database.Open(ctx, appDSN); err != nil {
		return err
	}
	testHash, err = auth.HashPassword(seedPassword)
	return err
}

// noLimiter mematikan rate limit agar tes bisa menjangkau logika di belakangnya.
type noLimiter struct{}

func (noLimiter) Allow(string, int, time.Duration, time.Time) (bool, time.Duration) { return true, 0 }
func (noLimiter) Reset(string)                                                      {}

type options struct {
	requireMFA bool
	limiter    auth.Limiter
}

// fresh mengisi ulang data dummy dan menyalakan API baru.
func fresh(t *testing.T, opt options) *httptest.Server {
	t.Helper()
	if os.Getenv("HRIS_TEST_DATABASE_URL") == "" {
		t.Skip("HRIS_TEST_DATABASE_URL kosong")
	}
	ctx := context.Background()
	if err := devsim.Reset(ctx, ownerPool); err != nil {
		t.Fatalf("reset: %v", err)
	}
	if err := devsim.Seed(ctx, ownerPool, seedPassword, time.Now()); err != nil {
		t.Fatalf("seed: %v", err)
	}

	_, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}
	cipher, err := fieldcrypt.New(key)
	if err != nil {
		t.Fatal(err)
	}
	limiter := opt.limiter
	if limiter == nil {
		limiter = noLimiter{}
	}
	signer := auth.NewTokenSigner(private, "test")
	authSvc, err := auth.NewService(appPool, signer, cipher, opt.requireMFA)
	if err != nil {
		t.Fatal(err)
	}
	q := store.New(appPool)
	srv := httptest.NewServer(server.New(server.Deps{
		Auth:         auth.NewHandler(authSvc, false, "", limiter),
		Org:          org.NewHandler(org.NewService(appPool)),
		Employee:     employee.NewHandler(employee.NewService(appPool, authSvc.ResolveScope)),
		Announcement: announcement.NewHandler(announcement.NewService(appPool)),
		Dashboard:    dashboard.NewHandler(q),
		Report:       report.NewHandler(report.NewService(q, authSvc.ResolveScope)),
		Location:     location.NewHandler(location.NewService(appPool)),
		Device:       device.NewHandler(device.NewService(appPool)),
		Shift:        shift.NewHandler(shift.NewService(appPool)),
		Signer:       signer,
		SessionCheck: authSvc.SessionState,
		Limiter:      limiter,
		Ready:        appPool.Ping,
		Options:      server.Options{},
	}))
	t.Cleanup(srv.Close)
	return srv
}

// client menyimpan cookie seperti browser dan mengirim X-CSRF-Token pada non-GET.
type client struct {
	t       *testing.T
	srv     *httptest.Server
	cookies map[string]string
}

func newClient(t *testing.T, srv *httptest.Server) *client {
	return &client{t: t, srv: srv, cookies: map[string]string{}}
}

// clone menyalin cookie, mis. untuk memakai refresh token lama.
func (c *client) clone() *client {
	copied := newClient(c.t, c.srv)
	for k, v := range c.cookies {
		copied.cookies[k] = v
	}
	return copied
}

type response struct {
	status int
	header http.Header
	body   []byte
}

func (c *client) do(method, path string, body any) response {
	c.t.Helper()
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			c.t.Fatal(err)
		}
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, c.srv.URL+path, reader)
	if err != nil {
		c.t.Fatal(err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if method != http.MethodGet {
		req.Header.Set(auth.HeaderCSRF, c.cookies[auth.CookieCSRF])
	}
	for name, value := range c.cookies {
		req.AddCookie(&http.Cookie{Name: name, Value: value})
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		c.t.Fatal(err)
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(res.Body)
	if err != nil {
		c.t.Fatal(err)
	}
	for _, cookie := range res.Cookies() {
		if cookie.MaxAge < 0 {
			delete(c.cookies, cookie.Name)
		} else {
			c.cookies[cookie.Name] = cookie.Value
		}
	}
	return response{status: res.StatusCode, header: res.Header, body: raw}
}

// want memeriksa status; bila gagal, isi respons ikut ditampilkan.
func (r response) want(t *testing.T, status int) response {
	t.Helper()
	if r.status != status {
		t.Fatalf("status %d, ingin %d: %s", r.status, status, r.body)
	}
	return r
}

// wantError memeriksa status dan kode error dari registry.
func (r response) wantError(t *testing.T, status int, code string) response {
	t.Helper()
	r.want(t, status)
	if got := r.errorField("code"); got != code {
		t.Fatalf("kode error %q, ingin %q: %s", got, code, r.body)
	}
	return r
}

func (r response) errorField(name string) any {
	var parsed struct {
		Error map[string]any `json:"error"`
	}
	if err := json.Unmarshal(r.body, &parsed); err != nil {
		return nil
	}
	return parsed.Error[name]
}

// fields mengambil pesan validasi per field dari error.details.fields.
func (r response) fields() map[string]any {
	details, _ := r.errorField("details").(map[string]any)
	fields, _ := details["fields"].(map[string]any)
	return fields
}

// data menguraikan { "data": ... } ke tipe T.
func data[T any](t *testing.T, r response) T {
	t.Helper()
	var parsed struct {
		Data T `json:"data"`
	}
	if err := json.Unmarshal(r.body, &parsed); err != nil {
		t.Fatalf("respons tidak bisa diurai: %v: %s", err, r.body)
	}
	return parsed.Data
}

const sdmEmail = devsim.SeedSDMEmail

func login(t *testing.T, srv *httptest.Server, email, password string) *client {
	t.Helper()
	c := newClient(t, srv)
	c.do(http.MethodPost, "/v1/auth/web/login", map[string]any{"email": email, "password": password}).want(t, http.StatusOK)
	return c
}

func loginSDM(t *testing.T, srv *httptest.Server) *client {
	t.Helper()
	return login(t, srv, sdmEmail, seedPassword)
}

// account adalah akun hasil seed yang siap dipakai masuk.
type account struct {
	email       string
	userID      string
	employeeID  string
	subdivision string
	division    string
}

// activeAccount mengaktifkan satu akun seed dengan role dan subdiv tertentu.
func activeAccount(t *testing.T, role, subdivisionCode string) account {
	t.Helper()
	ctx := context.Background()
	var a account
	err := ownerPool.QueryRow(ctx, `
		SELECT u.email, u.id::text, e.id::text, s.id::text, s.division_id::text
		FROM users u
		JOIN employees e ON e.id = u.employee_id
		JOIN subdivisions s ON s.id = e.subdivision_id
		WHERE u.role = $1 AND s.code = $2 AND e.status = 'active'
		ORDER BY e.nip LIMIT 1`, role, subdivisionCode).
		Scan(&a.email, &a.userID, &a.employeeID, &a.subdivision, &a.division)
	if err != nil {
		t.Fatalf("akun %s di %s: %v", role, subdivisionCode, err)
	}
	if _, err := ownerPool.Exec(ctx, `UPDATE users SET is_active = true, password_hash = $2 WHERE id = $1`, a.userID, testHash); err != nil {
		t.Fatal(err)
	}
	return a
}

// exec menjalankan SQL penyiapan data sebagai superuser.
func exec(t *testing.T, sql string, args ...any) {
	t.Helper()
	if _, err := ownerPool.Exec(context.Background(), sql, args...); err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
}

func countAudit(t *testing.T, action string) int {
	t.Helper()
	var n int
	if err := ownerPool.QueryRow(context.Background(), `SELECT count(*) FROM audit_logs WHERE action = $1`, action).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}
