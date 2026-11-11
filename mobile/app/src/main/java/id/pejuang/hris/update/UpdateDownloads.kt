package id.pejuang.hris.update

import android.app.DownloadManager
import android.content.Context
import android.content.Intent
import android.content.pm.PackageInfo
import android.content.pm.PackageManager
import android.net.Uri
import android.os.Build
import android.provider.Settings
import androidx.core.content.FileProvider
import androidx.core.content.pm.PackageInfoCompat
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import java.io.File
import java.io.IOException
import java.security.MessageDigest

sealed interface DownloadState {
    data object Idle : DownloadState
    data class Running(val percent: Int?) : DownloadState
    data class Ready(val info: ReleaseInfo) : DownloadState
    data class Failed(val message: String) : DownloadState
}

/** DownloadManager bertahan saat aplikasi ditutup; APK diverifikasi di penyimpanan privat. */
class UpdateDownloads(private val context: Context) {
    private val manager = context.getSystemService(DownloadManager::class.java)
    private val prefs = context.getSharedPreferences("app_updates", Context.MODE_PRIVATE)
    private val privateDir get() = File(context.filesDir, "updates")
    private val verifiedApk get() = File(privateDir, "release.apk")
    private val externalApk get() = context.getExternalFilesDir(null)?.let { File(it, "updates/release.apk") }

    fun start(info: ReleaseInfo) {
        discard()
        val target = externalApk ?: throw IOException("Penyimpanan unduhan tidak tersedia")
        if (!target.parentFile!!.exists() && !target.parentFile!!.mkdirs()) throw IOException("Penyimpanan unduhan tidak tersedia")
        if (target.exists() && !target.delete()) throw IOException("Tidak dapat membersihkan unduhan lama")
        val request = DownloadManager.Request(Uri.parse(info.url))
            .setTitle("Pembaruan HRIS Yayasan Al Bahjah")
            .setDescription("Mengunduh ${info.tag}")
            .setNotificationVisibility(DownloadManager.Request.VISIBILITY_VISIBLE_NOTIFY_COMPLETED)
            .setDestinationInExternalFilesDir(context, null, "updates/release.apk")
        val id = manager.enqueue(request)
        if (!prefs.edit().putLong("id", id).putString("tag", info.tag).putString("url", info.url)
                .putString("sha", info.sha256).putLong("size", info.size).putBoolean("verified", false).commit()) {
            manager.remove(id)
            throw IOException("Tidak dapat menyimpan status unduhan")
        }
    }

    fun currentInfo(): ReleaseInfo? {
        val tag = prefs.getString("tag", null) ?: return null
        val url = prefs.getString("url", null) ?: return null
        val sha = prefs.getString("sha", null) ?: return null
        val size = prefs.getLong("size", 0)
        if (AppVersion.parse(tag) == null || !Regex("^[0-9a-f]{64}$").matches(sha) || size !in 1..ReleaseService.MAX_APK_BYTES) return null
        return ReleaseInfo(tag, url, sha, size)
    }

    fun status(): DownloadState {
        val info = currentInfo() ?: return DownloadState.Idle
        if (prefs.getBoolean("verified", false)) return if (verifiedApk.isFile) DownloadState.Ready(info) else DownloadState.Failed("Berkas pembaruan tidak ditemukan")
        val id = prefs.getLong("id", -1)
        if (id < 0) return DownloadState.Idle
        manager.query(DownloadManager.Query().setFilterById(id)).use { cursor ->
            if (cursor == null || !cursor.moveToFirst()) return DownloadState.Failed("Unduhan tidak ditemukan. Silakan coba lagi.")
            val status = cursor.getInt(cursor.getColumnIndexOrThrow(DownloadManager.COLUMN_STATUS))
            val bytes = cursor.getLong(cursor.getColumnIndexOrThrow(DownloadManager.COLUMN_BYTES_DOWNLOADED_SO_FAR))
            val total = cursor.getLong(cursor.getColumnIndexOrThrow(DownloadManager.COLUMN_TOTAL_SIZE_BYTES))
            if (bytes > info.size || total > info.size) {
                manager.remove(id)
                return DownloadState.Failed("Ukuran unduhan tidak sesuai")
            }
            return when (status) {
                DownloadManager.STATUS_SUCCESSFUL -> DownloadState.Ready(info)
                DownloadManager.STATUS_FAILED -> DownloadState.Failed("Unduhan gagal. Periksa koneksi dan ruang penyimpanan, lalu coba lagi.")
                DownloadManager.STATUS_PENDING, DownloadManager.STATUS_PAUSED, DownloadManager.STATUS_RUNNING ->
                    DownloadState.Running(if (total > 0) ((bytes * 100 / total).coerceIn(0, 100)).toInt() else null)
                else -> DownloadState.Failed("Status unduhan tidak dikenali")
            }
        }
    }

    suspend fun verify(): File = withContext(Dispatchers.IO) {
        val info = currentInfo() ?: throw IOException("Data pembaruan tidak ditemukan")
        if (!prefs.getBoolean("verified", false)) {
            val source = externalApk?.takeIf { it.isFile } ?: throw IOException("APK belum selesai diunduh")
            if (!privateDir.exists() && !privateDir.mkdirs()) throw IOException("Penyimpanan privat tidak tersedia")
            val temporary = File(privateDir, "release.tmp.apk")
            try {
                val digest = MessageDigest.getInstance("SHA-256")
                var bytes = 0L
                source.inputStream().use { input ->
                    temporary.outputStream().use { output ->
                        val buffer = ByteArray(64 * 1024)
                        while (true) {
                            val count = input.read(buffer)
                            if (count < 0) break
                            bytes += count
                            if (bytes > info.size) throw IOException("Ukuran APK tidak sesuai")
                            digest.update(buffer, 0, count)
                            output.write(buffer, 0, count)
                        }
                    }
                }
                if (bytes != info.size || digest.digest().toHex() != info.sha256) throw IOException("Checksum APK tidak sesuai")
                verifyPackage(temporary, info)
                if (verifiedApk.exists() && !verifiedApk.delete()) throw IOException("APK lama tidak dapat diganti")
                if (!temporary.renameTo(verifiedApk)) throw IOException("APK tidak dapat disimpan")
                if (!prefs.edit().putBoolean("verified", true).commit()) throw IOException("Status verifikasi tidak dapat disimpan")
                manager.remove(prefs.getLong("id", -1))
            } finally { temporary.delete() }
        }
        verifyPackage(verifiedApk, info)
        // Pastikan berkas privat tidak berubah sebelum setiap permintaan instalasi.
        val digest = MessageDigest.getInstance("SHA-256")
        verifiedApk.inputStream().use { input ->
            val buffer = ByteArray(64 * 1024)
            while (true) { val n = input.read(buffer); if (n < 0) break; digest.update(buffer, 0, n) }
        }
        if (verifiedApk.length() != info.size || digest.digest().toHex() != info.sha256) throw IOException("Checksum APK tidak sesuai")
        verifiedApk
    }

    @Suppress("DEPRECATION")
    private fun verifyPackage(file: File, info: ReleaseInfo) {
        val pm = context.packageManager
        val flags = if (Build.VERSION.SDK_INT >= 28) PackageManager.GET_SIGNING_CERTIFICATES else PackageManager.GET_SIGNATURES
        val apk = pm.getPackageArchiveInfo(file.absolutePath, flags) ?: throw IOException("APK tidak dapat dibaca")
        val installed = pm.getPackageInfo(context.packageName, flags)
        if (apk.packageName != context.packageName || apk.versionName != info.tag.removePrefix("v") ||
            PackageInfoCompat.getLongVersionCode(apk) <= PackageInfoCompat.getLongVersionCode(installed)) throw IOException("Paket atau versi APK tidak sesuai")
        fun certs(packageInfo: PackageInfo): Set<String> {
            val signatures = if (Build.VERSION.SDK_INT >= 28) packageInfo.signingInfo?.apkContentsSigners else packageInfo.signatures
            return signatures?.map { MessageDigest.getInstance("SHA-256").digest(it.toByteArray()).toHex() }?.toSet() ?: emptySet()
        }
        val expected = certs(installed)
        if (expected.isEmpty() || certs(apk) != expected) throw IOException("Tanda tangan APK tidak cocok")
    }

    fun canInstall() = context.packageManager.canRequestPackageInstalls()
    fun settingsIntent(): Intent = Intent(Settings.ACTION_MANAGE_UNKNOWN_APP_SOURCES, Uri.parse("package:${context.packageName}"))
    fun installIntent(file: File): Intent = Intent(Intent.ACTION_VIEW).apply {
        setDataAndType(FileProvider.getUriForFile(context, "${context.packageName}.updates", file), "application/vnd.android.package-archive")
        addFlags(Intent.FLAG_GRANT_READ_URI_PERMISSION)
    }
    fun discard() {
        val id = prefs.getLong("id", -1)
        if (id >= 0) manager.remove(id)
        externalApk?.delete()
        verifiedApk.delete()
        prefs.edit().clear().commit()
    }
}

private fun ByteArray.toHex() = joinToString("") { "%02x".format(it.toInt() and 0xff) }
