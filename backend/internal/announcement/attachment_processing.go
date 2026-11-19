package announcement

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/binary"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"
	"net/http"
	"strings"

	"github.com/rzarxx/HRIS/backend/internal/platform/filescan"
	"github.com/rzarxx/HRIS/backend/internal/platform/httpx"
)

const maxImagePixels = 24_000_000
const maxImageSide = 2560
const processedSuffix = ".safe-v1"

func (s *Service) WithScanner(scanner filescan.Scanner) *Service {
	s.scanner = scanner
	return s
}

func (s *Service) scan(ctx context.Context, data []byte) error {
	if s.scanner == nil {
		return httpx.NewError(http.StatusServiceUnavailable, "FILE_SCAN_UNAVAILABLE", "Pemeriksaan keamanan berkas belum tersedia. Silakan coba lagi nanti.")
	}
	if err := s.scanner.Scan(ctx, data); err != nil {
		if errors.Is(err, filescan.ErrUnsafe) {
			return httpx.NewError(http.StatusUnprocessableEntity, "FILE_UNSAFE", "Berkas tidak lolos pemeriksaan keamanan. Mohon gunakan berkas lain.")
		}
		return httpx.NewError(http.StatusServiceUnavailable, "FILE_SCAN_UNAVAILABLE", "Pemeriksaan keamanan berkas belum berhasil. Silakan coba lagi nanti.")
	}
	return nil
}

// prepareAttachment limits concurrent decodes and scans the original before any
// parser sees it. Re-encoded images are also scanned before publication.
func (s *Service) prepareAttachment(ctx context.Context, data []byte) ([]byte, string, string, error) {
	if len(data) > MaxAttachmentBytes {
		return nil, "", "", errAttachmentTooLarge()
	}
	contentType, ext, ok := detectAttachment(data)
	if !ok {
		return nil, "", "", httpx.Validation(map[string]string{"file": "Mohon gunakan berkas PDF, JPEG, atau PNG."})
	}
	select {
	case s.processing <- struct{}{}:
		defer func() { <-s.processing }()
	case <-ctx.Done():
		return nil, "", "", ctx.Err()
	}
	if err := s.scan(ctx, data); err != nil {
		return nil, "", "", err
	}
	if contentType != "application/pdf" {
		var err error
		data, err = optimizeImage(data, contentType)
		if err != nil {
			return nil, "", "", httpx.Validation(map[string]string{"file": "Gambar tidak dapat dibaca atau melebihi batas 24 juta piksel. Mohon gunakan gambar lain."})
		}
		if len(data) > MaxAttachmentBytes {
			return nil, "", "", errAttachmentTooLarge()
		}
		if err := s.scan(ctx, data); err != nil {
			return nil, "", "", err
		}
	}
	return data, contentType, ext, nil
}

// packAttachment compresses BEFORE encryption. Already compressed documents may
// not shrink; those retain their bytes rather than consume extra storage.
func packAttachment(data []byte) ([]byte, string, error) {
	var buf bytes.Buffer
	w, err := gzip.NewWriterLevel(&buf, gzip.BestCompression)
	if err != nil {
		return nil, "", err
	}
	if _, err := w.Write(data); err != nil {
		return nil, "", err
	}
	if err := w.Close(); err != nil {
		return nil, "", err
	}
	if buf.Len() < len(data) {
		return buf.Bytes(), processedSuffix + ".gz", nil
	}
	return data, processedSuffix, nil
}

func unpackAttachment(key string, data []byte, size int) ([]byte, error) {
	if size < 1 || size > MaxAttachmentBytes {
		return nil, errors.New("invalid attachment size")
	}
	if strings.HasSuffix(key, processedSuffix+".gz") {
		r, err := gzip.NewReader(bytes.NewReader(data))
		if err != nil {
			return nil, err
		}
		defer r.Close()
		data, err = io.ReadAll(io.LimitReader(r, int64(size)+1))
		if err != nil {
			return nil, err
		}
	}
	if len(data) != size {
		return nil, errors.New("attachment size mismatch")
	}
	return data, nil
}

func isProcessedAttachment(key string) bool {
	return strings.HasSuffix(key, processedSuffix) || strings.HasSuffix(key, processedSuffix+".gz")
}

func optimizeImage(data []byte, contentType string) ([]byte, error) {
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || cfg.Width < 1 || cfg.Height < 1 || int64(cfg.Width)*int64(cfg.Height) > maxImagePixels {
		return nil, errors.New("invalid image dimensions")
	}
	src, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	w, h := cfg.Width, cfg.Height
	if max(w, h) > maxImageSide {
		w, h = max(1, w*maxImageSide/max(cfg.Width, cfg.Height)), max(1, h*maxImageSide/max(cfg.Width, cfg.Height))
	}
	// Area average keeps text legible when a large poster is reduced. Premultiplied
	// alpha preserves PNG transparency. EXIF rotation is applied before metadata removal.
	orientation := jpegOrientation(data)
	ow, oh := w, h
	if orientation >= 5 {
		ow, oh = h, w
	}
	dst := image.NewRGBA(image.Rect(0, 0, ow, oh))
	for y := 0; y < h; y++ {
		y0, y1 := y*cfg.Height/h, max((y+1)*cfg.Height/h, y*cfg.Height/h+1)
		for x := 0; x < w; x++ {
			x0, x1 := x*cfg.Width/w, max((x+1)*cfg.Width/w, x*cfg.Width/w+1)
			var r, g, b, a, n uint64
			for sy := y0; sy < y1; sy++ {
				for sx := x0; sx < x1; sx++ {
					pr, pg, pb, pa := src.At(sx, sy).RGBA()
					r += uint64(pr)
					g += uint64(pg)
					b += uint64(pb)
					a += uint64(pa)
					n++
				}
			}
			dx, dy := orientedPoint(x, y, w, h, orientation)
			dst.SetRGBA(dx, dy, color.RGBA{uint8(r / n >> 8), uint8(g / n >> 8), uint8(b / n >> 8), uint8(a / n >> 8)})
		}
	}
	var out bytes.Buffer
	if contentType == "image/jpeg" {
		err = jpeg.Encode(&out, dst, &jpeg.Options{Quality: 85})
	} else {
		encoder := png.Encoder{CompressionLevel: png.BestCompression}
		err = encoder.Encode(&out, dst)
	}
	return out.Bytes(), err
}

func orientedPoint(x, y, w, h, orientation int) (int, int) {
	switch orientation {
	case 2:
		return w - 1 - x, y
	case 3:
		return w - 1 - x, h - 1 - y
	case 4:
		return x, h - 1 - y
	case 5:
		return y, x
	case 6:
		return h - 1 - y, x
	case 7:
		return h - 1 - y, w - 1 - x
	case 8:
		return y, w - 1 - x
	default:
		return x, y
	}
}

// Read only the bounded TIFF orientation entry in JPEG APP1; do not retain EXIF.
func jpegOrientation(data []byte) int {
	if !bytes.HasPrefix(data, []byte{0xff, 0xd8}) {
		return 1
	}
	for pos := 2; pos+4 <= len(data); {
		if data[pos] != 0xff {
			break
		}
		marker := data[pos+1]
		if marker == 0xda || marker == 0xd9 {
			break
		}
		size := int(binary.BigEndian.Uint16(data[pos+2 : pos+4]))
		if size < 2 || size > len(data)-pos-2 {
			break
		}
		segment := data[pos+4 : pos+2+size]
		if marker == 0xe1 && bytes.HasPrefix(segment, []byte("Exif\x00\x00")) {
			tiff := segment[6:]
			if len(tiff) < 8 {
				return 1
			}
			var order binary.ByteOrder
			switch string(tiff[:2]) {
			case "II":
				order = binary.LittleEndian
			case "MM":
				order = binary.BigEndian
			default:
				return 1
			}
			if order.Uint16(tiff[2:4]) != 42 {
				return 1
			}
			offset := uint64(order.Uint32(tiff[4:8]))
			if offset+2 > uint64(len(tiff)) {
				return 1
			}
			count := int(order.Uint16(tiff[offset : offset+2]))
			for i := 0; i < count; i++ {
				at := offset + 2 + uint64(i)*12
				if at+12 > uint64(len(tiff)) {
					return 1
				}
				e := tiff[at : at+12]
				if order.Uint16(e[:2]) == 0x112 && order.Uint16(e[2:4]) == 3 && order.Uint32(e[4:8]) == 1 {
					v := int(order.Uint16(e[8:10]))
					if v >= 1 && v <= 8 {
						return v
					}
					return 1
				}
			}
		}
		pos += size + 2
	}
	return 1
}
