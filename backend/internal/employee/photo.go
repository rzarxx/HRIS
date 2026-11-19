package employee

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	_ "image/png" // dekoder PNG untuk image.Decode
)

const (
	// MaxPhotoUploadBytes sama dengan batas body rute unggah (MASTER.md §5.7).
	MaxPhotoUploadBytes = 5 << 20
	// photoSide adalah sisi foto profil yang disimpan.
	photoSide = 512
	// photoMinSide menolak gambar yang terlalu kecil untuk dipakai sebagai foto profil.
	photoMinSide = 96
	// photoMaxPixels mencegah gambar raksasa (decompression bomb) dibongkar ke memori.
	photoMaxPixels = 40_000_000
	photoQuality   = 85
)

var (
	errPhotoFormat = errors.New("format foto tidak didukung")
	errPhotoSize   = errors.New("ukuran foto tidak sesuai")
)

// isPhotoMagic memeriksa tanda awal berkas JPEG atau PNG, bukan sekadar Content-Type dari klien.
func isPhotoMagic(data []byte) bool {
	return bytes.HasPrefix(data, []byte{0xFF, 0xD8, 0xFF}) ||
		bytes.HasPrefix(data, []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1A, '\n'})
}

// processPhoto memotong foto menjadi persegi di tengah, memperkecilnya ke paling besar 512 px,
// lalu menyimpannya ulang sebagai JPEG. Penyimpanan ulang membuang seluruh metadata, termasuk
// EXIF berisi lokasi atau model HP.
func processPhoto(data []byte) ([]byte, error) {
	if !isPhotoMagic(data) {
		return nil, errPhotoFormat
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || (format != "jpeg" && format != "png") {
		return nil, errPhotoFormat
	}
	if cfg.Width < photoMinSide || cfg.Height < photoMinSide || cfg.Width*cfg.Height > photoMaxPixels {
		return nil, errPhotoSize
	}
	src, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, errPhotoFormat
	}
	b := src.Bounds()
	side := min(b.Dx(), b.Dy())
	crop := image.Rect(0, 0, side, side)
	square := image.NewRGBA(crop)
	offset := image.Pt(b.Min.X+(b.Dx()-side)/2, b.Min.Y+(b.Dy()-side)/2)
	// Latar putih agar bagian transparan PNG tidak menjadi hitam di JPEG.
	draw.Draw(square, crop, image.NewUniform(color.White), image.Point{}, draw.Src)
	draw.Draw(square, crop, src, offset, draw.Over)

	out := square
	if side > photoSide {
		out = downscale(square, photoSide)
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, out, &jpeg.Options{Quality: photoQuality}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// downscale memperkecil gambar persegi dengan rata-rata area (box filter), yang cukup halus untuk
// perkecilan besar tanpa pustaka tambahan.
func downscale(src *image.RGBA, side int) *image.RGBA {
	n := src.Bounds().Dx()
	dst := image.NewRGBA(image.Rect(0, 0, side, side))
	for y := 0; y < side; y++ {
		y0, y1 := y*n/side, max((y+1)*n/side, y*n/side+1)
		for x := 0; x < side; x++ {
			x0, x1 := x*n/side, max((x+1)*n/side, x*n/side+1)
			var r, g, bl, count uint64
			for sy := y0; sy < y1; sy++ {
				row := src.Pix[sy*src.Stride:]
				for sx := x0; sx < x1; sx++ {
					i := sx * 4
					r += uint64(row[i])
					g += uint64(row[i+1])
					bl += uint64(row[i+2])
					count++
				}
			}
			i := dst.PixOffset(x, y)
			dst.Pix[i] = uint8(r / count)
			dst.Pix[i+1] = uint8(g / count)
			dst.Pix[i+2] = uint8(bl / count)
			dst.Pix[i+3] = 0xFF
		}
	}
	return dst
}
