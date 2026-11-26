package id.albahjah.hris.ui

import android.content.ContentValues
import android.content.Context
import android.content.Intent
import android.net.Uri
import android.os.Build
import android.os.Environment
import android.provider.MediaStore
import androidx.annotation.RequiresApi
import androidx.core.content.FileProvider
import java.io.File

/** Folder di cache aplikasi untuk berkas yang dibagikan; terdaftar di res/xml/file_paths.xml. */
private const val SHARED_DIR = "shared"

/**
 * Menyimpan PDF ke folder Unduhan tanpa izin penyimpanan (Android 10 ke atas, MediaStore). Android 8 dan 9
 * memakai pemilih lokasi sistem ([androidx.activity.result.contract.ActivityResultContracts.CreateDocument]).
 */
fun saveToDownloads(context: Context, fileName: String, bytes: ByteArray): Uri =
    if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.Q) saveToMediaStore(context, fileName, bytes)
    else error("Simpan langsung ke Unduhan butuh Android 10.")

@RequiresApi(Build.VERSION_CODES.Q)
private fun saveToMediaStore(context: Context, fileName: String, bytes: ByteArray): Uri {
    val resolver = context.contentResolver
    val values = ContentValues().apply {
        put(MediaStore.MediaColumns.DISPLAY_NAME, fileName)
        put(MediaStore.MediaColumns.MIME_TYPE, "application/pdf")
        put(MediaStore.MediaColumns.RELATIVE_PATH, Environment.DIRECTORY_DOWNLOADS)
        put(MediaStore.MediaColumns.IS_PENDING, 1)
    }
    val uri = resolver.insert(MediaStore.Downloads.EXTERNAL_CONTENT_URI, values)
        ?: error("Folder Unduhan tidak bisa dibuka.")
    try {
        resolver.openOutputStream(uri)?.use { it.write(bytes) } ?: error("Berkas tidak bisa ditulis.")
        values.clear()
        values.put(MediaStore.MediaColumns.IS_PENDING, 0)
        resolver.update(uri, values, null, null)
        return uri
    } catch (e: Exception) {
        resolver.delete(uri, null, null)
        throw e
    }
}

/** Menulis PDF ke lokasi yang dipilih pengguna lewat pemilih sistem. */
fun writeToUri(context: Context, uri: Uri, bytes: ByteArray) {
    context.contentResolver.openOutputStream(uri)?.use { it.write(bytes) } ?: error("Berkas tidak bisa ditulis.")
}

/**
 * Menyiapkan salinan sementara PDF di cache aplikasi untuk dibagikan lewat FileProvider. Salinan bagikan
 * sebelumnya dihapus lebih dulu agar slip lama tidak menumpuk.
 */
fun prepareShare(context: Context, fileName: String, bytes: ByteArray): Uri {
    val dir = File(context.cacheDir, SHARED_DIR).apply { mkdirs() }
    dir.listFiles()?.forEach { it.delete() }
    val file = File(dir, fileName).apply { writeBytes(bytes) }
    return FileProvider.getUriForFile(context, "${context.packageName}.files", file)
}

/** Membuka lembar bagikan Android untuk salinan dari [prepareShare]. */
fun sharePdf(context: Context, uri: Uri, title: String) {
    val send = Intent(Intent.ACTION_SEND).apply {
        type = "application/pdf"
        putExtra(Intent.EXTRA_STREAM, uri)
        putExtra(Intent.EXTRA_SUBJECT, title)
        addFlags(Intent.FLAG_GRANT_READ_URI_PERMISSION)
    }
    context.startActivity(Intent.createChooser(send, "Bagikan slip ubar").addFlags(Intent.FLAG_ACTIVITY_NEW_TASK))
}

/** Nama berkas slip, sama dengan nama unduhan dari server: Slip-Ubar-2026-11-NIPY.pdf. */
fun payslipFileName(year: Int, month: Int, nip: String): String =
    "Slip-Ubar-$year-${month.toString().padStart(2, '0')}-${nip.filter { it.isLetterOrDigit() }}.pdf"
