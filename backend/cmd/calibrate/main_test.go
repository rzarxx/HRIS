package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCalibrateMatchesSFaceBenchmarkD43(t *testing.T) {
	// Data representatif uji banding SFace (D43):
	// pasangan orang berbeda tertinggi 0.448; pasangan orang sama mayoritas >= 0.50 dengan satu borderline 0.472.
	ds := Dataset{
		GenuineScores:  []float64{0.472, 0.515, 0.542, 0.588, 0.612, 0.634, 0.655, 0.681, 0.702, 0.745},
		ImpostorScores: []float64{-0.08, 0.04, 0.11, 0.19, 0.24, 0.29, 0.33, 0.38, 0.412, 0.448},
		StraightTurns:  []float64{0.01, 0.03, 0.05, 0.08, 0.11, 0.14},
		GestureTurns:   []float64{0.22, 0.25, 0.29, 0.34, 0.38},
		CleanRisks:     []int{0, 0, 10, 15, 20},
		SuspiciousRisk: []int{30, 40, 45},
		AttackRisks:    []int{60, 70, 85},
	}

	report, err := Calibrate(ds)
	if err != nil {
		t.Fatalf("Calibrate gagal: %v", err)
	}
	if report.FalseAcceptAtReview != 0 {
		t.Fatalf("FAR pada ambang review harus 0, dapat %d", report.FalseAcceptAtReview)
	}
	if report.TARAtFARZero != 1.0 {
		t.Fatalf("TAR@FAR=0 harus 1.0, dapat %.4f", report.TARAtFARZero)
	}
	if report.FaceThresholdReview != 0.46 {
		t.Errorf("FaceThresholdReview = %.2f, ingin 0.46", report.FaceThresholdReview)
	}
	if report.FaceThresholdPass != 0.50 {
		t.Errorf("FaceThresholdPass = %.2f, ingin 0.50", report.FaceThresholdPass)
	}
	if report.BorderlineGenuine != 1 || report.FalseRejectAtReview != 0 {
		t.Errorf("borderline=%d falseReject=%d, ingin borderline=1 falseReject=0", report.BorderlineGenuine, report.FalseRejectAtReview)
	}
	if report.RiskReviewMin != 30 || report.RiskRejectMin != 60 {
		t.Errorf("ambang risiko = (%d, %d), ingin (30, 60)", report.RiskReviewMin, report.RiskRejectMin)
	}
}

func TestRunWithDatasetFileAndOutputReport(t *testing.T) {
	dir := t.TempDir()
	inFile := filepath.Join(dir, "dataset.json")
	outFile := filepath.Join(dir, "report.json")

	ds := Dataset{
		GenuineScores:  []float64{0.52, 0.58, 0.64, 0.71},
		ImpostorScores: []float64{0.10, 0.22, 0.35, 0.44},
	}
	raw, err := json.Marshal(ds)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(inFile, raw, 0o600); err != nil {
		t.Fatal(err)
	}

	var stdout bytes.Buffer
	if err := run([]string{"-dataset", inFile, "-out", outFile}, &stdout); err != nil {
		t.Fatalf("run gagal: %v", err)
	}
	outText := stdout.String()
	for _, key := range []string{
		"FACE_THRESHOLD_PASS=",
		"FACE_THRESHOLD_REVIEW=",
		"FACE_ENROLL_MIN_SIMILARITY=",
		"RISK_REVIEW_MIN=",
		"RISK_REJECT_MIN=",
	} {
		if !strings.Contains(outText, key) {
			t.Errorf("keluaran tidak memuat %s:\n%s", key, outText)
		}
	}

	saved, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("membaca laporan JSON: %v", err)
	}
	var report CalibrationReport
	if err := json.Unmarshal(saved, &report); err != nil {
		t.Fatalf("mengurai laporan JSON: %v", err)
	}
	if report.Genuine.Count != 4 || report.Impostor.Count != 4 {
		t.Errorf("jumlah pasangan di laporan salah: %+v", report)
	}
}
