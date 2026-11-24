package settings

import (
	"errors"
	"strings"
	"testing"

	"github.com/rzarxx/HRIS/backend/internal/platform/httpx"
)

func TestNormalizeAppVersion(t *testing.T) {
	got, err := normalizeAppVersion(AppVersionInput{
		LatestCode: 1400, LatestName: " 0.14.0 ", MinCode: 1300, StoreURL: DefaultStoreURL,
		Notes: "  Baris satu  \r\n\tBaris dua\x07 ",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.LatestName != "0.14.0" || got.StoreURL != "" || got.Notes != "Baris satu\nBaris dua" {
		t.Errorf("hasil = %+v", got)
	}

	bad := map[string]AppVersionInput{
		"min_code":    {LatestCode: 1400, LatestName: "0.14.0", MinCode: 1500},
		"latest_name": {LatestCode: 1400, LatestName: "v14"},
		"store_url":   {LatestCode: 1400, LatestName: "0.14.0", StoreURL: "http://contoh.id/app.apk"},
		"notes":       {LatestCode: 1400, LatestName: "0.14.0", Notes: strings.Repeat("a", 501)},
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
	// Belum diatur: semua nol tetap sah.
	if _, err := normalizeAppVersion(AppVersionInput{}); err != nil {
		t.Errorf("kosong = %v", err)
	}
}
