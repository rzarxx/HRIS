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
 * Konfirmasi pemilik HP dengan sidik jari, wajah, atau PIN/pola HP untuk masuk cepat (D64).
 * BIOMETRIC_WEAK | DEVICE_CREDENTIAL dipakai karena kombinasi STRONG dengan PIN belum didukung Android 10.
 */
object Biometric {
    private const val AUTHENTICATORS = BIOMETRIC_WEAK or DEVICE_CREDENTIAL

    /**
     * HP punya sidik jari, wajah, atau PIN/pola yang bisa dipakai. Hanya Android 10 ke atas: di versi lebih
     * lama pustaka memakai dialog pengganti dan PIN sebagai cadangan belum didukung penuh.
     */
    fun available(context: Context): Boolean =
        Build.VERSION.SDK_INT >= Build.VERSION_CODES.Q &&
            BiometricManager.from(context).canAuthenticate(AUTHENTICATORS) == BiometricManager.BIOMETRIC_SUCCESS

    /** Menampilkan dialog sistem; [onResult] true bila berhasil. Batal atau gagal memberi false. */
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
