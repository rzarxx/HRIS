package id.pejuang.hris

import id.pejuang.hris.data.AttendanceSignals
import id.pejuang.hris.data.ClockInput
import id.pejuang.hris.data.DeviceRegisterInput
import id.pejuang.hris.data.MobileLoginRequest
import id.pejuang.hris.data.apiJson
import id.pejuang.hris.data.normalizeBaseUrl
import id.pejuang.hris.data.sha256Hex
import id.pejuang.hris.face.FaceSample
import id.pejuang.hris.face.LivenessState
import id.pejuang.hris.face.LivenessTracker
import id.pejuang.hris.face.scaledSize
import id.pejuang.hris.location.roundCoordinate
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

class LivenessTrackerTest {
    private fun face(yaw: Float = 0f, eyes: Float? = 0.9f, smile: Float? = 0.1f) =
        FaceSample(leftEyeOpen = eyes, rightEyeOpen = eyes, smiling = smile, yaw = yaw)

    @Test
    fun blinkNeedsOpenClosedOpen() {
        val t = LivenessTracker("blink")
        assertTrue(t.update(listOf(face())) is LivenessState.Waiting)
        assertTrue(t.update(listOf(face(eyes = 0.1f))) is LivenessState.Waiting)
        assertEquals(LivenessState.Passed, t.update(listOf(face(eyes = 0.9f))))
    }

    @Test
    fun closedEyesFromStartDoNotPass() {
        val t = LivenessTracker("blink")
        repeat(3) { assertTrue(t.update(listOf(face(eyes = 0.1f))) is LivenessState.Waiting) }
    }

    @Test
    fun turnNeedsStraightFirst() {
        val t = LivenessTracker("turn_left")
        // Kepala sudah menoleh sejak awal: belum dianggap gerakan.
        assertTrue(t.update(listOf(face(yaw = 40f))) is LivenessState.Waiting)
        t.update(listOf(face(yaw = 2f)))
        // Gerakan selesai, tetapi foto baru diambil setelah wajah lurus kembali.
        assertTrue(t.update(listOf(face(yaw = -30f))) is LivenessState.Waiting)
        assertTrue(t.update(listOf(face(yaw = 20f))) is LivenessState.Waiting)
        assertEquals(LivenessState.Passed, t.update(listOf(face(yaw = 3f))))
    }

    @Test
    fun passedFaceStillMustBeAloneAndOpenEyed() {
        val t = LivenessTracker("smile")
        t.update(listOf(face()))
        assertEquals(LivenessState.Passed, t.update(listOf(face(smile = 0.9f))))
        assertEquals(LivenessState.ManyFaces, t.update(listOf(face(), face())))
        assertTrue(t.update(listOf(face(eyes = 0.1f))) is LivenessState.Waiting)
    }

    @Test
    fun smileNeedsNeutralFirst() {
        val t = LivenessTracker("smile")
        assertTrue(t.update(listOf(face(smile = 0.95f))) is LivenessState.Waiting)
        t.update(listOf(face(smile = 0.1f)))
        assertEquals(LivenessState.Passed, t.update(listOf(face(smile = 0.9f))))
    }

    @Test
    fun noFaceAndManyFaces() {
        val t = LivenessTracker("blink")
        assertEquals(LivenessState.NoFace, t.update(emptyList()))
        assertEquals(LivenessState.ManyFaces, t.update(listOf(face(), face())))
    }

    @Test
    fun interruptedGestureMustStartOver() {
        for (interruption in listOf(emptyList(), listOf(face(), face()))) {
            val t = LivenessTracker("blink")
            t.update(listOf(face()))
            t.update(listOf(face(eyes = 0.1f)))
            t.update(interruption)
            // Membuka mata setelah interupsi hanya menetapkan posisi awal baru.
            assertTrue(t.update(listOf(face())) is LivenessState.Waiting)
            assertTrue(t.update(listOf(face())) is LivenessState.Waiting)
            t.update(listOf(face(eyes = 0.1f)))
            assertEquals(LivenessState.Passed, t.update(listOf(face())))
        }
    }

    @Test
    fun interruptionAfterGestureRequiresNewGesture() {
        val t = LivenessTracker("turn_left")
        t.update(listOf(face()))
        t.update(listOf(face(yaw = 30f)))
        t.update(emptyList())
        assertTrue(t.update(listOf(face())) is LivenessState.Waiting)
        assertTrue(t.update(listOf(face())) is LivenessState.Waiting)
    }

    @Test
    fun unknownClassificationDoesNotEstablishNeutralOrFinishCapture() {
        val t = LivenessTracker("smile")
        t.update(listOf(face(eyes = null)))
        t.update(listOf(face(smile = 0.9f)))
        assertTrue(t.update(listOf(face(smile = 0.9f))) is LivenessState.Waiting)

        t.update(listOf(face(smile = null)))
        assertTrue(t.update(listOf(face(smile = 0.9f))) is LivenessState.Waiting)
        t.update(listOf(face()))
        assertTrue(t.update(listOf(face(eyes = null, smile = 0.9f))) is LivenessState.Waiting)
        assertEquals(LivenessState.Passed, t.update(listOf(face())))
    }
}

class DataTest {
    @Test
    fun validationFieldsAreShown() {
        val err = id.pejuang.hris.data.errorFromBody(
            """{"error":{"code":"VALIDATION_FAILED","message":"Periksa kembali isian form.","details":{"fields":{"samples":"Foto wajah tidak konsisten, silakan ulangi."}},"request_id":"r1"}}""",
        )
        assertEquals("VALIDATION_FAILED", err?.code)
        assertEquals("Foto wajah tidak konsisten, silakan ulangi.", err?.message)
    }

    @Test
    fun knownCodesUseFriendlyMessage() {
        val err = id.pejuang.hris.data.errorFromBody(
            """{"error":{"code":"ATTENDANCE_BLOCKED","message":"x","details":{},"request_id":"r2"}}""",
        )
        assertEquals("Presensi ditolak. Hubungi SDM bila ini terus terjadi.", err?.message)
        assertEquals(null, id.pejuang.hris.data.errorFromBody("bukan json"))
    }

    @Test
    fun scaledSizeNeverEnlarges() {
        assertEquals(640 to 480, scaledSize(1280, 960))
        assertEquals(480 to 640, scaledSize(1080, 1440))
        assertEquals(320 to 240, scaledSize(320, 240))
    }

    @Test
    fun coordinatesHaveSixDecimals() {
        assertEquals(-6.200100, roundCoordinate(-6.2001004), 0.0)
        assertEquals(106.816601, roundCoordinate(106.8166009), 0.0)
    }

    @Test
    fun sha256IsLowerHex() {
        val hash = sha256Hex("abc".toByteArray())
        assertEquals("ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad", hash)
    }

    @Test
    fun baseUrlEndsWithSlash() {
        assertEquals("http://10.0.0.2:8080/", normalizeBaseUrl(" http://10.0.0.2:8080 "))
        assertEquals("https://api.domain.id/", normalizeBaseUrl("https://api.domain.id/"))
        // Isian tanpa skema di build debug dianggap http, bukan galat Retrofit yang membingungkan.
        assertEquals("http://192.168.1.10:8080/", normalizeBaseUrl("192.168.1.10:8080"))
    }

    @Test
    fun loginOmitsEmptyMfaAndUsesSnakeCase() {
        val json = apiJson.encodeToString(
            MobileLoginRequest.serializer(),
            MobileLoginRequest("a@b.id", "rahasia", null, DeviceRegisterInput("h")),
        )
        assertFalse(json.contains("mfa_code"))
        assertTrue(json.contains("\"device_hash\":\"h\""))
    }

    @Test
    fun clockPayloadHasContractFields() {
        val json = apiJson.encodeToString(
            ClockInput.serializer(),
            ClockInput("c", -6.2, 106.8, 12.5, "2026-11-09T01:00:00Z", AttendanceSignals(false, "0.1.0"), "ab", true, "blink"),
        )
        for (field in listOf("challenge_id", "accuracy_m", "captured_at", "selfie_sha256", "liveness_passed", "liveness_action", "is_mock_location", "app_version")) {
            assertTrue("$field ada", json.contains("\"$field\""))
        }
    }
}

class UiLogicTest {
    @Test
    fun crashSummaryKeepsTypeMessageAndRootCause() {
        val error = RuntimeException("luar", IllegalStateException("akar"))
        val text = CrashLog.summary(error, "main", "0.2.0", "Xiaomi Redmi · Android 13", java.time.Instant.parse("2026-11-09T00:30:00Z"))
        val lines = text.lines()
        assertEquals("2026-11-09 07:30:00 WIB · v0.2.0 · Xiaomi Redmi · Android 13 · thread main", lines[0])
        assertEquals("java.lang.RuntimeException: luar", lines[1])
        assertTrue(text.contains("Caused by java.lang.IllegalStateException: akar"))
    }

    @Test
    fun labelsAndGreeting() {
        assertEquals("SN", id.pejuang.hris.ui.initials("Siti Nurhaliza Rahmawati"))
        assertEquals("Terlambat", id.pejuang.hris.ui.statusLabel("late").first)
        assertEquals("Belum presensi", id.pejuang.hris.ui.statusLabel(null).first)
        val morning = java.time.ZonedDateTime.of(2026, 11, 9, 7, 0, 0, 0, id.pejuang.hris.ui.Wib)
        assertEquals("Selamat pagi", id.pejuang.hris.ui.greeting(morning))
        assertEquals("Selamat malam", id.pejuang.hris.ui.greeting(morning.withHour(20)))
        assertEquals("07:42", id.pejuang.hris.ui.wibTime("2026-11-09T00:42:00Z"))
        assertEquals("--:--", id.pejuang.hris.ui.wibTime(null))
    }

    @Test
    fun workedMinutesAndPeriod() {
        val now = java.time.ZonedDateTime.of(2026, 11, 9, 9, 42, 0, 0, id.pejuang.hris.ui.Wib)
        val open = id.pejuang.hris.data.AttendanceRecord("a", "2026-11-09", "present", "2026-11-09T00:42:00Z", workMode = "onsite")
        val closed = id.pejuang.hris.data.AttendanceRecord("b", "2026-11-06", "late", "2026-11-06T01:17:00Z", "2026-11-06T10:05:00Z", 17, 528, "onsite")
        val forgot = id.pejuang.hris.data.AttendanceRecord("c", "2026-11-05", "present", "2026-11-05T00:51:00Z", workMode = "onsite")
        val lastMonth = id.pejuang.hris.data.AttendanceRecord("d", "2026-10-31", "present", workedMinutes = 480, workMode = "onsite")
        // Presensi hari ini yang belum pulang dihitung sampai sekarang; hari lalu tanpa pulang tidak dihitung.
        assertEquals(120, id.pejuang.hris.ui.workedMinutesOf(open, now))
        assertEquals(528, id.pejuang.hris.ui.workedMinutesOf(closed, now))
        assertEquals(0, id.pejuang.hris.ui.workedMinutesOf(forgot, now))
        assertEquals(648, id.pejuang.hris.ui.monthWorkedMinutes(listOf(open, closed, forgot, lastMonth), now))
        assertEquals("10:48", id.pejuang.hris.ui.hourMinute(648))
        assertEquals("1 Nov 2026 - 30 Nov 2026", id.pejuang.hris.ui.monthPeriodLabel(now.toLocalDate()))
        assertEquals("9 November 2026", id.pejuang.hris.ui.dateLabel(now.toLocalDate()))
    }

    @Test
    fun clockActionFollowsState() {
        val me = id.pejuang.hris.data.Me(
            employeeId = "e", fullName = "Siti", nip = "1",
            subdivision = id.pejuang.hris.data.Ref("s", "S"), division = id.pejuang.hris.data.Ref("d", "D"),
            deviceStatus = "approved", faceEnrolled = true,
        )
        val inOnly = id.pejuang.hris.data.AttendanceRecord("a", "2026-11-09", "present", "2026-11-09T00:42:00Z", workMode = "onsite")
        assertEquals(id.pejuang.hris.data.PURPOSE_CLOCK_IN, id.pejuang.hris.ui.clockAction(me, null).purpose)
        assertEquals(id.pejuang.hris.data.PURPOSE_CLOCK_OUT, id.pejuang.hris.ui.clockAction(me, inOnly).purpose)
        assertEquals(null, id.pejuang.hris.ui.clockAction(me, inOnly.copy(clockOutAt = "2026-11-09T10:00:00Z")).purpose)
        assertEquals(id.pejuang.hris.data.PURPOSE_FACE_ENROLL, id.pejuang.hris.ui.clockAction(me.copy(faceEnrolled = false), null).purpose)
        assertEquals(null, id.pejuang.hris.ui.clockAction(me.copy(deviceStatus = "pending"), null).purpose)
        assertEquals(null, id.pejuang.hris.ui.clockAction(me.copy(mustChangePassword = true), null).purpose)
        val onLeave = id.pejuang.hris.data.AttendanceRecord("l", "2026-11-09", "leave", workMode = "onsite")
        assertEquals(null, id.pejuang.hris.ui.clockAction(me, onLeave).purpose)
    }

    @Test
    fun passwordFormChecks() {
        val ok = id.pejuang.hris.ui.PasswordForm(current = "lama", next = "baru-panjang-1", repeat = "baru-panjang-1")
        assertEquals(null, id.pejuang.hris.ui.passwordProblem(ok))
        assertEquals("Minimal 10 karakter.", id.pejuang.hris.ui.passwordProblem(ok.copy(next = "pendek", repeat = "")))
        assertEquals("Kata sandi tidak sama.", id.pejuang.hris.ui.passwordProblem(ok.copy(repeat = "beda-sekali-1")))
        val body = apiJson.encodeToString(
            id.pejuang.hris.data.PasswordChangeRequest.serializer(),
            id.pejuang.hris.data.PasswordChangeRequest("lama", "baru-panjang-1"),
        )
        assertEquals("""{"current_password":"lama","new_password":"baru-panjang-1"}""", body)
    }
}
