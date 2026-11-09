package id.pejuang.hris

import android.content.Context
import android.os.Build
import java.time.Instant
import java.time.ZoneId
import java.time.format.DateTimeFormatter

/**
 * Mencatat ringkasan crash terakhir di HP, lalu menampilkannya saat aplikasi dibuka lagi agar
 * Pejuang bisa menyalin dan mengirimkannya ke tim. Tidak berisi token atau data pribadi.
 */
object CrashLog {
    private const val PREFS = "hris_crash"
    private const val KEY = "last"

    fun install(context: Context) {
        val prefs = context.applicationContext.getSharedPreferences(PREFS, Context.MODE_PRIVATE)
        val previous = Thread.getDefaultUncaughtExceptionHandler()
        Thread.setDefaultUncaughtExceptionHandler { thread, error ->
            runCatching {
                val device = "${Build.MANUFACTURER} ${Build.MODEL} · Android ${Build.VERSION.RELEASE}"
                // commit(), bukan apply(): proses akan segera berhenti.
                prefs.edit().putString(KEY, summary(error, thread.name, BuildConfig.VERSION_NAME, device, Instant.now())).commit()
            }
            previous?.uncaughtException(thread, error)
        }
    }

    /** Mengambil lalu menghapus catatan crash terakhir. */
    fun take(context: Context): String? {
        val prefs = context.getSharedPreferences(PREFS, Context.MODE_PRIVATE)
        val value = prefs.getString(KEY, null) ?: return null
        prefs.edit().remove(KEY).apply()
        return value
    }

    private val timeFormat = DateTimeFormatter.ofPattern("yyyy-MM-dd HH:mm:ss").withZone(ZoneId.of("Asia/Jakarta"))

    fun summary(error: Throwable, thread: String, version: String, device: String, at: Instant): String {
        val root = generateSequence(error) { it.cause }.last()
        val lines = mutableListOf(
            "${timeFormat.format(at)} WIB · v$version · $device · thread $thread",
            "${error.javaClass.name}: ${error.message.orEmpty().take(300)}",
        )
        lines += error.stackTrace.take(8).map { "  at $it" }
        if (root !== error) {
            lines += "Caused by ${root.javaClass.name}: ${root.message.orEmpty().take(300)}"
            lines += root.stackTrace.take(5).map { "  at $it" }
        }
        return lines.joinToString("\n")
    }
}
