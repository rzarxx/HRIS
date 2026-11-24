package id.albahjah.hris.data

import android.content.Context
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableFloatStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue

/** Pilihan ukuran huruf di Profil; dikalikan dengan ukuran huruf sistem HP. */
enum class FontSize(val label: String, val scale: Float) {
    Small("Kecil", 0.9f),
    Normal("Normal", 1f),
    Large("Besar", 1.15f),
    ExtraLarge("Sangat besar", 1.3f),
    ;

    companion object {
        fun of(scale: Float): FontSize = entries.minBy { kotlin.math.abs(it.scale - scale) }
    }
}

/**
 * Preferensi tampilan di HP ini: ukuran huruf pilihan pengguna dan footer terakhir dari server
 * (agar tetap tampil saat luring). Nilainya berupa state Compose sehingga layar ikut berubah.
 */
class AppPrefs(context: Context) {
    private val prefs = context.getSharedPreferences("hris_prefs", Context.MODE_PRIVATE)

    var fontSize by mutableStateOf(FontSize.of(prefs.getFloat(KEY_FONT, 1f)))
        private set

    var fontScale by mutableFloatStateOf(fontSize.scale)
        private set

    /** Footer jadi dari /v1/branding; null sampai pertama kali dimuat. */
    var footer by mutableStateOf(prefs.getString(KEY_FOOTER, null))
        private set

    /** NIPY, email, atau nomor WA yang diingat di layar masuk (D62). Kata sandi tidak pernah disimpan. */
    var rememberedIdentifier by mutableStateOf(prefs.getString(KEY_IDENTIFIER, null))
        private set

    /** Kunci aplikasi dengan sidik jari, wajah, atau PIN HP (D62). */
    var appLock by mutableStateOf(prefs.getBoolean(KEY_APP_LOCK, false))
        private set

    /** Tawaran mengaktifkan kunci sudah ditampilkan setelah masuk, agar tidak muncul berulang. */
    val appLockOffered: Boolean get() = prefs.getBoolean(KEY_APP_LOCK_OFFERED, false)

    fun rememberIdentifier(value: String?) {
        val clean = value?.trim()?.takeIf { it.isNotEmpty() }
        rememberedIdentifier = clean
        prefs.edit().apply { if (clean == null) remove(KEY_IDENTIFIER) else putString(KEY_IDENTIFIER, clean) }.apply()
    }

    fun updateAppLock(on: Boolean) {
        appLock = on
        prefs.edit().putBoolean(KEY_APP_LOCK, on).putBoolean(KEY_APP_LOCK_OFFERED, true).apply()
    }

    fun markAppLockOffered() {
        prefs.edit().putBoolean(KEY_APP_LOCK_OFFERED, true).apply()
    }

    /** Saat keluar kunci dimatikan dan tawarannya diulang untuk akun yang masuk berikutnya. */
    fun resetAppLock() {
        appLock = false
        prefs.edit().remove(KEY_APP_LOCK).remove(KEY_APP_LOCK_OFFERED).apply()
    }

    fun updateFontSize(size: FontSize) {
        fontSize = size
        fontScale = size.scale
        prefs.edit().putFloat(KEY_FONT, size.scale).apply()
    }

    fun saveFooter(text: String) {
        if (text.isBlank() || text == footer) return
        footer = text
        prefs.edit().putString(KEY_FOOTER, text).apply()
    }

    /** Tanggal WIB (yyyy-MM-dd) terakhir pop-up pengumuman [id] ditutup di HP ini (D54). */
    fun popupClosedOn(id: String): String? = prefs.getString(KEY_POPUP + id, null)

    fun markPopupClosed(id: String, date: String) {
        prefs.edit().putString(KEY_POPUP + id, date).apply()
    }

    private companion object {
        const val KEY_FONT = "font_scale"
        const val KEY_FOOTER = "footer"
        const val KEY_POPUP = "popup_closed_"
        const val KEY_IDENTIFIER = "remembered_identifier"
        const val KEY_APP_LOCK = "app_lock"
        const val KEY_APP_LOCK_OFFERED = "app_lock_offered"
    }
}
