package id.albahjah.hris.data

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable
import kotlinx.serialization.json.JsonObject

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
    @SerialName("public_key") val publicKey: String? = null,
)

// schema: MobileLoginRequest
@Serializable
data class MobileLoginRequest(
    @SerialName("identifier") val identifier: String,
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
    @SerialName("email") val email: String? = null,
    @SerialName("position") val position: String? = null,
    @SerialName("subdivision") val subdivision: Ref,
    @SerialName("division") val division: Ref,
    @SerialName("permissions") val permissions: List<String> = emptyList(),
    @SerialName("must_change_password") val mustChangePassword: Boolean = false,
    @SerialName("device_status") val deviceStatus: String? = null,
    @SerialName("face_enrolled") val faceEnrolled: Boolean = false,
    @SerialName("phone") val phone: String? = null,
    @SerialName("has_photo") val hasPhoto: Boolean = false,
    @SerialName("photo_updated_at") val photoUpdatedAt: String? = null,
    @SerialName("pending_email") val pendingEmail: String? = null,
)

// schema: MeUpdate
/** Body PATCH /v1/me. String kosong mengosongkan; null tidak dikirim sehingga nilai lama tetap. */
@Serializable
data class MeUpdate(
    @SerialName("phone") val phone: String? = null,
    @SerialName("email") val email: String? = null,
    @SerialName("current_password") val currentPassword: String? = null,
)

// schema: EmployeeChild
@Serializable
data class Child(
    @SerialName("child_order") val childOrder: Int,
    @SerialName("full_name") val fullName: String,
    @SerialName("gender") val gender: String? = null,
    @SerialName("birth_place") val birthPlace: String? = null,
    @SerialName("birth_date") val birthDate: String? = null,
    @SerialName("relation") val relation: String? = null,
    @SerialName("education") val education: String? = null,
    @SerialName("school_name") val schoolName: String? = null,
)

// schema: MyFamily
@Serializable
data class MyFamily(
    @SerialName("has_children") val hasChildren: Boolean? = null,
    @SerialName("children_count") val childrenCount: Int? = null,
    @SerialName("children") val children: List<Child> = emptyList(),
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
    @SerialName("fake_gps_app_detected") val fakeGpsAppDetected: Boolean = false,
    @SerialName("root_detected") val rootDetected: Boolean = false,
    @SerialName("emulator_detected") val emulatorDetected: Boolean = false,
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
    @SerialName("integrity_token") val integrityToken: String? = null,
    @SerialName("device_proof") val deviceProof: String? = null,
    @SerialName("attestation_chain") val attestationChain: List<String>? = null,
    @SerialName("signals") val signals: AttendanceSignals,
    @SerialName("selfie_sha256") val selfieSha256: String,
    @SerialName("liveness_passed") val livenessPassed: Boolean,
    @SerialName("liveness_action") val livenessAction: String,
    @SerialName("gesture_sha256") val gestureSha256: String,
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
    @SerialName("integrity_token") val integrityToken: String? = null,
    @SerialName("device_proof") val deviceProof: String? = null,
    @SerialName("attestation_chain") val attestationChain: List<String>? = null,
    @SerialName("sample_sha256") val sampleSha256: List<String>,
    @SerialName("gesture_sha256") val gestureSha256: String,
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

// schema: PasswordChangeRequest
@Serializable
data class PasswordChangeRequest(
    @SerialName("current_password") val currentPassword: String,
    @SerialName("new_password") val newPassword: String,
)

// schema: PasswordForgotRequest
@Serializable
data class PasswordForgotRequest(
    @SerialName("identifier") val identifier: String,
)

// schema: PasswordForgotResult
@Serializable
data class PasswordForgotResult(
    @SerialName("message") val message: String,
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

// schema: LeaveBalance
@Serializable
data class LeaveBalance(
    @SerialName("year") val year: Int,
    @SerialName("entitled_days") val entitledDays: Int,
    @SerialName("used_days") val usedDays: Int,
    @SerialName("remaining_days") val remainingDays: Int,
    @SerialName("eligible_from") val eligibleFrom: String? = null,
)

// schema: LeaveType
@Serializable
data class LeaveType(
    @SerialName("id") val id: String,
    @SerialName("code") val code: String,
    @SerialName("name") val name: String,
    @SerialName("paid") val paid: Boolean = true,
    @SerialName("quota_kind") val quotaKind: String,
    @SerialName("annual_quota_days") val annualQuotaDays: Int? = null,
    @SerialName("is_active") val isActive: Boolean = true,
    @SerialName("balance") val balance: LeaveBalance? = null,
)

// schema: LeaveRequest.leave_type
@Serializable
data class LeaveTypeRef(
    @SerialName("id") val id: String,
    @SerialName("code") val code: String,
    @SerialName("name") val name: String,
)

// schema: ApprovalStep
@Serializable
data class ApprovalStep(
    @SerialName("step_order") val stepOrder: Int,
    @SerialName("approver_role") val approverRole: String,
    @SerialName("approver_name") val approverName: String? = null,
    @SerialName("status") val status: String,
    @SerialName("note") val note: String? = null,
    @SerialName("acted_at") val actedAt: String? = null,
)

// schema: LeaveRequest
@Serializable
data class LeaveRequest(
    @SerialName("id") val id: String,
    @SerialName("employee_id") val employeeId: String,
    @SerialName("leave_type") val leaveType: LeaveTypeRef,
    @SerialName("start_date") val startDate: String,
    @SerialName("end_date") val endDate: String,
    @SerialName("days") val days: Int,
    @SerialName("reason") val reason: String,
    @SerialName("status") val status: String,
    @SerialName("created_at") val createdAt: String,
    @SerialName("decided_at") val decidedAt: String? = null,
    @SerialName("approvals") val approvals: List<ApprovalStep> = emptyList(),
)

// schema: LeaveInput
@Serializable
data class LeaveInput(
    @SerialName("leave_type_id") val leaveTypeId: String,
    @SerialName("start_date") val startDate: String,
    @SerialName("end_date") val endDate: String,
    @SerialName("reason") val reason: String,
)

// schema: ApprovalItem.requester
@Serializable
data class ApprovalRequester(
    @SerialName("employee_id") val employeeId: String,
    @SerialName("full_name") val fullName: String,
    @SerialName("subdivision_name") val subdivisionName: String = "",
)

// schema: ApprovalItem.summary
@Serializable
data class ApprovalSummary(
    @SerialName("leave_type") val leaveType: String? = null,
    @SerialName("start_date") val startDate: String? = null,
    @SerialName("end_date") val endDate: String? = null,
    @SerialName("days") val days: Int? = null,
    @SerialName("reason") val reason: String? = null,
)

// schema: ApprovalItem
@Serializable
data class ApprovalItem(
    @SerialName("id") val id: String,
    @SerialName("request_type") val requestType: String,
    @SerialName("request_id") val requestId: String,
    @SerialName("step_order") val stepOrder: Int,
    @SerialName("approver_role") val approverRole: String,
    @SerialName("status") val status: String,
    @SerialName("requester") val requester: ApprovalRequester,
    @SerialName("summary") val summary: ApprovalSummary = ApprovalSummary(),
    @SerialName("note") val note: String? = null,
    @SerialName("acted_at") val actedAt: String? = null,
    @SerialName("created_at") val createdAt: String,
    @SerialName("steps") val steps: List<ApprovalStep>? = null,
)

// schema: MyAnnouncement
@Serializable
data class MyAnnouncement(
    @SerialName("id") val id: String,
    @SerialName("title") val title: String,
    @SerialName("body") val body: String,
    @SerialName("type") val type: String,
    @SerialName("starts_at") val startsAt: String,
    @SerialName("ends_at") val endsAt: String? = null,
    @SerialName("min_read_seconds") val minReadSeconds: Int = 0,
    @SerialName("opened_at") val openedAt: String? = null,
    @SerialName("read_at") val readAt: String? = null,
    @SerialName("read_wait_seconds") val readWaitSeconds: Int = 0,
    @SerialName("attachments") val attachments: List<AnnouncementAttachment> = emptyList(),
)

// schema: AnnouncementAttachment
@Serializable
data class AnnouncementAttachment(
    @SerialName("id") val id: String,
    @SerialName("file_name") val fileName: String,
    @SerialName("content_type") val contentType: String,
    @SerialName("size_bytes") val sizeBytes: Int = 0,
)

// schema: AppVersion
@Serializable
data class AppVersion(
    @SerialName("latest_code") val latestCode: Int = 0,
    @SerialName("latest_name") val latestName: String = "",
    @SerialName("min_code") val minCode: Int = 0,
    @SerialName("store_url") val storeUrl: String = "",
    @SerialName("notes") val notes: String = "",
    @SerialName("updated_at") val updatedAt: String? = null,
)

// schema: Branding
@Serializable
data class Branding(
    @SerialName("footer") val footer: String,
    @SerialName("footer_template") val footerTemplate: String = "",
    @SerialName("is_default") val isDefault: Boolean = true,
    @SerialName("updated_at") val updatedAt: String? = null,
)

// schema: ApprovalDecision
@Serializable
data class ApprovalDecision(
    @SerialName("note") val note: String? = null,
)

// Bentuk error dari MASTER.md §7.3.
@Serializable
data class ErrorEnvelope(val error: ErrorBody)

@Serializable
data class ErrorBody(
    val code: String,
    val message: String,
    // Untuk VALIDATION_FAILED berisi {"fields": {"nama_field": "pesan"}}.
    val details: JsonObject? = null,
    @SerialName("request_id") val requestId: String? = null,
)

// schema: PayslipSummary
@Serializable
data class PayslipSummary(
    @SerialName("id") val id: String,
    @SerialName("year") val year: Int,
    @SerialName("month") val month: Int,
    @SerialName("start_date") val startDate: String,
    @SerialName("end_date") val endDate: String,
    @SerialName("status") val status: String,
    @SerialName("net") val net: String,
    @SerialName("published_at") val publishedAt: String? = null,
    @SerialName("unread") val unread: Boolean = false,
)

// schema: PayrollAttendance
@Serializable
data class PayslipAttendance(
    @SerialName("work_days") val workDays: String = "0",
    @SerialName("present_days") val presentDays: Int = 0,
    @SerialName("late_count") val lateCount: Int = 0,
    @SerialName("late_minutes") val lateMinutes: Int = 0,
    @SerialName("absent_days") val absentDays: Int = 0,
    @SerialName("leave_days") val leaveDays: Int = 0,
    @SerialName("unpaid_leave_days") val unpaidLeaveDays: Int = 0,
)

// schema: PayrollItem
@Serializable
data class PayslipItem(
    @SerialName("code") val code: String,
    @SerialName("name") val name: String,
    @SerialName("kind") val kind: String,
    @SerialName("quantity") val quantity: String? = null,
    @SerialName("rate") val rate: String? = null,
    @SerialName("amount") val amount: String,
)

// schema: Payslip
@Serializable
data class Payslip(
    @SerialName("id") val id: String,
    @SerialName("document_number") val documentNumber: String,
    @SerialName("year") val year: Int,
    @SerialName("month") val month: Int,
    @SerialName("start_date") val startDate: String,
    @SerialName("end_date") val endDate: String,
    @SerialName("status") val status: String,
    @SerialName("published_at") val publishedAt: String? = null,
    @SerialName("nip") val nip: String,
    @SerialName("full_name") val fullName: String,
    @SerialName("position") val position: String = "",
    @SerialName("division") val division: String = "",
    @SerialName("subdivision") val subdivision: String = "",
    @SerialName("attendance") val attendance: PayslipAttendance = PayslipAttendance(),
    @SerialName("items") val items: List<PayslipItem> = emptyList(),
    @SerialName("gross") val gross: String,
    @SerialName("deductions") val deductions: String,
    @SerialName("net") val net: String,
    @SerialName("net_clamped") val netClamped: Boolean = false,
)

