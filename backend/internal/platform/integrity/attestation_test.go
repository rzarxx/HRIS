package integrity

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"math/big"
	"strings"
	"testing"
	"time"
)

type testAttestationOpts struct {
	securityLevel     int
	challenge         string
	origin            int
	deviceLocked      bool
	verifiedBootState int
	packageName       string
	certDigest        []byte
	serial            int64
}

func defaultTestAttestationOpts(challenge string, certDigest []byte) testAttestationOpts {
	return testAttestationOpts{
		securityLevel:     SecurityLevelTrustedEnvironment,
		challenge:         challenge,
		origin:            KeyOriginGenerated,
		deviceLocked:      true,
		verifiedBootState: VerifiedBootStateVerified,
		packageName:       "id.pejuang.hris",
		certDigest:        certDigest,
		serial:            100,
	}
}

func buildTestAttestationChain(t *testing.T, now time.Time, opts testAttestationOpts) ([]string, []byte, *x509.CertPool) {
	t.Helper()
	rootKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	rootTmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "Test Attestation Root"},
		NotBefore:             now.Add(-time.Hour),
		NotAfter:              now.Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	rootDER, err := x509.CreateCertificate(rand.Reader, rootTmpl, rootTmpl, &rootKey.PublicKey, rootKey)
	if err != nil {
		t.Fatal(err)
	}
	rootCert, err := x509.ParseCertificate(rootDER)
	if err != nil {
		t.Fatal(err)
	}

	leafKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	leafPubDER, err := x509.MarshalPKIXPublicKey(&leafKey.PublicKey)
	if err != nil {
		t.Fatal(err)
	}

	extValue := marshalTestKeyDescription(t, opts)
	leafTmpl := &x509.Certificate{
		SerialNumber: big.NewInt(opts.serial),
		Subject:      pkix.Name{CommonName: "Android Keystore Leaf"},
		NotBefore:    now.Add(-time.Minute),
		NotAfter:     now.Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtraExtensions: []pkix.Extension{{
			Id:    oidAndroidKeyAttestation,
			Value: extValue,
		}},
	}
	leafDER, err := x509.CreateCertificate(rand.Reader, leafTmpl, rootCert, &leafKey.PublicKey, rootKey)
	if err != nil {
		t.Fatal(err)
	}

	pool := x509.NewCertPool()
	pool.AddCert(rootCert)
	chain := []string{
		base64.StdEncoding.EncodeToString(leafDER),
		base64.StdEncoding.EncodeToString(rootDER),
	}
	return chain, leafPubDER, pool
}

func marshalTestKeyDescription(t *testing.T, opts testAttestationOpts) []byte {
	t.Helper()
	originBytes, err := asn1.Marshal(opts.origin)
	if err != nil {
		t.Fatal(err)
	}
	rotBytes, err := asn1.Marshal(rootOfTrust{
		VerifiedBootKey:   bytes.Repeat([]byte{0xAA}, 32),
		DeviceLocked:      opts.deviceLocked,
		VerifiedBootState: asn1.Enumerated(opts.verifiedBootState),
	})
	if err != nil {
		t.Fatal(err)
	}
	appIDStruct := attestationApplicationID{
		PackageInfos:     []attestationPackageInfo{{PackageName: []byte(opts.packageName), Version: 1}},
		SignatureDigests: [][]byte{opts.certDigest},
	}
	appIDDER, err := asn1.Marshal(appIDStruct)
	if err != nil {
		t.Fatal(err)
	}
	appIDOctet, err := asn1.Marshal(appIDDER)
	if err != nil {
		t.Fatal(err)
	}

	var hwBuf []byte
	for _, field := range []asn1.RawValue{
		{Class: asn1.ClassContextSpecific, Tag: tagOrigin, IsCompound: true, Bytes: originBytes},
		{Class: asn1.ClassContextSpecific, Tag: tagRootOfTrust, IsCompound: true, Bytes: rotBytes},
	} {
		b, err := asn1.Marshal(field)
		if err != nil {
			t.Fatal(err)
		}
		hwBuf = append(hwBuf, b...)
	}
	hwSeq, err := asn1.Marshal(asn1.RawValue{Class: asn1.ClassUniversal, Tag: asn1.TagSequence, IsCompound: true, Bytes: hwBuf})
	if err != nil {
		t.Fatal(err)
	}

	swField, err := asn1.Marshal(asn1.RawValue{
		Class:      asn1.ClassContextSpecific,
		Tag:        tagAttestationApplicationID,
		IsCompound: true,
		Bytes:      appIDOctet,
	})
	if err != nil {
		t.Fatal(err)
	}
	swSeq, err := asn1.Marshal(asn1.RawValue{Class: asn1.ClassUniversal, Tag: asn1.TagSequence, IsCompound: true, Bytes: swField})
	if err != nil {
		t.Fatal(err)
	}

	desc := keyDescription{
		AttestationVersion:       200,
		AttestationSecurityLevel: asn1.Enumerated(opts.securityLevel),
		KeyMintVersion:           200,
		KeyMintSecurityLevel:     asn1.Enumerated(opts.securityLevel),
		AttestationChallenge:     []byte(opts.challenge),
		UniqueID:                 []byte{},
		SoftwareEnforced:         asn1.RawValue{FullBytes: swSeq},
		HardwareEnforced:         asn1.RawValue{FullBytes: hwSeq},
	}
	out, err := asn1.Marshal(desc)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestKeyAttestationVerifier(t *testing.T) {
	now := time.Date(2026, 11, 13, 12, 0, 0, 0, time.UTC)
	hash, err := AttendanceHash(strings.Repeat("a", 64), -6.2001, 106.8166, now, strings.Repeat("b", 64), strings.Repeat("c", 64))
	if err != nil {
		t.Fatal(err)
	}
	certDigest := sha256.Sum256([]byte("signing-cert"))

	t.Run("valid TEE attestation chain passes", func(t *testing.T) {
		opts := defaultTestAttestationOpts(hash, certDigest[:])
		chain, pubDER, roots := buildTestAttestationChain(t, now, opts)
		v := KeyAttestationVerifier{
			PackageName:       "id.pejuang.hris",
			SigningCertSHA256: certDigest[:],
			Roots:             roots,
			Now:               func() time.Time { return now },
		}
		if err := v.Verify(chain, pubDER, hash); err != nil {
			t.Fatalf("Verify gagal: %v", err)
		}
	})

	t.Run("rejects software security level", func(t *testing.T) {
		opts := defaultTestAttestationOpts(hash, certDigest[:])
		opts.securityLevel = SecurityLevelSoftware
		chain, pubDER, roots := buildTestAttestationChain(t, now, opts)
		v := KeyAttestationVerifier{PackageName: "id.pejuang.hris", Roots: roots, Now: func() time.Time { return now }}
		if err := v.Verify(chain, pubDER, hash); !errors.Is(err, ErrInvalid) {
			t.Fatalf("ingin ErrInvalid untuk software keystore, dapat %v", err)
		}
	})

	t.Run("rejects unlocked bootloader or unverified boot", func(t *testing.T) {
		opts := defaultTestAttestationOpts(hash, certDigest[:])
		opts.deviceLocked = false
		chain, pubDER, roots := buildTestAttestationChain(t, now, opts)
		v := KeyAttestationVerifier{PackageName: "id.pejuang.hris", Roots: roots, Now: func() time.Time { return now }}
		if err := v.Verify(chain, pubDER, hash); !errors.Is(err, ErrInvalid) {
			t.Fatalf("ingin ErrInvalid untuk bootloader terbuka, dapat %v", err)
		}
	})

	t.Run("rejects challenge mismatch with ErrNonceMismatch", func(t *testing.T) {
		otherHash, _ := AttendanceHash(strings.Repeat("f", 64), -6.2001, 106.8166, now, strings.Repeat("b", 64), strings.Repeat("c", 64))
		opts := defaultTestAttestationOpts(otherHash, certDigest[:])
		chain, pubDER, roots := buildTestAttestationChain(t, now, opts)
		v := KeyAttestationVerifier{PackageName: "id.pejuang.hris", Roots: roots, Now: func() time.Time { return now }}
		err := v.Verify(chain, pubDER, hash)
		if !errors.Is(err, ErrInvalid) || !errors.Is(err, ErrNonceMismatch) {
			t.Fatalf("ingin ErrNonceMismatch + ErrInvalid, dapat %v", err)
		}
	})

	t.Run("rejects wrong package or cert digest or revoked serial", func(t *testing.T) {
		opts := defaultTestAttestationOpts(hash, certDigest[:])
		chain, pubDER, roots := buildTestAttestationChain(t, now, opts)

		vPkg := KeyAttestationVerifier{PackageName: "com.fake.app", Roots: roots, Now: func() time.Time { return now }}
		if err := vPkg.Verify(chain, pubDER, hash); !errors.Is(err, ErrInvalid) {
			t.Fatalf("ingin ErrInvalid untuk paket salah, dapat %v", err)
		}

		serialHex := strings.ToLower(hex.EncodeToString(big.NewInt(opts.serial).Bytes()))
		vCRL := KeyAttestationVerifier{
			PackageName:    "id.pejuang.hris",
			Roots:          roots,
			RevokedSerials: map[string]bool{serialHex: true},
			Now:            func() time.Time { return now },
		}
		if err := vCRL.Verify(chain, pubDER, hash); !errors.Is(err, ErrInvalid) {
			t.Fatalf("ingin ErrInvalid untuk sertifikat dicabut, dapat %v", err)
		}
	})

	t.Run("accepts per-request attestation chain bound via AttestationChallengeForKey", func(t *testing.T) {
		if got := AttestationChallengeForKey([]byte("pub-der-bytes"), "IZ0EvC-bV2dDeuNUnZAODJqQOd0txdevb-S9qNhGkZc"); got != "pLGtaxaQ7rwXnUDWCCdpdWegDdHT-XVaZktMnbXHywM" {
			t.Fatalf("vektor AttestationChallengeForKey = %s", got)
		}
		_, boundPubDER, _ := buildTestAttestationChain(t, now, defaultTestAttestationOpts(hash, certDigest[:]))
		boundChallenge := AttestationChallengeForKey(boundPubDER, hash)
		opts := defaultTestAttestationOpts(boundChallenge, certDigest[:])
		chain, _, roots := buildTestAttestationChain(t, now, opts)
		v := KeyAttestationVerifier{
			PackageName:       "id.pejuang.hris",
			SigningCertSHA256: certDigest[:],
			Roots:             roots,
			Now:               func() time.Time { return now },
		}
		if err := v.Verify(chain, boundPubDER, hash); err != nil {
			t.Fatalf("Verify rantai terikat AttestationChallengeForKey gagal: %v", err)
		}
		otherHash, _ := AttendanceHash(strings.Repeat("9", 64), -6.2001, 106.8166, now, strings.Repeat("b", 64), strings.Repeat("c", 64))
		if err := v.Verify(chain, boundPubDER, otherHash); !errors.Is(err, ErrNonceMismatch) {
			t.Fatalf("ingin ErrNonceMismatch saat requestHash berbeda, dapat %v", err)
		}
	})
}
