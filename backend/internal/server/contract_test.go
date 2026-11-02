package server

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/rzarxx/HRIS/backend/internal/permission"
)

// Tes ini menjaga Aturan Emas no. 3 dan 4 (MASTER.md §0): kontrak, kode, dan dokumen
// harus memakai key yang sama. Bila salah satu berubah sendirian, tes gagal.

type openAPIDoc struct {
	Paths      map[string]map[string]yaml.Node `yaml:"paths"`
	Components struct {
		Schemas map[string]struct {
			Enum []string `yaml:"enum"`
		} `yaml:"schemas"`
	} `yaml:"components"`
}

func loadOpenAPI(t *testing.T) openAPIDoc {
	t.Helper()
	raw, err := os.ReadFile("../../api/openapi.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var doc openAPIDoc
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	return doc
}

func loadMaster(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile("../../../MASTER.md")
	if err != nil {
		t.Fatal(err)
	}
	return strings.ReplaceAll(string(raw), "\r\n", "\n")
}

// section mengambil isi dari heading sampai heading berikutnya.
func section(t *testing.T, doc, heading string) string {
	t.Helper()
	_, rest, ok := strings.Cut(doc, "\n"+heading+"\n")
	if !ok {
		t.Fatalf("heading %q tidak ada", heading)
	}
	if i := strings.Index(rest, "\n### "); i >= 0 {
		rest = rest[:i]
	}
	return rest
}

func TestRoutesMatchOpenAPI(t *testing.T) {
	doc := loadOpenAPI(t)
	inSpec := map[string]bool{}
	for path, ops := range doc.Paths {
		if !strings.HasPrefix(path, "/v1/") {
			continue
		}
		for method := range ops {
			switch method {
			case "get", "post", "put", "patch", "delete":
				inSpec[strings.ToUpper(method)+" "+path] = true
			}
		}
	}
	inCode := map[string]bool{}
	for _, r := range routes(Deps{}) {
		inCode[r.Method+" "+r.Path] = true
	}
	for key := range inCode {
		if !inSpec[key] {
			t.Errorf("route ada di kode tetapi tidak di openapi.yaml: %s", key)
		}
	}
	for key := range inSpec {
		if !inCode[key] {
			t.Errorf("route ada di openapi.yaml tetapi tidak di kode: %s", key)
		}
	}
}

var errorCodeInCode = regexp.MustCompile(`"([A-Z][A-Z0-9]*(?:_[A-Z0-9]+)+|NOT_FOUND)"`)

func goSources(t *testing.T) map[string]string {
	t.Helper()
	sources := map[string]string{}
	err := filepath.WalkDir("..", func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && d.Name() == "store" {
			return filepath.SkipDir
		}
		if !d.IsDir() && strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, "_test.go") {
			raw, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			sources[path] = string(raw)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return sources
}

func TestErrorCodesAreRegistered(t *testing.T) {
	doc := loadOpenAPI(t)
	enum := doc.Components.Schemas["ErrorCode"].Enum
	if len(enum) == 0 {
		t.Fatal("enum ErrorCode kosong")
	}
	registry := section(t, loadMaster(t), "### 7.4 Registry Kode Error")
	for _, code := range enum {
		if !strings.Contains(registry, "`"+code+"`") {
			t.Errorf("kode %s ada di openapi.yaml tetapi tidak di MASTER.md §7.4", code)
		}
	}
	for path, src := range goSources(t) {
		// Hanya baris yang membentuk error, agar konstanta lain tidak ikut terbaca.
		for _, line := range strings.Split(src, "\n") {
			if !strings.Contains(line, "NewError(") && !strings.Contains(line, "Code:") {
				continue
			}
			for _, m := range errorCodeInCode.FindAllStringSubmatch(line, -1) {
				if !slices.Contains(enum, m[1]) {
					t.Errorf("%s memakai kode %s yang tidak terdaftar", path, m[1])
				}
			}
		}
	}
}

var auditEvent = regexp.MustCompile(`=\s*"([a-z_]+\.[a-z_]+)"`)

func TestAuditEventsAreRegistered(t *testing.T) {
	registry := section(t, loadMaster(t), "### 5.11 Registry Event Audit")
	raw, err := os.ReadFile("../audit/audit.go")
	if err != nil {
		t.Fatal(err)
	}
	matches := auditEvent.FindAllStringSubmatch(string(raw), -1)
	if len(matches) == 0 {
		t.Fatal("tidak ada event audit terbaca")
	}
	for _, m := range matches {
		if !strings.Contains(registry, "`"+m[1]+"`") {
			t.Errorf("event %s tidak ada di MASTER.md §5.11", m[1])
		}
	}
}

func TestPermissionsMatchOpenAPI(t *testing.T) {
	doc := loadOpenAPI(t)
	enum := doc.Components.Schemas["Permission"].Enum
	all := map[string]bool{}
	for _, role := range allRoles {
		for _, key := range permission.Keys(role) {
			all[key] = true
		}
	}
	got := make([]string, 0, len(all))
	for key := range all {
		got = append(got, key)
	}
	sort.Strings(got)
	want := slices.Clone(enum)
	sort.Strings(want)
	if !slices.Equal(got, want) {
		t.Errorf("permission kode %v\nbeda dengan openapi.yaml %v", got, want)
	}
}

// Setiap permission yang dipakai route harus tercantum di matriks MASTER.md §3.2.
func TestRoutePermissionsAreInMaster(t *testing.T) {
	matrix := section(t, loadMaster(t), "### 3.2 Matriks Permission")
	for _, r := range routes(Deps{}) {
		for _, key := range r.Permissions {
			if !strings.Contains(matrix, "`"+key+"`") {
				t.Errorf("%s %s memakai %s yang tidak ada di §3.2", r.Method, r.Path, key)
			}
		}
	}
}
