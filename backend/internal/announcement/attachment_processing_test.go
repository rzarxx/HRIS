package announcement

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"testing"

	"github.com/rzarxx/HRIS/backend/internal/platform/filescan"
)

type scanFunc func(context.Context, []byte) error

func (f scanFunc) Scan(ctx context.Context, b []byte) error { return f(ctx, b) }

func TestPrepareScansOriginalAndFailsClosed(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 20, 10))
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	input := append(buf.Bytes(), []byte("hidden trailing payload")...)
	for _, scanErr := range []error{filescan.ErrUnsafe, errors.New("offline")} {
		s := NewService(nil).WithScanner(scanFunc(func(_ context.Context, got []byte) error {
			if !bytes.Equal(got, input) {
				t.Fatal("original payload must be scanned")
			}
			return scanErr
		}))
		if out, _, _, err := s.prepareAttachment(context.Background(), input); err == nil || out != nil {
			t.Fatal("unsafe file accepted")
		}
	}
	if _, _, _, err := NewService(nil).prepareAttachment(context.Background(), input); err == nil {
		t.Fatal("missing scanner accepted file")
	}
	scans := 0
	s := NewService(nil).WithScanner(scanFunc(func(_ context.Context, got []byte) error {
		scans++
		if scans == 1 && !bytes.Equal(got, input) {
			t.Fatal("original not scanned")
		}
		if scans == 2 && bytes.Contains(got, []byte("hidden trailing payload")) {
			t.Fatal("metadata retained")
		}
		return nil
	}))
	if _, _, _, err := s.prepareAttachment(context.Background(), input); err != nil {
		t.Fatal(err)
	}
	if scans != 2 {
		t.Fatalf("scans = %d", scans)
	}
	// A valid signature with a truncated body must still fail image decoding.
	if _, _, _, err := s.prepareAttachment(context.Background(), []byte{0xff, 0xd8, 0xff}); err == nil {
		t.Fatal("truncated image accepted")
	}
}

func TestImageOptimization(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 3000, 600))
	src.SetNRGBA(0, 0, color.NRGBA{R: 255, A: 128})
	var original bytes.Buffer
	if err := png.Encode(&original, src); err != nil {
		t.Fatal(err)
	}
	optimized, err := optimizeImage(original.Bytes(), "image/png")
	if err != nil {
		t.Fatal(err)
	}
	got, err := png.Decode(bytes.NewReader(optimized))
	if err != nil {
		t.Fatal(err)
	}
	if got.Bounds().Dx() != 2560 || got.Bounds().Dy() != 512 {
		t.Fatalf("dimensions: %v", got.Bounds())
	}
	_, _, _, alpha := got.At(100, 100).RGBA()
	if alpha != 0 {
		t.Fatal("PNG transparency lost")
	}
	if len(optimized) >= original.Len() {
		t.Fatalf("expected smaller image: %d >= %d", len(optimized), original.Len())
	}
}

func TestImageDimensionsRejectedBeforeDecode(t *testing.T) {
	var input bytes.Buffer
	if err := png.Encode(&input, image.NewRGBA(image.Rect(0, 0, 20, 20))); err != nil {
		t.Fatal(err)
	}
	b := bytes.Clone(input.Bytes())
	binary.BigEndian.PutUint32(b[16:20], 50_000)
	binary.BigEndian.PutUint32(b[20:24], 50_000)
	binary.BigEndian.PutUint32(b[29:33], crc32.ChecksumIEEE(b[12:29]))
	if _, _, err := image.DecodeConfig(bytes.NewReader(b)); err != nil {
		t.Fatal("fixture must have valid dimensions header", err)
	}
	if _, err := optimizeImage(b, "image/png"); err == nil {
		t.Fatal("oversized decoded dimensions accepted")
	}
}

func TestJPEGOrientationPreserved(t *testing.T) {
	var original bytes.Buffer
	if err := jpeg.Encode(&original, image.NewRGBA(image.Rect(0, 0, 20, 10)), nil); err != nil {
		t.Fatal(err)
	}
	// Little-endian EXIF with orientation=6 (90 degrees clockwise).
	exif := []byte{'E', 'x', 'i', 'f', 0, 0, 'I', 'I', 42, 0, 8, 0, 0, 0, 1, 0, 0x12, 1, 3, 0, 1, 0, 0, 0, 6, 0, 0, 0, 0, 0, 0, 0}
	input := append([]byte{0xff, 0xd8, 0xff, 0xe1, 0, byte(len(exif) + 2)}, exif...)
	input = append(input, original.Bytes()[2:]...)
	optimized, err := optimizeImage(input, "image/jpeg")
	if err != nil {
		t.Fatal(err)
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(optimized))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Width != 10 || cfg.Height != 20 || bytes.Contains(optimized, []byte("Exif")) {
		t.Fatal("orientation or metadata incorrect")
	}
}

func TestStorageCompressionRoundTripAndBounds(t *testing.T) {
	input := bytes.Repeat([]byte("%PDF-1.4 document\n"), 1000)
	packed, suffix, err := packAttachment(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(packed) >= len(input) || suffix != processedSuffix+".gz" {
		t.Fatal("document not compressed")
	}
	got, err := unpackAttachment("file"+suffix, packed, len(input))
	if err != nil || !bytes.Equal(got, input) {
		t.Fatal("PDF changed after compression", err)
	}
	if _, err := unpackAttachment("file"+suffix, packed, 20); err == nil {
		t.Fatal("decompression bound bypassed")
	}
	packed[len(packed)-1] ^= 1
	if _, err := unpackAttachment("file"+suffix, packed, len(input)); err == nil {
		t.Fatal("damaged gzip accepted")
	}
	if got, err := unpackAttachment("legacy.pdf", input, len(input)); err != nil || !bytes.Equal(got, input) {
		t.Fatal("legacy file broken")
	}
	_, suffix, err = packAttachment([]byte("small"))
	if err != nil || suffix != processedSuffix {
		t.Fatal("compression increased storage")
	}
}
