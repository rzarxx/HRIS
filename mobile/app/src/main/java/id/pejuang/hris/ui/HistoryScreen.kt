package id.pejuang.hris.ui

import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import id.pejuang.hris.data.AttendanceRecord
import id.pejuang.hris.data.Repository
import id.pejuang.hris.data.toApiError

private val statusLabel = mapOf(
    "present" to "Hadir",
    "late" to "Terlambat",
    "absent" to "Tidak hadir",
    "leave" to "Cuti/izin",
    "review" to "Ditinjau SDM",
)

private val modeLabel = mapOf("onsite" to "Kantor", "wfa" to "WFA", "field" to "Di luar kantor")

@Composable
fun HistoryScreen(repository: Repository, onBack: () -> Unit) {
    var rows by remember { mutableStateOf<List<AttendanceRecord>?>(null) }
    var error by remember { mutableStateOf<String?>(null) }

    LaunchedEffect(Unit) {
        try {
            rows = repository.history()
        } catch (e: Exception) {
            error = e.toApiError().message
        }
    }

    Screen("Riwayat absen") {
        OutlinedButton(onClick = onBack) { Text("Kembali") }
        val list = rows
        when {
            error != null -> ErrorText(error)
            list == null -> CircularProgressIndicator()
            list.isEmpty() -> Text("Belum ada absen dalam 31 hari terakhir.")
            else -> list.forEach { r ->
                val late = if (r.lateMinutes > 0) " · telat ${r.lateMinutes} menit" else ""
                val place = r.locationName ?: modeLabel[r.workMode] ?: r.workMode
                InfoCard(
                    "${r.workDate} · ${statusLabel[r.status] ?: r.status}",
                    "Masuk ${wibTime(r.clockInAt)} · Pulang ${wibTime(r.clockOutAt)}$late\n$place",
                    when (r.status) {
                        "late", "review" -> Amber
                        "absent" -> Red
                        else -> LightGreen
                    },
                )
            }
        }
    }
}
