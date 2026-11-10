package id.pejuang.hris.data

import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.JsonPrimitive
import retrofit2.HttpException
import java.io.IOException

/** Kegagalan API yang sudah diterjemahkan untuk ditampilkan ke Pejuang. */
class ApiError(val code: String, message: String, val requestId: String? = null) : Exception(message)

/** Pesan untuk kode error yang perlu penjelasan lebih dari pesan server (MASTER.md §7.4). */
private val friendly = mapOf(
    "DEVICE_PENDING_APPROVAL" to "HP ini menunggu persetujuan SDM. Presensi bisa dilakukan setelah disetujui.",
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
        body?.let { errorFromBody(it) } ?: ApiError("HTTP_${code()}", "Server menjawab ${code()}. Coba lagi nanti.")
    }
    is IOException -> ApiError("NETWORK", "Tidak bisa terhubung ke server. Periksa koneksi internet.")
    else -> ApiError("UNKNOWN", message ?: "Terjadi kesalahan.")
}

/**
 * Menerjemahkan body error server. Pesan per field (VALIDATION_FAILED) ikut ditampilkan, karena
 * pesan umumnya ("Periksa kembali isian form.") tidak menjelaskan apa yang salah.
 */
fun errorFromBody(body: String): ApiError? {
    val error = runCatching { apiJson.decodeFromString(ErrorEnvelope.serializer(), body).error }.getOrNull() ?: return null
    val fields = (error.details?.get("fields") as? JsonObject)
        ?.values
        ?.mapNotNull { (it as? JsonPrimitive)?.takeIf { p -> p.isString }?.content }
        .orEmpty()
    val base = friendly[error.code] ?: error.message
    val message = when {
        fields.isNotEmpty() -> fields.joinToString("\n")
        error.code == "INTERNAL_ERROR" && error.requestId != null -> "$base (kode: ${error.requestId})"
        else -> base
    }
    return ApiError(error.code, message, error.requestId)
}
