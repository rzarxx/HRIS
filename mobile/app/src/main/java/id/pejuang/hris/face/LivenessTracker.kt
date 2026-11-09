package id.pejuang.hris.face

import kotlin.math.abs

/** Ringkasan satu wajah dari ML Kit. Nilai probabilitas null bila ML Kit tidak yakin. */
data class FaceSample(
    val leftEyeOpen: Float?,
    val rightEyeOpen: Float?,
    val smiling: Float?,
    val yaw: Float,
)

sealed interface LivenessState {
    data object NoFace : LivenessState
    data object ManyFaces : LivenessState
    /** Wajah terlihat; tunggu posisi lurus lalu gerakan yang diminta. */
    data class Waiting(val hint: String) : LivenessState
    data object Passed : LivenessState
}

/**
 * Memeriksa gerakan liveness dari challenge server (`liveness_action`, D42). Wajah harus terlihat
 * lurus dulu, baru melakukan gerakan, agar foto diam atau video lama tidak lolos begitu saja.
 * Server tetap memutuskan; ini hanya mencegah pengiriman sebelum gerakan dilakukan.
 */
class LivenessTracker(private val action: String) {
    private var neutralSeen = false
    private var eyesClosedSeen = false
    private var passed = false

    val prompt: String = when (action) {
        "blink" -> "Kedipkan mata"
        "turn_left" -> "Tolehkan kepala ke kiri"
        "turn_right" -> "Tolehkan kepala ke kanan"
        "smile" -> "Tersenyum"
        else -> "Lihat ke kamera"
    }

    fun update(faces: List<FaceSample>): LivenessState {
        if (passed) return LivenessState.Passed
        if (faces.isEmpty()) return LivenessState.NoFace
        if (faces.size > 1) return LivenessState.ManyFaces
        val face = faces.first()

        if (!neutralSeen) {
            val eyesOpen = (face.leftEyeOpen ?: 1f) > OPEN && (face.rightEyeOpen ?: 1f) > OPEN
            val notSmiling = action != "smile" || (face.smiling ?: 0f) < NEUTRAL_SMILE
            if (abs(face.yaw) < STRAIGHT_YAW && eyesOpen && notSmiling) {
                neutralSeen = true
                return LivenessState.Waiting(prompt)
            }
            return LivenessState.Waiting("Lihat lurus ke kamera")
        }

        passed = when (action) {
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
        return if (passed) LivenessState.Passed else LivenessState.Waiting(prompt)
    }

    companion object {
        const val OPEN = 0.6f
        const val CLOSED = 0.25f
        const val SMILE = 0.8f
        const val NEUTRAL_SMILE = 0.4f
        const val STRAIGHT_YAW = 12f
        const val TURN_YAW = 25f
    }
}
