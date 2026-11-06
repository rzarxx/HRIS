// Package face menghitung embedding wajah dari selfie di server (D42). HP hanya mengirim foto,
// sehingga angka wajah tidak bisa dipalsukan dari aplikasi yang dibongkar.
//
// Mesin sungguhan memakai dlib lewat go-face dan hanya dibangun dengan tag `dlib`, karena butuh cgo
// dan pustaka C di server. Tanpa tag itu, NewDlib mengembalikan error dan pemanggil memakai Engine
// lain, misalnya pengganti di tes.
package face

import (
	"bytes"
	"context"
	"errors"
	"math"
)

// ModelDlib adalah versi model yang dicatat di face_templates.model_version.
const ModelDlib = "dlib-resnet-v1"

var (
	// ErrNoFace berarti tidak ada wajah yang terdeteksi.
	ErrNoFace = errors.New("wajah tidak terdeteksi")
	// ErrManyFaces berarti ada lebih dari satu wajah di foto.
	ErrManyFaces = errors.New("lebih dari satu wajah")
	// ErrBadImage berarti berkas bukan JPEG yang bisa dibaca.
	ErrBadImage = errors.New("gambar tidak valid")
)

// Engine menghitung embedding dari satu foto JPEG yang berisi tepat satu wajah.
type Engine interface {
	// Model adalah nama versi model. Embedding dari model berbeda tidak bisa dibandingkan.
	Model() string
	Embed(ctx context.Context, jpeg []byte) ([]float64, error)
}

// IsJPEG memeriksa tanda awal berkas JPEG.
func IsJPEG(data []byte) bool {
	return bytes.HasPrefix(data, []byte{0xFF, 0xD8, 0xFF})
}

// Distance adalah jarak Euclid dua embedding. Hasilnya +Inf bila panjangnya berbeda.
func Distance(a, b []float64) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return math.Inf(1)
	}
	var sum float64
	for i := range a {
		d := a[i] - b[i]
		sum += d * d
	}
	return math.Sqrt(sum)
}

// Mean adalah rata-rata beberapa embedding berdimensi sama.
func Mean(vectors [][]float64) []float64 {
	if len(vectors) == 0 {
		return nil
	}
	out := make([]float64, len(vectors[0]))
	for _, v := range vectors {
		for i, x := range v {
			out[i] += x
		}
	}
	for i := range out {
		out[i] /= float64(len(vectors))
	}
	return out
}
