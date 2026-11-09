package server

import (
	"os"
	"regexp"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// Model JSON aplikasi Android ditulis tangan (mobile/.../data/Dto.kt). Tes ini memastikan setiap
// @SerialName pada kelas bertanda "// schema: X" adalah properti skema X di openapi.yaml, sehingga
// aplikasi tidak bisa mengirim atau membaca field yang tidak ada di kontrak.

type schemaNode struct {
	Ref        string                `yaml:"$ref"`
	Properties map[string]schemaNode `yaml:"properties"`
}

var (
	schemaMarker = regexp.MustCompile(`(?m)^// schema: ([A-Za-z0-9_.]+)\s*$`)
	serialName   = regexp.MustCompile(`@SerialName\("([a-z0-9_]+)"\)`)
)

func TestMobileDTOsMatchOpenAPI(t *testing.T) {
	raw, err := os.ReadFile("../../api/openapi.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Components struct {
			Schemas map[string]schemaNode `yaml:"schemas"`
		} `yaml:"components"`
	}
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	src, err := os.ReadFile("../../../mobile/app/src/main/java/id/pejuang/hris/data/Dto.kt")
	if err != nil {
		t.Fatal(err)
	}
	code := strings.ReplaceAll(string(src), "\r\n", "\n")

	markers := schemaMarker.FindAllStringSubmatchIndex(code, -1)
	if len(markers) < 10 {
		t.Fatalf("penanda schema terlalu sedikit: %d", len(markers))
	}
	for i, m := range markers {
		path := code[m[2]:m[3]]
		end := len(code)
		if i+1 < len(markers) {
			end = markers[i+1][0]
		}
		// Isi kelas sampai penanda berikutnya; kelas tanpa penanda ditaruh setelah penanda terakhir.
		block := code[m[1]:end]
		if j := strings.Index(block, "\n\n"); j >= 0 {
			block = block[:j]
		}
		node, ok := resolveSchema(doc.Components.Schemas, path)
		if !ok {
			t.Errorf("skema %s tidak ada di openapi.yaml", path)
			continue
		}
		names := serialName.FindAllStringSubmatch(block, -1)
		if len(names) == 0 {
			t.Errorf("kelas untuk skema %s tidak punya @SerialName", path)
		}
		for _, n := range names {
			if _, ok := node.Properties[n[1]]; !ok {
				t.Errorf("Dto.kt %s memakai field %q yang tidak ada di openapi.yaml", path, n[1])
			}
		}
	}
}

// resolveSchema menerima "Nama" atau "Nama.properti" untuk objek inline.
func resolveSchema(schemas map[string]schemaNode, path string) (schemaNode, bool) {
	parts := strings.Split(path, ".")
	node, ok := schemas[parts[0]]
	for _, p := range parts[1:] {
		if !ok {
			return node, false
		}
		node, ok = node.Properties[p]
	}
	for ok && node.Ref != "" {
		node, ok = schemas[node.Ref[strings.LastIndex(node.Ref, "/")+1:]]
	}
	return node, ok
}
