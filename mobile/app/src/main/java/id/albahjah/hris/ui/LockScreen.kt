package id.albahjah.hris.ui

import androidx.compose.foundation.clickable
import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Lock
import androidx.compose.runtime.Composable
import androidx.compose.runtime.remember
import androidx.compose.ui.Modifier

/**
 * Layar kunci aplikasi (D62). Menutupi seluruh isi dan menahan sentuhan sampai dibuka dengan sidik jari,
 * wajah, atau PIN HP. "Masuk dengan kata sandi" keluar dari sesi, untuk HP yang dipinjam orang lain.
 */
@Composable
fun LockContent(onUnlock: () -> Unit, onUsePassword: () -> Unit) {
    StatusBarIcons(dark = true)
    CenterMessage(
        title = "Aplikasi terkunci",
        body = "Buka dengan sidik jari, wajah, atau PIN HP Anda.",
        icon = Icons.Filled.Lock,
        tone = Tone.Info,
        modifier = Modifier.clickable(interactionSource = remember { MutableInteractionSource() }, indication = null) {},
    ) {
        PrimaryButton("Buka", onClick = onUnlock)
        SecondaryButton("Masuk dengan kata sandi", onClick = onUsePassword)
    }
}
