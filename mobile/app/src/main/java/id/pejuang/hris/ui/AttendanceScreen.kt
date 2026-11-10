package id.pejuang.hris.ui

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.offset
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.statusBarsPadding
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Icon
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import id.pejuang.hris.data.AttendanceRecord
import id.pejuang.hris.data.Me
import id.pejuang.hris.data.PURPOSE_CLOCK_IN
import id.pejuang.hris.data.PURPOSE_CLOCK_OUT
import id.pejuang.hris.data.PURPOSE_FACE_ENROLL
import java.time.Duration
import java.time.Instant
import java.time.LocalDate
import java.time.ZonedDateTime

/** Menit kerja satu catatan; presensi hari ini yang belum pulang dihitung sampai [now]. */
fun workedMinutesOf(record: AttendanceRecord, now: ZonedDateTime): Int {
    record.workedMinutes?.let { return it }
    val start = record.clockInAt?.let { runCatching { Instant.parse(it) }.getOrNull() } ?: return 0
    val end = record.clockOutAt?.let { runCatching { Instant.parse(it) }.getOrNull() }
        ?: if (record.workDate == now.toLocalDate().toString()) now.toInstant() else return 0
    return Duration.between(start, end).toMinutes().toInt().coerceAtLeast(0)
}

/** Total menit kerja pada bulan berjalan (periode ubar sementara, sebelum cutoff D31). */
fun monthWorkedMinutes(records: List<AttendanceRecord>, now: ZonedDateTime): Int {
    val month = now.toLocalDate().withDayOfMonth(1)
    return records.filter { r -> runCatching { !LocalDate.parse(r.workDate).isBefore(month) }.getOrDefault(false) }
        .sumOf { workedMinutesOf(it, now) }
}

/** Isi tombol utama Presensi sesuai keadaan Pejuang hari ini. */
data class ClockAction(val label: String, val purpose: String?, val note: String?)

fun clockAction(me: Me, today: AttendanceRecord?): ClockAction = when {
    me.deviceStatus == "pending" -> ClockAction("Presensi masuk sekarang", null, "HP ini menunggu persetujuan SDM.")
    me.deviceStatus != "approved" -> ClockAction("Presensi masuk sekarang", null, "HP ini sudah dicabut. Hubungi SDM.")
    me.mustChangePassword -> ClockAction("Presensi masuk sekarang", null, "Ganti kata sandi dulu di menu Profil.")
    !me.faceEnrolled -> ClockAction("Daftarkan wajah sekarang", PURPOSE_FACE_ENROLL, "Sekali saja, di lokasi kantor, sebelum presensi pertama.")
    today?.status == "leave" && today?.clockInAt == null -> ClockAction("Hari ini cuti/izin", null, "Cuti atau izin Anda hari ini sudah disetujui.")
    today?.clockInAt == null -> ClockAction("Presensi masuk sekarang", PURPOSE_CLOCK_IN, null)
    today?.clockOutAt == null -> ClockAction("Presensi pulang sekarang", PURPOSE_CLOCK_OUT, null)
    else -> ClockAction("Presensi hari ini selesai", null, null)
}

/** Lebar maksimum isi di tablet dan layar lebar, agar kartu tidak melebar sepanjang layar. */
val ContentWidth = Modifier.widthIn(max = 560.dp)

/** Tab Presensi, mengikuti frame "Attendant" di kit Figma. */
@Composable
fun AttendanceContent(
    me: Me,
    records: List<AttendanceRecord>?,
    error: String?,
    now: ZonedDateTime,
    onCapture: (String) -> Unit,
    onRetry: () -> Unit,
    modifier: Modifier = Modifier,
) {
    val todayKey = now.toLocalDate().toString()
    val today = records?.firstOrNull { it.workDate == todayKey }
    val action = clockAction(me, today)
    val (title, subtitle) = when {
        today?.clockOutAt != null -> "Presensi selesai!" to "Terima kasih, sampai jumpa besok"
        today?.clockInAt != null -> "Ayo presensi pulang!" to "Jangan lupa presensi sebelum pulang"
        else -> "Ayo presensi masuk!" to "Jangan lewatkan jadwal presensi Anda"
    }
    val past = records.orEmpty().filter { it.workDate != todayKey }

    Box(modifier.fillMaxSize().background(Fg.Page)) {
        FigmaTabHeader(title, subtitle)
        // Daftar dimulai di bawah status bar agar kartu yang digulir tidak menembus jam dan ikon sistem.
        LazyColumn(
            Modifier.fillMaxSize().statusBarsPadding(),
            contentPadding = PaddingValues(start = 12.dp, end = 12.dp, bottom = bottomContentSpace(16.dp)),
            verticalArrangement = Arrangement.spacedBy(16.dp),
            horizontalAlignment = Alignment.CenterHorizontally,
        ) {
            item {
                // Kartu menimpa header: 144dp dari atas frame, yaitu 100dp di bawah status bar.
                Column(ContentWidth.padding(top = 100.dp)) {
                        WorkingHourCard(
                            todayMinutes = today?.let { workedMinutesOf(it, now) } ?: 0,
                            periodMinutes = records?.let { monthWorkedMinutes(it, now) } ?: 0,
                            period = monthPeriodLabel(now.toLocalDate()),
                            action = action,
                            onCapture = onCapture,
                        )
                }
            }
            when {
                error != null -> item {
                    FigmaCard(ContentWidth, title = "Riwayat belum bisa dimuat", subtitle = error) {
                        FigmaSecondaryButton("Coba lagi", onRetry)
                    }
                }
                records == null -> items(3) { SkeletonCard(ContentWidth, lines = 2) }
                past.isEmpty() -> item {
                    FigmaCard(ContentWidth, title = "Belum ada riwayat", subtitle = "Presensi 31 hari terakhir akan muncul di sini.") {}
                }
                else -> items(past, key = { it.id }) { AttendanceHistoryCard(it, now, ContentWidth) }
            }
        }
    }
}

@Composable
private fun WorkingHourCard(todayMinutes: Int, periodMinutes: Int, period: String, action: ClockAction, onCapture: (String) -> Unit) {
    FigmaCard(title = "Total jam kerja", subtitle = "Periode $period", bottom = 16.dp) {
        Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.spacedBy(8.dp)) {
            HourTile("Hari ini", "${hourMinute(todayMinutes)} Jam", Modifier.weight(1f))
            HourTile("Periode ini", "${hourMinute(periodMinutes)} Jam", Modifier.weight(1f))
        }
        FigmaPrimaryButton(action.label, onClick = { action.purpose?.let(onCapture) }, enabled = action.purpose != null)
        if (action.note != null) {
            Text(action.note, style = figmaText(12, color = Fg.TextSecondary, lineHeight = 1.4f), textAlign = TextAlign.Center, modifier = Modifier.fillMaxWidth())
        }
    }
}

@Composable
private fun HourTile(label: String, value: String, modifier: Modifier) {
    Column(
        modifier
            .heightIn(min = 72.dp)
            .clip(RoundedCornerShape(8.dp))
            .background(androidx.compose.ui.graphics.Color(0xFFF9F9F9))
            .border(1.dp, Fg.Gray50, RoundedCornerShape(8.dp))
            .padding(start = 11.dp, end = 8.dp, top = 11.dp, bottom = 8.dp),
        verticalArrangement = Arrangement.spacedBy(4.dp),
    ) {
        Row(verticalAlignment = Alignment.Bottom, horizontalArrangement = Arrangement.spacedBy(4.dp)) {
            Icon(FigmaIcons.ClockBold, contentDescription = null, tint = Fg.Gray300, modifier = Modifier.size(16.dp))
            Text(label, style = figmaText(12, FontWeight.Medium, Fg.TextSecondary, tracking = -0.5f))
        }
        Text(
            value,
            style = figmaText(22, color = Fg.TextStrong).copy(lineHeight = 28.sp),
            maxLines = 1,
            overflow = TextOverflow.Ellipsis,
        )
    }
}

/** Kartu riwayat ("Attendant/Attendant History"). */
@Composable
fun AttendanceHistoryCard(record: AttendanceRecord, now: ZonedDateTime, modifier: Modifier = Modifier) {
    val date = runCatching { LocalDate.parse(record.workDate) }.getOrNull()
    val (status, tone) = statusLabel(record.status)
    FigmaCard(modifier) {
        Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(4.dp)) {
            Icon(FigmaIcons.CalendarOutline, contentDescription = null, tint = Fg.Accent, modifier = Modifier.size(16.dp))
            Text(date?.let(::dateLabel) ?: record.workDate, style = figmaText(14, FontWeight.SemiBold, lineHeight = 1.4f), modifier = Modifier.weight(1f))
            if (record.status != "present") StatusChip(status, tone)
        }
        Row(
            Modifier
                .fillMaxWidth()
                .clip(RoundedCornerShape(12.dp))
                .background(Fg.Gray100)
                .border(1.dp, Fg.Gray200, RoundedCornerShape(12.dp))
                .padding(12.dp),
            horizontalArrangement = Arrangement.SpaceBetween,
            verticalAlignment = Alignment.CenterVertically,
        ) {
            HistoryValue("Total jam", "${hourMinute(workedMinutesOf(record, now))}:00 jam", Modifier.weight(1f))
            HistoryValue("Masuk & pulang", "${wibTime(record.clockInAt)}  — ${wibTime(record.clockOutAt)}")
        }
    }
}

@Composable
private fun HistoryValue(label: String, value: String, modifier: Modifier = Modifier) {
    Column(modifier, verticalArrangement = Arrangement.spacedBy(4.dp)) {
        Text(label, style = figmaText(12, FontWeight.Medium, Fg.TextSecondary, tracking = -0.5f))
        Text(value, style = figmaText(16, FontWeight.Medium, Fg.TextBody, tracking = -0.5f), maxLines = 1, overflow = TextOverflow.Ellipsis)
    }
}
