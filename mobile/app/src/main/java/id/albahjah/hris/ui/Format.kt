package id.albahjah.hris.ui

import java.time.Instant
import java.time.LocalDate
import java.time.ZoneId
import java.time.ZonedDateTime
import java.time.format.DateTimeFormatter
import java.util.Locale

val Wib: ZoneId = ZoneId.of("Asia/Jakarta")
private val indonesian: Locale = Locale.forLanguageTag("id-ID")
private val timeFormat = DateTimeFormatter.ofPattern("HH:mm")
private val longDate = DateTimeFormatter.ofPattern("EEEE, d MMMM yyyy", indonesian)
private val monthShort = DateTimeFormatter.ofPattern("MMM", indonesian)
private val dayShort = DateTimeFormatter.ofPattern("EEE", indonesian)

/** Jam WIB dari waktu RFC3339 server, atau "--:--" bila kosong. */
fun wibTime(iso: String?): String = iso?.let {
    runCatching { Instant.parse(it).atZone(Wib).format(timeFormat) }.getOrNull()
} ?: "--:--"

fun longDateLabel(date: LocalDate): String = date.format(longDate)

fun shortMonth(date: LocalDate): String = date.format(monthShort).removeSuffix(".")

fun shortDay(date: LocalDate): String = date.format(dayShort).removeSuffix(".")

fun greeting(now: ZonedDateTime): String = when (now.hour) {
    in 4..10 -> "Selamat pagi"
    in 11..14 -> "Selamat siang"
    in 15..17 -> "Selamat sore"
    else -> "Selamat malam"
}

/** Inisial dua huruf untuk avatar. */
fun initials(name: String): String =
    name.split(' ').filter { it.isNotBlank() }.take(2).joinToString("") { it.first().uppercase() }.ifEmpty { "?" }

fun statusLabel(status: String?): Pair<String, Tone> = when (status) {
    "present" -> "Hadir" to Tone.Success
    "late" -> "Terlambat" to Tone.Warning
    "absent" -> "Tidak hadir" to Tone.Danger
    "leave" -> "Cuti/izin" to Tone.Info
    "review" -> "Ditinjau SDM" to Tone.Warning
    null -> "Belum presensi" to Tone.Neutral
    else -> status to Tone.Neutral
}

fun workModeLabel(mode: String): String = when (mode) {
    "onsite" -> "Kantor"
    "wfa" -> "WFA"
    "field" -> "Di luar kantor"
    else -> mode
}

fun durationLabel(minutes: Int): String {
    val h = minutes / 60
    val m = minutes % 60
    return if (h == 0) "$m mnt" else "$h j $m mnt"
}

private val dayMonthYear = DateTimeFormatter.ofPattern("d MMMM yyyy", indonesian)
private val dayMonthShortYear = DateTimeFormatter.ofPattern("d MMM yyyy", indonesian)

/** Tanggal seperti "9 November 2026". */
fun dateLabel(date: LocalDate): String = date.format(dayMonthYear)

/** Periode bulan berjalan, mis. "1 Nov 2026 - 30 Nov 2026". */
fun monthPeriodLabel(date: LocalDate): String {
    val first = date.withDayOfMonth(1)
    val last = date.withDayOfMonth(date.lengthOfMonth())
    return "${first.format(dayMonthShortYear).replace(".", "")} - ${last.format(dayMonthShortYear).replace(".", "")}"
}

/** Durasi format jam:menit, mis. 527 menit menjadi "08:47". */
fun hourMinute(minutes: Int): String = "%02d:%02d".format(Locale.ROOT, minutes / 60, minutes % 60)

private val dayMonthShort = DateTimeFormatter.ofPattern("d MMM", indonesian)
private val dateTimeShort = DateTimeFormatter.ofPattern("d MMM yyyy, HH:mm", indonesian)

private fun short(date: LocalDate, pattern: DateTimeFormatter) = date.format(pattern).replace(".", "")

/**
 * Rentang tanggal ringkas: "16 November 2026", "16–18 November 2026", "30 Okt – 2 Nov 2026", atau
 * "28 Des 2026 – 2 Jan 2027".
 */
fun dateRangeLabel(start: LocalDate, end: LocalDate): String = when {
    start == end -> dateLabel(start)
    start.year != end.year -> "${short(start, dayMonthShortYear)} – ${short(end, dayMonthShortYear)}"
    start.month != end.month -> "${short(start, dayMonthShort)} – ${short(end, dayMonthShortYear)}"
    else -> "${start.dayOfMonth}–${dateLabel(end)}"
}

/** Rentang dari teks tanggal server (yyyy-MM-dd); teks asli bila tidak bisa dibaca. */
fun dateRangeLabel(start: String, end: String): String {
    val s = runCatching { LocalDate.parse(start) }.getOrNull()
    val e = runCatching { LocalDate.parse(end) }.getOrNull()
    return if (s != null && e != null) dateRangeLabel(s, e) else "$start – $end"
}

/** Tanggal dan jam WIB, mis. "9 Nov 2026, 07:45". */
fun wibDateTime(iso: String?): String = iso?.let {
    runCatching { Instant.parse(it).atZone(Wib).format(dateTimeShort).replace(".", "") }.getOrNull()
} ?: "-"

/**
 * Keterangan banner mode offline (D62): kapan data yang tampil terakhir diperbarui, mis. "pukul 07:45"
 * untuk hari ini atau "9 Nov 2026, 07:45" untuk hari lain.
 */
fun offlineSinceText(savedAt: Long, now: ZonedDateTime = ZonedDateTime.now(Wib)): String {
    val then = Instant.ofEpochMilli(savedAt).atZone(Wib)
    return if (then.toLocalDate() == now.toLocalDate()) "pukul ${then.format(timeFormat)}"
    else then.format(dateTimeShort).replace(".", "")
}

/** Waktu relatif dari [now]: "baru saja", "5 menit lalu", "3 jam lalu", "2 hari lalu", lalu tanggal. */
fun timeAgo(iso: String, now: ZonedDateTime): String {
    val then = runCatching { Instant.parse(iso) }.getOrNull() ?: return iso
    val minutes = java.time.Duration.between(then, now.toInstant()).toMinutes()
    return when {
        minutes < 1 -> "baru saja"
        minutes < 60 -> "$minutes menit lalu"
        minutes < 24 * 60 -> "${minutes / 60} jam lalu"
        minutes < 7 * 24 * 60 -> "${minutes / (24 * 60)} hari lalu"
        else -> dateLabel(then.atZone(Wib).toLocalDate())
    }
}

/** Status pengajuan (RequestStatus). */
fun requestStatusLabel(status: String): Pair<String, Tone> = when (status) {
    "pending" -> "Menunggu" to Tone.Warning
    "approved" -> "Disetujui" to Tone.Success
    "rejected" -> "Tidak disetujui" to Tone.Danger
    "cancelled" -> "Dibatalkan" to Tone.Neutral
    else -> status to Tone.Neutral
}

/** Peran penyetuju di alur persetujuan. */
fun approverRoleLabel(role: String): String = when (role) {
    "subdiv" -> "Kepala Subdiv"
    "kadiv" -> "Kepala Divisi"
    "sdm" -> "SDM"
    else -> role
}

/** Jenis pengajuan (RequestKind). */
fun requestKindLabel(kind: String): String = when (kind) {
    "leave" -> "Cuti/izin"
    "cash_advance" -> "Pinjaman"
    "transfer" -> "Mutasi"
    else -> kind
}

/** Footer bawaan sebelum footer dari server termuat; sama dengan teks bawaan di backend. */
fun defaultFooter(year: Int = ZonedDateTime.now(Wib).year): String = "© $year PUTI (Pusat Teknologi Informasi) Al Bahjah"

/** Tanggal kalender ke milidetik UTC tengah malam, format yang dipakai DatePicker Material3. */
fun LocalDate.toPickerMillis(): Long = atStartOfDay(java.time.ZoneOffset.UTC).toInstant().toEpochMilli()

fun pickerMillisToDate(millis: Long): LocalDate = Instant.ofEpochMilli(millis).atZone(java.time.ZoneOffset.UTC).toLocalDate()

private val monthNames = listOf(
    "Januari", "Februari", "Maret", "April", "Mei", "Juni",
    "Juli", "Agustus", "September", "Oktober", "November", "Desember",
)

/** Nama periode slip, mis. "November 2026". */
fun periodName(year: Int, month: Int): String = "${monthNames.getOrElse(month - 1) { "" }} $year".trim()

/**
 * Rupiah tanpa sen dengan titik ribuan dari angka desimal server ("4973250.00" menjadi "Rp 4.973.250").
 * Angka yang tidak terbaca ditampilkan apa adanya.
 */
fun rupiah(raw: String): String {
    val value = raw.trim().toBigDecimalOrNull() ?: return raw
    val whole = value.setScale(0, java.math.RoundingMode.HALF_UP)
    val digits = whole.abs().toPlainString()
    val grouped = digits.reversed().chunked(3).joinToString(".").reversed()
    return (if (whole.signum() < 0) "-Rp " else "Rp ") + grouped
}

/** Rentang tanggal periode, mis. "26 Okt 2026 - 25 Nov 2026". */
fun dateRangeLabel(start: String, end: String): String {
    val from = runCatching { LocalDate.parse(start).format(dayMonthShortYear).replace(".", "") }.getOrDefault(start)
    val to = runCatching { LocalDate.parse(end).format(dayMonthShortYear).replace(".", "") }.getOrDefault(end)
    return "$from - $to"
}
