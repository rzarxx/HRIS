package id.pejuang.hris.data

import android.os.Build
import id.pejuang.hris.BuildConfig
import id.pejuang.hris.location.Fix
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import okhttp3.HttpUrl.Companion.toHttpUrlOrNull
import java.time.Instant
import java.time.temporal.ChronoUnit

/** Versi teks persetujuan biometrik dan lokasi yang ditampilkan aplikasi. */
const val CONSENT_VERSION = "2026-11"

const val PURPOSE_CLOCK_IN = "clock_in"
const val PURPOSE_CLOCK_OUT = "clock_out"
const val PURPOSE_FACE_ENROLL = "face_enroll"

/** Ringkasan hasil presensi untuk layar hasil. */
data class ClockOutcome(val title: String, val detail: String, val needsReview: Boolean)

class Repository(private val session: Session) {
    @Volatile
    private var client: ApiClient? = null

    val serverUrl: String
        get() = session.serverUrl?.takeIf { BuildConfig.SERVER_EDITABLE && it.isNotBlank() } ?: BuildConfig.API_BASE_URL

    val loggedIn: Boolean get() = session.loggedIn

    private fun api(): HrisApi = clientFor(serverUrl).api

    @Synchronized
    private fun clientFor(url: String): ApiClient {
        if (url.isBlank()) throw ApiError("NO_SERVER", "Alamat server belum diisi.")
        if (normalizeBaseUrl(url).toHttpUrlOrNull() == null) {
            throw ApiError("BAD_SERVER", "Alamat server tidak valid. Contoh: http://192.168.1.10:8080")
        }
        val existing = client
        if (existing != null && lastUrl == url) return existing
        return ApiClient(url, session).also {
            client = it
            lastUrl = url
        }
    }

    private var lastUrl: String? = null

    private suspend fun <T> call(block: suspend HrisApi.() -> T): T = withContext(Dispatchers.IO) {
        try {
            api().block()
        } catch (e: CancellationException) {
            throw e
        } catch (e: Throwable) {
            throw e.toApiError()
        }
    }

    suspend fun login(server: String, email: String, password: String, mfaCode: String?): MobileTokens {
        if (BuildConfig.SERVER_EDITABLE) session.serverUrl = server.trim()
        val tokens = call {
            login(
                MobileLoginRequest(
                    email = email.trim(),
                    password = password,
                    mfaCode = mfaCode?.trim()?.takeIf { it.isNotEmpty() },
                    device = DeviceRegisterInput(
                        deviceHash = session.deviceHash,
                        model = "${Build.MANUFACTURER} ${Build.MODEL}".take(100),
                        osVersion = "Android ${Build.VERSION.RELEASE}",
                    ),
                ),
            ).data
        }
        session.save(tokens)
        return tokens
    }

    /** Memulihkan sesi dari refresh token tersimpan. False berarti harus masuk ulang. */
    suspend fun resume(): Boolean = withContext(Dispatchers.IO) {
        if (!session.loggedIn) return@withContext false
        runCatching { clientFor(serverUrl).refreshNow() }.getOrDefault(false)
    }

    suspend fun me(): Me = call { me().data }

    suspend fun logout() {
        runCatching { call { logout() } }
        session.clear()
    }

    suspend fun challenge(purpose: String): AttendanceChallenge =
        call { challenge(AttendanceChallengeInput(purpose)).data }

    private fun signals(fix: Fix) = AttendanceSignals(isMockLocation = fix.isMock, appVersion = BuildConfig.VERSION_NAME)

    private fun now(): String = Instant.now().truncatedTo(ChronoUnit.SECONDS).toString()

    suspend fun clock(purpose: String, challenge: AttendanceChallenge, selfie: ByteArray, fix: Fix): ClockOutcome {
        val payload = apiJson.encodeToString(
            ClockInput.serializer(),
            ClockInput(
                challengeId = challenge.challengeId,
                latitude = fix.latitude,
                longitude = fix.longitude,
                accuracyM = fix.accuracyM,
                capturedAt = now(),
                signals = signals(fix),
                selfieSha256 = sha256Hex(selfie),
                livenessPassed = true,
                livenessAction = challenge.livenessAction,
            ),
        )
        val part = jpegPart("selfie", "selfie.jpg", selfie)
        return if (purpose == PURPOSE_CLOCK_IN) {
            val r = call { clockIn(jsonPart(payload), part).data }
            val late = if (r.lateMinutes > 0) "Terlambat ${r.lateMinutes} menit. " else ""
            val place = r.officeLocation?.name ?: when (r.workMode) {
                "wfa" -> "WFA"
                "field" -> "Di luar kantor"
                else -> "Kantor"
            }
            ClockOutcome("Presensi masuk tercatat", "${late}Lokasi: $place.", r.requiresReview)
        } else {
            val r = call { clockOut(jsonPart(payload), part).data }
            val hours = r.workedMinutes / 60
            val minutes = r.workedMinutes % 60
            ClockOutcome("Presensi pulang tercatat", "Jam kerja $hours jam $minutes menit.", r.requiresReview)
        }
    }

    suspend fun enroll(challenge: AttendanceChallenge, samples: List<ByteArray>, fix: Fix): FaceStatus {
        val payload = apiJson.encodeToString(
            FaceEnrollInput.serializer(),
            FaceEnrollInput(
                challengeId = challenge.challengeId,
                capturedAt = now(),
                sampleSha256 = samples.map { sha256Hex(it) },
                livenessPassed = true,
                livenessAction = challenge.livenessAction,
                consentVersion = CONSENT_VERSION,
                latitude = fix.latitude,
                longitude = fix.longitude,
                accuracyM = fix.accuracyM,
                signals = signals(fix),
            ),
        )
        val parts = samples.mapIndexed { i, bytes -> jpegPart("samples", "sampel-${i + 1}.jpg", bytes) }
        return call { enroll(jsonPart(payload), parts).data }
    }

    /** Persetujuan biometrik dicatat server saat daftar wajah; persetujuan lokasi dicatat di sini. */
    suspend fun consentLocation() {
        call { consent(ConsentInput("location", CONSENT_VERSION)) }
    }

    suspend fun history(): List<AttendanceRecord> = call { history().data }

    /** Jenis cuti aktif beserta saldo tahun ini. */
    suspend fun leaveTypes(): List<LeaveType> = call { leaveTypes().data }

    /** Pengajuan cuti/izin milik sendiri, terbaru dulu, lengkap dengan timeline. */
    suspend fun leaves(): List<LeaveRequest> = call { leaves().data }

    suspend fun createLeave(input: LeaveInput): LeaveRequest = call { createLeave(input).data }

    suspend fun cancelLeave(id: String): LeaveRequest = call { cancelLeave(id).data }

    /** Langkah persetujuan yang menunggu keputusan akun ini (izin approval:act). */
    suspend fun pendingApprovals(): List<ApprovalItem> = call { pendingApprovals().data }

    /** Detail langkah persetujuan beserta seluruh langkahnya. */
    suspend fun approval(id: String): ApprovalItem = call { approval(id).data }

    suspend fun decide(id: String, approve: Boolean, note: String?): ApprovalItem = call {
        val body = ApprovalDecision(note?.trim()?.takeIf { it.isNotEmpty() })
        if (approve) approve(id, body).data else reject(id, body).data
    }

    /** Mengganti kata sandi sendiri. Server mencabut sesi lain, sesi HP ini tetap aktif. */
    suspend fun changePassword(current: String, next: String) {
        call {
            val response = changePassword(PasswordChangeRequest(current, next))
            if (!response.isSuccessful) throw retrofit2.HttpException(response)
        }
    }
}
