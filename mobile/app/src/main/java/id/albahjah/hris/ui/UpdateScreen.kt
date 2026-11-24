package id.albahjah.hris.ui

import androidx.compose.foundation.clickable
import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Refresh
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.remember
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.style.TextAlign
import id.albahjah.hris.data.AppVersion

/**
 * Layar wajib perbarui (D63): versi aplikasi ini di bawah versi minimum, jadi server menolak permintaannya.
 * Menutupi seluruh isi sampai aplikasi diperbarui.
 */
@Composable
fun UpdateRequiredContent(version: AppVersion, onUpdate: () -> Unit) {
    StatusBarIcons(dark = true)
    val target = version.latestName.takeIf { it.isNotBlank() }?.let { "versi $it" } ?: "versi terbaru"
    CenterMessage(
        title = "Perbarui aplikasi",
        body = "Versi aplikasi ini sudah tidak didukung. Perbarui ke $target untuk melanjutkan presensi dan pengajuan.",
        icon = Icons.Filled.Refresh,
        tone = Tone.Info,
        modifier = Modifier.clickable(interactionSource = remember { MutableInteractionSource() }, indication = null) {},
        extra = {
            if (version.notes.isNotBlank()) {
                Text(version.notes, style = MaterialTheme.typography.bodyMedium, color = Hc.InkSoft, textAlign = TextAlign.Center)
            }
        },
    ) {
        PrimaryButton("Perbarui sekarang", onClick = onUpdate)
    }
}
