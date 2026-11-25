package id.albahjah.hris.data

import android.os.Build
import id.albahjah.hris.BuildConfig
import id.albahjah.hris.location.Fix
import id.albahjah.hris.security.DeviceIdentity
import id.albahjah.hris.security.DeviceRisk
import id.albahjah.hris.security.RequestHash
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.withContext
import kotlinx.serialization.KSerializer
import kotlinx.serialization.builtins.ListSerializer
import okhttp3.HttpUrl.Companion.toHttpUrlOrNull
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.RequestBody.Companion.toRequestBody
import java.time.Instant
import java.time.temporal.ChronoUnit

/** Versi teks persetujuan biometrik dan lokasi yang ditampilkan aplikasi. */
const val CONSENT_VERSION = "2026-11"

const val PURPOSE_CLOCK_IN = "clock_in"
const val PURPOSE_CLOCK_OUT = "clock_out"
const val PURPOSE_FACE_ENROLL = "face_enroll"

/** Ringkasan hasil presensi untuk layar hasil. */
data class ClockOutcome(val title: String, val detail: String, val needsReview: Boolean)

/** Hasil memulihkan sesi saat aplikasi dibuka (D62). */
enum class ResumeState {
    /** Sesi diperpanjang server. */
    Active,

    /** Server belum bisa dihubungi; sesi tersimpan tetap dipakai dengan data cache. */
    Offline,

    /** Belum masuk, atau server menolak sesi (dicabut atau kedaluwarsa): harus masuk ulang. */
    LoggedOut,
}

/**
 * Lama waktu untuk dibaca: di bawah 60 menit "45 menit", selebihnya "1 jam 15 menit" atau "2 jam".
 * Dipakai untuk lama terlambat (D62).
 */
fun durationText(minutes: Int): String {
    if (minutes < 60) return "$minutes menit"
    val hours = minutes / 60
    val rest = minutes % 60
    return if (rest == 0) "$hours jam" else "$hours jam $rest menit"
}

class Repository(
    private val session: Session,
    val prefs: AppPrefs,
    private val identity: DeviceIdentity? = null,
    private val cache: LocalCache? = null,
    /** Ingat NIPY dan kata sandi serta masuk cepat (D64); null di tes. */
    val savedLogin: SavedLogin? = null,
) {
    private val offline = MutableStateFlow<Long?>(null)
    private val update = MutableStateFlow<AppUpdateState?>(null)
    private var versionCheckedAt = 0L

    /** Hasil pemeriksaan versi terakhir; Required juga terisi saat server menjawab 426 (D63). */
    val appUpdate: StateFlow<AppUpdateState?> = update.asStateFlow()

    /** Waktu data cache yang sedang tampil karena server belum bisa dihubungi; null bila online (D62). */
    val offlineSince: StateFlow<Long?> = offline.asStateFlow()

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
            throw ApiError("BAD_SERVER", "Mohon periksa kembali alamat server. Contoh: http://192.168.1.10:8080")
        }
        val existing = client
        if (existing != null && lastUrl == url) return existing
        return ApiClient(url, session).also {
            client = it
            lastUrl = url
        }
    }

    private var lastUrl: String? = null

    /**
     * Memuat dari server lalu menyimpan salinannya. Bila server belum bisa dihubungi, salinan terakhir
     * dipakai dan [offlineSince] terisi; galat lain (mis. ditolak server) tetap dilempar.
     */
    private suspend fun <T> cached(key: String, serializer: KSerializer<T>, block: suspend HrisApi.() -> T): T {
        try {
            val value = call(block)
            withContext(Dispatchers.IO) { cache?.put(key, serializer, value) }
            offline.value = null
            return value
        } catch (e: ApiError) {
            if (!unreachable(e)) throw e
            val saved = withContext(Dispatchers.IO) { cache?.get(key, serializer) } ?: throw e
            offline.value = saved.savedAt
            return saved.value
        }
    }

    /**
     * Server tidak terjangkau: tanpa internet, gateway 5xx tanpa body JSON (server mati atau sedang
     * deploy), atau 401 yang refresh-nya gagal karena jaringan sehingga sesi tersimpan masih utuh.
     */
    private fun unreachable(e: ApiError): Boolean =
        e.code == "NETWORK" || e.code.startsWith("HTTP_5") || (e.code == "AUTH_TOKEN_INVALID" && session.loggedIn)

    /** Profil terakhir dari cache, agar beranda langsung tampil sebelum server menjawab. */
    suspend fun cachedMe(): Me? = withContext(Dispatchers.IO) { cache?.get(KEY_ME, Me.serializer())?.value }

    suspend fun cachedHistory(): List<AttendanceRecord>? =
        withContext(Dispatchers.IO) { cache?.get(KEY_HISTORY, ListSerializer(AttendanceRecord.serializer()))?.value }

    private suspend fun <T> call(block: suspend HrisApi.() -> T): T = withContext(Dispatchers.IO) {
        try {
            api().block()
        } catch (e: CancellationException) {
            throw e
        } catch (e: Throwable) {
            val err = e.toApiError()
            if (err.code == "APP_UPDATE_REQUIRED") {
                update.value = AppUpdateState(UpdateLevel.Required, update.value?.version ?: AppVersion())
            }
            throw err
        }
    }

    /**
     * Membandingkan versi terpasang dengan versi di server (D63), paling sering sekali per jam kecuali [force].
     * Gagal dimuat (mis. offline) mengembalikan hasil terakhir, atau null bila belum pernah berhasil.
     */
    suspend fun checkAppVersion(force: Boolean = false): AppUpdateState? {
        val now = System.currentTimeMillis()
        if (!force && versionCheckedAt != 0L && now - versionCheckedAt < APP_VERSION_CHECK_MS) return update.value
        return try {
            val version = call { appVersion().data }
            versionCheckedAt = now
            AppUpdateState(updateLevel(BuildConfig.VERSION_CODE, version), version).also { update.value = it }
        } catch (e: CancellationException) {
            throw e
        } catch (e: Exception) {
            update.value
        }
    }

    suspend fun login(server: String, identifier: String, password: String, mfaCode: String?): MobileTokens {
        if (BuildConfig.SERVER_EDITABLE) session.serverUrl = server.trim()
        val tokens = call {
            login(
                MobileLoginRequest(
                    identifier = identifier.trim(),
                    password = password,
                    mfaCode = mfaCode?.trim()?.takeIf { it.isNotEmpty() },
                    device = DeviceRegisterInput(
                        deviceHash = session.deviceHash,
                        model = "${Build.MANUFACTURER} ${Build.MODEL}".take(100),
                        osVersion = "Android ${Build.VERSION.RELEASE}",
                        publicKey = identity?.publicKeyBase64(),
                    ),
                ),
            ).data
        }
        // Salinan data dari akun sebelumnya di HP ini tidak boleh tampil untuk akun yang baru masuk.
        withContext(Dispatchers.IO) { cache?.clear() }
        offline.value = null
        photoCache = null
        session.save(tokens)
        return tokens
    }

    /**
     * Memulihkan sesi dari refresh token tersimpan. Internet putus atau server sibuk tidak membuat
     * keluar (D62): sesi hanya dianggap berakhir bila server menolak refresh token.
     */
    suspend fun resume(): ResumeState = withContext(Dispatchers.IO) {
        if (!session.loggedIn) return@withContext ResumeState.LoggedOut
        val refreshed = runCatching { clientFor(serverUrl).refreshNow() }.getOrDefault(false)
        when {
            refreshed -> ResumeState.Active
            session.loggedIn -> ResumeState.Offline
            else -> {
                // Sesi ditolak server: buang salinan data agar tidak tertinggal untuk akun berikutnya.
                cache?.clear()
                ResumeState.LoggedOut
            }
        }
    }

    suspend fun me(): Me = cached(KEY_ME, Me.serializer()) { me().data }

    suspend fun logout() {
        runCatching { call { logout() } }
        session.clear()
        withContext(Dispatchers.IO) { cache?.clear() }
        offline.value = null
        // NIPY, kata sandi yang diingat, dan masuk cepat sengaja tetap ada agar bisa masuk lagi dengan cepat (D64).
        photoCache = null
    }

    suspend fun challenge(purpose: String): AttendanceChallenge =
        call { challenge(AttendanceChallengeInput(purpose)).data }

    private fun signals(fix: Fix, risk: DeviceRisk) = AttendanceSignals(
        isMockLocation = fix.isMock,
        fakeGpsAppDetected = risk.fakeGpsAppDetected,
        rootDetected = risk.rootDetected,
        emulatorDetected = risk.emulatorDetected,
        appVersion = BuildConfig.VERSION_NAME,
    )

    private fun now(): String = Instant.now().truncatedTo(ChronoUnit.SECONDS).toString()

    /**
     * Presensi masuk atau pulang. [selfie] adalah foto lurus dan [gesture] foto saat kepala menoleh,
     * yang dipakai server untuk membuktikan liveness (D50).
     */
    suspend fun clock(
        purpose: String,
        challenge: AttendanceChallenge,
        selfie: ByteArray,
        gesture: ByteArray,
        fix: Fix,
        risk: DeviceRisk,
    ): ClockOutcome {
        val capturedAt = now()
        val selfieHash = sha256Hex(selfie)
        val gestureHash = sha256Hex(gesture)
        val reqHash = runCatching {
            RequestHash.attendance(
                nonce = challenge.nonce,
                latitude = fix.latitude,
                longitude = fix.longitude,
                capturedAt = capturedAt,
                selfieSha256 = selfieHash,
                gestureSha256 = gestureHash,
            )
        }.getOrNull()
        val integrityToken = reqHash?.let { identity?.requestIntegrityToken(it) }
        val payload = apiJson.encodeToString(
            ClockInput.serializer(),
            ClockInput(
                challengeId = challenge.challengeId,
                latitude = fix.latitude,
                longitude = fix.longitude,
                accuracyM = fix.accuracyM,
                capturedAt = capturedAt,
                integrityToken = integrityToken,
                deviceProof = reqHash?.let { identity?.signProof(it) },
                attestationChain = if (integrityToken == null && reqHash != null) identity?.attestationChainBase64(reqHash) else null,
                signals = signals(fix, risk),
                selfieSha256 = selfieHash,
                livenessPassed = true,
                livenessAction = challenge.livenessAction,
                gestureSha256 = gestureHash,
            ),
        )
        val part = jpegPart("selfie", "selfie.jpg", selfie)
        val gesturePart = jpegPart("gesture", "gerakan.jpg", gesture)
        return if (purpose == PURPOSE_CLOCK_IN) {
            val r = call { clockIn(jsonPart(payload), part, gesturePart).data }
            val late = if (r.lateMinutes > 0) "Terlambat ${durationText(r.lateMinutes)}. " else ""
            val place = r.officeLocation?.name ?: when (r.workMode) {
                "wfa" -> "WFA"
                "field" -> "Di luar kantor"
                else -> "Kantor"
            }
            ClockOutcome("Presensi masuk tercatat", "${late}Lokasi: $place.", r.requiresReview)
        } else {
            val r = call { clockOut(jsonPart(payload), part, gesturePart).data }
            val hours = r.workedMinutes / 60
            val minutes = r.workedMinutes % 60
            ClockOutcome("Presensi pulang tercatat", "Jam kerja $hours jam $minutes menit.", r.requiresReview)
        }
    }

    suspend fun enroll(
        challenge: AttendanceChallenge,
        samples: List<ByteArray>,
        gesture: ByteArray,
        fix: Fix,
        risk: DeviceRisk,
    ): FaceStatus {
        val capturedAt = now()
        val sampleHashes = samples.map { sha256Hex(it) }
        val gestureHash = sha256Hex(gesture)
        val reqHash = runCatching {
            RequestHash.enrollment(
                nonce = challenge.nonce,
                latitude = fix.latitude,
                longitude = fix.longitude,
                capturedAt = capturedAt,
                consentVersion = CONSENT_VERSION,
                action = challenge.livenessAction,
                samples = sampleHashes,
                gestureSha256 = gestureHash,
            )
        }.getOrNull()
        val integrityToken = reqHash?.let { identity?.requestIntegrityToken(it) }
        val payload = apiJson.encodeToString(
            FaceEnrollInput.serializer(),
            FaceEnrollInput(
                challengeId = challenge.challengeId,
                capturedAt = capturedAt,
                integrityToken = integrityToken,
                deviceProof = reqHash?.let { identity?.signProof(it) },
                attestationChain = if (integrityToken == null && reqHash != null) identity?.attestationChainBase64(reqHash) else null,
                sampleSha256 = sampleHashes,
                gestureSha256 = gestureHash,
                livenessPassed = true,
                livenessAction = challenge.livenessAction,
                consentVersion = CONSENT_VERSION,
                latitude = fix.latitude,
                longitude = fix.longitude,
                accuracyM = fix.accuracyM,
                signals = signals(fix, risk),
            ),
        )
        val parts = samples.mapIndexed { i, bytes -> jpegPart("samples", "sampel-${i + 1}.jpg", bytes) }
        return call { enroll(jsonPart(payload), parts, jpegPart("gesture", "gerakan.jpg", gesture)).data }
    }

    /** Persetujuan biometrik dicatat server saat daftar wajah; persetujuan lokasi dicatat di sini. */
    suspend fun consentLocation() {
        call { consent(ConsentInput("location", CONSENT_VERSION)) }
    }

    suspend fun history(): List<AttendanceRecord> = cached(KEY_HISTORY, ListSerializer(AttendanceRecord.serializer())) { history().data }

    /** Jenis cuti aktif beserta hak cuti tahun ini. */
    suspend fun leaveTypes(): List<LeaveType> = cached("leave_types", ListSerializer(LeaveType.serializer())) { leaveTypes().data }

    /** Pengajuan cuti/izin milik sendiri, terbaru dulu, lengkap dengan timeline. */
    suspend fun leaves(): List<LeaveRequest> = cached("leaves", ListSerializer(LeaveRequest.serializer())) { leaves().data }

    suspend fun createLeave(input: LeaveInput): LeaveRequest = call { createLeave(input).data }

    suspend fun cancelLeave(id: String): LeaveRequest = call { cancelLeave(id).data }

    /** Langkah persetujuan yang menunggu keputusan akun ini (izin approval:act). */
    suspend fun pendingApprovals(): List<ApprovalItem> =
        cached("approvals", ListSerializer(ApprovalItem.serializer())) { pendingApprovals().data }

    /** Detail langkah persetujuan beserta seluruh langkahnya. */
    suspend fun approval(id: String): ApprovalItem = call { approval(id).data }

    suspend fun decide(id: String, approve: Boolean, note: String?): ApprovalItem = call {
        val body = ApprovalDecision(note?.trim()?.takeIf { it.isNotEmpty() })
        if (approve) approve(id, body).data else reject(id, body).data
    }

    /** Pengumuman yang sedang tayang untuk Pejuang ini; belum dibaca lebih dulu. */
    suspend fun announcements(): List<MyAnnouncement> =
        cached("announcements", ListSerializer(MyAnnouncement.serializer())) { announcements().data }

    /** Mencatat isi dibuka; server memulai timer baca sekali saja (D27). */
    suspend fun openAnnouncement(id: String): MyAnnouncement = call { openAnnouncement(id).data }

    /** Menandai sudah dibaca; ditolak READ_TOO_SOON bila jeda baca belum lewat. */
    suspend fun readAnnouncement(id: String): MyAnnouncement = call { readAnnouncement(id).data }

    /**
     * Isi lampiran pengumuman (PDF atau gambar), dibuka di dalam aplikasi. Isi lampiran tidak berubah
     * untuk id yang sama, jadi salinan lokal dipakai lebih dulu dan bisa dibuka saat offline (D62).
     */
    suspend fun attachment(announcementId: String, attachmentId: String): ByteArray {
        val key = "attachment:$attachmentId"
        withContext(Dispatchers.IO) { cache?.getBlob(key) }?.let { return it }
        val bytes = call { announcementAttachment(announcementId, attachmentId).use { it.bytes() } }
        withContext(Dispatchers.IO) { cache?.putBlob(key, bytes) }
        return bytes
    }

    /**
     * Memuat footer dari server dan menyimpannya untuk tampilan luring. Gagal dimuat tidak masalah:
     * footer terakhir atau teks bawaan tetap tampil.
     */
    suspend fun refreshBranding() {
        try {
            prefs.saveFooter(call { branding().data }.footer)
        } catch (e: CancellationException) {
            throw e
        } catch (e: Exception) {
            // Abaikan: bukan fitur inti.
        }
    }

    /** Mengganti kata sandi sendiri. Server mencabut sesi lain, sesi HP ini tetap aktif. */
    /**
     * Meminta tautan atur ulang kata sandi ke email akun (D30). Server menjawab pesan yang sama
     * baik akun ada atau tidak; tautannya dibuka di web.
     */
    suspend fun forgotPassword(server: String, identifier: String): String {
        if (BuildConfig.SERVER_EDITABLE) session.serverUrl = server.trim()
        return call { forgotPassword(PasswordForgotRequest(identifier.trim())).data.message }
    }

    suspend fun changePassword(current: String, next: String) {
        call {
            val response = changePassword(PasswordChangeRequest(current, next))
            if (!response.isSuccessful) throw retrofit2.HttpException(response)
        }
        // Kata sandi yang diingat ikut diganti agar masuk cepat tetap jalan.
        savedLogin?.updatePassword(next)
    }

    /** Mengubah nomor WA dan email sendiri. String kosong mengosongkan; ganti email wajib kata sandi. */
    suspend fun updateContact(phone: String, email: String, currentPassword: String?): Me =
        call { updateMe(MeUpdate(phone = phone, email = email, currentPassword = currentPassword?.takeIf { it.isNotEmpty() })).data }

    /** Membatalkan ganti email yang masih menunggu konfirmasi (D59). */
    suspend fun cancelEmailChange() {
        call {
            val response = cancelEmailChange()
            if (!response.isSuccessful) throw retrofit2.HttpException(response)
        }
    }

    suspend fun myFamily(): MyFamily = cached("family", MyFamily.serializer()) { myFamily().data }

    /** Mengunggah foto profil JPEG; server memotong persegi 512 px dan membuang EXIF. */
    suspend fun setPhoto(jpeg: ByteArray): Me {
        val me = call { setMyPhoto(jpeg.toRequestBody("image/jpeg".toMediaType())).data }
        photoCache = null
        return me
    }

    suspend fun deletePhoto() {
        call {
            val response = deleteMyPhoto()
            if (!response.isSuccessful) throw retrofit2.HttpException(response)
        }
        photoCache = null
    }

    /** Foto profil terakhir yang diunduh, ditandai dengan photo_updated_at agar tidak diunduh ulang. */
    @Volatile
    private var photoCache: Pair<String, ByteArray>? = null

    /** Foto profil sendiri sebagai bytes JPEG, atau null bila belum ada foto. */
    suspend fun myPhoto(me: Me): ByteArray? {
        val version = me.photoUpdatedAt ?: return null
        if (!me.hasPhoto) return null
        val key = "${me.employeeId}@$version"
        photoCache?.takeIf { it.first == key }?.let { return it.second }
        // Foto per versi tidak berubah, jadi salinan lokal dipakai lebih dulu (D62).
        val bytes = withContext(Dispatchers.IO) { cache?.getBlob("photo:$key") }
            ?: call { photo(me.employeeId).use { it.bytes() } }.also { b -> withContext(Dispatchers.IO) { cache?.putBlob("photo:$key", b) } }
        photoCache = key to bytes
        return bytes
    }

    private companion object {
        const val KEY_ME = "me"
        const val KEY_HISTORY = "history"
    }
}
