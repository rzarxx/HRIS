package id.pejuang.hris.data

import android.content.Context
import android.security.keystore.KeyGenParameterSpec
import android.security.keystore.KeyProperties
import android.util.Base64
import java.security.KeyStore
import javax.crypto.Cipher
import javax.crypto.KeyGenerator
import javax.crypto.SecretKey
import javax.crypto.spec.GCMParameterSpec

/**
 * Menyimpan nilai rahasia (refresh token) terenkripsi AES-GCM dengan kunci di Android Keystore.
 * Kuncinya tidak pernah keluar dari Keystore, jadi salinan berkas preferensi tidak bisa dibuka.
 */
class SecureStore(context: Context) {
    private val prefs = context.getSharedPreferences("hris_secure", Context.MODE_PRIVATE)

    private fun key(): SecretKey {
        val keyStore = KeyStore.getInstance(KEYSTORE).apply { load(null) }
        (keyStore.getEntry(ALIAS, null) as? KeyStore.SecretKeyEntry)?.let { return it.secretKey }
        val generator = KeyGenerator.getInstance(KeyProperties.KEY_ALGORITHM_AES, KEYSTORE)
        generator.init(
            KeyGenParameterSpec.Builder(ALIAS, KeyProperties.PURPOSE_ENCRYPT or KeyProperties.PURPOSE_DECRYPT)
                .setBlockModes(KeyProperties.BLOCK_MODE_GCM)
                .setEncryptionPaddings(KeyProperties.ENCRYPTION_PADDING_NONE)
                .setKeySize(256)
                .build(),
        )
        return generator.generateKey()
    }

    fun put(name: String, value: String?) {
        if (value == null) {
            prefs.edit().remove(name).apply()
            return
        }
        val cipher = Cipher.getInstance(TRANSFORMATION)
        cipher.init(Cipher.ENCRYPT_MODE, key())
        val sealed = cipher.iv + cipher.doFinal(value.toByteArray(Charsets.UTF_8))
        prefs.edit().putString(name, Base64.encodeToString(sealed, Base64.NO_WRAP)).apply()
    }

    fun get(name: String): String? {
        val raw = prefs.getString(name, null) ?: return null
        return try {
            val sealed = Base64.decode(raw, Base64.NO_WRAP)
            val cipher = Cipher.getInstance(TRANSFORMATION)
            cipher.init(Cipher.DECRYPT_MODE, key(), GCMParameterSpec(128, sealed, 0, IV_SIZE))
            String(cipher.doFinal(sealed, IV_SIZE, sealed.size - IV_SIZE), Charsets.UTF_8)
        } catch (e: Exception) {
            // Kunci hilang (mis. kunci layar diubah) atau data rusak: anggap belum masuk.
            prefs.edit().remove(name).apply()
            null
        }
    }

    private companion object {
        const val KEYSTORE = "AndroidKeyStore"
        const val ALIAS = "hris_session"
        const val TRANSFORMATION = "AES/GCM/NoPadding"
        const val IV_SIZE = 12
    }
}
