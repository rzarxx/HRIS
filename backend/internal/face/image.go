package face

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"math"
	"sort"
)

// Bagian ini meniru OpenCV (modules/objdetect/src/face_detect.cpp dan face_recognize.cpp) agar
// embedding sama dengan yang diukur di uji banding. Semuanya Go murni sehingga bisa diuji tanpa
// ONNX Runtime; CI membandingkan hasil akhirnya dengan OpenCV (TestSFaceMatchesOpenCV).

// bgrImage menyimpan piksel BGR float32, baris demi baris, seperti cv::Mat CV_8UC3.
type bgrImage struct {
	w, h int
	pix  []float32 // panjang w*h*3, urutan B, G, R
}

func (m bgrImage) at(x, y, ch int) float32 {
	return m.pix[(y*m.w+x)*3+ch]
}

// decodeJPEG membaca JPEG menjadi BGR. Orientasi EXIF tidak diterapkan, jadi aplikasi wajib
// mengirim foto yang sudah tegak.
func decodeJPEG(data []byte) (bgrImage, error) {
	img, err := jpeg.Decode(bytes.NewReader(data))
	if err != nil {
		return bgrImage{}, ErrBadImage
	}
	b := img.Bounds()
	out := bgrImage{w: b.Dx(), h: b.Dy()}
	if out.w < 32 || out.h < 32 || out.w > 4096 || out.h > 4096 {
		return bgrImage{}, ErrBadImage
	}
	out.pix = make([]float32, out.w*out.h*3)
	for y := 0; y < out.h; y++ {
		for x := 0; x < out.w; x++ {
			c := color.RGBAModel.Convert(img.At(b.Min.X+x, b.Min.Y+y)).(color.RGBA)
			i := (y*out.w + x) * 3
			out.pix[i], out.pix[i+1], out.pix[i+2] = float32(c.B), float32(c.G), float32(c.R)
		}
	}
	return out, nil
}

// Detection adalah satu wajah dari YuNet: kotak dan lima titik (mata kanan, mata kiri, hidung,
// sudut mulut kanan, sudut mulut kiri).
type Detection struct {
	X, Y, W, H float64
	Landmarks  [5][2]float64
	Score      float64
}

const (
	yunetScoreMin   = 0.7
	yunetNMS        = 0.3
	manyFaceAreaMin = 0.25 // wajah lain dihitung bila luasnya minimal 25% wajah terbesar
	// Wajah utama harus cukup besar dan di tengah foto, sesuai oval di kamera aplikasi.
	faceWidthMin = 0.20 // lebar kotak wajah minimal 20% lebar foto
	faceCenterIn = 0.60 // pusat wajah harus di dalam 60% bagian tengah foto pada tiap sumbu
)

var yunetStrides = []int{8, 16, 32}

// YuNetOutputs adalah nama keluaran model YuNet 2023mar, sesuai urutan OpenCV.
var YuNetOutputs = []string{"cls_8", "cls_16", "cls_32", "obj_8", "obj_16", "obj_32", "bbox_8", "bbox_16", "bbox_32", "kps_8", "kps_16", "kps_32"}

// YuNetSize adalah ukuran input tetap model YuNet 2023mar. OpenCV mengubah ukuran input sendiri,
// tetapi ONNX Runtime tidak, jadi gambar diperkecil agar muat lalu dipad (lihat detectorInput).
const YuNetSize = 640

// detectorInput memperkecil gambar (tidak pernah memperbesar) agar sisi terpanjangnya paling
// banyak YuNetSize. Hasilnya gambar kecil dan faktor skalanya; koordinat deteksi dibagi faktor itu
// untuk kembali ke gambar asli.
func detectorInput(img bgrImage) (bgrImage, float64) {
	longest := max(img.w, img.h)
	if longest <= YuNetSize {
		return img, 1
	}
	scale := float64(YuNetSize) / float64(longest)
	w := max(int(math.Round(float64(img.w)*scale)), 1)
	h := max(int(math.Round(float64(img.h)*scale)), 1)
	return resizeBilinear(img, w, h), scale
}

// resizeBilinear mengubah ukuran seperti cv::resize(INTER_LINEAR): piksel tujuan dipetakan dari
// pusatnya, dan nilainya dibulatkan ke uint8.
func resizeBilinear(img bgrImage, w, h int) bgrImage {
	out := bgrImage{w: w, h: h, pix: make([]float32, w*h*3)}
	sx, sy := float64(img.w)/float64(w), float64(img.h)/float64(h)
	for y := 0; y < h; y++ {
		fy := (float64(y)+0.5)*sy - 0.5
		y0 := int(math.Floor(fy))
		dy := fy - float64(y0)
		if y0 < 0 {
			y0, dy = 0, 0
		}
		y1 := min(y0+1, img.h-1)
		if y0 >= img.h-1 {
			y0, y1, dy = img.h-1, img.h-1, 0
		}
		for x := 0; x < w; x++ {
			fx := (float64(x)+0.5)*sx - 0.5
			x0 := int(math.Floor(fx))
			dx := fx - float64(x0)
			if x0 < 0 {
				x0, dx = 0, 0
			}
			x1 := min(x0+1, img.w-1)
			if x0 >= img.w-1 {
				x0, x1, dx = img.w-1, img.w-1, 0
			}
			for ch := 0; ch < 3; ch++ {
				v := float64(img.at(x0, y0, ch))*(1-dx)*(1-dy) + float64(img.at(x1, y0, ch))*dx*(1-dy) +
					float64(img.at(x0, y1, ch))*(1-dx)*dy + float64(img.at(x1, y1, ch))*dx*dy
				out.pix[(y*w+x)*3+ch] = float32(math.Max(0, math.Min(255, math.Round(v))))
			}
		}
	}
	return out
}

// scaleDetection mengembalikan koordinat deteksi dari gambar kecil ke gambar asli.
func scaleDetection(d Detection, scale float64) Detection {
	d.X, d.Y, d.W, d.H = d.X/scale, d.Y/scale, d.W/scale, d.H/scale
	for i := range d.Landmarks {
		d.Landmarks[i][0] /= scale
		d.Landmarks[i][1] /= scale
	}
	return d
}

// yunetBlob menyiapkan input YuNet: gambar BGR dipad ke YuNetSize x YuNetSize di kanan dan bawah,
// tanpa skala dan tanpa pengurangan rata-rata, dalam tata letak NCHW.
func yunetBlob(img bgrImage) ([]float32, int, int) {
	pw, ph := YuNetSize, YuNetSize
	blob := make([]float32, 3*pw*ph)
	for ch := 0; ch < 3; ch++ {
		plane := blob[ch*pw*ph:]
		for y := 0; y < img.h; y++ {
			for x := 0; x < img.w; x++ {
				plane[y*pw+x] = img.at(x, y, ch)
			}
		}
	}
	return blob, pw, ph
}

func clamp01(v float32) float64 {
	return math.Max(0, math.Min(1, float64(v)))
}

// decodeYuNet mengubah keluaran YuNet (urutan YuNetOutputs) menjadi deteksi setelah NMS.
func decodeYuNet(outputs [][]float32, padW, padH int) []Detection {
	var faces []Detection
	for i, stride := range yunetStrides {
		cols, rows := padW/stride, padH/stride
		cls, obj, bbox, kps := outputs[i], outputs[i+3], outputs[i+6], outputs[i+9]
		for r := 0; r < rows; r++ {
			for c := 0; c < cols; c++ {
				idx := r*cols + c
				if idx >= len(cls) || idx >= len(obj) || idx*4+3 >= len(bbox) || idx*10+9 >= len(kps) {
					continue
				}
				score := math.Sqrt(clamp01(cls[idx]) * clamp01(obj[idx]))
				if score < yunetScoreMin {
					continue
				}
				s := float64(stride)
				cx := (float64(c) + float64(bbox[idx*4])) * s
				cy := (float64(r) + float64(bbox[idx*4+1])) * s
				w := math.Exp(float64(bbox[idx*4+2])) * s
				h := math.Exp(float64(bbox[idx*4+3])) * s
				d := Detection{X: cx - w/2, Y: cy - h/2, W: w, H: h, Score: score}
				for n := 0; n < 5; n++ {
					d.Landmarks[n][0] = (float64(kps[idx*10+2*n]) + float64(c)) * s
					d.Landmarks[n][1] = (float64(kps[idx*10+2*n+1]) + float64(r)) * s
				}
				faces = append(faces, d)
			}
		}
	}
	return nms(faces)
}

func iou(a, b Detection) float64 {
	x1, y1 := math.Max(a.X, b.X), math.Max(a.Y, b.Y)
	x2, y2 := math.Min(a.X+a.W, b.X+b.W), math.Min(a.Y+a.H, b.Y+b.H)
	inter := math.Max(0, x2-x1) * math.Max(0, y2-y1)
	union := a.W*a.H + b.W*b.H - inter
	if union <= 0 {
		return 0
	}
	return inter / union
}

func nms(faces []Detection) []Detection {
	sort.SliceStable(faces, func(i, j int) bool { return faces[i].Score > faces[j].Score })
	var keep []Detection
	for _, f := range faces {
		ok := true
		for _, k := range keep {
			if iou(f, k) > yunetNMS {
				ok = false
				break
			}
		}
		if ok {
			keep = append(keep, f)
		}
	}
	return keep
}

// pickFace memilih wajah terbesar pada foto berukuran width x height. Wajah lain yang cukup besar
// berarti selfie berisi lebih dari satu orang; wajah kecil jauh di belakang diabaikan. Wajah utama
// yang terlalu kecil atau jauh dari tengah ditolak, karena aplikasi hanya memotret wajah yang pas
// di oval; foto seperti itu biasanya hasil kamera yang miring atau wajah yang terpotong.
func pickFace(faces []Detection, width, height int) (Detection, error) {
	if len(faces) == 0 {
		return Detection{}, ErrNoFace
	}
	best := faces[0]
	for _, f := range faces[1:] {
		if f.W*f.H > best.W*best.H {
			best = f
		}
	}
	for _, f := range faces {
		if f != best && f.W*f.H >= manyFaceAreaMin*best.W*best.H {
			return Detection{}, ErrManyFaces
		}
	}
	if !centered(best, width, height) {
		return Detection{}, ErrFaceNotCentered
	}
	return best, nil
}

// centered memeriksa ukuran dan posisi wajah utama terhadap foto.
func centered(f Detection, width, height int) bool {
	w, h := float64(width), float64(height)
	if w <= 0 || h <= 0 || f.W < faceWidthMin*w {
		return false
	}
	margin := (1 - faceCenterIn) / 2
	cx, cy := f.X+f.W/2, f.Y+f.H/2
	return cx >= margin*w && cx <= (1-margin)*w && cy >= margin*h && cy <= (1-margin)*h
}

// arcfaceTemplate adalah posisi lima titik pada gambar 112x112 yang dipakai SFace.
var arcfaceTemplate = [5][2]float64{{38.2946, 51.6963}, {73.5318, 51.5014}, {56.0252, 71.7366}, {41.5493, 92.3655}, {70.7299, 92.2041}}

// similarityTransform menghitung transformasi kemiripan (rotasi, skala, geser) dari titik wajah ke
// template, sama dengan getSimilarityTransformMatrix di OpenCV (Umeyama untuk dua dimensi).
// Hasilnya matriks 2x3 [a b tx; c d ty].
func similarityTransform(src [5][2]float64) [6]float64 {
	var srcMean, dstMean [2]float64
	for i := 0; i < 5; i++ {
		for k := 0; k < 2; k++ {
			srcMean[k] += src[i][k] / 5
			dstMean[k] += arcfaceTemplate[i][k] / 5
		}
	}
	var a00, a01, a10, a11, variance float64
	for i := 0; i < 5; i++ {
		sx, sy := src[i][0]-srcMean[0], src[i][1]-srcMean[1]
		dx, dy := arcfaceTemplate[i][0]-dstMean[0], arcfaceTemplate[i][1]-dstMean[1]
		a00 += dx * sx / 5
		a01 += dx * sy / 5
		a10 += dy * sx / 5
		a11 += dy * sy / 5
		variance += (sx*sx + sy*sy) / 5
	}
	// Rotasi terbaik tanpa pencerminan, dan jumlah nilai singular bertanda yang menjadi pembilang skala.
	n := math.Hypot(a00+a11, a10-a01)
	if n == 0 || variance == 0 {
		return [6]float64{1, 0, 0, 0, 1, 0}
	}
	cos, sin := (a00+a11)/n, (a10-a01)/n
	scale := n / variance
	a, b := scale*cos, -scale*sin
	c, d := scale*sin, scale*cos
	tx := dstMean[0] - (a*srcMean[0] + b*srcMean[1])
	ty := dstMean[1] - (c*srcMean[0] + d*srcMean[1])
	return [6]float64{a, b, tx, c, d, ty}
}

// alignCrop memotong wajah ke 112x112 dengan interpolasi bilinear dan tepi hitam, seperti
// warpAffine(INTER_LINEAR, BORDER_CONSTANT). Hasilnya blob SFace: NCHW, urutan RGB, nilai 0-255.
func alignCrop(img bgrImage, m [6]float64) []float32 {
	const size = 112
	det := m[0]*m[4] - m[1]*m[3]
	// Invers transformasi: dari titik tujuan ke titik sumber.
	ia, ib := m[4]/det, -m[1]/det
	ic, id := -m[3]/det, m[0]/det
	itx := -(ia*m[2] + ib*m[5])
	ity := -(ic*m[2] + id*m[5])
	blob := make([]float32, 3*size*size)
	sample := func(x, y, ch int) float64 {
		if x < 0 || y < 0 || x >= img.w || y >= img.h {
			return 0
		}
		return float64(img.at(x, y, ch))
	}
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			sx := ia*float64(x) + ib*float64(y) + itx
			sy := ic*float64(x) + id*float64(y) + ity
			x0, y0 := int(math.Floor(sx)), int(math.Floor(sy))
			fx, fy := sx-float64(x0), sy-float64(y0)
			for ch := 0; ch < 3; ch++ {
				v := sample(x0, y0, ch)*(1-fx)*(1-fy) + sample(x0+1, y0, ch)*fx*(1-fy) +
					sample(x0, y0+1, ch)*(1-fx)*fy + sample(x0+1, y0+1, ch)*fx*fy
				// warpAffine menghasilkan uint8, jadi nilainya dibulatkan seperti OpenCV.
				v = math.Max(0, math.Min(255, math.Round(v)))
				// BGR ke RGB (swapRB pada blobFromImage).
				blob[(2-ch)*size*size+y*size+x] = float32(v)
			}
		}
	}
	return blob
}

// imageFromRGBA dipakai tes untuk membuat gambar uji.
func imageFromRGBA(img *image.RGBA) bgrImage {
	b := img.Bounds()
	out := bgrImage{w: b.Dx(), h: b.Dy(), pix: make([]float32, b.Dx()*b.Dy()*3)}
	for y := 0; y < out.h; y++ {
		for x := 0; x < out.w; x++ {
			c := img.RGBAAt(b.Min.X+x, b.Min.Y+y)
			i := (y*out.w + x) * 3
			out.pix[i], out.pix[i+1], out.pix[i+2] = float32(c.B), float32(c.G), float32(c.R)
		}
	}
	return out
}
