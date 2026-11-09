package id.pejuang.hris.ui

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.DateRange
import androidx.compose.material.icons.filled.LocationOn
import androidx.compose.material.icons.filled.Warning
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import id.pejuang.hris.data.AttendanceRecord
import id.pejuang.hris.data.Repository
import id.pejuang.hris.data.toApiError
import kotlinx.coroutines.CancellationException
import java.time.LocalDate

@Composable
fun HistoryScreen(repository: Repository, onBack: () -> Unit) {
    var rows by remember { mutableStateOf<List<AttendanceRecord>?>(null) }
    var error by remember { mutableStateOf<String?>(null) }
    var reload by remember { mutableIntStateOf(0) }

    LaunchedEffect(reload) {
        error = null
        try {
            rows = repository.history()
        } catch (e: CancellationException) {
            throw e
        } catch (e: Exception) {
            error = e.toApiError().message
        }
    }

    HistoryContent(rows = rows, error = error, onBack = onBack, onRetry = { rows = null; reload++ })
}

@Composable
fun HistoryContent(rows: List<AttendanceRecord>?, error: String?, onBack: () -> Unit, onRetry: () -> Unit) {
    Column(Modifier.fillMaxSize().background(Hc.Canvas)) {
        TopBar("Riwayat absen", onBack = onBack)
        val modifier = Modifier.weight(1f)
        when {
            error != null -> CenterMessage(
                title = "Riwayat belum bisa dimuat",
                body = error,
                icon = Icons.Filled.Warning,
                tone = Tone.Warning,
                modifier = modifier,
            ) {
                PrimaryButton("Coba lagi", onClick = onRetry)
            }
            rows == null -> CenterMessage(title = "Memuat riwayat", body = null, loading = true, modifier = modifier)
            rows.isEmpty() -> CenterMessage(
                title = "Belum ada absen",
                body = "Absen dalam 31 hari terakhir akan muncul di sini.",
                icon = Icons.Filled.DateRange,
                tone = Tone.Neutral,
                modifier = modifier,
            )
            else -> LazyColumn(
                modifier = modifier,
                contentPadding = PaddingValues(start = 20.dp, end = 20.dp, top = 16.dp, bottom = 24.dp),
                verticalArrangement = Arrangement.spacedBy(10.dp),
            ) {
                item { Summary(rows) }
                item { SectionLabel("31 hari terakhir") }
                items(rows, key = { it.id }) { HistoryRow(it) }
                item { Spacer(Modifier.navigationBarsPadding()) }
            }
        }
    }
}

@Composable
private fun Summary(rows: List<AttendanceRecord>) {
    val counts = rows.groupingBy { it.status }.eachCount()
    SectionCard(padding = 16.dp) {
        Row(Modifier.fillMaxWidth()) {
            SummaryCell("Hadir", counts["present"] ?: 0, Tone.Success, Modifier.weight(1f))
            SummaryCell("Terlambat", counts["late"] ?: 0, Tone.Warning, Modifier.weight(1f))
            SummaryCell("Tidak hadir", counts["absent"] ?: 0, Tone.Danger, Modifier.weight(1f))
            SummaryCell("Cuti/izin", counts["leave"] ?: 0, Tone.Info, Modifier.weight(1f))
        }
    }
}

@Composable
private fun SummaryCell(label: String, value: Int, tone: Tone, modifier: Modifier) {
    Column(modifier, horizontalAlignment = Alignment.CenterHorizontally) {
        Text(value.toString(), style = MaterialTheme.typography.headlineSmall, color = tone.content)
        Text(label, style = MaterialTheme.typography.labelSmall, color = Hc.InkSoft, maxLines = 1)
    }
}

@Composable
fun HistoryRow(r: AttendanceRecord) {
    val date = runCatching { LocalDate.parse(r.workDate) }.getOrNull()
    val (label, tone) = statusLabel(r.status)
    SectionCard(padding = 14.dp) {
        Row(verticalAlignment = Alignment.CenterVertically) {
            Column(
                Modifier.width(52.dp).clip(MaterialTheme.shapes.medium).background(Hc.Canvas).padding(vertical = 8.dp),
                horizontalAlignment = Alignment.CenterHorizontally,
            ) {
                Text(date?.let(::shortDay) ?: "", style = MaterialTheme.typography.labelSmall, color = Hc.InkSoft)
                Text(date?.dayOfMonth?.toString() ?: r.workDate, style = MaterialTheme.typography.titleLarge, color = Hc.Ink)
                Text(date?.let(::shortMonth) ?: "", style = MaterialTheme.typography.labelSmall, color = Hc.InkSoft)
            }
            Spacer(Modifier.width(14.dp))
            Column(Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(4.dp)) {
                Row(verticalAlignment = Alignment.CenterVertically) {
                    TimePair(HrisIcons.ClockIn, wibTime(r.clockInAt))
                    Spacer(Modifier.width(14.dp))
                    TimePair(HrisIcons.ClockOut, wibTime(r.clockOutAt))
                }
                Row(verticalAlignment = Alignment.CenterVertically) {
                    Icon(Icons.Filled.LocationOn, contentDescription = null, tint = Hc.InkMuted, modifier = Modifier.size(14.dp))
                    Spacer(Modifier.width(4.dp))
                    Text(
                        listOfNotNull(
                            r.locationName ?: workModeLabel(r.workMode),
                            r.lateMinutes.takeIf { it > 0 }?.let { "telat $it mnt" },
                        ).joinToString(" · "),
                        style = MaterialTheme.typography.bodySmall,
                        color = Hc.InkSoft,
                        maxLines = 1,
                        overflow = TextOverflow.Ellipsis,
                    )
                }
            }
            Spacer(Modifier.width(8.dp))
            StatusChip(label, tone)
        }
    }
}

@Composable
private fun TimePair(icon: androidx.compose.ui.graphics.vector.ImageVector, time: String) {
    Row(verticalAlignment = Alignment.CenterVertically) {
        Icon(icon, contentDescription = null, tint = Hc.Brand, modifier = Modifier.size(16.dp))
        Spacer(Modifier.width(4.dp))
        Text(time, style = MaterialTheme.typography.titleSmall, color = if (time == "--:--") Hc.InkMuted else Hc.Ink)
    }
}
