package id.albahjah.hris.ui

import android.content.Context
import android.graphics.Bitmap
import android.graphics.BitmapFactory
import android.graphics.ImageDecoder
import android.net.Uri
import android.os.Build
import androidx.compose.runtime.compositionLocalOf
import androidx.compose.ui.graphics.ImageBitmap
import androidx.compose.ui.graphics.asImageBitmap
import java.io.ByteArrayOutputStream
import kotlin.math.max
import kotlin.math.min

/** Foto profil sendiri yang sedang tampil; disediakan MainScreen agar Beranda dan Profil memakai foto yang sama. */
val LocalMyPhoto = compositionLocalOf<ImageBitmap?> { null }

/** Sisi foto profil yang dikirim. Server memperkecilnya lagi ke 512 px. */
const val PROFILE_PHOTO_SIDE = 1024

/** Kotak persegi di tengah gambar: (kiri, atas, sisi). */
fun centerSquare(width: Int, height: Int): Triple<Int, Int, Int> {
    val side = min(width, height)
    return Triple((width - side) / 2, (height - side) / 2, side)
}

/** Faktor inSampleSize terbesar (pangkat dua) yang hasilnya tetap paling sedikit [target] px di sisi pendek. */
fun sampleSizeFor(width: Int, height: Int, target: Int = PROFILE_PHOTO_SIDE): Int {
    var sample = 1
    while (min(width, height) / (sample * 2) >= target) sample *= 2
    return sample
}

/**
 * Membaca foto dari galeri, menegakkannya sesuai orientasi EXIF (ImageDecoder, Android 9 ke atas),
 * memotong persegi di tengah, memperkecil, lalu menyimpan ulang sebagai JPEG tanpa metadata.
 */
fun prepareProfilePhoto(context: Context, uri: Uri): ByteArray {
    val source = decodeUpright(context, uri)
    val (left, top, side) = centerSquare(source.width, source.height)
    val target = min(side, PROFILE_PHOTO_SIDE)
    val square = Bitmap.createBitmap(source, left, top, side, side)
    val scaled = if (target == side) square else Bitmap.createScaledBitmap(square, target, target, true)
    val out = ByteArrayOutputStream()
    scaled.compress(Bitmap.CompressFormat.JPEG, 88, out)
    listOf(source, square, scaled).distinct().forEach { it.recycle() }
    return out.toByteArray()
}

private fun decodeUpright(context: Context, uri: Uri): Bitmap {
    if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.P) {
        val src = ImageDecoder.createSource(context.contentResolver, uri)
        return ImageDecoder.decodeBitmap(src) { decoder, info, _ ->
            decoder.setTargetSampleSize(sampleSizeFor(info.size.width, info.size.height))
            decoder.allocator = ImageDecoder.ALLOCATOR_SOFTWARE
        }
    }
    val bounds = BitmapFactory.Options().apply { inJustDecodeBounds = true }
    context.contentResolver.openInputStream(uri)?.use { BitmapFactory.decodeStream(it, null, bounds) }
    val options = BitmapFactory.Options().apply { inSampleSize = sampleSizeFor(max(bounds.outWidth, 1), max(bounds.outHeight, 1)) }
    return context.contentResolver.openInputStream(uri)?.use { BitmapFactory.decodeStream(it, null, options) }
        ?: error("Foto tidak bisa dibaca")
}

/** Bytes JPEG dari server menjadi gambar Compose; null bila tidak bisa dibaca. */
fun decodePhoto(bytes: ByteArray?): ImageBitmap? =
    bytes?.let { BitmapFactory.decodeByteArray(it, 0, it.size) }?.asImageBitmap()
