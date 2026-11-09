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
import id.pejuang.hris.data.AttendanceRecord
import id.pejuang.hris.data.ClockOutcome
import id.pejuang.hris.data.Me
import id.pejuang.hris.data.Ref
import id.pejuang.hris.face.LivenessState
import id.pejuang.hris.face.cameraMessage
import id.pejuang.hris.ui.CameraOverlay
import id.pejuang.hris.ui.ConsentContent
import id.pejuang.hris.ui.FailedContent
import id.pejuang.hris.ui.HistoryContent
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

    private fun shot(content: @Composable () -> Unit) = paparazzi.snapshot { HrisTheme { content() } }

    @Test
    fun login() = shot {
        LoginContent(
            form = LoginForm(email = "siti@pejuang.id", password = "rahasia"),
            showServer = false,
            version = "0.4.0",
            crash = null,
            onCopyCrash = {}, onDismissCrash = {}, onServer = {}, onEmail = {}, onPassword = {}, onMfa = {}, onSubmit = {},
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
            onCopyCrash = {}, onDismissCrash = {}, onServer = {}, onEmail = {}, onPassword = {}, onMfa = {}, onSubmit = {},
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
    fun sending() = shot { ProgressContent("Absen masuk", "Memverifikasi wajah", "Mengirim foto dan lokasi. Jangan tutup aplikasi.", null) }

    @Test
    fun camera() = shot {
        val (phase, message) = cameraMessage(LivenessState.Waiting("Kedipkan mata"), shots = 1, taken = 0)
        Box(
            Modifier
                .fillMaxSize()
                .background(Brush.verticalGradient(listOf(Color(0xFF6B7280), Color(0xFF374151)))),
        ) {
            CameraOverlay(title = "Absen masuk", message = message, phase = phase, gesture = "Kedipkan mata", onClose = {})
        }
    }

    @Test
    fun resultSuccess() = shot {
        ResultContent(ClockOutcome("Absen masuk tercatat", "Lokasi: Kantor Pusat Cirebon.", false), now, onClose = {})
    }

    @Test
    fun resultReview() = shot {
        ResultContent(ClockOutcome("Absen pulang tercatat", "Jam kerja 8 jam 47 menit.", true), now.withHour(17), onClose = {})
    }

    @Test
    fun failed() = shot {
        FailedContent("Absen masuk", "Wajah tidak cocok dengan data terdaftar. Coba lagi di tempat yang lebih terang.", onRetry = {}, onClose = {})
    }

    @Test
    fun historyList() = shot { HistoryContent(rows = history, error = null, onBack = {}, onRetry = {}) }

    @Test
    fun historyEmpty() = shot { HistoryContent(rows = emptyList(), error = null, onBack = {}, onRetry = {}) }
}
