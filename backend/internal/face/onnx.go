//go:build onnx

package face

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"

	ort "github.com/yalue/onnxruntime_go"
)

// Built menandai biner yang dibangun dengan ONNX Runtime.
const Built = true

// Berkas model di folder FACE_MODELS_DIR (D43).
const (
	YuNetFile = "yunet.onnx"
	SFaceFile = "sface.onnx"
)

var envOnce sync.Once
var envErr error

type sfaceEngine struct {
	yunet, sface *ort.DynamicAdvancedSession
	// slots membatasi pengenalan yang berjalan bersamaan agar CPU server tidak habis.
	slots chan struct{}
}

// NewSFace memuat ONNX Runtime dari libPath, lalu YuNet dan SFace dari modelsDir.
func NewSFace(modelsDir, libPath string) (Engine, error) {
	if _, err := os.Stat(libPath); err != nil {
		return nil, fmt.Errorf("pustaka ONNX Runtime %s: %w", libPath, err)
	}
	envOnce.Do(func() {
		ort.SetSharedLibraryPath(libPath)
		envErr = ort.InitializeEnvironment()
	})
	if envErr != nil {
		return nil, fmt.Errorf("memuat ONNX Runtime: %w", envErr)
	}
	opts, err := ort.NewSessionOptions()
	if err != nil {
		return nil, err
	}
	defer opts.Destroy()
	if err := opts.SetIntraOpNumThreads(2); err != nil {
		return nil, err
	}
	yunetIn, _, err := ort.GetInputOutputInfo(filepath.Join(modelsDir, YuNetFile))
	if err != nil || len(yunetIn) != 1 {
		return nil, fmt.Errorf("membaca %s: %v", YuNetFile, err)
	}
	yunet, err := ort.NewDynamicAdvancedSession(filepath.Join(modelsDir, YuNetFile), []string{yunetIn[0].Name}, YuNetOutputs, opts)
	if err != nil {
		return nil, fmt.Errorf("memuat %s: %w", YuNetFile, err)
	}
	inputs, outputs, err := ort.GetInputOutputInfo(filepath.Join(modelsDir, SFaceFile))
	if err != nil || len(inputs) != 1 || len(outputs) != 1 {
		yunet.Destroy()
		return nil, fmt.Errorf("membaca %s: %v", SFaceFile, err)
	}
	sface, err := ort.NewDynamicAdvancedSession(filepath.Join(modelsDir, SFaceFile), []string{inputs[0].Name}, []string{outputs[0].Name}, opts)
	if err != nil {
		yunet.Destroy()
		return nil, fmt.Errorf("memuat %s: %w", SFaceFile, err)
	}
	return &sfaceEngine{yunet: yunet, sface: sface, slots: make(chan struct{}, max(runtime.NumCPU()/2, 1))}, nil
}

func (e *sfaceEngine) Model() string { return ModelSFace }

func run(s *ort.DynamicAdvancedSession, input []float32, shape ort.Shape, outputs int) ([][]float32, error) {
	tensor, err := ort.NewTensor(shape, input)
	if err != nil {
		return nil, err
	}
	defer tensor.Destroy()
	values := make([]ort.Value, outputs)
	if err := s.Run([]ort.Value{tensor}, values); err != nil {
		return nil, err
	}
	out := make([][]float32, outputs)
	for i, v := range values {
		t, ok := v.(*ort.Tensor[float32])
		if !ok {
			v.Destroy()
			return nil, errors.New("keluaran model bukan float32")
		}
		out[i] = append([]float32(nil), t.GetData()...)
		v.Destroy()
	}
	return out, nil
}

// Detect mengembalikan wajah yang ditemukan YuNet, dalam koordinat gambar asli.
func (e *sfaceEngine) Detect(img bgrImage) ([]Detection, error) {
	small, scale := detectorInput(img)
	blob, pw, ph := yunetBlob(small)
	outputs, err := run(e.yunet, blob, ort.NewShape(1, 3, int64(ph), int64(pw)), len(YuNetOutputs))
	if err != nil {
		return nil, err
	}
	faces := decodeYuNet(outputs, pw, ph)
	for i := range faces {
		faces[i] = scaleDetection(faces[i], scale)
	}
	return faces, nil
}

func (e *sfaceEngine) Embed(ctx context.Context, jpeg []byte) ([]float64, error) {
	if !IsJPEG(jpeg) {
		return nil, ErrBadImage
	}
	select {
	case e.slots <- struct{}{}:
		defer func() { <-e.slots }()
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	img, err := decodeJPEG(jpeg)
	if err != nil {
		return nil, err
	}
	faces, err := e.Detect(img)
	if err != nil {
		return nil, err
	}
	face, err := pickFace(faces)
	if err != nil {
		return nil, err
	}
	return e.embedFace(img, face)
}

// embedFace menghitung embedding satu wajah yang sudah terdeteksi.
func (e *sfaceEngine) embedFace(img bgrImage, face Detection) ([]float64, error) {
	aligned := alignCrop(img, similarityTransform(face.Landmarks))
	out, err := run(e.sface, aligned, ort.NewShape(1, 3, 112, 112), 1)
	if err != nil {
		return nil, err
	}
	v := make([]float64, len(out[0]))
	for i, x := range out[0] {
		v[i] = float64(x)
	}
	if n := Normalize(v); n != nil {
		return n, nil
	}
	return nil, ErrNoFace
}
