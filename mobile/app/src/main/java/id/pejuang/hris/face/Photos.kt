package id.pejuang.hris.face

import android.graphics.Bitmap
import android.graphics.Matrix
import java.io.ByteArrayOutputStream
import kotlin.math.max
import kotlin.math.roundToInt

/** Sisi terpanjang selfie yang dikirim, sesuai saran server (sekitar 640 px, maks. 1 MB). */
const val SELFIE_LONGEST_SIDE = 640

/** Ukuran hasil perkecil; tidak pernah memperbesar. */
fun scaledSize(width: Int, height: Int, longest: Int = SELFIE_LONGEST_SIDE): Pair<Int, Int> {
    val current = max(width, height)
    if (current <= longest) return width to height
    val scale = longest.toFloat() / current
    return max((width * scale).roundToInt(), 1) to max((height * scale).roundToInt(), 1)
}

/**
 * Menegakkan foto sesuai rotasi kamera, memperkecil, lalu menyimpan ulang sebagai JPEG tanpa EXIF.
 * Server tidak membaca orientasi EXIF (D43), jadi foto harus sudah tegak di piksel.
 */
fun toUprightJpeg(source: Bitmap, rotationDegrees: Int, quality: Int = 85): ByteArray {
    val (w, h) = scaledSize(source.width, source.height)
    val matrix = Matrix().apply {
        postScale(w.toFloat() / source.width, h.toFloat() / source.height)
        postRotate(rotationDegrees.toFloat())
    }
    val upright = Bitmap.createBitmap(source, 0, 0, source.width, source.height, matrix, true)
    val out = ByteArrayOutputStream()
    upright.compress(Bitmap.CompressFormat.JPEG, quality, out)
    if (upright !== source) upright.recycle()
    return out.toByteArray()
}
