package id.pejuang.hris.face

import kotlin.math.abs
import kotlin.math.cos
import kotlin.math.max
import kotlin.math.sin
import kotlin.math.sqrt

/** Kotak dalam piksel. Kotlin murni agar logika posisi wajah bisa dites tanpa Android. */
data class FitRect(val left: Float, val top: Float, val right: Float, val bottom: Float) {
    val width get() = right - left
    val height get() = bottom - top
    val centerX get() = (left + right) / 2
    val centerY get() = (top + bottom) / 2
}

/**
 * Oval wajah di layar kamera: 70% lebar tampilan, sedikit di atas tengah agar instruksi muat di
 * bawahnya. Dipakai bersama oleh overlay (yang digambar) dan [checkFit] (yang diperiksa).
 */
fun faceOval(viewWidth: Float, viewHeight: Float): FitRect {
    val w = viewWidth * 0.70f
    val h = w * 1.32f
    val cx = viewWidth / 2
    val cy = viewHeight * 0.40f
    return FitRect(cx - w / 2, cy - h / 2, cx + w / 2, cy + h / 2)
}

/**
 * Memetakan kotak wajah dari ML Kit ke koordinat tampilan pratinjau.
 *
 * ML Kit melaporkan kotak pada gambar yang sudah ditegakkan sesuai [rotationDegrees], jadi untuk
 * rotasi 90/270 lebar dan tinggi frame [frameWidth] x [frameHeight] bertukar. Pratinjau memakai
 * `PreviewView.ScaleType.FILL_CENTER`: gambar diperbesar sampai menutup tampilan lalu dipotong di
 * tengah. Pratinjau kamera depan dicerminkan, sedangkan frame analisis tidak, jadi [mirror] membalik
 * sumbu x agar kotak berada di tempat wajah terlihat.
 */
fun mapToView(
    box: FitRect,
    frameWidth: Int,
    frameHeight: Int,
    rotationDegrees: Int,
    viewWidth: Float,
    viewHeight: Float,
    mirror: Boolean,
): FitRect {
    val sideways = rotationDegrees % 180 != 0
    val imageWidth = (if (sideways) frameHeight else frameWidth).toFloat()
    val imageHeight = (if (sideways) frameWidth else frameHeight).toFloat()
    if (imageWidth <= 0f || imageHeight <= 0f) return box
    val scale = max(viewWidth / imageWidth, viewHeight / imageHeight)
    val dx = (viewWidth - imageWidth * scale) / 2
    val dy = (viewHeight - imageHeight * scale) / 2
    val left = if (mirror) imageWidth - box.right else box.left
    val right = if (mirror) imageWidth - box.left else box.right
    return FitRect(left * scale + dx, box.top * scale + dy, right * scale + dx, box.bottom * scale + dy)
}

/** Hasil pemeriksaan posisi wajah terhadap oval, sekaligus pesan arahan untuk Pejuang. */
enum class FaceFit(val hint: String) {
    Fit("Tahan posisi"),
    Outside("Masukkan wajah ke dalam bingkai"),
    TooSmall("Dekatkan wajah"),
    TooLarge("Jauhkan sedikit"),
    OffCenter("Geser ke tengah"),
    Tilted("Tegakkan kepala"),
}

/** Ambang "pas di oval". Nilai dalam proporsi oval, sudut dalam derajat. */
object FitRules {
    const val CENTER_X = 0.12f
    const val CENTER_Y = 0.10f
    const val WIDTH_MIN = 0.55f
    const val WIDTH_MAX = 0.85f
    const val MAX_TILT = 12f
    /** Toleransi tepi: wajah boleh menyentuh garis oval sedikit (5% jari-jari). */
    const val EDGE_TOLERANCE = 0.05f
    /** Jumlah frame berturut-turut yang harus pas sebelum dianggap stabil. */
    const val HOLD_FRAMES = 8
}

/**
 * Memeriksa apakah wajah [face] (koordinat tampilan, lihat [mapToView]) pas di [oval].
 * [roll] adalah `headEulerAngleZ`, [pitch] adalah `headEulerAngleX`.
 *
 * Urutan pemeriksaan menentukan pesan: wajah di luar bingkai dulu, lalu jarak, posisi, dan kemiringan.
 */
fun checkFit(face: FitRect, oval: FitRect, roll: Float, pitch: Float): FaceFit {
    if (ellipseDistance(face.centerX, face.centerY, oval) > 1f) return FaceFit.Outside
    val widthRatio = face.width / oval.width
    if (widthRatio < FitRules.WIDTH_MIN) return FaceFit.TooSmall
    if (widthRatio > FitRules.WIDTH_MAX) return FaceFit.TooLarge
    if (abs(face.centerX - oval.centerX) > FitRules.CENTER_X * oval.width ||
        abs(face.centerY - oval.centerY) > FitRules.CENTER_Y * oval.height
    ) {
        return FaceFit.OffCenter
    }
    if (!insideOval(face, oval)) {
        return if (face.height > oval.height * FitRules.WIDTH_MAX) FaceFit.TooLarge else FaceFit.OffCenter
    }
    if (abs(roll) >= FitRules.MAX_TILT || abs(pitch) >= FitRules.MAX_TILT) return FaceFit.Tilted
    return FaceFit.Fit
}

/** Jarak ternormalisasi titik ke pusat oval: 1 tepat di garis oval, kurang dari 1 di dalam. */
private fun ellipseDistance(x: Float, y: Float, oval: FitRect): Float {
    val nx = (x - oval.centerX) / (oval.width / 2)
    val ny = (y - oval.centerY) / (oval.height / 2)
    return sqrt(nx * nx + ny * ny)
}

/**
 * Seluruh wajah di dalam oval. Wajah berbentuk lonjong, jadi yang diperiksa elips di dalam kotak
 * wajah (16 titik di kelilingnya); sudut kotak sendiri memang berada di luar wajah.
 */
private fun insideOval(face: FitRect, oval: FitRect): Boolean {
    val limit = 1f + FitRules.EDGE_TOLERANCE
    for (i in 0 until 16) {
        val angle = i * Math.PI / 8
        val x = face.centerX + (face.width / 2) * cos(angle).toFloat()
        val y = face.centerY + (face.height / 2) * sin(angle).toFloat()
        if (ellipseDistance(x, y, oval) > limit) return false
    }
    return true
}

/**
 * Wajah yang dihitung: wajah lain baru dianggap orang kedua bila luasnya minimal 25% wajah terbesar,
 * sama dengan server (`pickFace`). Wajah kecil jauh di belakang diabaikan.
 */
fun <T> significantFaces(faces: List<T>, box: (T) -> FitRect): List<T> {
    val largest = faces.maxOfOrNull { box(it).let { b -> b.width * b.height } } ?: return emptyList()
    return faces.filter { box(it).let { b -> b.width * b.height >= 0.25f * largest } }
}

/** Ambang pemeriksaan foto hasil jepretan sebelum dikirim. */
object PhotoRules {
    /** Lebar wajah minimal terhadap oval; lebih longgar dari [FitRules.WIDTH_MIN] karena orang bisa sedikit bergerak. */
    const val WIDTH_OF_OVAL_MIN = 0.45f
    /** Batas server: lebar wajah minimal 20% foto. Di HP diberi cadangan agar tidak ditolak server. */
    const val WIDTH_OF_PHOTO_MIN = 0.22f
    /** Batas server: pusat wajah di 60% bagian tengah foto. Di HP dipersempit sedikit. */
    const val CENTER_SPAN = 0.56f
    /** Ulang foto otomatis paling banyak dua kali bila hasilnya tidak pas. */
    const val RETRIES = 2
}

/**
 * Memeriksa foto hasil jepretan (sudah ditegakkan, [photoWidth] x [photoHeight]): tepat satu wajah,
 * wajah itu di dalam oval yang terlihat di layar, cukup besar, dan di tengah foto.
 *
 * Batas lebar memakai oval, bukan lebar foto saja: pratinjau `FILL_CENTER` memotong sisi kiri-kanan
 * foto 4:3 di layar yang tinggi, sehingga wajah yang pas di oval hanya sekitar 24–38% lebar foto.
 */
fun photoFits(faces: List<FitRect>, photoWidth: Int, photoHeight: Int, viewWidth: Float, viewHeight: Float, mirror: Boolean): Boolean {
    val counted = significantFaces(faces) { it }
    if (counted.size != 1 || photoWidth <= 0 || photoHeight <= 0) return false
    val face = counted.first()
    val margin = (1 - PhotoRules.CENTER_SPAN) / 2
    if (face.width < PhotoRules.WIDTH_OF_PHOTO_MIN * photoWidth) return false
    if (face.centerX !in margin * photoWidth..(1 - margin) * photoWidth) return false
    if (face.centerY !in margin * photoHeight..(1 - margin) * photoHeight) return false
    if (viewWidth <= 0f || viewHeight <= 0f) return true
    val oval = faceOval(viewWidth, viewHeight)
    val onScreen = mapToView(face, photoWidth, photoHeight, 0, viewWidth, viewHeight, mirror)
    return ellipseDistance(onScreen.centerX, onScreen.centerY, oval) <= 1f &&
        onScreen.width >= PhotoRules.WIDTH_OF_OVAL_MIN * oval.width
}
