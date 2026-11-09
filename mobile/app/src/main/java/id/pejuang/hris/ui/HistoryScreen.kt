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
import androidx.compose.foundation.layout.statusBarsPadding
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowBack
import androidx.compose.material.icons.filled.DateRange
import androidx.compose.material.icons.filled.LocationOn
import androidx.compose.material.icons.filled.Warning
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
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
import androidx.compose.ui.graphics.Color
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
    StatusBarIcons(dark = false)
    Column(Modifier.fillMaxSize().background(Hc.Canvas)) {
        Column(Modifier.fillMaxWidth().background(Hc.Brand).statusBarsPadding().padding(bottom = 20.dp)) {
            Row(verticalAlignment = Alignment.CenterVertically) {
                IconButton(onClick = onBack) { Icon(Icons.AutoMirrored.Filled.ArrowBack, contentDescription = "Kembali", tint = Color.White) }
                Text("Presensi", style = MaterialTheme.typography.titleMedium, color = Color.White)
            }
            Text("Riwayat kehadiran", style = MaterialTheme.typography.headlineSmall, color = Color.White, modifier = Modifier.padding(horizontal = 16.dp))
            Text("Catatan presensi 31 hari terakhir", style = MaterialTheme.typography.bodyMedium, color = Color.White.copy(alpha = 0.85f), modifier = Modifier.padding(horizontal = 16.dp))
        }
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
                contentPadding = PaddingValues(start = 12.dp, end = 12.dp, top = 16.dp, bottom = 24.dp),
                verticalArrangement = Arrangement.spacedBy(16.dp),
            ) {
                item { Summary(rows) }
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
    SectionCard(padding = 16.dp) {
        Row(verticalAlignment = Alignment.CenterVertically) {
            Icon(Icons.Filled.DateRange, contentDescription = null, tint = Hc.Brand, modifier = Modifier.size(18.dp))
            Spacer(Modifier.width(8.dp))
            Text(date?.let(::longDateLabel) ?: r.workDate, style = MaterialTheme.typography.titleSmall, color = Hc.Ink, modifier = Modifier.weight(1f))
            StatusChip(label, tone)
        }
        Spacer(Modifier.size(12.dp))
        Column(Modifier.fillMaxWidth().clip(MaterialTheme.shapes.medium).background(Hc.Canvas).padding(12.dp), verticalArrangement = Arrangement.spacedBy(6.dp)) {
                Row(verticalAlignment = Alignment.CenterVertically) {
                    Text("Masuk / Pulang", style = MaterialTheme.typography.bodySmall, color = Hc.InkSoft, modifier = Modifier.weight(1f))
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
