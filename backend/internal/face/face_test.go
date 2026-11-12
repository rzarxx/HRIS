package face

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"math"
	"testing"
)

func near(a, b, tol float64) bool { return math.Abs(a-b) <= tol }

func TestCosineAndTemplate(t *testing.T) {
	a := []float64{1, 0, 0}
	if c := Cosine(a, []float64{2, 0, 0}); !near(c, 1, 1e-12) {
		t.Errorf("searah = %v", c)
	}
	if c := Cosine(a, []float64{0, 3, 0}); !near(c, 0, 1e-12) {
		t.Errorf("tegak lurus = %v", c)
	}
	if Cosine(a, []float64{1, 0}) != -1 || Cosine(a, []float64{0, 0, 0}) != -1 || Cosine(nil, nil) != -1 {
		t.Error("vektor tidak sebanding harus -1")
	}
	tpl := Template([][]float64{{1, 0}, {0, 1}})
	if !near(tpl[0], math.Sqrt(0.5), 1e-12) || !near(tpl[1], math.Sqrt(0.5), 1e-12) {
		t.Errorf("template = %v", tpl)
	}
	if Template(nil) != nil || Template([][]float64{{1, 0}, {0, 0}}) != nil {
		t.Error("template tidak valid harus nil")
	}
	if MinSimilarityReview >= MinSimilarityPass {
		t.Error("ambang review harus di bawah ambang lolos")
	}
}

func TestIsJPEG(t *testing.T) {
	if !IsJPEG([]byte{0xFF, 0xD8, 0xFF, 0xE0}) || IsJPEG([]byte("\x89PNG")) || IsJPEG(nil) {
		t.Error("deteksi JPEG")
	}
}

// Transformasi kemiripan harus menemukan kembali rotasi, skala, dan geseran yang diketahui.
func TestSimilarityTransform(t *testing.T) {
	angle, scale, tx, ty := 0.3, 1.7, -12.0, 40.0
	// Titik sumber dibuat dari template dengan transformasi kebalikan, jadi hasilnya harus persis.
	inv := func(p [2]float64) [2]float64 {
		x, y := p[0]-tx, p[1]-ty
		c, s := math.Cos(-angle)/scale, math.Sin(-angle)/scale
		return [2]float64{c*x - s*y, s*x + c*y}
	}
	var src [5][2]float64
	for i, p := range arcfaceTemplate {
		src[i] = inv(p)
	}
	m := similarityTransform(src)
	for i, p := range src {
		x := m[0]*p[0] + m[1]*p[1] + m[2]
		y := m[3]*p[0] + m[4]*p[1] + m[5]
		if !near(x, arcfaceTemplate[i][0], 1e-6) || !near(y, arcfaceTemplate[i][1], 1e-6) {
			t.Errorf("titik %d ke (%v, %v), harus %v", i, x, y, arcfaceTemplate[i])
		}
	}
	if !near(math.Hypot(m[0], m[3]), scale, 1e-9) {
		t.Errorf("skala = %v", math.Hypot(m[0], m[3]))
	}
}

func gradient(w, h int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetRGBA(x, y, color.RGBA{R: uint8(x % 256), G: uint8(y % 256), B: 100, A: 255})
		}
	}
	return img
}

func TestAlignCropIdentity(t *testing.T) {
	img := imageFromRGBA(gradient(200, 200))
	// Geser 10 piksel: titik (x, y) di gambar tujuan diambil dari (x+10, y+20) di sumber.
	blob := alignCrop(img, [6]float64{1, 0, -10, 0, 1, -20})
	at := func(ch, x, y int) float32 { return blob[ch*112*112+y*112+x] }
	if at(0, 5, 7) != 15 || at(1, 5, 7) != 27 || at(2, 5, 7) != 100 {
		t.Errorf("RGB di (5,7) = %v %v %v", at(0, 5, 7), at(1, 5, 7), at(2, 5, 7))
	}
	// Di luar gambar sumber diisi hitam.
	edge := alignCrop(img, [6]float64{1, 0, 150, 0, 1, 0})
	if edge[0] != 0 {
		t.Errorf("tepi = %v", edge[0])
	}
}

func TestDetectorInputAndResize(t *testing.T) {
	small := imageFromRGBA(gradient(320, 240))
	if out, scale := detectorInput(small); scale != 1 || out.w != 320 {
		t.Errorf("gambar kecil tidak boleh diubah: %v %v", out.w, scale)
	}
	big := imageFromRGBA(gradient(1280, 960))
	out, scale := detectorInput(big)
	if scale != 0.5 || out.w != 640 || out.h != 480 {
		t.Fatalf("diperkecil ke %dx%d skala %v", out.w, out.h, scale)
	}
	// Setengah ukuran: piksel (10, 4) adalah rata-rata sumber x 20 dan 21, dibulatkan.
	if got := out.at(10, 4, 2); got != 20 && got != 21 { // kanal R = x sumber
		t.Errorf("R = %v", got)
	}
	d := scaleDetection(Detection{X: 10, W: 20, Landmarks: [5][2]float64{{5, 6}}}, 0.5)
	if d.X != 20 || d.W != 40 || d.Landmarks[0] != [2]float64{10, 12} {
		t.Errorf("skala balik = %+v", d)
	}
	blob, pw, ph := yunetBlob(out)
	if pw != YuNetSize || ph != YuNetSize || len(blob) != 3*YuNetSize*YuNetSize {
		t.Errorf("blob %dx%d", pw, ph)
	}
	// Kanal B bernilai tetap 100; baris di bawah tinggi gambar adalah pad hitam.
	if blob[0*pw*ph+0*pw+30] != 100 || blob[0*pw*ph+500*pw+30] != 0 {
		t.Error("isi blob atau pad salah")
	}
}

func TestDecodeYuNetAndPick(t *testing.T) {
	outputs := make([][]float32, 12)
	for i, stride := range yunetStrides {
		n := (YuNetSize / stride) * (YuNetSize / stride)
		outputs[i] = make([]float32, n)
		outputs[i+3] = make([]float32, n)
		outputs[i+6] = make([]float32, n*4)
		outputs[i+9] = make([]float32, n*10)
	}
	// Satu wajah di sel (c=10, r=5) stride 16: kotak 64x64 berpusat di (168, 88).
	set := func(stride, c, r int, score float32, size float64) {
		i := map[int]int{8: 0, 16: 1, 32: 2}[stride]
		idx := r*(YuNetSize/stride) + c
		outputs[i][idx], outputs[i+3][idx] = score, score
		outputs[i+6][idx*4+0], outputs[i+6][idx*4+1] = 0.5, 0.5
		outputs[i+6][idx*4+2] = float32(math.Log(size / float64(stride)))
		outputs[i+6][idx*4+3] = float32(math.Log(size / float64(stride)))
		outputs[i+9][idx*10] = 1
	}
	set(16, 10, 5, 0.9, 64)
	set(16, 11, 5, 0.8, 64) // tumpang tindih, dibuang NMS
	set(8, 2, 2, 0.5, 16)   // skor rendah, dibuang
	faces := decodeYuNet(outputs, YuNetSize, YuNetSize)
	if len(faces) != 1 {
		t.Fatalf("wajah = %d", len(faces))
	}
	f := faces[0]
	if !near(f.X, 136, 1e-3) || !near(f.Y, 56, 1e-3) || !near(f.W, 64, 1e-3) || !near(f.Landmarks[0][0], 176, 1e-3) {
		t.Errorf("deteksi = %+v", f)
	}
	if !near(f.Score, 0.9, 1e-6) {
		t.Errorf("skor = %v", f.Score)
	}

	// Foto 300x160: wajah 64 px (21% lebar) berpusat di (168, 88), di dalam 60% bagian tengah.
	if _, err := pickFace(nil, 300, 160); err != ErrNoFace {
		t.Errorf("tanpa wajah: %v", err)
	}
	small := Detection{X: 250, W: 10, H: 10}
	if got, err := pickFace([]Detection{small, f}, 300, 160); err != nil || got != f {
		t.Errorf("wajah kecil di belakang harus diabaikan: %v", err)
	}
	other := Detection{X: 30, W: 50, H: 50}
	if _, err := pickFace([]Detection{f, other}, 300, 160); err != ErrManyFaces {
		t.Errorf("dua wajah: %v", err)
	}
}

func TestPickFaceNeedsCenteredFace(t *testing.T) {
	// Foto 1000x800: 60% bagian tengah adalah x 200-800 dan y 160-640; lebar minimal 200 px.
	box := func(cx, cy, w float64) Detection { return Detection{X: cx - w/2, Y: cy - w/2, W: w, H: w} }
	cases := []struct {
		name string
		face Detection
		want error
	}{
		{"pas di tengah", box(500, 400, 400), nil},
		{"tepat batas lebar", box(500, 400, 200), nil},
		{"tepat batas tengah", box(800, 640, 300), nil},
		{"terlalu kecil", box(500, 400, 199), ErrFaceNotCentered},
		{"di tepi kiri", box(150, 400, 300), ErrFaceNotCentered},
		{"di tepi kanan", box(850, 400, 300), ErrFaceNotCentered},
		{"terlalu atas", box(500, 120, 300), ErrFaceNotCentered},
		{"terlalu bawah", box(500, 700, 300), ErrFaceNotCentered},
	}
	for _, c := range cases {
		if _, err := pickFace([]Detection{c.face}, 1000, 800); err != c.want {
			t.Errorf("%s: %v, ingin %v", c.name, err, c.want)
		}
	}
	// Banyak wajah tetap dilaporkan lebih dulu, walau wajah utamanya kecil.
	if _, err := pickFace([]Detection{box(100, 100, 50), box(300, 300, 50)}, 1000, 800); err != ErrManyFaces {
		t.Errorf("dua wajah kecil: %v", err)
	}
	if _, err := pickFace([]Detection{box(500, 400, 400)}, 0, 0); err != ErrFaceNotCentered {
		t.Errorf("ukuran foto nol: %v", err)
	}
}

func TestDecodeJPEG(t *testing.T) {
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, gradient(64, 48), &jpeg.Options{Quality: 100}); err != nil {
		t.Fatal(err)
	}
	img, err := decodeJPEG(buf.Bytes())
	if err != nil || img.w != 64 || img.h != 48 {
		t.Fatalf("decode: %v %dx%d", err, img.w, img.h)
	}
	if b := img.at(10, 10, 0); math.Abs(float64(b)-100) > 3 {
		t.Errorf("B = %v", b)
	}
	if _, err := decodeJPEG([]byte{0xFF, 0xD8, 0xFF, 0x00}); err != ErrBadImage {
		t.Errorf("JPEG rusak: %v", err)
	}
	var tiny bytes.Buffer
	if err := jpeg.Encode(&tiny, gradient(8, 8), nil); err != nil {
		t.Fatal(err)
	}
	if _, err := decodeJPEG(tiny.Bytes()); err != ErrBadImage {
		t.Errorf("gambar terlalu kecil: %v", err)
	}
}

// headLandmarks memproyeksikan model kepala sederhana yang menoleh yaw derajat: mata di (±0.5, 0, 0),
// hidung 0.4 di bawah mata dan menonjol 0.55 ke depan, sudut mulut di (±0.35, 0.8, 0.2).
func headLandmarks(yaw float64) [5][2]float64 {
	rad := yaw * math.Pi / 180
	project := func(x, y, z float64) [2]float64 {
		return [2]float64{100 + 60*(x*math.Cos(rad)+z*math.Sin(rad)), 100 + 60*y}
	}
	return [5][2]float64{
		project(-0.5, 0, 0), project(0.5, 0, 0), project(0, 0.4, 0.55),
		project(-0.35, 0.8, 0.2), project(0.35, 0.8, 0.2),
	}
}

func TestTurnRatio(t *testing.T) {
	if r := TurnRatio(headLandmarks(0)); !near(r, 0, 1e-9) {
		t.Errorf("lurus = %v", r)
	}
	// Arah toleh menentukan tanda, besarnya sama.
	if l, r := TurnRatio(headLandmarks(25)), TurnRatio(headLandmarks(-25)); !near(l, -r, 1e-9) || l == 0 {
		t.Errorf("toleh kiri %v, kanan %v", l, r)
	}
	// Ambang cocok dengan ambang aplikasi: wajah lurus aplikasi (< 12 derajat) lolos sebagai lurus,
	// toleh aplikasi (> 25 derajat) lolos sebagai gerakan.
	if r := math.Abs(TurnRatio(headLandmarks(12))); r > StraightTurnMax {
		t.Errorf("12 derajat = %.3f, melebihi batas lurus %.2f", r, StraightTurnMax)
	}
	if r := math.Abs(TurnRatio(headLandmarks(25))); r < GestureTurnMin || r < GestureTurnDelta {
		t.Errorf("25 derajat = %.3f, kurang dari batas gerakan %.2f", r, GestureTurnMin)
	}
	// Ukuran wajah tidak memengaruhi rasio.
	var big [5][2]float64
	for i, p := range headLandmarks(20) {
		big[i] = [2]float64{p[0] * 3, p[1] * 3}
	}
	if !near(TurnRatio(big), TurnRatio(headLandmarks(20)), 1e-9) {
		t.Error("rasio harus sama untuk wajah besar dan kecil")
	}
	if TurnRatio([5][2]float64{}) != 0 {
		t.Error("titik kosong = 0")
	}
}

func TestPickMainIgnoresPosition(t *testing.T) {
	edge := Detection{X: 0, Y: 0, W: 50, H: 50}
	if _, err := pickMain([]Detection{edge}); err != nil {
		t.Errorf("pickMain tidak memeriksa posisi: %v", err)
	}
	if _, err := pickFace([]Detection{edge}, 1000, 800); err != ErrFaceNotCentered {
		t.Errorf("pickFace tetap memeriksa posisi: %v", err)
	}
}
