package id.albahjah.hris.security

import android.content.Context
import android.os.Build
import androidx.biometric.BiometricManager
import androidx.biometric.BiometricManager.Authenticators.BIOMETRIC_WEAK
import androidx.biometric.BiometricManager.Authenticators.DEVICE_CREDENTIAL
import androidx.biometric.BiometricPrompt
import androidx.core.content.ContextCompat
import androidx.fragment.app.FragmentActivity

/**
 * Kunci aplikasi dengan sidik jari, wajah, atau PIN/pola HP (D62). Sesi tetap tersimpan di HP seperti
 * biasa; kunci ini hanya menahan layar sampai pemilik HP membuka, jadi kata sandi tidak pernah disimpan.
 * BIOMETRIC_WEAK | DEVICE_CREDENTIAL dipakai karena kombinasi STRONG dengan PIN belum didukung Android 10.
 */
object AppLock {
    private const val AUTHENTICATORS = BIOMETRIC_WEAK or DEVICE_CREDENTIAL

    /** Lama di latar belakang sebelum aplikasi dikunci lagi. */
    const val RELOCK_AFTER_MS = 5L * 60 * 1000

    /**
     * HP punya sidik jari, wajah, atau PIN/pola yang bisa dipakai membuka kunci. Hanya Android 10 ke atas:
     * di versi lebih lama pustaka memakai dialog pengganti dan PIN sebagai cadangan belum didukung penuh.
     */
    fun available(context: Context): Boolean =
        Build.VERSION.SDK_INT >= Build.VERSION_CODES.Q &&
            BiometricManager.from(context).canAuthenticate(AUTHENTICATORS) == BiometricManager.BIOMETRIC_SUCCESS

    /**
     * Apakah layar perlu dikunci saat aplikasi tampil lagi. [backgroundSince] null berarti aplikasi baru
     * dibuka (bukan kembali dari latar), jadi selalu dikunci bila fitur aktif dan sesi masih ada.
     */
    fun shouldLock(enabled: Boolean, loggedIn: Boolean, backgroundSince: Long?, now: Long): Boolean =
        enabled && loggedIn && (backgroundSince == null || now - backgroundSince >= RELOCK_AFTER_MS)

    /** Menampilkan dialog sistem; [onResult] true bila berhasil dibuka. Batal atau gagal memberi false. */
    fun prompt(activity: FragmentActivity, title: String, subtitle: String, onResult: (Boolean) -> Unit) {
        val callback = object : BiometricPrompt.AuthenticationCallback() {
            override fun onAuthenticationSucceeded(result: BiometricPrompt.AuthenticationResult) = onResult(true)

            override fun onAuthenticationError(errorCode: Int, errString: CharSequence) = onResult(false)
            // onAuthenticationFailed (sidik jari tidak cocok) tidak menutup dialog; pengguna bisa mencoba lagi.
        }
        val info = BiometricPrompt.PromptInfo.Builder()
            .setTitle(title)
            .setSubtitle(subtitle)
            .setAllowedAuthenticators(AUTHENTICATORS)
            .build()
        BiometricPrompt(activity, ContextCompat.getMainExecutor(activity), callback).authenticate(info)
    }
}
