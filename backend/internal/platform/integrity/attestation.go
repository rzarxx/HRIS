package integrity

import (
	"bytes"
	"crypto/sha256"
	"crypto/subtle"
	"crypto/x509"
	"encoding/asn1"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
	"time"
)

// OID ekstensi Android Key Attestation (https://source.android.com/docs/security/features/keystore/attestation).
var oidAndroidKeyAttestation = asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 11129, 2, 1, 17}

const (
	SecurityLevelSoftware           = 0
	SecurityLevelTrustedEnvironment = 1
	SecurityLevelStrongBox          = 2

	VerifiedBootStateVerified = 0
	KeyOriginGenerated        = 0

	tagPurpose                  = 1
	tagOrigin                   = 702
	tagRootOfTrust              = 704
	tagAttestationApplicationID = 709
)

// KeyAttestationVerifier memverifikasi rantai sertifikat Android Key Attestation (Jalur B, MASTER.md §5.4.1).
type KeyAttestationVerifier struct {
	PackageName       string
	SigningCertSHA256 []byte
	Roots             *x509.CertPool
	RevokedSerials    map[string]bool
	Now               func() time.Time
}

type keyDescription struct {
	AttestationVersion       int
	AttestationSecurityLevel asn1.Enumerated
	KeyMintVersion           int
	KeyMintSecurityLevel     asn1.Enumerated
	AttestationChallenge     []byte
	UniqueID                 []byte
	SoftwareEnforced         asn1.RawValue
	HardwareEnforced         asn1.RawValue
}

type rootOfTrust struct {
	VerifiedBootKey   []byte
	DeviceLocked      bool
	VerifiedBootState asn1.Enumerated
	VerifiedBootHash  []byte `asn1:"optional"`
}

type attestationApplicationID struct {
	PackageInfos     []attestationPackageInfo `asn1:"set"`
	SignatureDigests [][]byte                 `asn1:"set"`
}

type attestationPackageInfo struct {
	PackageName []byte
	Version     int64
}

// Verify memvalidasi rantai sertifikat X.509 hingga root CA, memeriksa status pencabutan (CRL),
// memastikan kunci dibuat di TEE/StrongBox, bootloader terkunci (Verified Boot), paket aplikasi cocok,
// dan attestationChallenge sama persis dengan requestHash.
func (v KeyAttestationVerifier) Verify(chainBase64 []string, expectedPublicKeyDER []byte, requestHash string) error {
	if len(chainBase64) < 2 || len(chainBase64) > 8 || len(requestHash) != 43 {
		return ErrInvalid
	}
	if rawHash, err := base64.RawURLEncoding.DecodeString(requestHash); err != nil || len(rawHash) != 32 {
		return ErrInvalid
	}
	if v.Roots == nil {
		return ErrInvalid
	}

	certs := make([]*x509.Certificate, 0, len(chainBase64))
	for i, b64 := range chainBase64 {
		der, err := base64.StdEncoding.DecodeString(strings.TrimSpace(b64))
		if err != nil {
			return ErrInvalid
		}
		cert, err := x509.ParseCertificate(der)
		if err != nil {
			return ErrInvalid
		}
		serialHex := strings.ToLower(hex.EncodeToString(cert.SerialNumber.Bytes()))
		if v.RevokedSerials != nil && v.RevokedSerials[serialHex] {
			return fmt.Errorf("%w: sertifikat dicabut", ErrInvalid)
		}
		if i > 0 && hasAttestationExtension(cert) {
			return fmt.Errorf("%w: ekstensi attestation pada CA", ErrInvalid)
		}
		certs = append(certs, cert)
	}

	leaf := certs[0]
	if _, err := ParseDevicePublicKey(leaf.RawSubjectPublicKeyInfo); err != nil {
		return ErrInvalid
	}

	inter := x509.NewCertPool()
	for _, c := range certs[1 : len(certs)-1] {
		inter.AddCert(c)
	}
	now := time.Now()
	if v.Now != nil {
		now = v.Now()
	}
	if _, err := leaf.Verify(x509.VerifyOptions{
		Roots:         v.Roots,
		Intermediates: inter,
		CurrentTime:   now,
		KeyUsages:     []x509.ExtKeyUsage{x509.ExtKeyUsageAny},
	}); err != nil {
		return fmt.Errorf("%w: rantai sertifikat: %v", ErrInvalid, err)
	}

	extBytes, ok := findAttestationExtension(leaf)
	if !ok {
		return fmt.Errorf("%w: ekstensi key attestation tidak ada", ErrInvalid)
	}
	var desc keyDescription
	rest, err := asn1.Unmarshal(extBytes, &desc)
	if err != nil || len(rest) != 0 {
		return ErrInvalid
	}
	if !isHardwareLevel(int(desc.AttestationSecurityLevel)) || !isHardwareLevel(int(desc.KeyMintSecurityLevel)) {
		return fmt.Errorf("%w: bukan hardware keystore", ErrInvalid)
	}
	directChallengeMatch := subtle.ConstantTimeCompare(desc.AttestationChallenge, []byte(requestHash)) == 1
	boundChallengeMatch := len(expectedPublicKeyDER) > 0 &&
		subtle.ConstantTimeCompare(desc.AttestationChallenge, []byte(AttestationChallengeForKey(expectedPublicKeyDER, requestHash))) == 1
	if !directChallengeMatch && !boundChallengeMatch {
		return fmt.Errorf("%w: %w", ErrNonceMismatch, ErrInvalid)
	}
	if directChallengeMatch && len(expectedPublicKeyDER) > 0 && !bytes.Equal(leaf.RawSubjectPublicKeyInfo, expectedPublicKeyDER) {
		return fmt.Errorf("%w: kunci publik tidak cocok", ErrInvalid)
	}

	hwOrigin, rot, hwAppID, err := parseAuthorizationList(desc.HardwareEnforced.Bytes)
	if err != nil {
		return ErrInvalid
	}
	if !hwOrigin.valid || hwOrigin.value != KeyOriginGenerated {
		return fmt.Errorf("%w: kunci tidak dibuat di hardware", ErrInvalid)
	}
	if rot == nil || !rot.DeviceLocked || int(rot.VerifiedBootState) != VerifiedBootStateVerified {
		return fmt.Errorf("%w: bootloader tidak terkunci atau belum terverifikasi", ErrInvalid)
	}

	_, _, swAppID, err := parseAuthorizationList(desc.SoftwareEnforced.Bytes)
	if err != nil {
		return ErrInvalid
	}
	appIDBytes := hwAppID
	if len(appIDBytes) == 0 {
		appIDBytes = swAppID
	}
	if v.PackageName != "" {
		if err := verifyApplicationID(appIDBytes, v.PackageName, v.SigningCertSHA256); err != nil {
			return err
		}
	}
	return nil
}

func isHardwareLevel(level int) bool {
	return level == SecurityLevelTrustedEnvironment || level == SecurityLevelStrongBox
}

func hasAttestationExtension(cert *x509.Certificate) bool {
	_, ok := findAttestationExtension(cert)
	return ok
}

func findAttestationExtension(cert *x509.Certificate) ([]byte, bool) {
	for _, ext := range cert.Extensions {
		if ext.Id.Equal(oidAndroidKeyAttestation) {
			return ext.Value, true
		}
	}
	return nil, false
}

type optionalInt struct {
	value int
	valid bool
}

func parseAuthorizationList(raw []byte) (optionalInt, *rootOfTrust, []byte, error) {
	var origin optionalInt
	var rot *rootOfTrust
	var appID []byte

	rest := raw
	for len(rest) > 0 {
		var elem asn1.RawValue
		var err error
		rest, err = asn1.Unmarshal(rest, &elem)
		if err != nil {
			return origin, nil, nil, err
		}
		if elem.Class != asn1.ClassContextSpecific {
			continue
		}
		switch elem.Tag {
		case tagOrigin:
			var v int
			if _, err := asn1.Unmarshal(elem.Bytes, &v); err != nil {
				return origin, nil, nil, err
			}
			origin = optionalInt{value: v, valid: true}
		case tagRootOfTrust:
			var r rootOfTrust
			if _, err := asn1.Unmarshal(elem.Bytes, &r); err != nil {
				return origin, nil, nil, err
			}
			rot = &r
		case tagAttestationApplicationID:
			if _, err := asn1.Unmarshal(elem.Bytes, &appID); err != nil {
				return origin, nil, nil, err
			}
		}
	}
	return origin, rot, appID, nil
}

func verifyApplicationID(der []byte, wantPackage string, wantCertSHA256 []byte) error {
	if len(der) == 0 {
		return fmt.Errorf("%w: attestationApplicationId kosong", ErrInvalid)
	}
	var appID attestationApplicationID
	rest, err := asn1.Unmarshal(der, &appID)
	if err != nil || len(rest) != 0 {
		return ErrInvalid
	}
	pkgMatched := false
	for _, p := range appID.PackageInfos {
		if string(p.PackageName) == wantPackage {
			pkgMatched = true
			break
		}
	}
	if !pkgMatched {
		return fmt.Errorf("%w: paket aplikasi tidak cocok", ErrInvalid)
	}
	if len(wantCertSHA256) > 0 {
		sigMatched := false
		for _, digest := range appID.SignatureDigests {
			if subtle.ConstantTimeCompare(digest, wantCertSHA256) == 1 {
				sigMatched = true
				break
			}
		}
		if !sigMatched {
			return fmt.Errorf("%w: sidik jari sertifikat aplikasi tidak cocok", ErrInvalid)
		}
	}
	return nil
}

// AttestationChallengeForKey menurunkan challenge Key Attestation 43 karakter (base64url SHA-256)
// yang mengikat kunci publik instalasi perangkat (PKIX DER) dengan requestHash tantangan presensi.
func AttestationChallengeForKey(pubKeyDER []byte, requestHash string) string {
	h := sha256.New()
	h.Write(pubKeyDER)
	h.Write([]byte{'|'})
	h.Write([]byte(requestHash))
	sum := h.Sum(nil)
	return base64.RawURLEncoding.EncodeToString(sum)
}

// LoadAttestationRoots memuat berkas PEM yang berisi sertifikat Root CA Key Attestation.
func LoadAttestationRoots(pemPath string) (*x509.CertPool, error) {
	raw, err := os.ReadFile(pemPath)
	if err != nil {
		return nil, err
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(raw) {
		return nil, fmt.Errorf("%w: tidak ada sertifikat PEM yang valid di %s", ErrInvalid, pemPath)
	}
	return pool, nil
}

// ParseAttestationCertSHA256 mengurai sidik jari SHA-256 heksadesimal (dengan atau tanpa pemisah ':').
func ParseAttestationCertSHA256(rawHex string) ([]byte, error) {
	cleaned := strings.ReplaceAll(strings.TrimSpace(rawHex), ":", "")
	if len(cleaned) != 64 {
		return nil, fmt.Errorf("%w: sidik jari SHA-256 harus 64 karakter heksadesimal", ErrInvalid)
	}
	out, err := hex.DecodeString(cleaned)
	if err != nil || len(out) != 32 {
		return nil, fmt.Errorf("%w: sidik jari SHA-256 tidak valid", ErrInvalid)
	}
	return out, nil
}
