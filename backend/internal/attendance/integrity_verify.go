package attendance

import (
	"context"
	"errors"

	"github.com/rzarxx/HRIS/backend/internal/platform/integrity"
	"github.com/rzarxx/HRIS/backend/internal/store"
)

const (
	flagIntegrityFailed        = "integrity_failed"
	flagIntegrityNonceMismatch = "integrity_nonce_mismatch"
)

// AttestationVerifier memverifikasi rantai sertifikat Android Key Attestation (Jalur B, §5.4.1).
type AttestationVerifier interface {
	Verify(chainBase64 []string, expectedPublicKeyDER []byte, requestHash string) error
}

// verifyDeviceIntegrity memeriksa bukti kunci perangkat (VerifyDeviceProof) dan bukti sesuai
// integrity_tier perangkat (MASTER.md §5.3, §5.4, §5.4.1).
// Bila allowUnavailableReview = true (clock_in/clock_out), kegagalan jaringan ke layanan Google
// (integrity.ErrUnavailable) menghasilkan sinyal lunak integrity_unavailable (+30 -> review).
func (s *Service) verifyDeviceIntegrity(
	ctx context.Context,
	device store.Device,
	requestHash string,
	integrityToken string,
	deviceProof string,
	attestationChain []string,
	allowUnavailableReview bool,
) (softFlags []string, reasonCode string, ok bool) {
	// Jika perangkat sudah mengikat kunci publik instalasi (atau diwajibkan oleh konfigurasi),
	// tanda tangan ECDSA P-256 atas request_hash wajib valid di semua jalur.
	if len(device.PublicKey) > 0 {
		if err := integrity.VerifyDeviceProof(device.PublicKey, requestHash, deviceProof); err != nil {
			return nil, flagIntegrityFailed, false
		}
	} else if s.opt.RequireDeviceKey || deviceProof != "" {
		return nil, flagIntegrityFailed, false
	}

	tier := device.IntegrityTier
	if tier == "" {
		tier = store.DeviceIntegrityTierPlay
	}

	switch tier {
	case store.DeviceIntegrityTierPlay:
		if s.opt.PlayVerifier == nil {
			return nil, flagIntegrityFailed, false
		}
		err := s.opt.PlayVerifier.Verify(ctx, integrityToken, requestHash)
		if err == nil {
			return nil, "", true
		}
		if allowUnavailableReview && errors.Is(err, integrity.ErrBasicOnly) {
			return []string{flagIntegrityBasicOnly}, "", true
		}
		if allowUnavailableReview && errors.Is(err, integrity.ErrUnavailable) {
			return []string{flagIntegrityUnavailable}, "", true
		}
		if errors.Is(err, integrity.ErrNonceMismatch) {
			return nil, flagIntegrityNonceMismatch, false
		}
		return nil, flagIntegrityFailed, false

	case store.DeviceIntegrityTierAttestation:
		if s.opt.AttestationVerifier == nil || len(attestationChain) == 0 || len(device.PublicKey) == 0 {
			return nil, flagIntegrityFailed, false
		}
		if err := s.opt.AttestationVerifier.Verify(attestationChain, device.PublicKey, requestHash); err != nil {
			if errors.Is(err, integrity.ErrNonceMismatch) {
				return nil, flagIntegrityNonceMismatch, false
			}
			return nil, flagIntegrityFailed, false
		}
		return nil, "", true

	case store.DeviceIntegrityTierManual:
		now := s.opt.Now()
		if !device.ManualApprovedUntil.Valid || !device.ManualApprovedUntil.Time.After(now) {
			return nil, flagIntegrityFailed, false
		}
		if allowUnavailableReview {
			return []string{flagIntegrityManual}, "", true
		}
		return nil, "", true

	case store.DeviceIntegrityTierKiosk:
		if len(device.PublicKey) == 0 {
			return nil, flagIntegrityFailed, false
		}
		if s.opt.PlayVerifier != nil && integrityToken != "" {
			if err := s.opt.PlayVerifier.Verify(ctx, integrityToken, requestHash); err != nil {
				if errors.Is(err, integrity.ErrNonceMismatch) {
					return nil, flagIntegrityNonceMismatch, false
				}
				return nil, flagIntegrityFailed, false
			}
		} else if s.opt.AttestationVerifier != nil && len(attestationChain) > 0 {
			if err := s.opt.AttestationVerifier.Verify(attestationChain, device.PublicKey, requestHash); err != nil {
				if errors.Is(err, integrity.ErrNonceMismatch) {
					return nil, flagIntegrityNonceMismatch, false
				}
				return nil, flagIntegrityFailed, false
			}
		}
		return nil, "", true

	default:
		return nil, flagIntegrityFailed, false
	}
}
