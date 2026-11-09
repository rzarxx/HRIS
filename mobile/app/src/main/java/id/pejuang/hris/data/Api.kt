package id.pejuang.hris.data

import kotlinx.serialization.json.Json
import okhttp3.Authenticator
import okhttp3.Interceptor
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.MultipartBody
import okhttp3.OkHttpClient
import okhttp3.Request
import okhttp3.RequestBody
import okhttp3.RequestBody.Companion.toRequestBody
import okhttp3.Response
import okhttp3.Route
import retrofit2.Call
import retrofit2.Retrofit
import retrofit2.converter.kotlinx.serialization.asConverterFactory
import retrofit2.http.Body
import retrofit2.http.GET
import retrofit2.http.Multipart
import retrofit2.http.POST
import retrofit2.http.Part
import retrofit2.http.Query
import java.util.concurrent.TimeUnit

interface HrisApi {
    @POST("v1/auth/mobile/login")
    suspend fun login(@Body body: MobileLoginRequest): DataResponse<MobileTokens>

    @POST("v1/auth/logout")
    suspend fun logout(): retrofit2.Response<Unit>

    @GET("v1/me")
    suspend fun me(): DataResponse<Me>

    @POST("v1/attendance/challenge")
    suspend fun challenge(@Body body: AttendanceChallengeInput): DataResponse<AttendanceChallenge>

    @Multipart
    @POST("v1/attendance/clock-in")
    suspend fun clockIn(@Part("payload") payload: RequestBody, @Part selfie: MultipartBody.Part): DataResponse<ClockInResult>

    @Multipart
    @POST("v1/attendance/clock-out")
    suspend fun clockOut(@Part("payload") payload: RequestBody, @Part selfie: MultipartBody.Part): DataResponse<ClockOutResult>

    @GET("v1/attendance")
    suspend fun history(@Query("from") from: String? = null, @Query("to") to: String? = null): DataResponse<List<AttendanceRecord>>

    @GET("v1/face/status")
    suspend fun faceStatus(): DataResponse<FaceStatus>

    @Multipart
    @POST("v1/face/enroll")
    suspend fun enroll(@Part("payload") payload: RequestBody, @Part samples: List<MultipartBody.Part>): DataResponse<FaceStatus>

    @POST("v1/consents")
    suspend fun consent(@Body body: ConsentInput): retrofit2.Response<Unit>
}

/** Dipakai terpisah dari [HrisApi] agar perpanjangan sesi tidak memicu dirinya sendiri. */
interface RefreshApi {
    @POST("v1/auth/mobile/refresh")
    fun refresh(@Body body: MobileRefreshRequest): Call<DataResponse<MobileTokens>>
}

val apiJson = Json {
    ignoreUnknownKeys = true
    encodeDefaults = true
    explicitNulls = false
}

private val jsonMedia = "application/json".toMediaType()

/** Alamat tanpa skema dianggap http (hanya build debug yang mengizinkan isian bebas). */
fun normalizeBaseUrl(raw: String): String {
    val trimmed = raw.trim()
    val withScheme = if (trimmed.startsWith("http://", true) || trimmed.startsWith("https://", true)) trimmed else "http://$trimmed"
    return if (withScheme.endsWith("/")) withScheme else "$withScheme/"
}

/** Membuat klien API untuk satu alamat server. */
class ApiClient(baseUrl: String, private val session: Session) {
    private val base = normalizeBaseUrl(baseUrl)

    private val plain = OkHttpClient.Builder()
        .connectTimeout(15, TimeUnit.SECONDS)
        .readTimeout(30, TimeUnit.SECONDS)
        .writeTimeout(30, TimeUnit.SECONDS)
        .build()

    private val refreshApi: RefreshApi = Retrofit.Builder()
        .baseUrl(base)
        .client(plain)
        .addConverterFactory(apiJson.asConverterFactory(jsonMedia))
        .build()
        .create(RefreshApi::class.java)

    val api: HrisApi = Retrofit.Builder()
        .baseUrl(base)
        .client(
            plain.newBuilder()
                .addInterceptor(AuthHeaders(session))
                .authenticator(Refresher(session, refreshApi))
                .build(),
        )
        .addConverterFactory(apiJson.asConverterFactory(jsonMedia))
        .build()
        .create(HrisApi::class.java)

    /** Memperpanjang sesi saat aplikasi dibuka kembali, sebelum access token ada di memori. */
    fun refreshNow(): Boolean = Refresher.refresh(session, refreshApi)
}

private class AuthHeaders(private val session: Session) : Interceptor {
    override fun intercept(chain: Interceptor.Chain): Response {
        val builder = chain.request().newBuilder()
        session.accessToken?.let { builder.header("Authorization", "Bearer $it") }
        // Route presensi dan wajah memeriksa perangkat dari header ini (§7.5).
        session.deviceId?.let { builder.header("X-Device-Id", it) }
        return chain.proceed(builder.build())
    }
}

/**
 * Menjawab 401 dengan memperpanjang sesi sekali lalu mengulang permintaan. Refresh token lama tidak
 * boleh dipakai ulang (F6), jadi perpanjangan dijalankan satu per satu.
 */
private class Refresher(private val session: Session, private val refreshApi: RefreshApi) : Authenticator {
    override fun authenticate(route: Route?, response: Response): Request? {
        val sent = response.request.header("Authorization") ?: return null
        if (response.priorResponse != null) return null
        synchronized(lock) {
            val current = session.accessToken
            if (current != null && sent != "Bearer $current") {
                return response.request.newBuilder().header("Authorization", "Bearer $current").build()
            }
            if (!refresh(session, refreshApi)) return null
            return response.request.newBuilder().header("Authorization", "Bearer ${session.accessToken}").build()
        }
    }

    companion object {
        private val lock = Any()

        fun refresh(session: Session, refreshApi: RefreshApi): Boolean = synchronized(lock) {
            val token = session.refreshToken ?: return false
            val result = try {
                refreshApi.refresh(MobileRefreshRequest(token)).execute()
            } catch (e: Exception) {
                return false
            }
            if (result.code() == 401) {
                // Sesi dicabut atau kedaluwarsa (30 hari, D40): harus masuk ulang.
                session.clear()
                return false
            }
            val tokens = result.body()?.data ?: return false
            session.save(tokens)
            true
        }
    }
}

fun jsonPart(body: String): RequestBody = body.toRequestBody(jsonMedia)

fun jpegPart(field: String, name: String, bytes: ByteArray): MultipartBody.Part =
    MultipartBody.Part.createFormData(field, name, bytes.toRequestBody("image/jpeg".toMediaType()))
