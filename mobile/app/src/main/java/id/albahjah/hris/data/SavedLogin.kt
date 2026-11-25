package id.albahjah.hris.data

import android.content.Context
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue

/**
 * "Ingat NIPY dan kata sandi" serta masuk cepat dengan sidik jari, wajah, atau PIN HP (D64).
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

    /** Masuk cepat aktif; hanya berarti bila kata sandi tersimpan. */
    var quickLogin by mutableStateOf(prefs.getBoolean(KEY_QUICK, false))
        private set

    /** Tawaran masuk cepat sudah muncul setelah masuk, agar tidak berulang. */
    val quickLoginOffered: Boolean get() = prefs.getBoolean(KEY_QUICK_OFFERED, false)

    init {
        // Kunci aplikasi dari 0.14.0 diganti masuk cepat; pengaturannya tidak dipakai lagi.
        prefs.edit().remove(LEGACY_APP_LOCK).remove(LEGACY_APP_LOCK_OFFERED).apply()
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
        setQuickLogin(false)
    }

    fun setQuickLogin(on: Boolean) {
        quickLogin = on
        prefs.edit().putBoolean(KEY_QUICK, on).putBoolean(KEY_QUICK_OFFERED, true).apply()
    }

    fun markQuickLoginOffered() {
        prefs.edit().putBoolean(KEY_QUICK_OFFERED, true).apply()
    }

    /** Lupakan semuanya: dipakai "Gunakan akun lain" dan saat centang Ingat dilepas. */
    fun clear() {
        prefs.edit().remove(KEY_IDENTIFIER).remove(KEY_QUICK).remove(KEY_QUICK_OFFERED).apply()
        secure.put(KEY_PASSWORD, null)
        identifier = null
        hasPassword = false
        quickLogin = false
    }

    private companion object {
        const val KEY_IDENTIFIER = "remembered_identifier"
        const val KEY_PASSWORD = "saved_password"
        const val KEY_QUICK = "quick_login"
        const val KEY_QUICK_OFFERED = "quick_login_offered"
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
