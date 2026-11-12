package id.pejuang.hris.face

import kotlin.math.abs

/**
 * Ringkasan satu wajah dari ML Kit. Nilai probabilitas null bila ML Kit tidak yakin. [fit] adalah
 * posisi wajah terhadap oval di layar (lihat [checkFit]).
 */
data class FaceSample(
    val leftEyeOpen: Float?,
    val rightEyeOpen: Float?,
    val smiling: Float?,
    val yaw: Float,
    val fit: FaceFit = FaceFit.Fit,
)

sealed interface LivenessState {
    data object NoFace : LivenessState
    data object ManyFaces : LivenessState
    /**
     * Wajah terlihat; tunggu posisi pas, lurus, lalu gerakan yang diminta. [aligned] berarti wajah
     * pas di oval pada frame ini; [progress] (0 sampai 1) adalah kemajuan menahan posisi.
     */
    data class Waiting(val hint: String, val aligned: Boolean = false, val progress: Float = 0f) : LivenessState
    data object Passed : LivenessState
}

/**
 * Memeriksa gerakan liveness dari challenge server (`liveness_action`, D42). Wajah harus pas di oval
 * dan lurus selama [holdFrames] frame berturut-turut dulu, baru melakukan gerakan, lalu pas dan lurus
 * lagi sebelum foto diambil, agar foto diam, video lama, atau wajah yang hanya terlihat sebagian di
 * tepi tidak lolos. Selama gerakan cukup pusat wajah yang tetap di dalam oval; wajah yang keluar oval
 * atau hilang membuat semuanya diulang dari awal. Server tetap memutuskan.
 */
class LivenessTracker(private val action: String, private val holdFrames: Int = FitRules.HOLD_FRAMES) {
    private var neutralSeen = false
    private var eyesClosedSeen = false
    private var gestureDone = false
    private var held = 0

    /**
     * Gerakan sudah terlihat dan belum dibatalkan. Saat berubah menjadi true, frame itu diambil
     * sebagai foto gerakan yang dibuktikan server (D50); saat kembali false, foto itu dibuang.
     */
    val gestureSeen: Boolean get() = gestureDone

    val prompt: String = when (action) {
        "blink" -> "Kedipkan mata"
        "turn_left" -> "Tolehkan kepala ke kiri"
        "turn_right" -> "Tolehkan kepala ke kanan"
        "smile" -> "Tersenyum"
        else -> "Lihat ke kamera"
    }

    fun update(faces: List<FaceSample>): LivenessState {
        if (faces.size != 1) {
            // Gerakan sebelum wajah hilang atau wajah tambahan muncul tidak boleh digunakan lagi.
            reset()
            return if (faces.isEmpty()) LivenessState.NoFace else LivenessState.ManyFaces
        }
        val face = faces.first()
        if (face.fit == FaceFit.Outside) {
            reset()
            return LivenessState.Waiting(FaceFit.Outside.hint)
        }

        // Foto diambil saat wajah sudah pas, lurus, dan mata terbuka lagi, bukan di tengah gerakan,
        // agar selfie mirip dengan template dan antarsampel pendaftaran saling mirip.
        if (gestureDone) {
            val ready = abs(face.yaw) < STRAIGHT_YAW && eyesOpen(face)
            return hold(face, ready, "Bagus. Sekarang lihat lurus ke kamera") { LivenessState.Passed }
        }

        if (!neutralSeen) {
            val notSmiling = action != "smile" || (face.smiling != null && face.smiling < NEUTRAL_SMILE)
            val ready = abs(face.yaw) < STRAIGHT_YAW && eyesOpen(face) && notSmiling
            return hold(face, ready, "Lihat lurus ke kamera") {
                neutralSeen = true
                LivenessState.Waiting(prompt, aligned = true)
            }
        }

        gestureDone = when (action) {
            "blink" -> {
                val left = face.leftEyeOpen
                val right = face.rightEyeOpen
                if (left != null && right != null && left < CLOSED && right < CLOSED) eyesClosedSeen = true
                eyesClosedSeen && left != null && right != null && left > OPEN && right > OPEN
            }
            // Arah toleh pada gambar kamera depan belum diuji di HP sungguhan, jadi kedua arah
            // diterima selama kepala benar-benar menoleh.
            "turn_left", "turn_right" -> abs(face.yaw) > TURN_YAW
            "smile" -> (face.smiling ?: 0f) > SMILE
            else -> false
        }
        if (!gestureDone) return LivenessState.Waiting(prompt, aligned = face.fit == FaceFit.Fit)
        held = 0
        return update(faces)
    }

    /**
     * Menghitung frame berturut-turut yang pas di oval dan [ready]. Setelah [holdFrames] frame,
     * menjalankan [done]; sebelumnya mengembalikan arahan posisi atau kemajuan menahan posisi.
     */
    private fun hold(face: FaceSample, ready: Boolean, notReadyHint: String, done: () -> LivenessState): LivenessState {
        if (face.fit != FaceFit.Fit) {
            held = 0
            return LivenessState.Waiting(face.fit.hint)
        }
        if (!ready) {
            held = 0
            return LivenessState.Waiting(notReadyHint, aligned = true)
        }
        held++
        if (held < holdFrames) {
            return LivenessState.Waiting(FaceFit.Fit.hint, aligned = true, progress = held.toFloat() / holdFrames)
        }
        held = 0
        return done()
    }

    private fun reset() {
        neutralSeen = false
        eyesClosedSeen = false
        gestureDone = false
        held = 0
    }

    private fun eyesOpen(face: FaceSample) =
        face.leftEyeOpen != null && face.rightEyeOpen != null && face.leftEyeOpen > OPEN && face.rightEyeOpen > OPEN

    companion object {
        const val OPEN = 0.6f
        const val CLOSED = 0.25f
        const val SMILE = 0.8f
        const val NEUTRAL_SMILE = 0.4f
        const val STRAIGHT_YAW = 12f
        const val TURN_YAW = 25f
    }
}
