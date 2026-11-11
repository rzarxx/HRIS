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
import androidx.compose.ui.semantics.semantics
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
    val subtitle = when {
        today?.clockOutAt != null -> "Presensi hari ini selesai. Sampai jumpa besok."
        today?.clockInAt != null -> "Jangan lupa presensi pulang."
        else -> "Catat kehadiran sesuai jadwal kerja."
    }
    val past = records.orEmpty().filter { it.workDate != todayKey }

    LazyColumn(
        modifier.fillMaxSize().background(Hc.Canvas).statusBarsPadding(),
        contentPadding = PaddingValues(start = 16.dp, end = 16.dp, top = 8.dp, bottom = bottomContentSpace(20.dp)),
        verticalArrangement = Arrangement.spacedBy(12.dp),
        horizontalAlignment = Alignment.CenterHorizontally,
    ) {
        item(key = "judul") { PageHeader("Presensi", subtitle, ContentWidth) }
        item(key = "jam") {
            WorkingHourCard(
                todayMinutes = today?.let { workedMinutesOf(it, now) } ?: 0,
                periodMinutes = records?.let { monthWorkedMinutes(it, now) } ?: 0,
                period = monthPeriodLabel(now.toLocalDate()),
                action = action,
                onCapture = onCapture,
                modifier = ContentWidth,
            )
        }
        item(key = "judul-riwayat") {
            Text(
                "Riwayat 31 hari",
                style = figmaText(16, FontWeight.SemiBold),
                modifier = ContentWidth.fillMaxWidth().padding(start = 4.dp, top = 12.dp, bottom = 2.dp),
            )
        }
        when {
            error != null -> item {
                FigmaCard(ContentWidth, title = "Riwayat belum bisa dimuat", subtitle = error) {
                    FigmaSecondaryButton("Coba lagi", onRetry)
                }
            }
            records == null -> items(3) { SkeletonCard(ContentWidth, lines = 1) }
            past.isEmpty() -> item {
                FigmaCard(ContentWidth, title = "Belum ada riwayat", subtitle = "Presensi 31 hari terakhir akan muncul di sini.") {}
            }
            else -> items(past, key = { it.id }) { AttendanceHistoryCard(it, now, ContentWidth) }
        }
    }
}

@Composable
private fun WorkingHourCard(todayMinutes: Int, periodMinutes: Int, period: String, action: ClockAction, onCapture: (String) -> Unit, modifier: Modifier) {
    FigmaCard(modifier, title = "Total jam kerja", subtitle = "Periode $period", gap = 16.dp) {
        Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.spacedBy(12.dp)) {
            HourTile("Hari ini", hourMinute(todayMinutes), Modifier.weight(1f))
            HourTile("Periode ini", hourMinute(periodMinutes), Modifier.weight(1f))
        }
        FigmaPrimaryButton(action.label, onClick = { action.purpose?.let(onCapture) }, enabled = action.purpose != null)
        if (action.note != null) {
            Text(action.note, style = figmaText(12, color = Hc.InkSoft, lineHeight = 1.4f), textAlign = TextAlign.Center, modifier = Modifier.fillMaxWidth())
        }
    }
}

@Composable
private fun HourTile(label: String, value: String, modifier: Modifier) {
    Column(
        modifier
            .clip(RoundedCornerShape(16.dp))
            .background(Hc.Field)
            .padding(horizontal = 16.dp, vertical = 14.dp)
            .semantics(mergeDescendants = true) {},
        verticalArrangement = Arrangement.spacedBy(2.dp),
    ) {
        Text(label, style = figmaText(12, FontWeight.Medium, Hc.InkSoft))
        Row(verticalAlignment = Alignment.Bottom) {
            Text(value, style = figmaText(24, FontWeight.Bold, Hc.Ink, tracking = -0.3f).copy(lineHeight = 30.sp), maxLines = 1)
            Text(" jam", style = figmaText(13, color = Hc.InkSoft), modifier = Modifier.padding(bottom = 4.dp))
        }
    }
}

/** Baris riwayat ringkas: tanggal di kiri, jam masuk dan pulang serta total jam, chip bila bukan hadir. */
@Composable
fun AttendanceHistoryCard(record: AttendanceRecord, now: ZonedDateTime, modifier: Modifier = Modifier) {
    val date = runCatching { LocalDate.parse(record.workDate) }.getOrNull()
    val (status, tone) = statusLabel(record.status)
    val worked = workedMinutesOf(record, now)
    Row(
        modifier
            .fillMaxWidth()
            .softShadow(RoundedCornerShape(18.dp), 6.dp)
            .clip(RoundedCornerShape(18.dp))
            .background(Hc.Surface)
            .padding(14.dp)
            .semantics(mergeDescendants = true) {},
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(14.dp),
    ) {
        Column(
            Modifier.size(52.dp).clip(RoundedCornerShape(14.dp)).background(Hc.Field),
            horizontalAlignment = Alignment.CenterHorizontally,
            verticalArrangement = Arrangement.Center,
        ) {
            Text(date?.let(::shortDay) ?: "", style = figmaText(11, FontWeight.Medium, Hc.InkSoft).copy(lineHeight = 14.sp))
            Text(date?.dayOfMonth?.toString() ?: "?", style = figmaText(18, FontWeight.Bold, Hc.Ink).copy(lineHeight = 22.sp))
        }
        Column(Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(2.dp)) {
            Text(
                if (record.clockInAt == null) "Tidak ada presensi" else "${wibTime(record.clockInAt)} – ${wibTime(record.clockOutAt)}",
                style = figmaText(15, FontWeight.SemiBold, if (record.clockInAt == null) Hc.InkSoft else Hc.Ink),
                maxLines = 1,
            )
            Text(
                listOfNotNull(
                    date?.let { "${it.dayOfMonth} ${shortMonth(it)}" },
                    durationLabel(worked).takeIf { worked > 0 },
                    record.locationName ?: workModeLabel(record.workMode).takeIf { record.clockInAt != null },
                ).joinToString(" · "),
                style = figmaText(12, color = Hc.InkSoft),
                maxLines = 1,
                overflow = TextOverflow.Ellipsis,
            )
        }
        if (record.status != "present") StatusChip(status, tone)
    }
}
