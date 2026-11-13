package integrity

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
)

// VerifyDeviceProof memeriksa tanda tangan kunci instalasi atas hash challenge presensi.
// publicKeyDER harus diambil dari database setelah proses pendaftaran dan persetujuan yang tepercaya,
// bukan dari permintaan yang sedang diperiksa. Fungsi ini sendiri tidak membuktikan asal perangkat
// atau apakah kunci disimpan pada hardware; itu membutuhkan validasi attestation saat pendaftaran.
func VerifyDeviceProof(publicKeyDER []byte, requestHash, signature string) error {
	hash, err := base64.RawURLEncoding.DecodeString(requestHash)
	if err != nil || len(hash) != sha256.Size || base64.RawURLEncoding.EncodeToString(hash) != requestHash {
		return ErrInvalid
	}
	if len(signature) == 0 || len(signature) > 128 {
		return ErrInvalid
	}
	key, err := ParseDevicePublicKey(publicKeyDER)
	if err != nil {
		return err
	}
	sig, err := base64.RawURLEncoding.DecodeString(signature)
	if err != nil || base64.RawURLEncoding.EncodeToString(sig) != signature {
		return ErrInvalid
	}
	digest := sha256.Sum256([]byte("HRIS-DEVICE-PROOF-V1\n" + requestHash))
	if !ecdsa.VerifyASN1(key, digest[:], sig) {
		return ErrInvalid
	}
	return nil
}

// ParseDevicePublicKey memvalidasi kunci publik ECDSA P-256 dalam format PKIX DER kanonis.
func ParseDevicePublicKey(publicKeyDER []byte) (*ecdsa.PublicKey, error) {
	if len(publicKeyDER) == 0 || len(publicKeyDER) > 256 {
		return nil, ErrInvalid
	}
	parsed, err := x509.ParsePKIXPublicKey(publicKeyDER)
	if err != nil {
		return nil, ErrInvalid
	}
	key, ok := parsed.(*ecdsa.PublicKey)
	if !ok || key.Curve != elliptic.P256() || key.X == nil || key.Y == nil || !key.Curve.IsOnCurve(key.X, key.Y) {
		return nil, ErrInvalid
	}
	canonical, err := x509.MarshalPKIXPublicKey(key)
	if err != nil || !bytes.Equal(canonical, publicKeyDER) {
		return nil, ErrInvalid
	}
	return key, nil
}
