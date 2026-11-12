package id.pejuang.hris

import id.pejuang.hris.data.AttendanceSignals
import id.pejuang.hris.data.ClockInput
import id.pejuang.hris.data.DeviceRegisterInput
import id.pejuang.hris.data.MobileLoginRequest
import id.pejuang.hris.data.apiJson
import id.pejuang.hris.data.normalizeBaseUrl
import id.pejuang.hris.data.sha256Hex
import id.pejuang.hris.face.FaceFit
import id.pejuang.hris.face.FaceSample
import id.pejuang.hris.face.FitRect
import id.pejuang.hris.face.FitRules
import id.pejuang.hris.face.LivenessState
import id.pejuang.hris.face.LivenessTracker
import id.pejuang.hris.face.checkFit
import id.pejuang.hris.face.faceOval
import id.pejuang.hris.face.mapToView
import id.pejuang.hris.face.photoFits
import id.pejuang.hris.face.scaledSize
import id.pejuang.hris.location.roundCoordinate
import id.pejuang.hris.security.BuildInfo
import id.pejuang.hris.security.DeviceRisk
import id.pejuang.hris.security.DeviceRules
import id.pejuang.hris.ui.toPickerMillis
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

class LivenessTrackerTest {
    private fun face(yaw: Float = 0f, eyes: Float? = 0.9f, smile: Float? = 0.1f, fit: FaceFit = FaceFit.Fit) =
        FaceSample(leftEyeOpen = eyes, rightEyeOpen = eyes, smiling = smile, yaw = yaw, fit = fit)

    // Tes gerakan memakai satu frame per tahap tahan posisi agar ringkas; aturan 8 frame dites terpisah.
    private fun tracker(action: String) = LivenessTracker(action, holdFrames = 1)

    @Test
    fun straightStageNeedsEightFitFrames() {
        val t = LivenessTracker("blink")
        assertEquals(8, FitRules.HOLD_FRAMES)
        repeat(7) { i ->
            val s = t.update(listOf(face())) as LivenessState.Waiting
            assertEquals("Tahan posisi", s.hint)
            assertTrue(s.aligned)
            assertEquals((i + 1) / 8f, s.progress, 1e-6f)
        }
        assertEquals(LivenessState.Waiting("Kedipkan mata", aligned = true), t.update(listOf(face())))
    }

    @Test
    fun badFrameRestartsHold() {
        val t = LivenessTracker("blink")
        repeat(5) { t.update(listOf(face())) }
        assertEquals(LivenessState.Waiting("Dekatkan wajah"), t.update(listOf(face(fit = FaceFit.TooSmall))))
        assertEquals(LivenessState.Waiting("Tegakkan kepala"), t.update(listOf(face(fit = FaceFit.Tilted))))
        repeat(7) { assertEquals("Tahan posisi", (t.update(listOf(face())) as LivenessState.Waiting).hint) }
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

class RequestsLogicTest {
    private val now = java.time.ZonedDateTime.of(2026, 11, 9, 7, 45, 0, 0, id.pejuang.hris.ui.Wib)

    @Test
    fun dateRangesAreCompact() {
        val d = { s: String -> java.time.LocalDate.parse(s) }
        assertEquals("16 November 2026", id.pejuang.hris.ui.dateRangeLabel(d("2026-11-16"), d("2026-11-16")))
        assertEquals("16–18 November 2026", id.pejuang.hris.ui.dateRangeLabel(d("2026-11-16"), d("2026-11-18")))
        assertTrue(id.pejuang.hris.ui.dateRangeLabel(d("2026-10-30"), d("2026-11-02")).matches(Regex("""30 \p{L}+ – 2 \p{L}+ 2026""")))
        assertTrue(id.pejuang.hris.ui.dateRangeLabel(d("2026-12-28"), d("2027-01-02")).matches(Regex("""28 \p{L}+ 2026 – 2 \p{L}+ 2027""")))
        assertEquals("x – y", id.pejuang.hris.ui.dateRangeLabel("x", "y"))
    }

    @Test
    fun timeAgoSteps() {
        assertEquals("baru saja", id.pejuang.hris.ui.timeAgo("2026-11-09T00:45:00Z", now))
        assertEquals("5 menit lalu", id.pejuang.hris.ui.timeAgo("2026-11-09T00:40:00Z", now))
        assertEquals("3 jam lalu", id.pejuang.hris.ui.timeAgo("2026-11-08T21:45:00Z", now))
        assertEquals("2 hari lalu", id.pejuang.hris.ui.timeAgo("2026-11-07T00:45:00Z", now))
        assertEquals("1 Oktober 2026", id.pejuang.hris.ui.timeAgo("2026-10-01T03:00:00Z", now))
    }

    @Test
    fun pickerMillisRoundTrip() {
        val day = java.time.LocalDate.of(2026, 11, 16)
        assertEquals(1794787200000L, day.toPickerMillis())
        assertEquals(day, id.pejuang.hris.ui.pickerMillisToDate(day.toPickerMillis()))
    }

    @Test
    fun leaveFormChecksMatchServerRules() {
        val today = java.time.LocalDate.of(2026, 11, 9)
        val ok = id.pejuang.hris.ui.LeaveForm("t1", today.plusDays(7), today.plusDays(9), "Acara keluarga")
        assertTrue(id.pejuang.hris.ui.validateLeave(ok, today).isEmpty())
        val empty = id.pejuang.hris.ui.validateLeave(id.pejuang.hris.ui.LeaveForm(), today)
        assertEquals(setOf("leave_type_id", "start_date", "reason"), empty.keys)
        assertTrue(id.pejuang.hris.ui.validateLeave(ok.copy(end = today), today).containsKey("start_date"))
        assertTrue(id.pejuang.hris.ui.validateLeave(ok.copy(start = java.time.LocalDate.of(2026, 12, 30), end = java.time.LocalDate.of(2027, 1, 2)), today).containsKey("start_date"))
        assertTrue(id.pejuang.hris.ui.validateLeave(ok.copy(start = today.minusDays(31), end = today), today).containsKey("start_date"))
        assertTrue(id.pejuang.hris.ui.validateLeave(ok.copy(reason = "  a "), today).containsKey("reason"))
    }

    @Test
    fun progressFollowsCurrentStep() {
        val type = id.pejuang.hris.data.LeaveTypeRef("t1", "annual", "Cuti Tahunan")
        fun leave(status: String, steps: List<id.pejuang.hris.data.ApprovalStep>) =
            id.pejuang.hris.data.LeaveRequest("l", "e", type, "2026-11-16", "2026-11-18", 3, "x", status, "2026-11-09T00:00:00Z", null, steps)
        val waitingSdm = leave("pending", listOf(
            id.pejuang.hris.data.ApprovalStep(1, "kadiv", "Umar", "approved"),
            id.pejuang.hris.data.ApprovalStep(2, "sdm", null, "pending"),
        ))
        assertEquals("Menunggu SDM", id.pejuang.hris.ui.progressText(waitingSdm))
        val atKadiv = leave("pending", listOf(id.pejuang.hris.data.ApprovalStep(1, "kadiv", "Umar", "pending")))
        assertEquals("Menunggu Kepala Divisi · Umar", id.pejuang.hris.ui.progressText(atKadiv))
        val rejected = leave("rejected", listOf(id.pejuang.hris.data.ApprovalStep(1, "kadiv", "Umar", "rejected", "Tanggal bentrok")))
        assertEquals("Ditolak: Tanggal bentrok", id.pejuang.hris.ui.progressText(rejected))
        assertEquals("Dibatalkan", id.pejuang.hris.ui.progressText(leave("cancelled", emptyList())))
    }

    @Test
    fun fieldErrorsAreKeptPerField() {
        val err = id.pejuang.hris.data.errorFromBody(
            """{"error":{"code":"VALIDATION_FAILED","message":"Periksa kembali isian form.","details":{"fields":{"reason":"Alasan wajib diisi, minimal 3 karakter.","end_date":"Rentang tanggal ini tidak berisi hari kerja."}}}}""",
        )
        assertEquals("Alasan wajib diisi, minimal 3 karakter.", err?.fields?.get("reason"))
        assertEquals("Rentang tanggal ini tidak berisi hari kerja.", err?.fields?.get("end_date"))
    }

    @Test
    fun leaveInputUsesContractFields() {
        val body = apiJson.encodeToString(
            id.pejuang.hris.data.LeaveInput.serializer(),
            id.pejuang.hris.data.LeaveInput("t1", "2026-11-16", "2026-11-18", "Acara keluarga"),
        )
        assertEquals("""{"leave_type_id":"t1","start_date":"2026-11-16","end_date":"2026-11-18","reason":"Acara keluarga"}""", body)
        val decision = apiJson.encodeToString(id.pejuang.hris.data.ApprovalDecision.serializer(), id.pejuang.hris.data.ApprovalDecision(null))
        assertEquals("{}", decision)
    }
}

class AnnouncementLogicTest {
    @Test
    fun decodesServerShape() {
        val a = apiJson.decodeFromString(
            id.pejuang.hris.data.MyAnnouncement.serializer(),
            """{"id":"a1","title":"Rapat","body":"Isi","type":"popup","starts_at":"2026-11-02T01:00:00Z","ends_at":null,
               "min_read_seconds":15,"opened_at":"2026-11-02T01:00:05Z","read_at":null,"read_wait_seconds":11}""",
        )
        assertEquals(15, a.minReadSeconds)
        assertEquals(11, a.readWaitSeconds)
        assertEquals(null, a.readAt)
    }

    @Test
    fun readTooSoonUsesFriendlyMessage() {
        val err = id.pejuang.hris.data.errorFromBody(
            """{"error":{"code":"READ_TOO_SOON","message":"x","details":{"wait_seconds":5}}}""",
        )
        assertEquals("READ_TOO_SOON", err?.code)
        assertTrue(err?.message.orEmpty().startsWith("Baca dulu isinya"))
    }
}

class BrandingLogicTest {
    @Test
    fun fontSizeSnapsToNearestOption() {
        assertEquals(id.pejuang.hris.data.FontSize.Normal, id.pejuang.hris.data.FontSize.of(1f))
        assertEquals(id.pejuang.hris.data.FontSize.Large, id.pejuang.hris.data.FontSize.of(1.12f))
        assertEquals(id.pejuang.hris.data.FontSize.ExtraLarge, id.pejuang.hris.data.FontSize.of(9f))
        assertEquals(id.pejuang.hris.data.FontSize.Small, id.pejuang.hris.data.FontSize.of(0.1f))
    }

    @Test
    fun defaultFooterMatchesServer() {
        assertEquals("© 2027 PUTI (Pusat Teknologi Informasi) Al Bahjah", id.pejuang.hris.ui.defaultFooter(2027))
    }

    @Test
    fun brandingDecodes() {
        val b = apiJson.decodeFromString(
            id.pejuang.hris.data.Branding.serializer(),
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
    }

    @Test
    fun tooSmallAndTooLarge() {
        assertEquals(FaceFit.TooSmall, fit(box(540f, 936f, 0.5f * 756f)))
        assertEquals(FaceFit.TooLarge, fit(box(540f, 936f, 0.9f * 756f)))
    }

    @Test
    fun faceAtEdge() {
        // Wajah yang baru terlihat sedikit di tepi bingkai: pusatnya di luar oval.
        assertEquals(FaceFit.Outside, fit(box(60f, 936f, 0.6f * 756f)))
        assertEquals(FaceFit.Outside, fit(box(540f, 200f, 0.6f * 756f)))
        // Di dalam oval tetapi bergeser lebih dari 12% lebar atau 10% tinggi oval.
        assertEquals(FaceFit.OffCenter, fit(box(540f + 0.15f * 756f, 936f, 0.6f * 756f)))
        assertEquals(FaceFit.OffCenter, fit(box(540f, 936f + 0.12f * 998f, 0.6f * 756f)))
        // Pusat dan ukuran lolos, tetapi sisi wajah menembus garis oval.
        assertEquals(FaceFit.OffCenter, fit(box(540f + 0.11f * 756f, 936f, 0.84f * 756f)))
    }

    @Test
    fun tiltedHead() {
        val face = box(540f, 936f, 0.7f * 756f)
        assertEquals(FaceFit.Tilted, fit(face, roll = 15f))
        assertEquals(FaceFit.Tilted, fit(face, pitch = -13f))
        assertEquals(FaceFit.Fit, fit(face, roll = 11f, pitch = -11f))
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
        assertFalse(check(box(480f, 512f, 180f)))
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
