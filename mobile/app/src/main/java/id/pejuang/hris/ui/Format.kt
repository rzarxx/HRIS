package id.pejuang.hris.ui

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
    null -> "Belum absen" to Tone.Neutral
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
