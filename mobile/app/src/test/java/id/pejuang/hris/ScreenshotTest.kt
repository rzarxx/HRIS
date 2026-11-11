package id.pejuang.hris

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import app.cash.paparazzi.DeviceConfig
import app.cash.paparazzi.Paparazzi
import id.pejuang.hris.data.ApprovalItem
import id.pejuang.hris.data.ApprovalRequester
import id.pejuang.hris.data.ApprovalStep
import id.pejuang.hris.data.ApprovalSummary
import id.pejuang.hris.data.AttendanceRecord
import id.pejuang.hris.data.ClockOutcome
import id.pejuang.hris.data.FontSize
import id.pejuang.hris.data.LeaveBalance
import id.pejuang.hris.data.LeaveRequest
import id.pejuang.hris.data.LeaveType
import id.pejuang.hris.data.LeaveTypeRef
import id.pejuang.hris.data.Me
import id.pejuang.hris.data.MyAnnouncement
import id.pejuang.hris.data.Ref
import id.pejuang.hris.ui.AnnouncementDetailContent
import id.pejuang.hris.ui.AnnouncementListContent
import id.pejuang.hris.ui.AnnouncementPopupContent
import id.pejuang.hris.ui.Decision
import id.pejuang.hris.ui.FontSizeContent
import id.pejuang.hris.ui.DecisionContent
import id.pejuang.hris.ui.LeaveDetailContent
import id.pejuang.hris.ui.LeaveForm
import id.pejuang.hris.ui.LeaveFormContent
import id.pejuang.hris.ui.RequestsData
import id.pejuang.hris.ui.RequestsUi
import id.pejuang.hris.ui.RequestsView
import id.pejuang.hris.ui.SheetPreview
import java.time.LocalDate
import id.pejuang.hris.face.LivenessState
import id.pejuang.hris.face.cameraMessage
import id.pejuang.hris.ui.CameraOverlay
import id.pejuang.hris.ui.ConsentContent
import id.pejuang.hris.ui.FailedContent
import id.pejuang.hris.ui.ChangePasswordContent
import id.pejuang.hris.ui.FigmaIcons
import id.pejuang.hris.ui.FigmaPopup
import id.pejuang.hris.ui.MainTab
import id.pejuang.hris.ui.MainTabs
import id.pejuang.hris.ui.PasswordForm
import id.pejuang.hris.ui.PersonalDataContent
import id.pejuang.hris.ui.HomeContent
import id.pejuang.hris.ui.HrisTheme
import id.pejuang.hris.ui.LoginContent
import id.pejuang.hris.ui.LoginForm
import id.pejuang.hris.ui.PermissionContent
import id.pejuang.hris.ui.ProgressContent
import id.pejuang.hris.ui.ResultContent
import id.pejuang.hris.ui.Wib
import org.junit.Rule
import org.junit.Test
import java.time.ZonedDateTime

/** Merender tiap layar dengan data contoh. PNG-nya diunggah CI untuk ditinjau. */
class ScreenshotTest {
    @get:Rule
    val paparazzi = Paparazzi(
        deviceConfig = DeviceConfig.PIXEL_5,
        theme = "android:Theme.Material.Light.NoActionBar",
        maxPercentDifference = 0.1,
    )

    private val now = ZonedDateTime.of(2026, 11, 9, 7, 45, 0, 0, Wib)

    private val me = Me(
        employeeId = "e1",
        fullName = "Siti Nurhaliza Rahmawati",
        nip = "2024.07.0153",
        email = "siti.nurhaliza@pejuang.id",
        position = "Staf Keuangan",
        subdivision = Ref("s1", "Akuntansi"),
        division = Ref("d1", "Keuangan"),
        deviceStatus = "approved",
        faceEnrolled = true,
    )

    private val today = AttendanceRecord(
        id = "a0",
        workDate = "2026-11-09",
        status = "present",
        clockInAt = "2026-11-09T00:42:00Z",
        workMode = "onsite",
        locationName = "Kantor Pusat Cirebon",
    )

    private val history = listOf(
        today,
        AttendanceRecord("a1", "2026-11-06", "late", "2026-11-06T01:17:00Z", "2026-11-06T10:05:00Z", 17, 528, "onsite", "Kantor Pusat Cirebon"),
        AttendanceRecord("a2", "2026-11-05", "present", "2026-11-05T00:51:00Z", "2026-11-05T10:02:00Z", 0, 551, "wfa", null),
        AttendanceRecord("a3", "2026-11-04", "review", "2026-11-04T00:58:00Z", "2026-11-04T09:59:00Z", 0, 541, "field", null),
        AttendanceRecord("a4", "2026-11-03", "absent", null, null, 0, null, "onsite", null),
        AttendanceRecord("a5", "2026-10-31", "leave", null, null, 0, null, "onsite", null),
    )

    private val types = listOf(
        LeaveType("t1", "annual", "Cuti Tahunan", true, "statutory", null, true, LeaveBalance(2026, 12, 4, 8, "2025-07-01")),
        LeaveType("t2", "sick", "Sakit", true, "none", null, true),
        LeaveType("t3", "permit", "Izin", false, "none", null, true),
        LeaveType("t4", "maternity", "Cuti Melahirkan", true, "custom", 90, true, LeaveBalance(2026, 90, 0, 90)),
    )

    private fun step(order: Int, role: String, name: String?, status: String, note: String? = null, at: String? = null) =
        ApprovalStep(order, role, name, status, note, at)

    private val leaves = listOf(
        LeaveRequest(
            "l1", "e1", LeaveTypeRef("t1", "annual", "Cuti Tahunan"), "2026-11-16", "2026-11-18", 3,
            "Menghadiri pernikahan saudara di Cirebon.", "pending", "2026-11-09T00:10:00Z", null,
            listOf(step(1, "kadiv", "Umar Said", "approved", "Silakan, tugas sudah diserahkan.", "2026-11-09T00:30:00Z"), step(2, "sdm", null, "pending")),
        ),
        LeaveRequest(
            "l2", "e1", LeaveTypeRef("t2", "sick", "Sakit"), "2026-10-27", "2026-10-28", 2,
            "Demam, surat dokter terlampir di SDM.", "approved", "2026-10-27T01:00:00Z", "2026-10-27T06:00:00Z",
            listOf(step(1, "kadiv", "Umar Said", "approved", null, "2026-10-27T02:00:00Z"), step(2, "sdm", "Siti Aminah", "approved", "Semoga lekas sembuh.", "2026-10-27T06:00:00Z")),
        ),
        LeaveRequest(
            "l3", "e1", LeaveTypeRef("t3", "permit", "Izin"), "2026-09-14", "2026-09-14", 1,
            "Mengurus dokumen kependudukan.", "rejected", "2026-09-10T03:00:00Z", "2026-09-11T04:00:00Z",
            listOf(
                step(1, "kadiv", "Umar Said", "rejected", "Bertepatan dengan acara Maulid, mohon ajukan tanggal lain.", "2026-09-11T04:00:00Z"),
                step(2, "sdm", null, "skipped", "Pengajuan sudah ditolak."),
            ),
        ),
        LeaveRequest(
            "l4", "e1", LeaveTypeRef("t1", "annual", "Cuti Tahunan"), "2026-08-28", "2026-09-02", 4,
            "Libur keluarga.", "cancelled", "2026-08-01T03:00:00Z", "2026-08-02T03:00:00Z",
            listOf(step(1, "kadiv", null, "skipped", "Pengajuan dibatalkan."), step(2, "sdm", null, "skipped", "Pengajuan dibatalkan.")),
        ),
    )

    private val pending = listOf(
        ApprovalItem(
            "p1", "leave", "x1", 1, "kadiv", "pending", ApprovalRequester("e7", "Ahmad Fauzi", "Pengajaran"),
            ApprovalSummary("Cuti Tahunan", "2026-11-23", "2026-11-25", 3, "Umrah bersama orang tua, sudah koordinasi dengan wali kelas pengganti."),
            createdAt = "2026-11-08T02:00:00Z",
        ),
        ApprovalItem(
            "p2", "leave", "x2", 1, "kadiv", "pending", ApprovalRequester("e8", "Nur Aisyah", "Keuangan"),
            ApprovalSummary("Sakit", "2026-11-09", "2026-11-09", 1, "Demam sejak semalam."),
            createdAt = "2026-11-09T00:20:00Z",
        ),
    )

    private val requestsData = RequestsData(types = types, leaves = leaves)

    private val announcements = listOf(
        MyAnnouncement(
            "n1", "Selamat datang di HRIS Pejuang",
            "Mulai bulan ini presensi, pengajuan cuti, dan slip ubar dapat diakses lewat aplikasi HRIS Pejuang. " +
                "Bila ada kendala, hubungi tim SDM di ruang administrasi.",
            "popup", "2026-11-04T01:00:00Z", "2026-12-04T01:00:00Z", 10, readWaitSeconds = 10,
        ),
        MyAnnouncement(
            "n2", "Briefing keselamatan kerja",
            "Seluruh Pejuang divisi Operasional wajib mengikuti briefing K3 setiap Senin pukul 07.45 WIB di halaman gudang.",
            "banner", "2026-10-28T01:00:00Z", null, 10, readWaitSeconds = 10,
        ),
        MyAnnouncement(
            "n3", "Upacara Hari Pahlawan", "Tanggal 10 November tetap masuk kerja seperti biasa. Upacara singkat pukul 08.00 WIB.",
            "banner", "2026-11-06T01:00:00Z", "2026-11-11T01:00:00Z", 10,
            openedAt = "2026-11-06T02:00:00Z", readAt = "2026-11-06T02:00:15Z",
        ),
    )

    private val footer = "© 2026 PUTI (Pusat Teknologi Informasi) Al Bahjah"

    private fun shot(fontScale: Float = 1f, content: @Composable () -> Unit) = paparazzi.snapshot { HrisTheme(fontScale) { content() } }

    @Test
    fun login() = shot {
        LoginContent(
            form = LoginForm(email = "siti@pejuang.id", password = "rahasia"),
            showServer = false,
            version = "0.4.0",
            crash = null,
            onCopyCrash = {}, onDismissCrash = {}, onServer = {}, onEmail = {}, onPassword = {}, onMfa = {}, onSubmit = {}, footer = footer,
        )
    }

    @Test
    fun loginDebugWithError() = shot {
        LoginContent(
            form = LoginForm(
                server = "http://192.168.1.10:8080",
                email = "siti@pejuang.id",
                password = "salah",
                askMfa = true,
                error = "Email atau kata sandi salah.",
            ),
            showServer = true,
            version = "0.4.0",
            crash = "2026-11-09 07:40:00 WIB · v0.2.0\njava.lang.IllegalStateException: contoh",
            onCopyCrash = {}, onDismissCrash = {}, onServer = {}, onEmail = {}, onPassword = {}, onMfa = {}, onSubmit = {}, footer = footer,
        )
    }

    @Test
    fun homeClockedIn() = shot {
        HomeContent(
            me = me, today = today, now = now, refreshing = false, crash = null,
            onCopyCrash = {}, onDismissCrash = {}, onCapture = {}, onHistory = {}, onRefresh = {}, onLogout = {},
        )
    }

    @Test
    fun homeNewDevice() = shot {
        HomeContent(
            me = me.copy(deviceStatus = "pending", faceEnrolled = false), today = null, now = now.withHour(19),
            refreshing = false, crash = null,
            onCopyCrash = {}, onDismissCrash = {}, onCapture = {}, onHistory = {}, onRefresh = {}, onLogout = {},
        )
    }

    @Test
    fun consent() = shot { ConsentContent(onAccept = {}, onCancel = {}) }

    @Test
    fun permissionDenied() = shot { PermissionContent(denied = true, onRequest = {}, onOpenSettings = {}, onCancel = {}) }

    @Test
    fun sending() = shot { ProgressContent("Presensi masuk", "Memverifikasi wajah", "Mengirim foto dan lokasi. Jangan tutup aplikasi.", null) }

    @Test
    fun camera() = shot {
        val (phase, message) = cameraMessage(LivenessState.Waiting("Kedipkan mata"), shots = 1, taken = 0)
        Box(
            Modifier
                .fillMaxSize()
                .background(Brush.verticalGradient(listOf(Color(0xFF6B7280), Color(0xFF374151)))),
        ) {
            CameraOverlay(title = "Presensi masuk", message = message, phase = phase, gesture = "Kedipkan mata", onClose = {})
        }
    }

    @Test
    fun resultSuccess() = shot {
        ResultContent(ClockOutcome("Presensi masuk tercatat", "Lokasi: Kantor Pusat Cirebon.", false), now, onClose = {})
    }

    @Test
    fun resultReview() = shot {
        ResultContent(ClockOutcome("Presensi pulang tercatat", "Jam kerja 8 jam 47 menit.", true), now.withHour(17), onClose = {})
    }

    @Test
    fun failed() = shot {
        FailedContent("Presensi masuk", "Wajah tidak cocok dengan data terdaftar. Coba lagi di tempat yang lebih terang.", onRetry = {}, onClose = {})
    }

    private fun tabs(
        tab: MainTab,
        me: Me = this.me,
        rows: List<AttendanceRecord>? = history,
        error: String? = null,
        popup: Boolean = false,
        requests: RequestsUi = RequestsUi(data = requestsData),
    ) = shot {
        Box(Modifier.fillMaxSize()) {
            MainTabs(
                tab = tab, onTab = {}, me = me, records = rows, historyError = error, now = now, refreshing = false, crash = null,
                onCopyCrash = {}, onDismissCrash = {}, onCapture = {}, onRefresh = {}, onPersonalData = {}, onChangePassword = {}, onLogout = {},
                requests = requests,
            )
            if (popup) {
                FigmaPopup(
                    icon = FigmaIcons.Logout, title = "Keluar dari aplikasi?",
                    body = "Anda perlu masuk lagi dengan email dan kata sandi untuk presensi berikutnya.",
                    primary = "Ya, keluar", onPrimary = {}, secondary = "Batal", danger = true,
                )
            }
        }
    }

    @Test
    fun tabHome() = tabs(MainTab.Home)

    @Test
    fun tabAttendanceClockedIn() = tabs(MainTab.Attendance)

    @Test
    fun tabAttendanceNewDevice() = tabs(MainTab.Attendance, me.copy(deviceStatus = "pending", faceEnrolled = false), rows = emptyList())

    @Test
    fun tabAttendanceError() = tabs(MainTab.Attendance, rows = null, error = "Tidak bisa terhubung ke server. Periksa koneksi internet.")

    @Test
    fun tabRequests() = tabs(MainTab.Requests)

    @Test
    fun tabRequestsLoading() = tabs(MainTab.Requests, requests = RequestsUi(data = RequestsData(refreshing = true)))

    @Test
    fun tabRequestsEmptyNewEmployee() = tabs(
        MainTab.Requests,
        requests = RequestsUi(
            data = RequestsData(
                types = listOf(types[0].copy(balance = LeaveBalance(2026, 0, 0, 0, "2027-07-01")), types[1], types[2]),
                leaves = emptyList(),
            ),
        ),
    )

    @Test
    fun tabRequestsError() = tabs(
        MainTab.Requests,
        requests = RequestsUi(data = RequestsData(error = "Tidak bisa terhubung ke server. Periksa koneksi internet.")),
    )

    @Test
    fun tabRequestsApproverMine() = tabs(
        MainTab.Requests,
        requests = RequestsUi(data = requestsData.copy(pending = pending), canApprove = true),
    )

    @Test
    fun tabRequestsInbox() = tabs(
        MainTab.Requests,
        requests = RequestsUi(data = requestsData.copy(pending = pending), canApprove = true, view = RequestsView.Inbox),
    )

    @Test
    fun tabRequestsInboxEmpty() = tabs(
        MainTab.Requests,
        requests = RequestsUi(data = requestsData.copy(pending = emptyList()), canApprove = true, view = RequestsView.Inbox),
    )

    @Test
    fun fontSizeScreen() = shot { FontSizeContent(FontSize.Large, onPick = {}, onBack = {}) }

    @Test
    fun homeExtraLargeFont() = shot(FontSize.ExtraLarge.scale) {
        Box(Modifier.fillMaxSize()) {
            MainTabs(
                tab = MainTab.Home, onTab = {}, me = me, records = history, historyError = null, now = now, refreshing = false, crash = null,
                onCopyCrash = {}, onDismissCrash = {}, onCapture = {}, onRefresh = {}, onPersonalData = {}, onChangePassword = {}, onLogout = {},
                announcements = announcements, footer = footer,
            )
        }
    }

    @Test
    fun profileSmallFont() = shot(FontSize.Small.scale) {
        Box(Modifier.fillMaxSize()) {
            MainTabs(
                tab = MainTab.Profile, onTab = {}, me = me, records = history, historyError = null, now = now, refreshing = false, crash = null,
                onCopyCrash = {}, onDismissCrash = {}, onCapture = {}, onRefresh = {}, onPersonalData = {}, onChangePassword = {}, onLogout = {},
                footer = footer, fontSize = FontSize.Small,
            )
        }
    }

    @Test
    fun homeWithAnnouncements() = shot {
        Box(Modifier.fillMaxSize()) {
            MainTabs(
                tab = MainTab.Home, onTab = {}, me = me, records = history, historyError = null, now = now, refreshing = false, crash = null,
                onCopyCrash = {}, onDismissCrash = {}, onCapture = {}, onRefresh = {}, onPersonalData = {}, onChangePassword = {}, onLogout = {},
                announcements = announcements,
            )
        }
    }

    @Test
    fun announcementPopupCountdown() = shot {
        Box(Modifier.fillMaxSize()) {
            MainTabs(
                tab = MainTab.Home, onTab = {}, me = me, records = history, historyError = null, now = now, refreshing = false, crash = null,
                onCopyCrash = {}, onDismissCrash = {}, onCapture = {}, onRefresh = {}, onPersonalData = {}, onChangePassword = {}, onLogout = {},
                announcements = announcements,
            )
            AnnouncementPopupContent(announcements[0], wait = 7, busy = false, error = null, onRead = {}, onLater = {})
        }
    }

    @Test
    fun announcementPopupReady() = shot {
        Box(Modifier.fillMaxSize()) {
            AnnouncementPopupContent(announcements[0], wait = 0, busy = false, error = null, onRead = {}, onLater = {})
        }
    }

    @Test
    fun announcementList() = shot { AnnouncementListContent(announcements, onBack = {}, onOpen = {}) }

    @Test
    fun announcementListEmpty() = shot { AnnouncementListContent(emptyList(), onBack = {}, onOpen = {}) }

    @Test
    fun announcementDetailWaiting() = shot {
        AnnouncementDetailContent(announcements[1], wait = 4, busy = false, error = null, onBack = {}, onRead = {})
    }

    @Test
    fun announcementDetailRead() = shot {
        AnnouncementDetailContent(announcements[2], wait = 0, busy = false, error = null, onBack = {}, onRead = {})
    }

    @Test
    fun tabHomeApprover() = tabs(MainTab.Home, requests = RequestsUi(data = requestsData.copy(pending = pending), canApprove = true))

    @Test
    fun leaveDetailPending() = shot { LeaveDetailContent(leaves[0], "Anda", cancelling = false, onBack = {}, onCancel = {}) }

    @Test
    fun leaveDetailRejected() = shot { LeaveDetailContent(leaves[2], "Anda", cancelling = false, onBack = {}, onCancel = {}) }

    @Test
    fun leaveFormFilled() = shot {
        SheetPreview {
            LeaveFormContent(
                form = LeaveForm(typeId = "t1", start = LocalDate.of(2026, 11, 16), end = LocalDate.of(2026, 11, 18), reason = "Menghadiri pernikahan saudara di Cirebon."),
                types = types, onType = {}, onPickDates = {}, onReason = {}, onSubmit = {}, onCancel = {},
            )
        }
    }

    @Test
    fun leaveFormErrors() = shot {
        SheetPreview {
            LeaveFormContent(
                form = LeaveForm(
                    typeId = "t1", start = LocalDate.of(2026, 11, 16), end = LocalDate.of(2026, 11, 27),
                    errors = mapOf("leave_type_id" to "Sisa Cuti Tahunan tahun 2026 tinggal 8 hari, pengajuan ini 10 hari kerja.", "reason" to "Alasan wajib diisi, minimal 3 karakter."),
                ),
                types = types, onType = {}, onPickDates = {}, onReason = {}, onSubmit = {}, onCancel = {},
            )
        }
    }

    @Test
    fun decisionReject() = shot {
        SheetPreview {
            DecisionContent(
                decision = Decision(pending[0], approve = false), note = "", busy = false,
                error = "Tulis alasan penolakan, minimal 3 karakter. Alasan ini dibaca pemohon.",
                steps = listOf(step(1, "kadiv", "Hasan Basri", "pending"), step(2, "sdm", null, "waiting")),
                onNote = {}, onSubmit = {}, onSwitch = {}, onCancel = {},
            )
        }
    }

    @Test
    fun decisionApprove() = shot {
        SheetPreview {
            DecisionContent(
                decision = Decision(pending[1], approve = true), note = "Semoga lekas sembuh.", busy = false, error = null,
                steps = null, onNote = {}, onSubmit = {}, onSwitch = {}, onCancel = {},
            )
        }
    }

    @Test
    fun tabProfile() = tabs(MainTab.Profile)

    @Test
    fun logoutPopup() = tabs(MainTab.Profile, popup = true)

    @Test
    fun personalData() = shot { PersonalDataContent(me, onBack = {}) }

    @Test
    fun changePasswordError() = shot {
        ChangePasswordContent(
            form = PasswordForm(current = "lama-sekali", next = "pendek", repeat = "beda", error = "Kata sandi saat ini salah."),
            onCurrent = {}, onNext = {}, onRepeat = {}, onSubmit = {}, onBack = {}, onFinished = {},
        )
    }

    @Test
    fun changePasswordDone() = shot {
        ChangePasswordContent(form = PasswordForm(done = true), onCurrent = {}, onNext = {}, onRepeat = {}, onSubmit = {}, onBack = {}, onFinished = {})
    }
}
