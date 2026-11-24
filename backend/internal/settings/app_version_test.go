package settings

import (
	"errors"
	"strings"
	"testing"

	"github.com/rzarxx/HRIS/backend/internal/platform/httpx"
)

func TestNormalizeAppVersion(t *testing.T) {
	got, err := normalizeAppVersion(AppVersionInput{
		LatestCode: 14000, LatestName: " 0.14.0 ", MinCode: 13000, StoreURL: DefaultStoreURL,
		Notes: "  Baris satu  \r\n\tBaris dua\x07 ",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.LatestName != "0.14.0" || got.StoreURL != "" || got.Notes != "Baris satu\nBaris dua" {
		t.Errorf("hasil = %+v", got)
	}

	bad := map[string]AppVersionInput{
		"min_code":    {LatestCode: 14000, LatestName: "0.14.0", MinCode: 15000},
		"latest_name": {LatestCode: 14000, LatestName: "v14"},
		"store_url":   {LatestCode: 14000, LatestName: "0.14.0", StoreURL: "http://contoh.id/app.apk"},
		"notes":       {LatestCode: 14000, LatestName: "0.14.0", Notes: strings.Repeat("a", 501)},
		"latest_code": {LatestCode: -1},
	}
	for field, in := range bad {
		_, err := normalizeAppVersion(in)
		var apiErr *httpx.Error
		if !errors.As(err, &apiErr) {
			t.Errorf("%s: galat = %v", field, err)
			continue
		}
		fields, _ := apiErr.Details["fields"].(map[string]string)
		if fields[field] == "" {
			t.Errorf("%s: field galat = %v", field, fields)
		}
	}
	// Kode harus cocok dengan rumus rilis (0.14.0 = 14000, bukan 1400).
	if _, err := normalizeAppVersion(AppVersionInput{LatestCode: 1400, LatestName: "0.14.0"}); err == nil {
		t.Error("kode yang tidak cocok dengan nama versi seharusnya ditolak")
	}
	for name, want := range map[string]int{"0.14.0": 14000, "1.2.3": 1002003, "0.13.1": 13001} {
		if got, ok := VersionCode(name); !ok || got != want {
			t.Errorf("VersionCode(%q) = %d, %v; ingin %d", name, got, ok, want)
		}
	}
	if _, ok := VersionCode("0.1000.0"); ok {
		t.Error("minor di atas 999 seharusnya ditolak")
	}
	// Belum diatur: semua nol tetap sah.
	if _, err := normalizeAppVersion(AppVersionInput{}); err != nil {
		t.Errorf("kosong = %v", err)
	}
}
