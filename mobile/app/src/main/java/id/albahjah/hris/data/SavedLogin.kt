package id.albahjah.hris.data

import android.content.Context
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue

/**
 * "Ingat NIPY dan kata sandi" di layar masuk (D64). Masuk cepat dengan sidik jari dihapus di D65.
 *
 * NIPY/email/WA disimpan biasa; kata sandi dienkripsi AES-GCM dengan kunci Android Keystore ([SecureStore]) dan
 * tidak pernah ditampilkan kembali. Data ini sengaja tetap ada setelah keluar agar bisa masuk lagi dengan cepat, dan
 * baru dihapus lewat "Gunakan akun lain", centang Ingat yang dilepas saat masuk, atau kata sandi yang ditolak server.
 */
class SavedLogin(context: Context, private val secure: SecureStore) {
    // Berkas yang sama dengan AppPrefs, agar NIPY yang diingat versi 0.14.0 tetap terbaca.
    private val prefs = context.getSharedPreferences("hris_prefs", Context.MODE_PRIVATE)

    var identifier by mutableStateOf(prefs.getString(KEY_IDENTIFIER, null))
        private set

    var hasPassword by mutableStateOf(secure.get(KEY_PASSWORD) != null)
        private set

    init {
        // Kunci aplikasi (0.14.0) dan masuk cepat (0.14.1) sudah dihapus; buang pengaturannya.
        prefs.edit().remove(LEGACY_APP_LOCK).remove(LEGACY_APP_LOCK_OFFERED).remove(LEGACY_QUICK).remove(LEGACY_QUICK_OFFERED).apply()
    }

    fun password(): String? = secure.get(KEY_PASSWORD)

    fun save(identifier: String, password: String) {
        val id = identifier.trim()
        prefs.edit().putString(KEY_IDENTIFIER, id).apply()
        secure.put(KEY_PASSWORD, password)
        this.identifier = id
        hasPassword = true
    }

    /** Kata sandi diganti dari aplikasi: salinan yang diingat ikut diperbarui. */
    fun updatePassword(password: String) {
        if (!hasPassword) return
        secure.put(KEY_PASSWORD, password)
    }

    /** Kata sandi tersimpan ditolak server (mis. diganti dari web): hapus, NIPY tetap diingat. */
    fun forgetPassword() {
        secure.put(KEY_PASSWORD, null)
        hasPassword = false
    }

    /** Lupakan semuanya: dipakai "Gunakan akun lain" dan saat centang Ingat dilepas. */
    fun clear() {
        prefs.edit().remove(KEY_IDENTIFIER).apply()
        secure.put(KEY_PASSWORD, null)
        identifier = null
        hasPassword = false
    }

    private companion object {
        const val KEY_IDENTIFIER = "remembered_identifier"
        const val KEY_PASSWORD = "saved_password"
        const val LEGACY_QUICK = "quick_login"
        const val LEGACY_QUICK_OFFERED = "quick_login_offered"
        const val LEGACY_APP_LOCK = "app_lock"
        const val LEGACY_APP_LOCK_OFFERED = "app_lock_offered"
    }
}

/**
 * Isi kolom kata sandi setelah pengguna mulai mengetik di atas kata sandi tersimpan yang tidak terlihat. Kata sandi
 * tersimpan tidak boleh ikut tersisa: huruf yang diketik di belakangnya menjadi awal kata sandi baru, dan menghapus
 * berarti mulai dari kosong.
 */
fun editSavedPassword(saved: String, typed: String): String =
    if (typed.length > saved.length && typed.startsWith(saved)) typed.removePrefix(saved) else ""
