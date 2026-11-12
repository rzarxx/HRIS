// Package face menghitung embedding wajah dari selfie di server (D42, D43). HP hanya mengirim foto,
// sehingga angka wajah tidak bisa dipalsukan dari aplikasi yang dibongkar.
//
// Mesinnya adalah detektor YuNet dan pengenal SFace dari OpenCV Zoo yang dijalankan ONNX Runtime.
// Bagian ONNX Runtime memakai cgo dan hanya dibangun dengan tag `onnx`; tanpa tag itu NewSFace
// mengembalikan error dan pemanggil memakai Engine lain, misalnya pengganti di tes.
package face

import (
	"bytes"
	"context"
	"errors"
	"math"
)

// ModelSFace adalah versi model yang dicatat di face_templates.model_version. Template dari model
// lain, termasuk dlib (D42), diperlakukan sebagai belum terdaftar sehingga Pejuang mendaftar ulang.
const ModelSFace = "sface-2021dec"

// Ambang kemiripan cosine untuk ModelSFace (D43), dipilih dari uji banding foto contoh: pasangan
// orang berbeda yang paling mirip bernilai 0.448. Semuanya nilai awal yang wajib dikalibrasi dengan
// selfie Pejuang dari HP; TestSFaceEngine memeriksanya terhadap semua pasangan foto contoh.
const (
	// MinSimilarityPass: sama atau lebih dari ini diterima tanpa catatan.
	MinSimilarityPass = 0.50
	// MinSimilarityReview: sampai nilai ini masih diterima dengan sinyal face_borderline; di bawahnya ditolak.
	MinSimilarityReview = 0.46
	// MinEnrollSimilarity: kemiripan terendah antarselfie saat pendaftaran.
	MinEnrollSimilarity = 0.50
)

var (
	// ErrNoFace berarti tidak ada wajah yang terdeteksi.
	ErrNoFace = errors.New("wajah tidak terdeteksi")
	// ErrManyFaces berarti ada lebih dari satu wajah yang cukup besar di foto.
	ErrManyFaces = errors.New("lebih dari satu wajah")
	// ErrFaceNotCentered berarti wajah utama terlalu kecil atau tidak di tengah foto.
	ErrFaceNotCentered = errors.New("wajah terlalu kecil atau tidak di tengah")
	// ErrBadImage berarti berkas bukan JPEG yang bisa dibaca.
	ErrBadImage = errors.New("gambar tidak valid")
)

// Engine menghitung embedding dari satu foto JPEG yang berisi tepat satu wajah.
type Engine interface {
	// Model adalah nama versi model. Embedding dari model berbeda tidak bisa dibandingkan.
	Model() string
	// Embed menghitung embedding wajah utama. Wajah yang terlalu kecil atau tidak di tengah ditolak
	// dengan ErrFaceNotCentered.
	Embed(ctx context.Context, jpeg []byte) ([]float64, error)
	// Analyze seperti Embed, ditambah pose kepala, tanpa menolak wajah yang tidak di tengah; pemanggil
	// memutuskan lewat Analysis.Centered. Dipakai untuk membuktikan gerakan liveness di server.
	Analyze(ctx context.Context, jpeg []byte) (Analysis, error)
}

// Analysis adalah hasil analisis wajah utama pada satu foto.
type Analysis struct {
	Vector []float64
	// Turn adalah rasio toleh kepala dari lima titik wajah (lihat TurnRatio): 0 berarti lurus.
	Turn float64
	// Centered berarti wajah cukup besar dan di tengah foto, syarat yang sama dengan Embed.
	Centered bool
}

// Ambang pose untuk liveness toleh kepala (D50). Nilai awal dari geometri wajah rata-rata: toleh
// sekitar 12 derajat memberi rasio ~0.13 dan toleh 25 derajat ~0.27. Wajib dikalibrasi dengan foto
// dari HP sungguhan, sama seperti ambang kemiripan.
const (
	// StraightTurnMax: foto lurus boleh menoleh sampai rasio ini.
	StraightTurnMax = 0.20
	// GestureTurnMin: foto gerakan harus menoleh paling sedikit rasio ini.
	GestureTurnMin = 0.18
	// GestureTurnDelta: selisih rasio foto gerakan dan foto lurus, agar dua foto lurus yang sedikit
	// miring tidak dianggap gerakan.
	GestureTurnDelta = 0.10
	// GestureMinSimilarity: kemiripan terendah wajah di foto gerakan dengan foto lurus. Wajah yang
	// menoleh kurang mirip dengan wajah lurus, jadi ambangnya lebih longgar dari MinSimilarityReview;
	// tujuannya menangkap foto gerakan milik orang lain, bukan mencocokkan identitas.
	GestureMinSimilarity = 0.30
)

// TurnRatio mengukur toleh kepala dari titik YuNet (mata kanan, mata kiri, hidung, ...): selisih jarak
// hidung ke kedua mata dibagi jumlahnya. Nilainya 0 saat lurus, makin besar saat menoleh, bertanda
// sesuai arah. Tidak bergantung pada ukuran wajah atau jarak ke kamera.
func TurnRatio(landmarks [5][2]float64) float64 {
	nose := landmarks[2]
	right := math.Hypot(nose[0]-landmarks[0][0], nose[1]-landmarks[0][1])
	left := math.Hypot(nose[0]-landmarks[1][0], nose[1]-landmarks[1][1])
	if right+left == 0 {
		return 0
	}
	return (right - left) / (right + left)
}

// IsJPEG memeriksa tanda awal berkas JPEG.
func IsJPEG(data []byte) bool {
	return bytes.HasPrefix(data, []byte{0xFF, 0xD8, 0xFF})
}

// Normalize mengembalikan vektor satuan, atau nil bila vektornya nol atau tidak valid.
func Normalize(v []float64) []float64 {
	var sum float64
	for _, x := range v {
		if math.IsNaN(x) || math.IsInf(x, 0) {
			return nil
		}
		sum += x * x
	}
	if sum == 0 {
		return nil
	}
	norm := math.Sqrt(sum)
	out := make([]float64, len(v))
	for i, x := range v {
		out[i] = x / norm
	}
	return out
}

// Cosine adalah kemiripan cosine dua embedding, -1 sampai 1. Hasilnya -1 bila dimensinya berbeda
// atau salah satunya tidak valid, agar tidak pernah dianggap cocok.
func Cosine(a, b []float64) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return -1
	}
	na, nb := Normalize(a), Normalize(b)
	if na == nil || nb == nil {
		return -1
	}
	var dot float64
	for i := range na {
		dot += na[i] * nb[i]
	}
	return dot
}

// Template menggabungkan beberapa embedding menjadi satu: rata-rata vektor satuan, lalu dinormalkan.
func Template(vectors [][]float64) []float64 {
	if len(vectors) == 0 {
		return nil
	}
	sum := make([]float64, len(vectors[0]))
	for _, v := range vectors {
		u := Normalize(v)
		if u == nil || len(u) != len(sum) {
			return nil
		}
		for i, x := range u {
			sum[i] += x
		}
	}
	return Normalize(sum)
}
