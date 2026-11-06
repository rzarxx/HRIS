//go:build !dlib

package face

import "errors"

// Built menandai biner yang dibangun dengan mesin dlib.
const Built = false

// NewDlib tidak tersedia tanpa tag build `dlib`.
func NewDlib(string) (Engine, error) {
	return nil, errors.New("API dibangun tanpa mesin wajah; build ulang dengan -tags dlib dan CGO_ENABLED=1")
}
