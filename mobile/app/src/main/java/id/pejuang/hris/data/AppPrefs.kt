package id.pejuang.hris.data

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

    private companion object {
        const val KEY_FONT = "font_scale"
        const val KEY_FOOTER = "footer"
    }
}
