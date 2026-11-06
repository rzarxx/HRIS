//go:build dlib

package face

import (
	"context"
	"errors"
	"fmt"
	"runtime"

	goface "github.com/Kagami/go-face"
)

// Built menandai biner yang dibangun dengan mesin dlib.
const Built = true

type dlibEngine struct {
	rec *goface.Recognizer
	// slots membatasi pengenalan yang berjalan bersamaan, karena dlib memakai CPU penuh per foto.
	slots chan struct{}
}

// NewDlib memuat model dari modelsDir: shape_predictor_5_face_landmarks.dat dan
// dlib_face_recognition_resnet_model_v1.dat.
func NewDlib(modelsDir string) (Engine, error) {
	rec, err := goface.NewRecognizer(modelsDir)
	if err != nil {
		return nil, fmt.Errorf("memuat model wajah dari %s: %w", modelsDir, err)
	}
	n := max(runtime.NumCPU()/2, 1)
	return &dlibEngine{rec: rec, slots: make(chan struct{}, n)}, nil
}

func (e *dlibEngine) Model() string { return ModelDlib }

func (e *dlibEngine) Embed(ctx context.Context, jpeg []byte) ([]float64, error) {
	if !IsJPEG(jpeg) {
		return nil, ErrBadImage
	}
	select {
	case e.slots <- struct{}{}:
		defer func() { <-e.slots }()
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	faces, err := e.rec.Recognize(jpeg)
	if err != nil {
		var load goface.ImageLoadError
		if errors.As(err, &load) {
			return nil, ErrBadImage
		}
		return nil, err
	}
	switch len(faces) {
	case 0:
		return nil, ErrNoFace
	case 1:
	default:
		return nil, ErrManyFaces
	}
	out := make([]float64, len(faces[0].Descriptor))
	for i, x := range faces[0].Descriptor {
		out[i] = float64(x)
	}
	return out, nil
}
