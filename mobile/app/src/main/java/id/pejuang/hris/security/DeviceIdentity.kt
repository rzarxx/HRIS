package id.pejuang.hris.security

import android.content.Context
import android.security.keystore.KeyGenParameterSpec
import android.security.keystore.KeyProperties
import com.google.android.play.core.integrity.IntegrityManagerFactory
import com.google.android.play.core.integrity.IntegrityTokenRequest
import id.pejuang.hris.BuildConfig
import kotlinx.coroutines.tasks.await
import java.security.KeyPairGenerator
import java.security.KeyStore
import java.security.spec.ECGenParameterSpec
import java.util.Base64

/**
 * Mengelola kunci instalasi ECDSA P-256 di Android Keystore serta permintaan token Play Integrity (§5.4).
 * Kunci privat tidak pernah keluar dari Keystore perangkat.
 */
class DeviceIdentity(private val context: Context) {

    private fun ensureEntry(): KeyStore.PrivateKeyEntry? = runCatching {
        val ks = KeyStore.getInstance(KEYSTORE).apply { load(null) }
        (ks.getEntry(ALIAS, null) as? KeyStore.PrivateKeyEntry)?.let { return@runCatching it }
        val gen = KeyPairGenerator.getInstance(KeyProperties.KEY_ALGORITHM_EC, KEYSTORE)
        gen.initialize(
            KeyGenParameterSpec.Builder(
                ALIAS,
                KeyProperties.PURPOSE_SIGN or KeyProperties.PURPOSE_VERIFY,
            )
                .setAlgorithmParameterSpec(ECGenParameterSpec("secp256r1"))
                .setDigests(KeyProperties.DIGEST_SHA256)
                .build(),
        )
        gen.generateKeyPair()
        ks.getEntry(ALIAS, null) as? KeyStore.PrivateKeyEntry
    }.getOrNull()

    /** Kunci publik PKIX DER dalam Base64 standar untuk dikirim saat login/registrasi perangkat. */
    fun publicKeyBase64(): String? = runCatching {
        val pub = ensureEntry()?.certificate?.publicKey?.encoded ?: return@runCatching null
        Base64.getEncoder().encodeToString(pub)
    }.getOrNull()

    /** Tanda tangan ECDSA P-256 base64url atas `HRIS-DEVICE-PROOF-V1\n<requestHash>`. */
    fun signProof(requestHash: String): String? = runCatching {
        val priv = ensureEntry()?.privateKey ?: return@runCatching null
        DeviceProof.sign(priv, requestHash)
    }.getOrNull()

    /**
     * Meminta token Play Integrity yang diikat ke [requestHash].
     * Mengembalikan null bila project number belum dikonfigurasi atau layanan Play tidak tersedia di perangkat.
     */
    suspend fun requestIntegrityToken(requestHash: String): String? {
        val projectNumber = BuildConfig.PLAY_CLOUD_PROJECT_NUMBER
        if (projectNumber <= 0L) return null
        return runCatching {
            val manager = IntegrityManagerFactory.create(context)
            val req = IntegrityTokenRequest.builder()
                .setCloudProjectNumber(projectNumber)
                .setNonce(requestHash)
                .build()
            manager.requestIntegrityToken(req).await().token()
        }.getOrNull()
    }

    private companion object {
        const val KEYSTORE = "AndroidKeyStore"
        const val ALIAS = "hris_device_proof_v1"
    }
}
