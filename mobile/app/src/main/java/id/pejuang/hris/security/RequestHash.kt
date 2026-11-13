package id.pejuang.hris.security

import java.security.MessageDigest
import java.security.PrivateKey
import java.security.Signature
import java.time.Instant
import java.time.ZoneOffset
import java.time.format.DateTimeFormatter
import java.util.Base64
import java.util.Locale
import kotlin.math.abs
import kotlin.math.round

/**
 * Perhitungan hash kanonis presensi & pendaftaran wajah (MASTER.md §5.4, D51).
 * Harus menghasilkan byte yang persis sama dengan `internal/platform/integrity/request_hash.go` di server.
 */
object RequestHash {
    private val HEX_32 = Regex("^[0-9a-f]{64}$")
    private val ACTIONS = setOf("blink", "turn_left", "turn_right", "smile")
    private val DATE_TIME_SECONDS = DateTimeFormatter.ofPattern("yyyy-MM-dd'T'HH:mm:ss", Locale.US).withZone(ZoneOffset.UTC)

    fun attendance(
        nonce: String,
        latitude: Double,
        longitude: Double,
        capturedAt: String,
        selfieSha256: String,
        gestureSha256: String,
    ): String {
        val fields = canonicalLocation(nonce, latitude, longitude, Instant.parse(capturedAt))
        require(HEX_32.matches(selfieSha256) && HEX_32.matches(gestureSha256)) { "Hash foto harus hex 64 karakter huruf kecil" }
        return hashFields(fields + listOf(selfieSha256, gestureSha256))
    }

    fun enrollment(
        nonce: String,
        latitude: Double,
        longitude: Double,
        capturedAt: String,
        consentVersion: String,
        action: String,
        samples: List<String>,
        gestureSha256: String,
    ): String {
        val fields = canonicalLocation(nonce, latitude, longitude, Instant.parse(capturedAt))
        require(samples.size in 3..5) { "Jumlah sampel wajah harus 3 sampai 5" }
        require(consentVersion.isNotEmpty() && consentVersion.trim() == consentVersion && !consentVersion.contains("|")) {
            "Versi persetujuan tidak valid"
        }
        require(action in ACTIONS) { "Gerakan liveness tidak valid" }
        require(samples.all { HEX_32.matches(it) } && HEX_32.matches(gestureSha256)) { "Hash sampel harus hex 64 karakter huruf kecil" }
        return hashFields(listOf("face_enroll") + fields + listOf(consentVersion, action) + samples + listOf(gestureSha256))
    }

    private fun canonicalLocation(nonce: String, latitude: Double, longitude: Double, capturedAt: Instant): List<String> {
        val year = capturedAt.atZone(ZoneOffset.UTC).year
        require(HEX_32.matches(nonce)) { "Nonce harus hex 64 karakter huruf kecil" }
        require(year in 1..9999) { "Tahun waktu tangkap di luar batas" }
        require(latitude.isFinite() && longitude.isFinite()) { "Koordinat harus berhingga" }
        require(latitude in -90.0..90.0 && longitude in -180.0..180.0) { "Koordinat di luar jangkauan" }
        require(abs(latitude - round(latitude * 1e6) / 1e6) < 1e-12 && abs(longitude - round(longitude * 1e6) / 1e6) < 1e-12) {
            "Koordinat maksimal 6 angka desimal"
        }
        val lat = if (latitude == 0.0) 0.0 else latitude
        val lon = if (longitude == 0.0) 0.0 else longitude
        return listOf(
            nonce,
            String.format(Locale.US, "%.6f", lat),
            String.format(Locale.US, "%.6f", lon),
            formatRfc3339Nano(capturedAt),
        )
    }

    internal fun formatRfc3339Nano(instant: Instant): String {
        val base = DATE_TIME_SECONDS.format(instant)
        val nano = instant.nano
        if (nano == 0) return "${base}Z"
        val frac = String.format(Locale.US, "%09d", nano).trimEnd('0')
        return "$base.${frac}Z"
    }

    private fun hashFields(fields: List<String>): String {
        val joined = fields.joinToString("|").toByteArray(Charsets.UTF_8)
        val digest = MessageDigest.getInstance("SHA-256").digest(joined)
        return Base64.getUrlEncoder().withoutPadding().encodeToString(digest)
    }
}

/**
 * Penandatanganan bukti kepemilikan kunci instalasi ECDSA P-256 (`HRIS-DEVICE-PROOF-V1\n<request_hash>`).
 * Cocok dengan `VerifyDeviceProof` pada `internal/platform/integrity/device_proof.go`.
 */
object DeviceProof {
    const val PREFIX = "HRIS-DEVICE-PROOF-V1\n"

    fun message(requestHash: String): ByteArray = (PREFIX + requestHash).toByteArray(Charsets.UTF_8)

    fun sign(privateKey: PrivateKey, requestHash: String): String {
        val sig = Signature.getInstance("SHA256withECDSA").run {
            initSign(privateKey)
            update(message(requestHash))
            sign()
        }
        return Base64.getUrlEncoder().withoutPadding().encodeToString(sig)
    }

    /**
     * Menurunkan challenge Key Attestation 43 karakter (base64url SHA-256) yang mengikat
     * kunci publik instalasi (PKIX DER) dengan [requestHash], cocok dengan `AttestationChallengeForKey` di server.
     */
    fun attestationChallenge(publicKeyDer: ByteArray, requestHash: String): String {
        val digest = MessageDigest.getInstance("SHA-256").run {
            update(publicKeyDer)
            update('|'.code.toByte())
            update(requestHash.toByteArray(Charsets.UTF_8))
            digest()
        }
        return Base64.getUrlEncoder().withoutPadding().encodeToString(digest)
    }
}
