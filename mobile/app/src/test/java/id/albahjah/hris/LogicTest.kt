package id.albahjah.hris

import id.albahjah.hris.data.Payslip
import id.albahjah.hris.data.PayslipItem
import id.albahjah.hris.ui.dateRangeLabel
import id.albahjah.hris.ui.payslipFileName
import id.albahjah.hris.ui.payslipItemLabel
import id.albahjah.hris.ui.periodName
import id.albahjah.hris.ui.rupiah
import id.albahjah.hris.data.AttendanceSignals
import id.albahjah.hris.data.ClockInput
import id.albahjah.hris.data.DeviceRegisterInput
import id.albahjah.hris.data.MobileLoginRequest
import id.albahjah.hris.data.apiJson
import id.albahjah.hris.data.normalizeBaseUrl
import id.albahjah.hris.data.sha256Hex
import id.albahjah.hris.face.FaceFit
import id.albahjah.hris.face.FaceSample
import id.albahjah.hris.face.FitRect
import id.albahjah.hris.face.FitRules
import id.albahjah.hris.face.LivenessState
import id.albahjah.hris.face.LivenessTracker
import id.albahjah.hris.face.checkFit
import id.albahjah.hris.face.faceOval
import id.albahjah.hris.face.LightRules
import id.albahjah.hris.face.mapToView
import id.albahjah.hris.face.meanLuma
import id.albahjah.hris.face.photoFits
import id.albahjah.hris.face.scaledSize
import id.albahjah.hris.location.roundCoordinate
import id.albahjah.hris.security.BuildInfo
import id.albahjah.hris.security.DeviceProof
import id.albahjah.hris.security.DeviceRisk
import id.albahjah.hris.security.DeviceRules
import id.albahjah.hris.security.RequestHash
import id.albahjah.hris.data.Child
import id.albahjah.hris.data.MeUpdate
import id.albahjah.hris.data.MyAnnouncement
import id.albahjah.hris.data.MyFamily
import id.albahjah.hris.ui.FamilyForm
import id.albahjah.hris.ui.centerSquare
import id.albahjah.hris.ui.from
import id.albahjah.hris.ui.localPhone
import id.albahjah.hris.ui.nextPopup
import id.albahjah.hris.ui.ANNOUNCEMENT_POLL_MS
import id.albahjah.hris.ui.ANNOUNCEMENT_RETRY_MS
import id.albahjah.hris.ui.sampleSizeFor
import id.albahjah.hris.ui.Wib
import id.albahjah.hris.ui.offlineSinceText
import id.albahjah.hris.data.durationText
import id.albahjah.hris.data.editSavedPassword
import id.albahjah.hris.data.AppVersion
import id.albahjah.hris.data.UpdateLevel
import id.albahjah.hris.data.appVersionHeader
import id.albahjah.hris.data.updateLevel
import id.albahjah.hris.ui.toPickerMillis
import java.security.KeyPairGenerator
import java.security.Signature
import java.security.spec.ECGenParameterSpec
import java.util.Base64
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

class LivenessTrackerTest {
    private fun face(yaw: Float = 0f, eyes: Float? = 0.9f, smile: Float? = 0.1f, fit: FaceFit = FaceFit.Fit) =
        FaceSample(leftEyeOpen = eyes, rightEyeOpen = eyes, smiling = smile, yaw = yaw, fit = fit)

    // Tes gerakan memakai satu frame per tahap tahan posisi agar ringkas; aturan 5 frame dites terpisah.
    private fun tracker(action: String) = LivenessTracker(action, holdFrames = 1)

    @Test
    fun straightStageNeedsFiveFitFrames() {
        val t = LivenessTracker("blink")
        assertEquals(5, FitRules.HOLD_FRAMES)
        repeat(4) { i ->
            val s = t.update(listOf(face())) as LivenessState.Waiting
            assertEquals("Tahan posisi", s.hint)
            assertTrue(s.aligned)
            assertEquals((i + 1) / 5f, s.progress, 1e-6f)
        }
        assertEquals(LivenessState.Waiting("Kedipkan mata", aligned = true), t.update(listOf(face())))
    }

    @Test
    fun badFrameRestartsHold() {
        val t = LivenessTracker("blink")
        repeat(3) { t.update(listOf(face())) }
        assertEquals(LivenessState.Waiting("Dekatkan wajah"), t.update(listOf(face(fit = FaceFit.TooSmall))))
        assertEquals(LivenessState.Waiting("Tegakkan kepala"), t.update(listOf(face(fit = FaceFit.Tilted))))
        repeat(4) { assertEquals("Tahan posisi", (t.update(listOf(face())) as LivenessState.Waiting).hint) }
        assertEquals("Kedipkan mata", (t.update(listOf(face())) as LivenessState.Waiting).hint)
    }

    @Test
    fun fitButNotStraightAsksToLookStraight() {
        val t = tracker("blink")
        assertEquals(LivenessState.Waiting("Lihat lurus ke kamera", aligned = true), t.update(listOf(face(yaw = 20f))))
    }

    @Test
    fun leavingOvalMustStartOver() {
        val t = tracker("blink")
        t.update(listOf(face()))
        t.update(listOf(face(eyes = 0.1f)))
        assertEquals(
            LivenessState.Waiting("Masukkan wajah ke dalam bingkai"),
            t.update(listOf(face(fit = FaceFit.Outside))),
        )
        // Membuka mata setelah keluar oval hanya menetapkan posisi awal baru.
        assertTrue(t.update(listOf(face())) is LivenessState.Waiting)
        assertTrue(t.update(listOf(face())) is LivenessState.Waiting)
    }

    @Test
    fun turnOnlyNeedsCenterInsideOval() {
        val t = tracker("turn_left")
        t.update(listOf(face()))
        // Saat menoleh kotak wajah bergeser; selama pusatnya di oval, gerakan tetap dihitung.
        assertEquals(LivenessState.Waiting("Geser ke tengah"), t.update(listOf(face(yaw = 30f, fit = FaceFit.OffCenter))))
        // Sebelum foto, wajah wajib pas lagi.
        assertEquals(LivenessState.Waiting("Tegakkan kepala"), t.update(listOf(face(yaw = 3f, fit = FaceFit.Tilted))))
        assertEquals(LivenessState.Passed, t.update(listOf(face(yaw = 3f))))
    }

    @Test
    fun straightStageNeedsFit() {
        val t = tracker("smile")
        assertEquals(LivenessState.Waiting("Jauhkan sedikit"), t.update(listOf(face(fit = FaceFit.TooLarge))))
        // Gerakan sebelum posisi awal pas belum dihitung.
        assertTrue(t.update(listOf(face(smile = 0.9f))) is LivenessState.Waiting)
        t.update(listOf(face()))
        assertEquals(LivenessState.Waiting("Jauhkan sedikit"), t.update(listOf(face(smile = 0.9f, fit = FaceFit.TooLarge))))
        assertEquals(LivenessState.Passed, t.update(listOf(face())))
    }

    @Test
    fun blinkNeedsOpenClosedOpen() {
        val t = tracker("blink")
        assertTrue(t.update(listOf(face())) is LivenessState.Waiting)
        assertTrue(t.update(listOf(face(eyes = 0.1f))) is LivenessState.Waiting)
        assertEquals(LivenessState.Passed, t.update(listOf(face(eyes = 0.9f))))
    }

    @Test
    fun closedEyesFromStartDoNotPass() {
        val t = tracker("blink")
        repeat(3) { assertTrue(t.update(listOf(face(eyes = 0.1f))) is LivenessState.Waiting) }
    }

    @Test
    fun turnNeedsStraightFirst() {
        val t = tracker("turn_left")
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
        val t = tracker("smile")
        t.update(listOf(face()))
        assertEquals(LivenessState.Passed, t.update(listOf(face(smile = 0.9f))))
        assertEquals(LivenessState.ManyFaces, t.update(listOf(face(), face())))
        assertTrue(t.update(listOf(face(eyes = 0.1f))) is LivenessState.Waiting)
    }

    @Test
    fun smileNeedsNeutralFirst() {
        val t = tracker("smile")
        assertTrue(t.update(listOf(face(smile = 0.95f))) is LivenessState.Waiting)
        t.update(listOf(face(smile = 0.1f)))
        assertEquals(LivenessState.Passed, t.update(listOf(face(smile = 0.9f))))
    }

    @Test
    fun noFaceAndManyFaces() {
        val t = tracker("blink")
        assertEquals(LivenessState.NoFace, t.update(emptyList()))
        assertEquals(LivenessState.ManyFaces, t.update(listOf(face(), face())))
    }

    @Test
    fun interruptedGestureMustStartOver() {
        for (interruption in listOf(emptyList(), listOf(face(), face()))) {
            val t = tracker("blink")
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
        val t = tracker("turn_left")
        t.update(listOf(face()))
        t.update(listOf(face(yaw = 30f)))
        t.update(emptyList())
        assertTrue(t.update(listOf(face())) is LivenessState.Waiting)
        assertTrue(t.update(listOf(face())) is LivenessState.Waiting)
    }

    @Test
    fun unknownClassificationDoesNotEstablishNeutralOrFinishCapture() {
        val t = tracker("smile")
        t.update(listOf(face(eyes = null)))
        t.update(listOf(face(smile = 0.9f)))
        assertTrue(t.update(listOf(face(smile = 0.9f))) is LivenessState.Waiting)

        t.update(listOf(face(smile = null)))
        assertTrue(t.update(listOf(face(smile = 0.9f))) is LivenessState.Waiting)
        t.update(listOf(face()))
        assertTrue(t.update(listOf(face(eyes = null, smile = 0.9f))) is LivenessState.Waiting)
        assertEquals(LivenessState.Passed, t.update(listOf(face())))
    }

    @Test
    fun gestureSeenMarksTheGestureFrame() {
        val t = tracker("turn_left")
        t.update(listOf(face()))
        assertFalse(t.gestureSeen)
        t.update(listOf(face(yaw = 30f)))
        assertTrue(t.gestureSeen)
        // Wajah keluar oval: foto gerakan lama harus dibuang dan gerakan diulang.
        t.update(listOf(face(fit = FaceFit.Outside)))
        assertFalse(t.gestureSeen)
    }
}

class DataTest {
    @Test
    fun validationFieldsAreShown() {
        val err = id.albahjah.hris.data.errorFromBody(
            """{"error":{"code":"VALIDATION_FAILED","message":"Periksa kembali isian form.","details":{"fields":{"samples":"Foto wajah tidak konsisten, silakan ulangi."}},"request_id":"r1"}}""",
        )
        assertEquals("VALIDATION_FAILED", err?.code)
        assertEquals("Foto wajah tidak konsisten, silakan ulangi.", err?.message)
    }

    @Test
    fun knownCodesUseFriendlyMessage() {
        val err = id.albahjah.hris.data.errorFromBody(
            """{"error":{"code":"ATTENDANCE_BLOCKED","message":"x","details":{},"request_id":"r2"}}""",
        )
        assertEquals("Presensi belum dapat dicatat. Silakan hubungi SDM bila hal ini terus terjadi.", err?.message)
        assertEquals(null, id.albahjah.hris.data.errorFromBody("bukan json"))
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
            ClockInput(
                challengeId = "c", latitude = -6.2, longitude = 106.8, accuracyM = 12.5, capturedAt = "2026-11-09T01:00:00Z",
                signals = AttendanceSignals(isMockLocation = false, rootDetected = true, appVersion = "0.1.0"),
                selfieSha256 = "ab", livenessPassed = true, livenessAction = "turn_left", gestureSha256 = "cd",
            ),
        )
        assertTrue(json.contains("\"root_detected\":true"))
        for (field in listOf(
            "challenge_id", "accuracy_m", "captured_at", "selfie_sha256", "liveness_passed", "liveness_action", "gesture_sha256",
            "is_mock_location", "fake_gps_app_detected", "root_detected", "emulator_detected", "app_version",
        )) {
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
        assertEquals("SN", id.albahjah.hris.ui.initials("Siti Nurhaliza Rahmawati"))
        assertEquals("Terlambat", id.albahjah.hris.ui.statusLabel("late").first)
        assertEquals("Belum presensi", id.albahjah.hris.ui.statusLabel(null).first)
        val morning = java.time.ZonedDateTime.of(2026, 11, 9, 7, 0, 0, 0, id.albahjah.hris.ui.Wib)
        assertEquals("Selamat pagi", id.albahjah.hris.ui.greeting(morning))
        assertEquals("Selamat malam", id.albahjah.hris.ui.greeting(morning.withHour(20)))
        assertEquals("07:42", id.albahjah.hris.ui.wibTime("2026-11-09T00:42:00Z"))
        assertEquals("--:--", id.albahjah.hris.ui.wibTime(null))
    }

    @Test
    fun workedMinutesAndPeriod() {
        val now = java.time.ZonedDateTime.of(2026, 11, 9, 9, 42, 0, 0, id.albahjah.hris.ui.Wib)
        val open = id.albahjah.hris.data.AttendanceRecord("a", "2026-11-09", "present", "2026-11-09T00:42:00Z", workMode = "onsite")
        val closed = id.albahjah.hris.data.AttendanceRecord("b", "2026-11-06", "late", "2026-11-06T01:17:00Z", "2026-11-06T10:05:00Z", 17, 528, "onsite")
        val forgot = id.albahjah.hris.data.AttendanceRecord("c", "2026-11-05", "present", "2026-11-05T00:51:00Z", workMode = "onsite")
        val lastMonth = id.albahjah.hris.data.AttendanceRecord("d", "2026-10-31", "present", workedMinutes = 480, workMode = "onsite")
        // Presensi hari ini yang belum pulang dihitung sampai sekarang; hari lalu tanpa pulang tidak dihitung.
        assertEquals(120, id.albahjah.hris.ui.workedMinutesOf(open, now))
        assertEquals(528, id.albahjah.hris.ui.workedMinutesOf(closed, now))
        assertEquals(0, id.albahjah.hris.ui.workedMinutesOf(forgot, now))
        assertEquals(648, id.albahjah.hris.ui.monthWorkedMinutes(listOf(open, closed, forgot, lastMonth), now))
        assertEquals("10:48", id.albahjah.hris.ui.hourMinute(648))
        assertEquals("1 Nov 2026 - 30 Nov 2026", id.albahjah.hris.ui.monthPeriodLabel(now.toLocalDate()))
        assertEquals("9 November 2026", id.albahjah.hris.ui.dateLabel(now.toLocalDate()))
    }

    @Test
    fun clockActionFollowsState() {
        val me = id.albahjah.hris.data.Me(
            employeeId = "e", fullName = "Siti", nip = "1",
            subdivision = id.albahjah.hris.data.Ref("s", "S"), division = id.albahjah.hris.data.Ref("d", "D"),
            deviceStatus = "approved", faceEnrolled = true,
        )
        val inOnly = id.albahjah.hris.data.AttendanceRecord("a", "2026-11-09", "present", "2026-11-09T00:42:00Z", workMode = "onsite")
        assertEquals(id.albahjah.hris.data.PURPOSE_CLOCK_IN, id.albahjah.hris.ui.clockAction(me, null).purpose)
        assertEquals(id.albahjah.hris.data.PURPOSE_CLOCK_OUT, id.albahjah.hris.ui.clockAction(me, inOnly).purpose)
        assertEquals(null, id.albahjah.hris.ui.clockAction(me, inOnly.copy(clockOutAt = "2026-11-09T10:00:00Z")).purpose)
        assertEquals(id.albahjah.hris.data.PURPOSE_FACE_ENROLL, id.albahjah.hris.ui.clockAction(me.copy(faceEnrolled = false), null).purpose)
        assertEquals(null, id.albahjah.hris.ui.clockAction(me.copy(deviceStatus = "pending"), null).purpose)
        assertEquals(null, id.albahjah.hris.ui.clockAction(me.copy(mustChangePassword = true), null).purpose)
        val onLeave = id.albahjah.hris.data.AttendanceRecord("l", "2026-11-09", "leave", workMode = "onsite")
        assertEquals(null, id.albahjah.hris.ui.clockAction(me, onLeave).purpose)
    }

    @Test
    fun passwordFormChecks() {
        val ok = id.albahjah.hris.ui.PasswordForm(current = "lama", next = "baru-panjang-1", repeat = "baru-panjang-1")
        assertEquals(null, id.albahjah.hris.ui.passwordProblem(ok))
        assertEquals("Minimal 10 karakter.", id.albahjah.hris.ui.passwordProblem(ok.copy(next = "pendek", repeat = "")))
        assertEquals("Kata sandi tidak sama.", id.albahjah.hris.ui.passwordProblem(ok.copy(repeat = "beda-sekali-1")))
        val body = apiJson.encodeToString(
            id.albahjah.hris.data.PasswordChangeRequest.serializer(),
            id.albahjah.hris.data.PasswordChangeRequest("lama", "baru-panjang-1"),
        )
        assertEquals("""{"current_password":"lama","new_password":"baru-panjang-1"}""", body)
    }
}

class RequestsLogicTest {
    private val now = java.time.ZonedDateTime.of(2026, 11, 9, 7, 45, 0, 0, id.albahjah.hris.ui.Wib)

    @Test
    fun dateRangesAreCompact() {
        val d = { s: String -> java.time.LocalDate.parse(s) }
        assertEquals("16 November 2026", id.albahjah.hris.ui.dateRangeLabel(d("2026-11-16"), d("2026-11-16")))
        assertEquals("16–18 November 2026", id.albahjah.hris.ui.dateRangeLabel(d("2026-11-16"), d("2026-11-18")))
        assertTrue(id.albahjah.hris.ui.dateRangeLabel(d("2026-10-30"), d("2026-11-02")).matches(Regex("""30 \p{L}+ – 2 \p{L}+ 2026""")))
        assertTrue(id.albahjah.hris.ui.dateRangeLabel(d("2026-12-28"), d("2027-01-02")).matches(Regex("""28 \p{L}+ 2026 – 2 \p{L}+ 2027""")))
        assertEquals("x – y", id.albahjah.hris.ui.dateRangeLabel("x", "y"))
    }

    @Test
    fun timeAgoSteps() {
        assertEquals("baru saja", id.albahjah.hris.ui.timeAgo("2026-11-09T00:45:00Z", now))
        assertEquals("5 menit lalu", id.albahjah.hris.ui.timeAgo("2026-11-09T00:40:00Z", now))
        assertEquals("3 jam lalu", id.albahjah.hris.ui.timeAgo("2026-11-08T21:45:00Z", now))
        assertEquals("2 hari lalu", id.albahjah.hris.ui.timeAgo("2026-11-07T00:45:00Z", now))
        assertEquals("1 Oktober 2026", id.albahjah.hris.ui.timeAgo("2026-10-01T03:00:00Z", now))
    }

    @Test
    fun pickerMillisRoundTrip() {
        val day = java.time.LocalDate.of(2026, 11, 16)
        assertEquals(1794787200000L, day.toPickerMillis())
        assertEquals(day, id.albahjah.hris.ui.pickerMillisToDate(day.toPickerMillis()))
    }

    @Test
    fun leaveFormChecksMatchServerRules() {
        val today = java.time.LocalDate.of(2026, 11, 9)
        val ok = id.albahjah.hris.ui.LeaveForm("t1", today.plusDays(7), today.plusDays(9), "Acara keluarga")
        assertTrue(id.albahjah.hris.ui.validateLeave(ok, today).isEmpty())
        val empty = id.albahjah.hris.ui.validateLeave(id.albahjah.hris.ui.LeaveForm(), today)
        assertEquals(setOf("leave_type_id", "start_date", "reason"), empty.keys)
        assertTrue(id.albahjah.hris.ui.validateLeave(ok.copy(end = today), today).containsKey("start_date"))
        assertTrue(id.albahjah.hris.ui.validateLeave(ok.copy(start = java.time.LocalDate.of(2026, 12, 30), end = java.time.LocalDate.of(2027, 1, 2)), today).containsKey("start_date"))
        assertTrue(id.albahjah.hris.ui.validateLeave(ok.copy(start = today.minusDays(31), end = today), today).containsKey("start_date"))
        assertTrue(id.albahjah.hris.ui.validateLeave(ok.copy(reason = "  a "), today).containsKey("reason"))
    }

    @Test
    fun progressFollowsCurrentStep() {
        val type = id.albahjah.hris.data.LeaveTypeRef("t1", "annual", "Cuti Tahunan")
        fun leave(status: String, steps: List<id.albahjah.hris.data.ApprovalStep>) =
            id.albahjah.hris.data.LeaveRequest("l", "e", type, "2026-11-16", "2026-11-18", 3, "x", status, "2026-11-09T00:00:00Z", null, steps)
        val waitingSdm = leave("pending", listOf(
            id.albahjah.hris.data.ApprovalStep(1, "kadiv", "Umar", "approved"),
            id.albahjah.hris.data.ApprovalStep(2, "sdm", null, "pending"),
        ))
        assertEquals("Menunggu SDM", id.albahjah.hris.ui.progressText(waitingSdm))
        val atKadiv = leave("pending", listOf(id.albahjah.hris.data.ApprovalStep(1, "kadiv", "Umar", "pending")))
        assertEquals("Menunggu Kepala Divisi · Umar", id.albahjah.hris.ui.progressText(atKadiv))
        val rejected = leave("rejected", listOf(id.albahjah.hris.data.ApprovalStep(1, "kadiv", "Umar", "rejected", "Tanggal bentrok")))
        assertEquals("Tidak disetujui: Tanggal bentrok", id.albahjah.hris.ui.progressText(rejected))
        assertEquals("Dibatalkan", id.albahjah.hris.ui.progressText(leave("cancelled", emptyList())))
    }

    @Test
    fun fieldErrorsAreKeptPerField() {
        val err = id.albahjah.hris.data.errorFromBody(
            """{"error":{"code":"VALIDATION_FAILED","message":"Periksa kembali isian form.","details":{"fields":{"reason":"Alasan wajib diisi, minimal 3 karakter.","end_date":"Rentang tanggal ini tidak berisi hari kerja."}}}}""",
        )
        assertEquals("Alasan wajib diisi, minimal 3 karakter.", err?.fields?.get("reason"))
        assertEquals("Rentang tanggal ini tidak berisi hari kerja.", err?.fields?.get("end_date"))
    }

    @Test
    fun leaveInputUsesContractFields() {
        val body = apiJson.encodeToString(
            id.albahjah.hris.data.LeaveInput.serializer(),
            id.albahjah.hris.data.LeaveInput("t1", "2026-11-16", "2026-11-18", "Acara keluarga"),
        )
        assertEquals("""{"leave_type_id":"t1","start_date":"2026-11-16","end_date":"2026-11-18","reason":"Acara keluarga"}""", body)
        val decision = apiJson.encodeToString(id.albahjah.hris.data.ApprovalDecision.serializer(), id.albahjah.hris.data.ApprovalDecision(null))
        assertEquals("{}", decision)
    }
}

class AnnouncementLogicTest {
    @Test
    fun decodesServerShape() {
        val a = apiJson.decodeFromString(
            id.albahjah.hris.data.MyAnnouncement.serializer(),
            """{"id":"a1","title":"Rapat","body":"Isi","type":"popup","starts_at":"2026-11-02T01:00:00Z","ends_at":null,
               "min_read_seconds":15,"opened_at":"2026-11-02T01:00:05Z","read_at":null,"read_wait_seconds":11}""",
        )
        assertEquals(15, a.minReadSeconds)
        assertEquals(11, a.readWaitSeconds)
        assertEquals(null, a.readAt)
    }

    @Test
    fun readTooSoonUsesFriendlyMessage() {
        val err = id.albahjah.hris.data.errorFromBody(
            """{"error":{"code":"READ_TOO_SOON","message":"x","details":{"wait_seconds":5}}}""",
        )
        assertEquals("READ_TOO_SOON", err?.code)
        assertTrue(err?.message.orEmpty().startsWith("Baca dulu isinya"))
    }
}

class BrandingLogicTest {
    @Test
    fun fontSizeSnapsToNearestOption() {
        assertEquals(id.albahjah.hris.data.FontSize.Normal, id.albahjah.hris.data.FontSize.of(1f))
        assertEquals(id.albahjah.hris.data.FontSize.Large, id.albahjah.hris.data.FontSize.of(1.12f))
        assertEquals(id.albahjah.hris.data.FontSize.ExtraLarge, id.albahjah.hris.data.FontSize.of(9f))
        assertEquals(id.albahjah.hris.data.FontSize.Small, id.albahjah.hris.data.FontSize.of(0.1f))
    }

    @Test
    fun defaultFooterMatchesServer() {
        assertEquals("© 2027 PUTI (Pusat Teknologi Informasi) Al Bahjah", id.albahjah.hris.ui.defaultFooter(2027))
    }

    @Test
    fun brandingDecodes() {
        val b = apiJson.decodeFromString(
            id.albahjah.hris.data.Branding.serializer(),
            """{"footer":"© 2026 PUTI","footer_template":"© {tahun} PUTI","is_default":false,"updated_at":"2026-11-11T01:00:00Z"}""",
        )
        assertEquals("© 2026 PUTI", b.footer)
        assertFalse(b.isDefault)
    }
}

class FaceFitTest {
    // Layar 1080x2340: oval lebar 756, tinggi 997.92, pusat (540, 936).
    private val viewWidth = 1080f
    private val viewHeight = 2340f
    private val oval = faceOval(viewWidth, viewHeight)

    private fun box(cx: Float, cy: Float, w: Float, h: Float = w) = FitRect(cx - w / 2, cy - h / 2, cx + w / 2, cy + h / 2)

    private fun fit(face: FitRect, roll: Float = 0f, pitch: Float = 0f) = checkFit(face, oval, roll, pitch)

    private fun assertRect(expected: FitRect, actual: FitRect) {
        assertEquals(expected.left, actual.left, 0.01f)
        assertEquals(expected.top, actual.top, 0.01f)
        assertEquals(expected.right, actual.right, 0.01f)
        assertEquals(expected.bottom, actual.bottom, 0.01f)
    }

    @Test
    fun ovalMatchesDrawing() {
        assertEquals(756f, oval.width, 0.01f)
        assertEquals(756f * 1.32f, oval.height, 0.01f)
        assertEquals(540f, oval.centerX, 0.01f)
        assertEquals(936f, oval.centerY, 0.01f)
    }

    @Test
    fun centeredFaceFits() {
        assertEquals(FaceFit.Fit, fit(box(540f, 936f, 0.7f * 756f)))
        // Masih pas di dekat batas posisi dan ukuran.
        assertEquals(FaceFit.Fit, fit(box(540f + 0.11f * 756f, 936f, 0.6f * 756f)))
        assertEquals(FaceFit.Fit, fit(box(540f, 936f - 0.09f * 998f, 0.6f * 756f)))
        // Dilonggarkan (D55): wajah agak jauh dan agak ke samping tetap pas.
        assertEquals(FaceFit.Fit, fit(box(540f + 0.16f * 756f, 936f, 0.45f * 756f)))
    }

    @Test
    fun tooSmallAndTooLarge() {
        assertEquals(FaceFit.TooSmall, fit(box(540f, 936f, 0.38f * 756f)))
        assertEquals(FaceFit.TooLarge, fit(box(540f, 936f, 0.97f * 756f)))
    }

    @Test
    fun faceAtEdge() {
        // Wajah yang baru terlihat sedikit di tepi bingkai: pusatnya di luar oval.
        assertEquals(FaceFit.Outside, fit(box(60f, 936f, 0.6f * 756f)))
        assertEquals(FaceFit.Outside, fit(box(540f, 200f, 0.6f * 756f)))
        // Di dalam oval tetapi bergeser lebih dari 18% lebar atau 16% tinggi oval.
        assertEquals(FaceFit.OffCenter, fit(box(540f + 0.21f * 756f, 936f, 0.6f * 756f)))
        assertEquals(FaceFit.OffCenter, fit(box(540f, 936f + 0.19f * 998f, 0.6f * 756f)))
        // Pusat dan ukuran lolos, tetapi sisi wajah menembus garis oval lebih dari toleransi.
        assertEquals(FaceFit.OffCenter, fit(box(540f + 0.17f * 756f, 936f, 0.9f * 756f)))
    }

    @Test
    fun tiltedHead() {
        val face = box(540f, 936f, 0.7f * 756f)
        assertEquals(FaceFit.Tilted, fit(face, roll = 22f))
        assertEquals(FaceFit.Tilted, fit(face, pitch = -21f))
        assertEquals(FaceFit.Fit, fit(face, roll = 19f, pitch = -19f))
    }

    @Test
    fun hintsMatchGuidance() {
        assertEquals("Masukkan wajah ke dalam bingkai", FaceFit.Outside.hint)
        assertEquals("Dekatkan wajah", FaceFit.TooSmall.hint)
        assertEquals("Jauhkan sedikit", FaceFit.TooLarge.hint)
        assertEquals("Geser ke tengah", FaceFit.OffCenter.hint)
        assertEquals("Tegakkan kepala", FaceFit.Tilted.hint)
        assertEquals("Tahan posisi", FaceFit.Fit.hint)
    }

    @Test
    fun fillCenterMapping() {
        // Frame 480x640 tegak di layar 1080x2340: skala 2340/640 = 3.65625, sisi kiri-kanan terpotong 337.5 px.
        val mapped = mapToView(FitRect(100f, 200f, 300f, 400f), 480, 640, 0, viewWidth, viewHeight, mirror = false)
        assertRect(FitRect(28.125f, 731.25f, 759.375f, 1462.5f), mapped)
    }

    @Test
    fun frontCameraIsMirrored() {
        val face = FitRect(100f, 200f, 300f, 400f)
        // Wajah di kiri frame analisis tampil di kanan pratinjau kamera depan.
        val mirrored = mapToView(face, 480, 640, 0, viewWidth, viewHeight, mirror = true)
        assertRect(FitRect(320.625f, 731.25f, 1051.875f, 1462.5f), mirrored)
        // Wajah di tengah frame tetap di tengah layar.
        val centered = mapToView(FitRect(190f, 200f, 290f, 400f), 480, 640, 0, viewWidth, viewHeight, mirror = true)
        assertEquals(540f, centered.centerX, 0.01f)
    }

    @Test
    fun rotation270SwapsFrameSides() {
        // Kamera depan biasanya memberi frame 640x480 dengan rotasi 270; kotak ML Kit sudah di gambar tegak 480x640.
        val face = FitRect(100f, 200f, 300f, 400f)
        val rotated = mapToView(face, 640, 480, 270, viewWidth, viewHeight, mirror = true)
        assertRect(mapToView(face, 480, 640, 0, viewWidth, viewHeight, mirror = true), rotated)
        assertRect(mapToView(face, 640, 480, 90, viewWidth, viewHeight, mirror = true), rotated)

        // Wajah di tengah frame (0.4 tinggi) selebar 145 px pas di oval setelah dipetakan.
        val onScreen = mapToView(FitRect(240f - 72.5f, 256f - 72.5f, 240f + 72.5f, 256f + 72.5f), 640, 480, 270, viewWidth, viewHeight, mirror = true)
        assertEquals(FaceFit.Fit, fit(onScreen))
    }

    @Test
    fun capturedPhotoChecks() {
        // Foto tegak 960x1280; di layar 1080x2340 oval selebar sekitar 413 px foto.
        fun check(vararg faces: FitRect) = photoFits(faces.toList(), 960, 1280, viewWidth, viewHeight, mirror = true)
        assertTrue(check(box(480f, 512f, 300f)))
        // 25% lebar foto tetap diterima karena sudah pas di oval yang terlihat.
        assertTrue(check(box(480f, 512f, 240f)))
        // Wajah kecil jauh di belakang diabaikan, seperti di server.
        assertTrue(check(box(480f, 512f, 300f), box(100f, 100f, 60f)))
        assertFalse(check(box(480f, 512f, 300f), box(800f, 500f, 280f)))
        assertFalse(check())
        // Dilonggarkan (D55): 19% lebar foto diterima, 15% belum.
        assertTrue(check(box(480f, 512f, 180f)))
        assertFalse(check(box(480f, 512f, 140f)))
        assertFalse(check(box(200f, 512f, 300f)))
        assertFalse(check(box(480f, 1000f, 300f)))
    }
}

class DeviceRulesTest {
    private val phone = BuildInfo(
        fingerprint = "Infinix/X6511-OP/Infinix-X6511:10/QP1A.190711.020/GH-201015V180:user/release-keys",
        model = "Infinix X6511",
        manufacturer = "INFINIX MOBILITY LIMITED",
        brand = "Infinix",
        device = "Infinix-X6511",
        product = "X6511-OP",
        hardware = "mt6762",
    )

    @Test
    fun realPhoneIsNotEmulator() {
        assertFalse(DeviceRules.emulator(phone))
        // HP murah yang dibangun dengan test-keys tetap dianggap HP biasa.
        assertFalse(DeviceRules.emulator(phone.copy(fingerprint = phone.fingerprint.replace("release-keys", "test-keys"))))
    }

    @Test
    fun emulatorsAreRecognized() {
        assertTrue(DeviceRules.emulator(phone.copy(fingerprint = "generic/sdk_gphone_x86/generic_x86:10/QSR1/123:userdebug/dev-keys")))
        assertTrue(DeviceRules.emulator(phone.copy(hardware = "ranchu")))
        assertTrue(DeviceRules.emulator(phone.copy(model = "Android SDK built for x86")))
        assertTrue(DeviceRules.emulator(phone.copy(product = "sdk_gphone64_arm64")))
        assertTrue(DeviceRules.emulator(phone.copy(manufacturer = "Genymotion")))
    }

    @Test
    fun rootAndFakeGpsNeedConcreteEvidence() {
        val none = { _: String -> false }
        assertFalse(DeviceRules.rooted(none, none))
        assertTrue(DeviceRules.rooted({ it == "/system/xbin/su" }, none))
        assertTrue(DeviceRules.rooted(none, { it == "com.topjohnwu.magisk" }))
        assertFalse(DeviceRules.fakeGpsApp(none))
        assertTrue(DeviceRules.fakeGpsApp { it == "com.lexa.fakegps" })
    }

    @Test
    fun riskMessageNamesTheProblem() {
        assertFalse(DeviceRisk.CLEAN.blocked)
        val fake = DeviceRisk(rootDetected = false, emulatorDetected = false, fakeGpsAppDetected = true)
        assertTrue(fake.blocked)
        assertTrue(fake.message.contains("pemalsu lokasi"))
    }

    @Test
    fun manifestQueriesMatchRules() {
        // Tes unit Gradle berjalan dari folder modul app.
        val manifest = java.io.File("src/main/AndroidManifest.xml").readText()
        val queried = Regex("""<package android:name="([^"]+)" />""").findAll(manifest).map { it.groupValues[1] }.toSet()
        assertEquals((DeviceRules.ROOT_PACKAGES + DeviceRules.FAKE_GPS_PACKAGES).toSet(), queried)
    }
}

class RequestHashTest {
    @Test
    fun matchesBackendCanonicalVectorsD51() {
        val nonce = "a".repeat(64)
        val selfie = "b".repeat(64)
        val sample2 = "c".repeat(64)
        val sample3 = "d".repeat(64)
        val gesture = "e".repeat(64)

        val attendanceHash = RequestHash.attendance(
            nonce = nonce,
            latitude = -6.2001,
            longitude = 106.8166,
            capturedAt = "2026-11-07T01:02:03Z",
            selfieSha256 = selfie,
            gestureSha256 = gesture,
        )
        assertEquals("IZ0EvC-bV2dDeuNUnZAODJqQOd0txdevb-S9qNhGkZc", attendanceHash)

        val enrollmentHash = RequestHash.enrollment(
            nonce = nonce,
            latitude = -6.2001,
            longitude = 106.8166,
            capturedAt = "2026-11-07T01:02:03Z",
            consentVersion = "2026-11",
            action = "blink",
            samples = listOf(selfie, sample2, sample3),
            gestureSha256 = gesture,
        )
        assertEquals("08XZRVOoOvRmUBBFtp1UJmnEKytWCDZLxj1MmInTFXU", enrollmentHash)
    }

    @Test
    fun deviceProofSignsCanonicalMessage() {
        val kp = KeyPairGenerator.getInstance("EC").apply {
            initialize(ECGenParameterSpec("secp256r1"))
        }.generateKeyPair()
        val reqHash = "IZ0EvC-bV2dDeuNUnZAODJqQOd0txdevb-S9qNhGkZc"
        val proof = DeviceProof.sign(kp.private, reqHash)
        val sigBytes = Base64.getUrlDecoder().decode(proof)
        val verified = Signature.getInstance("SHA256withECDSA").run {
            initVerify(kp.public)
            update("HRIS-DEVICE-PROOF-V1\n$reqHash".toByteArray(Charsets.UTF_8))
            verify(sigBytes)
        }
        assertTrue(verified)

        val attChallenge = DeviceProof.attestationChallenge("pub-der-bytes".toByteArray(Charsets.UTF_8), reqHash)
        assertEquals(43, attChallenge.length)
        assertEquals("pLGtaxaQ7rwXnUDWCCdpdWegDdHT-XVaZktMnbXHywM", attChallenge)
    }

    @Test
    fun lateDurationAndOfflineHelpers() {
        assertEquals("45 menit", durationText(45))
        assertEquals("1 jam", durationText(60))
        assertEquals("1 jam 15 menit", durationText(75))
        assertEquals("2 jam 1 menit", durationText(121))

        val now = java.time.ZonedDateTime.of(2026, 11, 9, 9, 0, 0, 0, Wib)
        assertEquals("pukul 07:45", offlineSinceText(now.withHour(7).withMinute(45).toInstant().toEpochMilli(), now))
        assertEquals("8 Nov 2026, 20:10", offlineSinceText(now.minusDays(1).withHour(20).withMinute(10).toInstant().toEpochMilli(), now))
    }

    @Test
    fun appUpdateLevelAndHeader() {
        val v = AppVersion(latestCode = 14000, latestName = "0.14.0", minCode = 13000)
        assertEquals(UpdateLevel.Required, updateLevel(12000, v))
        assertEquals(UpdateLevel.Available, updateLevel(13000, v))
        assertEquals(UpdateLevel.None, updateLevel(14000, v))
        // Belum diatur super admin: tidak ada pemberitahuan.
        assertEquals(UpdateLevel.None, updateLevel(12000, AppVersion()))
        assertEquals("0.14.0+14000", appVersionHeader("0.14.0", 14000))
        // Isi Dto sama dengan skema AppVersion di openapi.yaml.
        val parsed = apiJson.decodeFromString(AppVersion.serializer(), "{\"latest_code\":14000,\"latest_name\":\"0.14.0\",\"min_code\":0,\"store_url\":\"https://x\",\"notes\":\"\",\"updated_at\":null}")
        assertEquals(14000, parsed.latestCode)
    }

    @Test
    fun savedPasswordIsReplacedNotExtended() {
        // Mengetik di atas kata sandi tersimpan yang tidak terlihat memulai kata sandi baru dari huruf yang diketik.
        assertEquals("x", editSavedPassword("rahasia123", "rahasia123x"))
        // Menghapus (atau mengganti seluruh isi) mengosongkan kolom, tidak menyisakan sebagian kata sandi lama.
        assertEquals("", editSavedPassword("rahasia123", "rahasia12"))
        assertEquals("", editSavedPassword("rahasia123", "abc"))
    }

    @Test
    fun profileContactHelpers() {
        assertEquals("081234567890", localPhone("6281234567890"))
        assertEquals("", localPhone(null))
        // Ganti email dikirim bersama kata sandi; null tidak ikut terkirim.
        assertEquals("{\"phone\":\"0812\"}", apiJson.encodeToString(MeUpdate.serializer(), MeUpdate(phone = "0812")))
    }

    @Test
    fun profilePhotoCropAndSample() {
        assertEquals(Triple(200, 0, 600), centerSquare(1000, 600))
        assertEquals(Triple(0, 100, 400), centerSquare(400, 600))
        assertEquals(1, sampleSizeFor(1200, 1600))
        assertEquals(2, sampleSizeFor(3000, 4000))
        assertEquals(4, sampleSizeFor(4096, 4096))
    }

    @Test
    fun nextPopupSkipsPostponedAndRepeatsDaily() {
        fun ann(id: String, type: String = "popup", read: Boolean = false, ends: String? = null) =
            MyAnnouncement(id = id, title = id, body = "", type = type, startsAt = "2026-11-20T00:00:00Z", endsAt = ends, readAt = if (read) "2026-11-19T01:00:00Z" else null)
        val items = listOf(ann("banner", type = "banner"), ann("a"), ann("b", read = true, ends = "2026-11-30T00:00:00Z"), ann("c", read = true))
        val never: (String) -> String? = { null }
        // Banner tidak pernah jadi pop-up; yang belum dibaca didahulukan.
        assertEquals("a", nextPopup(items, emptySet(), "2026-11-20", never)?.id)
        // "Nanti saja" melewati pop-up itu sampai daftar tunda dikosongkan saat aplikasi dibuka lagi.
        assertEquals("b", nextPopup(items, setOf("a"), "2026-11-20", never)?.id)
        // Sudah dibaca dan sudah ditutup hari ini: tidak muncul lagi sampai besok; tanpa tanggal berakhir tidak muncul lagi.
        assertEquals(null, nextPopup(items, setOf("a"), "2026-11-20") { if (it == "b") "2026-11-20" else null })
        assertEquals("b", nextPopup(items, setOf("a"), "2026-11-21") { if (it == "b") "2026-11-20" else null }?.id)
        assertEquals(null, nextPopup(null, emptySet(), "2026-11-20", never))
        // Muat ulang berkala lebih jarang dari coba ulang, dan keduanya wajar untuk baterai.
        assertTrue(ANNOUNCEMENT_RETRY_MS < ANNOUNCEMENT_POLL_MS && ANNOUNCEMENT_POLL_MS >= 60_000L)
    }

    @Test
    fun familyFormFromServerSortsChildren() {
        val server = MyFamily(
            hasChildren = true, childrenCount = 3,
            children = listOf(Child(2, "Budi", gender = "L"), Child(1, "Aisyah", gender = "P", schoolName = "MI Al Bahjah")),
        )
        val form = FamilyForm.from(server)
        assertEquals(listOf("Aisyah", "Budi"), form.children.map { it.name })
        assertEquals("3", form.count)
        assertEquals("MI Al Bahjah", form.children[0].school)
        assertEquals("", form.children[1].school)
    }

    @Test
    fun meanLumaSamplesYPlane() {
        // Bidang Y 32x32 dengan rowStride 40 (ada padding): kiri gelap (20), kanan terang (220).
        val width = 32
        val height = 32
        val stride = 40
        val bytes = ByteArray(stride * height) { i -> if (i % stride < width / 2) 20 else 220.toByte() }
        val luma = meanLuma(java.nio.ByteBuffer.wrap(bytes), width, height, stride, 1, step = 4)
        assertEquals(120f, luma, 0.01f)
        assertTrue(meanLuma(java.nio.ByteBuffer.wrap(ByteArray(stride * height) { 30 }), width, height, stride, 1) < LightRules.LOW_LUMA)
        assertEquals(255f, meanLuma(java.nio.ByteBuffer.allocate(0), 0, 0, 0, 1))
    }

    @Test
    fun rupiahFormatsServerDecimals() {
        assertEquals("Rp 0", rupiah("0.00"))
        assertEquals("Rp 999", rupiah("999.00"))
        assertEquals("Rp 4.973.250", rupiah("4973250.00"))
        assertEquals("Rp 1.000.000.000", rupiah("1000000000"))
        assertEquals("Rp 15.001", rupiah("15000.50"))
        assertEquals("-Rp 1.500", rupiah("-1500"))
        assertEquals("abc", rupiah("abc"))
    }

    @Test
    fun payslipLabels() {
        assertEquals("November 2026", periodName(2026, 11))
        assertEquals("Slip-Ubar-2026-03-PJ2019.pdf", payslipFileName(2026, 3, "PJ/2019"))
        assertEquals("Lembur (4 x Rp 20.000)", payslipItemLabel(PayslipItem("LEMBUR", "Lembur", "earning", "4.00", "20000.00", "80000.00")))
        assertEquals("Lembur (2.5 x Rp 20.000)", payslipItemLabel(PayslipItem("LEMBUR", "Lembur", "earning", "2.50", "20000.00", "50000.00")))
        assertEquals("PPh 21", payslipItemLabel(PayslipItem("PPH21", "PPh 21", "deduction", amount = "1.00")))
        assertTrue(dateRangeLabel("2026-10-26", "2026-11-25").startsWith("26 Okt"))
    }

    @Test
    fun payslipDecodesServerJson() {
        val slip = apiJson.decodeFromString(
            Payslip.serializer(),
            """{"id":"s1","document_number":"SU/2026/11/AB","year":2026,"month":11,"start_date":"2026-10-26","end_date":"2026-11-25",
               "status":"final","published_at":"2026-11-26T03:00:00Z","nip":"PJ1","full_name":"Agus","position":"","division":"Ops",
               "subdivision":"Gudang","attendance":{"work_days":"23","present_days":22,"late_count":2,"late_minutes":35,"absent_days":1,
               "leave_days":0,"unpaid_leave_days":0,"review":0,"missed_clock_out":0},
               "items":[{"code":"UBAR_POKOK","name":"Ubar pokok","kind":"earning","quantity":null,"rate":null,"amount":"4600000.00","show_on_slip":true,"manual":false}],
               "gross":"4600000.00","deductions":"0.00","net":"4600000.00","net_clamped":false}""",
        )
        assertEquals(22, slip.attendance.presentDays)
        assertEquals("4600000.00", slip.items.single().amount)
    }
}
