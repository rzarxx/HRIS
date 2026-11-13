// Command calibrate mengevaluasi sebaran skor kemiripan wajah (SFace), rasio toleh kepala (D50),
// dan skor risiko presensi dari dataset uji maupun data lapangan di database untuk menghasilkan
// rekomendasi ambang konfigurasi (.env) sesuai MASTER.md §5.4 dan §13.3.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rzarxx/HRIS/backend/internal/face"
	"github.com/rzarxx/HRIS/backend/internal/platform/config"
	"github.com/rzarxx/HRIS/backend/internal/platform/database"
)

// Dataset adalah masukan kalibrasi dari berkas JSON atau hasil pembacaan direktori foto/database.
type Dataset struct {
	GenuineScores  []float64 `json:"genuine_scores"`
	ImpostorScores []float64 `json:"impostor_scores"`
	StraightTurns  []float64 `json:"straight_turns"`
	GestureTurns   []float64 `json:"gesture_turns"`
	CleanRisks     []int     `json:"clean_risks"`
	SuspiciousRisk []int     `json:"suspicious_risks"`
	AttackRisks    []int     `json:"attack_risks"`
}

// SummaryStats merangkum sebaran nilai numerik.
type SummaryStats struct {
	Count int     `json:"count"`
	Min   float64 `json:"min"`
	Mean  float64 `json:"mean"`
	P05   float64 `json:"p05"`
	P95   float64 `json:"p95"`
	Max   float64 `json:"max"`
}

// CalibrationReport memuat metrik evaluasi dan rekomendasi ambang .env.
type CalibrationReport struct {
	GeneratedAt             string       `json:"generated_at"`
	Genuine                 SummaryStats `json:"genuine"`
	Impostor                SummaryStats `json:"impostor"`
	StraightTurn            SummaryStats `json:"straight_turn"`
	GestureTurn             SummaryStats `json:"gesture_turn"`
	TARAtFARZero            float64      `json:"tar_at_far_zero"`
	EqualErrorRate          float64      `json:"equal_error_rate"`
	SeparabilityDPrime      float64      `json:"separability_dprime"`
	FalseAcceptAtReview     int          `json:"false_accept_at_review"`
	FalseRejectAtReview     int          `json:"false_reject_at_review"`
	BorderlineGenuine       int          `json:"borderline_genuine"`
	FaceThresholdPass       float64      `json:"face_threshold_pass"`
	FaceThresholdReview     float64      `json:"face_threshold_review"`
	FaceEnrollMinSimilarity float64      `json:"face_enroll_min_similarity"`
	FaceStraightTurnMax     float64      `json:"face_straight_turn_max"`
	FaceGestureTurnMin      float64      `json:"face_gesture_turn_min"`
	FaceGestureTurnDelta    float64      `json:"face_gesture_turn_delta"`
	FaceGestureMinSim       float64      `json:"face_gesture_min_similarity"`
	RiskReviewMin           int          `json:"risk_review_min"`
	RiskRejectMin           int          `json:"risk_reject_min"`
}

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "hris-calibrate: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string, out io.Writer) error {
	fs := flag.NewFlagSet("hris-calibrate", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	imagesDir := fs.String("images", "", "Direktori foto .jpg (nama berkas seperti nama1.jpg, nama2.jpg)")
	datasetPath := fs.String("dataset", "", "Berkas JSON dataset skor untuk simulasi/kalibrasi")
	fromDB := fs.Bool("from-db", false, "Ambil ringkasan skor wajah dan risiko dari database")
	outPath := fs.String("out", "", "Simpan laporan JSON ke berkas")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := config.LoadDotEnv(".env"); err != nil {
		return err
	}

	var ds Dataset
	if *datasetPath != "" {
		raw, err := os.ReadFile(*datasetPath)
		if err != nil {
			return err
		}
		if err := json.Unmarshal(raw, &ds); err != nil {
			return fmt.Errorf("membaca dataset JSON: %w", err)
		}
	}
	if *imagesDir != "" {
		modelsDir := os.Getenv("FACE_MODELS_DIR")
		onnxLib := os.Getenv("ONNXRUNTIME_LIB")
		if modelsDir == "" || onnxLib == "" {
			return errors.New("FACE_MODELS_DIR dan ONNXRUNTIME_LIB wajib diisi saat memakai -images")
		}
		engine, err := face.NewSFace(modelsDir, onnxLib)
		if err != nil {
			return err
		}
		imgDS, err := analyzeDirectory(context.Background(), engine, *imagesDir)
		if err != nil {
			return err
		}
		ds.GenuineScores = append(ds.GenuineScores, imgDS.GenuineScores...)
		ds.ImpostorScores = append(ds.ImpostorScores, imgDS.ImpostorScores...)
		ds.StraightTurns = append(ds.StraightTurns, imgDS.StraightTurns...)
	}
	if *fromDB {
		dbURL := os.Getenv("DATABASE_URL")
		if dbURL == "" {
			return errors.New("DATABASE_URL wajib diisi saat memakai -from-db")
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		pool, err := database.Open(ctx, dbURL)
		if err != nil {
			return err
		}
		defer pool.Close()
		dbDS, err := loadFromDatabase(ctx, pool)
		if err != nil {
			return err
		}
		ds.GenuineScores = append(ds.GenuineScores, dbDS.GenuineScores...)
		ds.ImpostorScores = append(ds.ImpostorScores, dbDS.ImpostorScores...)
		ds.CleanRisks = append(ds.CleanRisks, dbDS.CleanRisks...)
		ds.SuspiciousRisk = append(ds.SuspiciousRisk, dbDS.SuspiciousRisk...)
		ds.AttackRisks = append(ds.AttackRisks, dbDS.AttackRisks...)
	}

	report, err := Calibrate(ds)
	if err != nil {
		return err
	}
	if *outPath != "" {
		raw, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			return err
		}
		if err := os.WriteFile(*outPath, append(raw, '\n'), 0o600); err != nil {
			return err
		}
	}
	printReport(out, report)
	return nil
}

// Calibrate menghitung statistik sebaran dan merekomendasikan ambang yang menjaga FAR = 0
// pada pasangan impostor sekaligus meminimalkan penolakan Pejuang asli.
func Calibrate(ds Dataset) (CalibrationReport, error) {
	if len(ds.GenuineScores) == 0 || len(ds.ImpostorScores) == 0 {
		return CalibrationReport{}, errors.New("genuine_scores dan impostor_scores tidak boleh kosong")
	}
	genStats := summarize(ds.GenuineScores)
	impStats := summarize(ds.ImpostorScores)
	straightStats := summarize(ds.StraightTurns)
	gestureStats := summarize(ds.GestureTurns)

	// Ambang review dipasang sedikit di atas impostor tertinggi (dibulatkan ke atas 0.01, minimal 0.44, maksimal 0.65)
	// agar FAR = 0 pada sampel uji. Nilai awal standar adalah 0.46 (impostor tertinggi benchmark 0.448).
	review := math.Ceil((impStats.Max+0.01)*100) / 100
	if review < config.DefaultFaceThresholdReview {
		review = config.DefaultFaceThresholdReview
	}
	if review > 0.65 {
		review = 0.65
	}
	pass := math.Round((review+0.04)*100) / 100
	if pass < config.DefaultFaceThresholdPass {
		pass = config.DefaultFaceThresholdPass
	}
	if genStats.P05 > pass && genStats.P05 <= 0.60 {
		pass = math.Floor(genStats.P05*100) / 100
	}
	if pass <= review {
		pass = math.Round((review+0.02)*100) / 100
	}
	enrollMin := pass

	tarFAR0 := 0.0
	falseAccept, falseReject, borderline := 0, 0, 0
	for _, s := range ds.GenuineScores {
		if s > impStats.Max {
			tarFAR0++
		}
		if s < review {
			falseReject++
		} else if s < pass {
			borderline++
		}
	}
	tarFAR0 /= float64(len(ds.GenuineScores))
	for _, s := range ds.ImpostorScores {
		if s >= review {
			falseAccept++
		}
	}

	eer := computeEER(ds.GenuineScores, ds.ImpostorScores)
	dprime := computeDPrime(ds.GenuineScores, ds.ImpostorScores)

	straightMax := config.DefaultFaceStraightTurnMax
	if straightStats.Count > 0 {
		cand := math.Ceil((straightStats.P95+0.03)*100) / 100
		if cand >= 0.12 && cand <= 0.25 {
			straightMax = cand
		}
	}
	gestureMin := config.DefaultFaceGestureTurnMin
	if gestureStats.Count > 0 {
		cand := math.Floor((gestureStats.P05-0.01)*100) / 100
		if cand >= 0.14 && cand <= 0.28 {
			gestureMin = cand
		}
	}
	gestureDelta := config.DefaultFaceGestureTurnDelta
	if gestureMin-straightMax+0.08 > gestureDelta {
		cand := math.Round((gestureMin-straightMax+0.08)*100) / 100
		if cand >= 0.08 && cand <= 0.20 {
			gestureDelta = cand
		}
	}

	riskReview, riskReject := calibrateRisk(ds.CleanRisks, ds.SuspiciousRisk, ds.AttackRisks)

	return CalibrationReport{
		GeneratedAt:             time.Now().UTC().Format(time.RFC3339),
		Genuine:                 genStats,
		Impostor:                impStats,
		StraightTurn:            straightStats,
		GestureTurn:             gestureStats,
		TARAtFARZero:            math.Round(tarFAR0*10000) / 10000,
		EqualErrorRate:          math.Round(eer*10000) / 10000,
		SeparabilityDPrime:      math.Round(dprime*100) / 100,
		FalseAcceptAtReview:     falseAccept,
		FalseRejectAtReview:     falseReject,
		BorderlineGenuine:       borderline,
		FaceThresholdPass:       pass,
		FaceThresholdReview:     review,
		FaceEnrollMinSimilarity: enrollMin,
		FaceStraightTurnMax:     straightMax,
		FaceGestureTurnMin:      gestureMin,
		FaceGestureTurnDelta:    gestureDelta,
		FaceGestureMinSim:       config.DefaultFaceGestureMinSimilarity,
		RiskReviewMin:           riskReview,
		RiskRejectMin:           riskReject,
	}, nil
}

func calibrateRisk(clean, suspicious, attacks []int) (int, int) {
	reviewMin := config.DefaultRiskReviewMin
	rejectMin := config.DefaultRiskRejectMin
	if len(clean) > 0 {
		maxClean := 0
		for _, r := range clean {
			if r > maxClean {
				maxClean = r
			}
		}
		if maxClean >= reviewMin && maxClean < 45 {
			reviewMin = ((maxClean / 5) + 1) * 5
		}
	}
	if len(attacks) > 0 {
		minAttack := attacks[0]
		for _, r := range attacks[1:] {
			if r < minAttack {
				minAttack = r
			}
		}
		if minAttack > reviewMin && minAttack <= 80 {
			rejectMin = minAttack
		}
	}
	if len(suspicious) > 0 {
		for _, r := range suspicious {
			if r > 0 && r < reviewMin {
				reviewMin = r
			}
		}
	}
	if rejectMin <= reviewMin {
		rejectMin = reviewMin + 30
	}
	return reviewMin, rejectMin
}

func summarize(values []float64) SummaryStats {
	if len(values) == 0 {
		return SummaryStats{}
	}
	sorted := append([]float64(nil), values...)
	sort.Float64s(sorted)
	sum := 0.0
	for _, v := range sorted {
		sum += v
	}
	return SummaryStats{
		Count: len(sorted),
		Min:   round4(sorted[0]),
		Mean:  round4(sum / float64(len(sorted))),
		P05:   round4(quantile(sorted, 0.05)),
		P95:   round4(quantile(sorted, 0.95)),
		Max:   round4(sorted[len(sorted)-1]),
	}
}

func quantile(sorted []float64, q float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	if len(sorted) == 1 {
		return sorted[0]
	}
	pos := q * float64(len(sorted)-1)
	lo := int(math.Floor(pos))
	hi := int(math.Ceil(pos))
	if lo == hi {
		return sorted[lo]
	}
	weight := pos - float64(lo)
	return sorted[lo]*(1-weight) + sorted[hi]*weight
}

func computeEER(genuine, impostor []float64) float64 {
	candidates := make([]float64, 0, len(genuine)+len(impostor))
	candidates = append(candidates, genuine...)
	candidates = append(candidates, impostor...)
	sort.Float64s(candidates)

	bestGap := math.MaxFloat64
	bestEER := 0.0
	for _, t := range candidates {
		fa := 0
		for _, s := range impostor {
			if s >= t {
				fa++
			}
		}
		fr := 0
		for _, s := range genuine {
			if s < t {
				fr++
			}
		}
		far := float64(fa) / float64(len(impostor))
		frr := float64(fr) / float64(len(genuine))
		gap := math.Abs(far - frr)
		if gap < bestGap {
			bestGap = gap
			bestEER = (far + frr) / 2
		}
	}
	return bestEER
}

func computeDPrime(genuine, impostor []float64) float64 {
	gMean, gVar := meanVar(genuine)
	iMean, iVar := meanVar(impostor)
	denom := math.Sqrt((gVar + iVar) / 2)
	if denom == 0 {
		return 0
	}
	return (gMean - iMean) / denom
}

func meanVar(xs []float64) (float64, float64) {
	if len(xs) == 0 {
		return 0, 0
	}
	sum := 0.0
	for _, x := range xs {
		sum += x
	}
	mean := sum / float64(len(xs))
	varSum := 0.0
	for _, x := range xs {
		d := x - mean
		varSum += d * d
	}
	return mean, varSum / float64(len(xs))
}

func round4(v float64) float64 {
	return math.Round(v*10000) / 10000
}

func analyzeDirectory(ctx context.Context, engine face.Engine, dir string) (Dataset, error) {
	paths, err := filepath.Glob(filepath.Join(dir, "*.jpg"))
	if err != nil {
		return Dataset{}, err
	}
	sort.Strings(paths)
	if len(paths) < 2 {
		return Dataset{}, fmt.Errorf("butuh minimal 2 berkas .jpg di %s", dir)
	}
	type item struct {
		person string
		vector []float64
	}
	var items []item
	var ds Dataset
	for _, p := range paths {
		raw, err := os.ReadFile(p)
		if err != nil {
			return Dataset{}, err
		}
		analysis, err := engine.Analyze(ctx, raw)
		if err != nil {
			continue
		}
		base := strings.TrimSuffix(filepath.Base(p), filepath.Ext(p))
		person := strings.TrimRight(base, "0123456789_-")
		items = append(items, item{person: person, vector: analysis.Vector})
		ds.StraightTurns = append(ds.StraightTurns, math.Abs(analysis.Turn))
	}
	for i := range items {
		for j := i + 1; j < len(items); j++ {
			sim := face.Cosine(items[i].vector, items[j].vector)
			if items[i].person == items[j].person {
				ds.GenuineScores = append(ds.GenuineScores, sim)
			} else {
				ds.ImpostorScores = append(ds.ImpostorScores, sim)
			}
		}
	}
	return ds, nil
}

func loadFromDatabase(ctx context.Context, pool *pgxpool.Pool) (Dataset, error) {
	var ds Dataset
	rows, err := pool.Query(ctx, `
		SELECT fc.face_score::float8, COALESCE(aa.outcome, 'accepted'), COALESCE(aa.reason_code, 'ok'), COALESCE(aa.risk_score, 0)
		FROM face_captures fc
		LEFT JOIN attendance_attempts aa ON aa.challenge_id = fc.challenge_id
		WHERE fc.face_score IS NOT NULL
	`)
	if err != nil {
		return Dataset{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var score float64
		var outcome, reason string
		var risk int
		if err := rows.Scan(&score, &outcome, &reason, &risk); err != nil {
			return Dataset{}, err
		}
		if reason == "face_mismatch" {
			ds.ImpostorScores = append(ds.ImpostorScores, score)
		} else if outcome == "accepted" || outcome == "review" {
			ds.GenuineScores = append(ds.GenuineScores, score)
		}
		switch outcome {
		case "accepted":
			ds.CleanRisks = append(ds.CleanRisks, risk)
		case "review":
			ds.SuspiciousRisk = append(ds.SuspiciousRisk, risk)
		case "rejected":
			if risk > 0 {
				ds.AttackRisks = append(ds.AttackRisks, risk)
			}
		}
	}
	return ds, rows.Err()
}

func printReport(w io.Writer, r CalibrationReport) {
	fmt.Fprintf(w, "# Laporan Kalibrasi Ambang Presensi & Anti-Fraud (SFace)\n")
	fmt.Fprintf(w, "# Dibuat pada: %s\n", r.GeneratedAt)
	fmt.Fprintf(w, "# Pasangan orang sama (genuine): n=%d min=%.4f p05=%.4f mean=%.4f max=%.4f\n",
		r.Genuine.Count, r.Genuine.Min, r.Genuine.P05, r.Genuine.Mean, r.Genuine.Max)
	fmt.Fprintf(w, "# Pasangan orang beda (impostor): n=%d min=%.4f mean=%.4f p95=%.4f max=%.4f\n",
		r.Impostor.Count, r.Impostor.Min, r.Impostor.Mean, r.Impostor.P95, r.Impostor.Max)
	fmt.Fprintf(w, "# Metrik: TAR@FAR=0: %.2f%% | EER: %.2f%% | Separabilitas (d'): %.2f\n",
		r.TARAtFARZero*100, r.EqualErrorRate*100, r.SeparabilityDPrime)
	fmt.Fprintf(w, "# Evaluasi ambang: salah terima=%d | borderline=%d | salah tolak=%d\n\n",
		r.FalseAcceptAtReview, r.BorderlineGenuine, r.FalseRejectAtReview)
	fmt.Fprintf(w, "FACE_THRESHOLD_PASS=%.2f\n", r.FaceThresholdPass)
	fmt.Fprintf(w, "FACE_THRESHOLD_REVIEW=%.2f\n", r.FaceThresholdReview)
	fmt.Fprintf(w, "FACE_ENROLL_MIN_SIMILARITY=%.2f\n", r.FaceEnrollMinSimilarity)
	fmt.Fprintf(w, "FACE_STRAIGHT_TURN_MAX=%.2f\n", r.FaceStraightTurnMax)
	fmt.Fprintf(w, "FACE_GESTURE_TURN_MIN=%.2f\n", r.FaceGestureTurnMin)
	fmt.Fprintf(w, "FACE_GESTURE_TURN_DELTA=%.2f\n", r.FaceGestureTurnDelta)
	fmt.Fprintf(w, "FACE_GESTURE_MIN_SIMILARITY=%.2f\n", r.FaceGestureMinSim)
	fmt.Fprintf(w, "RISK_REVIEW_MIN=%d\n", r.RiskReviewMin)
	fmt.Fprintf(w, "RISK_REJECT_MIN=%d\n", r.RiskRejectMin)
}
