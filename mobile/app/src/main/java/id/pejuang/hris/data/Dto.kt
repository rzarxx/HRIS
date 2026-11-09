package id.pejuang.hris.data

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable

// Model JSON dari backend/api/openapi.yaml. Tiap kelas diberi penanda "schema:" agar tes kontrak
// di backend (internal/server/mobile_contract_test.go) memastikan setiap @SerialName ada di skema
// itu. Tambahkan penanda yang sama untuk kelas baru.

@Serializable
data class DataResponse<T>(val data: T)

// schema: DeviceRegisterInput
@Serializable
data class DeviceRegisterInput(
    @SerialName("device_hash") val deviceHash: String,
    @SerialName("model") val model: String? = null,
    @SerialName("os_version") val osVersion: String? = null,
)

// schema: MobileLoginRequest
@Serializable
data class MobileLoginRequest(
    @SerialName("email") val email: String,
    @SerialName("password") val password: String,
    @SerialName("mfa_code") val mfaCode: String? = null,
    @SerialName("device") val device: DeviceRegisterInput,
)

// schema: MobileRefreshRequest
@Serializable
data class MobileRefreshRequest(
    @SerialName("refresh_token") val refreshToken: String,
)

// schema: MobileTokens
@Serializable
data class MobileTokens(
    @SerialName("access_token") val accessToken: String,
    @SerialName("refresh_token") val refreshToken: String,
    @SerialName("expires_in") val expiresIn: Long,
    @SerialName("device_id") val deviceId: String,
    @SerialName("device_status") val deviceStatus: String,
)

// schema: Ref
@Serializable
data class Ref(
    @SerialName("id") val id: String,
    @SerialName("name") val name: String,
)

// schema: Me
@Serializable
data class Me(
    @SerialName("employee_id") val employeeId: String,
    @SerialName("full_name") val fullName: String,
    @SerialName("nip") val nip: String,
    @SerialName("position") val position: String? = null,
    @SerialName("subdivision") val subdivision: Ref,
    @SerialName("division") val division: Ref,
    @SerialName("permissions") val permissions: List<String> = emptyList(),
    @SerialName("must_change_password") val mustChangePassword: Boolean = false,
    @SerialName("device_status") val deviceStatus: String? = null,
    @SerialName("face_enrolled") val faceEnrolled: Boolean = false,
)

// schema: AttendanceChallengeInput
@Serializable
data class AttendanceChallengeInput(
    @SerialName("purpose") val purpose: String,
)

// schema: AttendanceChallenge
@Serializable
data class AttendanceChallenge(
    @SerialName("challenge_id") val challengeId: String,
    @SerialName("nonce") val nonce: String,
    @SerialName("expires_at") val expiresAt: String,
    @SerialName("liveness_action") val livenessAction: String,
)

// schema: AttendanceSignals
@Serializable
data class AttendanceSignals(
    @SerialName("is_mock_location") val isMockLocation: Boolean = false,
    @SerialName("app_version") val appVersion: String,
)

// schema: ClockInput
@Serializable
data class ClockInput(
    @SerialName("challenge_id") val challengeId: String,
    @SerialName("latitude") val latitude: Double,
    @SerialName("longitude") val longitude: Double,
    @SerialName("accuracy_m") val accuracyM: Double,
    @SerialName("captured_at") val capturedAt: String,
    @SerialName("signals") val signals: AttendanceSignals,
    @SerialName("selfie_sha256") val selfieSha256: String,
    @SerialName("liveness_passed") val livenessPassed: Boolean,
    @SerialName("liveness_action") val livenessAction: String,
)

// schema: ClockInResult.office_location
@Serializable
data class OfficeLocation(
    @SerialName("id") val id: String,
    @SerialName("name") val name: String,
)

// schema: ClockInResult
@Serializable
data class ClockInResult(
    @SerialName("work_date") val workDate: String,
    @SerialName("status") val status: String,
    @SerialName("clock_in_at") val clockInAt: String,
    @SerialName("late_minutes") val lateMinutes: Int,
    @SerialName("work_mode") val workMode: String,
    @SerialName("office_location") val officeLocation: OfficeLocation? = null,
    @SerialName("requires_review") val requiresReview: Boolean,
)

// schema: ClockOutResult
@Serializable
data class ClockOutResult(
    @SerialName("work_date") val workDate: String,
    @SerialName("clock_out_at") val clockOutAt: String,
    @SerialName("worked_minutes") val workedMinutes: Int,
    @SerialName("requires_review") val requiresReview: Boolean,
)

// schema: FaceEnrollInput
@Serializable
data class FaceEnrollInput(
    @SerialName("challenge_id") val challengeId: String,
    @SerialName("captured_at") val capturedAt: String,
    @SerialName("sample_sha256") val sampleSha256: List<String>,
    @SerialName("liveness_passed") val livenessPassed: Boolean,
    @SerialName("liveness_action") val livenessAction: String,
    @SerialName("consent_version") val consentVersion: String,
    @SerialName("latitude") val latitude: Double,
    @SerialName("longitude") val longitude: Double,
    @SerialName("accuracy_m") val accuracyM: Double,
    @SerialName("signals") val signals: AttendanceSignals,
)

// schema: FaceStatus
@Serializable
data class FaceStatus(
    @SerialName("enrolled") val enrolled: Boolean,
    @SerialName("model_version") val modelVersion: String? = null,
    @SerialName("enrolled_at") val enrolledAt: String? = null,
)

// schema: ConsentInput
@Serializable
data class ConsentInput(
    @SerialName("type") val type: String,
    @SerialName("version") val version: String,
)

// schema: AttendanceRecord
@Serializable
data class AttendanceRecord(
    @SerialName("id") val id: String,
    @SerialName("work_date") val workDate: String,
    @SerialName("status") val status: String,
    @SerialName("clock_in_at") val clockInAt: String? = null,
    @SerialName("clock_out_at") val clockOutAt: String? = null,
    @SerialName("late_minutes") val lateMinutes: Int = 0,
    @SerialName("worked_minutes") val workedMinutes: Int? = null,
    @SerialName("work_mode") val workMode: String,
    @SerialName("location_name") val locationName: String? = null,
)

// Bentuk error dari MASTER.md §7.3. Detail per field tidak dipakai aplikasi.
@Serializable
data class ErrorEnvelope(val error: ErrorBody)

@Serializable
data class ErrorBody(
    val code: String,
    val message: String,
    @SerialName("request_id") val requestId: String? = null,
)
