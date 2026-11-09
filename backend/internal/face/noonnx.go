//go:build !onnx

package face

import "errors"

// Built menandai biner yang dibangun dengan ONNX Runtime.
const Built = false

// NewSFace tidak tersedia tanpa tag build `onnx`.
func NewSFace(string, string) (Engine, error) {
	return nil, errors.New("API dibangun tanpa mesin wajah; build ulang dengan -tags onnx dan CGO_ENABLED=1")
}
