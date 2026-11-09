package id.pejuang.hris.ui

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ColumnScope
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.Card
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.material3.lightColorScheme
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.unit.dp
import java.time.Instant
import java.time.ZoneId
import java.time.format.DateTimeFormatter

val Green = Color(0xFF15803D)

@Composable
fun HrisTheme(content: @Composable () -> Unit) {
    MaterialTheme(
        colorScheme = lightColorScheme(primary = Green, secondary = Color(0xFFCA8A04)),
        content = content,
    )
}

@Composable
fun Screen(title: String, content: @Composable ColumnScope.() -> Unit) {
    Column(
        modifier = Modifier
            .fillMaxSize()
            .verticalScroll(rememberScrollState())
            .padding(20.dp),
        verticalArrangement = Arrangement.spacedBy(12.dp),
    ) {
        Text(title, style = MaterialTheme.typography.headlineSmall, color = Green)
        content()
    }
}

@Composable
fun InfoCard(title: String, body: String, tone: Color = Color(0xFFF1F5F9)) {
    Card(colors = CardDefaults.cardColors(containerColor = tone), modifier = Modifier.fillMaxWidth()) {
        Column(Modifier.padding(16.dp), verticalArrangement = Arrangement.spacedBy(4.dp)) {
            Text(title, style = MaterialTheme.typography.titleMedium)
            Text(body, style = MaterialTheme.typography.bodyMedium)
        }
    }
}

@Composable
fun ErrorText(message: String?) {
    if (message != null) Text(message, color = MaterialTheme.colorScheme.error)
}

val Amber = Color(0xFFFEF3C7)
val Red = Color(0xFFFEE2E2)
val LightGreen = Color(0xFFDCFCE7)

private val wib = ZoneId.of("Asia/Jakarta")
private val timeFormat = DateTimeFormatter.ofPattern("HH:mm")

/** Jam WIB dari waktu RFC3339 server, atau "-" bila kosong. */
fun wibTime(iso: String?): String = iso?.let {
    runCatching { Instant.parse(it).atZone(wib).format(timeFormat) }.getOrNull()
} ?: "-"
