package id.pejuang.hris.data

import retrofit2.HttpException
import java.io.IOException

/** Kegagalan API yang sudah diterjemahkan untuk ditampilkan ke Pejuang. */
class ApiError(val code: String, message: String, val requestId: String? = null) : Exception(message)

/** Pesan untuk kode error yang perlu penjelasan lebih dari pesan server (MASTER.md §7.4). */
private val friendly = mapOf(
    "DEVICE_PENDING_APPROVAL" to "HP ini menunggu persetujuan SDM. Absen bisa dilakukan setelah disetujui.",
    "DEVICE_REVOKED" to "HP ini sudah dicabut. Hubungi SDM.",
    "DEVICE_NOT_REGISTERED" to "HP ini belum terdaftar. Keluar lalu masuk lagi.",
    "AUTH_TOKEN_INVALID" to "Sesi berakhir. Silakan masuk lagi.",
    "FACE_SERVICE_UNAVAILABLE" to "Verifikasi wajah di server sedang tidak tersedia. Coba lagi nanti.",
    "ATTENDANCE_BLOCKED" to "Presensi ditolak. Hubungi SDM bila ini terus terjadi.",
)

fun Throwable.toApiError(): ApiError = when (this) {
    is ApiError -> this
    is HttpException -> {
        val body = try {
            response()?.errorBody()?.string()
        } catch (e: IOException) {
            null
        }
        val parsed = body?.let { runCatching { apiJson.decodeFromString(ErrorEnvelope.serializer(), it).error }.getOrNull() }
        if (parsed != null) {
            ApiError(parsed.code, friendly[parsed.code] ?: parsed.message, parsed.requestId)
        } else {
            ApiError("HTTP_${code()}", "Server menjawab ${code()}. Coba lagi nanti.")
        }
    }
    is IOException -> ApiError("NETWORK", "Tidak bisa terhubung ke server. Periksa koneksi internet.")
    else -> ApiError("UNKNOWN", message ?: "Terjadi kesalahan.")
}
