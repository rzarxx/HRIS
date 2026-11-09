package id.pejuang.hris.update

import android.content.ActivityNotFoundException
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.LinearProgressIndicator
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.unit.dp
import androidx.lifecycle.Lifecycle
import androidx.lifecycle.LifecycleEventObserver
import androidx.lifecycle.compose.LocalLifecycleOwner
import id.pejuang.hris.BuildConfig
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch

private sealed interface UpdateUi {
    data object Hidden : UpdateUi
    data class Available(val info: ReleaseInfo) : UpdateUi
    data class Downloading(val percent: Int?) : UpdateUi
    data object Verifying : UpdateUi
    data class Ready(val info: ReleaseInfo) : UpdateUi
    data class Error(val message: String) : UpdateUi
}

/** Pemeriksaan tidak memerlukan login; debug memakai applicationId berbeda dari APK rilis. */
@Composable
fun UpdatePrompt(allowDialog: Boolean) {
    if (BuildConfig.DEBUG) return
    val context = LocalContext.current
    val downloads = remember { UpdateDownloads(context.applicationContext) }
    val service = remember { ReleaseService() }
    val scope = rememberCoroutineScope()
    var ui by remember { mutableStateOf<UpdateUi>(UpdateUi.Hidden) }
    var check by remember { androidx.compose.runtime.mutableIntStateOf(0) }
    var installAllowed by remember { mutableStateOf(downloads.canInstall()) }
    val lifecycle = LocalLifecycleOwner.current.lifecycle
    DisposableEffect(lifecycle) {
        val observer = LifecycleEventObserver { _, event ->
            if (event == Lifecycle.Event.ON_RESUME) {
                installAllowed = downloads.canInstall()
                if (ui is UpdateUi.Hidden && downloads.currentInfo() != null) check++
            }
        }
        lifecycle.addObserver(observer)
        onDispose { lifecycle.removeObserver(observer) }
    }

    suspend fun verify(info: ReleaseInfo) {
        ui = UpdateUi.Verifying
        try {
            downloads.verify()
            ui = UpdateUi.Ready(info)
        } catch (e: CancellationException) { throw e } catch (e: Exception) {
            downloads.discard()
            ui = UpdateUi.Error(e.message ?: "APK tidak dapat diverifikasi")
        }
    }

    LaunchedEffect(check) {
        try {
            val saved = downloads.currentInfo()
            val current = AppVersion.parse(BuildConfig.VERSION_NAME)
            if (saved != null && current != null && (AppVersion.parse(saved.tag) ?: current) <= current) downloads.discard()
            when (val state = downloads.status()) {
                is DownloadState.Running -> ui = UpdateUi.Downloading(state.percent)
                is DownloadState.Ready -> verify(state.info)
                is DownloadState.Failed -> { downloads.discard(); ui = UpdateUi.Error(state.message) }
                DownloadState.Idle -> {
                    val release = service.latest(BuildConfig.VERSION_NAME)
                    ui = if (release == null) UpdateUi.Hidden else UpdateUi.Available(release)
                }
            }
        } catch (e: CancellationException) { throw e } catch (e: Exception) {
            ui = UpdateUi.Error(e.message ?: "Pembaruan tidak dapat diperiksa")
        }
    }
    LaunchedEffect(Unit) {
        while (true) {
            delay(700)
            if (ui !is UpdateUi.Downloading) continue
            try {
                when (val state = downloads.status()) {
                    is DownloadState.Running -> ui = UpdateUi.Downloading(state.percent)
                    is DownloadState.Ready -> verify(state.info)
                    is DownloadState.Failed -> { downloads.discard(); ui = UpdateUi.Error(state.message) }
                    DownloadState.Idle -> ui = UpdateUi.Error("Unduhan tidak ditemukan")
                }
            } catch (e: CancellationException) { throw e } catch (e: Exception) {
                ui = UpdateUi.Error(e.message ?: "Status unduhan tidak tersedia")
            }
        }
    }
    if (!allowDialog) return
    when (val state = ui) {
        UpdateUi.Hidden -> Unit
        is UpdateUi.Available -> AlertDialog(
            onDismissRequest = { ui = UpdateUi.Hidden }, title = { Text("Pembaruan tersedia") },
            text = { Text("Versi ${state.info.tag} tersedia. Unduh untuk memperbarui HRIS Pejuang.") },
            confirmButton = { TextButton(onClick = {
                try { downloads.start(state.info); ui = UpdateUi.Downloading(null) }
                catch (e: Exception) { ui = UpdateUi.Error(e.message ?: "Unduhan tidak dapat dimulai") }
            }) { Text("Unduh") } },
            dismissButton = { TextButton(onClick = { ui = UpdateUi.Hidden }) { Text("Nanti") } },
        )
        is UpdateUi.Downloading -> AlertDialog(
            onDismissRequest = { ui = UpdateUi.Hidden }, title = { Text("Mengunduh pembaruan") },
            text = { Column(verticalArrangement = Arrangement.spacedBy(12.dp)) {
                Text(state.percent?.let { "$it%" } ?: "Menunggu unduhan")
                if (state.percent == null) LinearProgressIndicator() else LinearProgressIndicator(progress = { state.percent / 100f })
                Text("Unduhan dapat berlanjut saat aplikasi ditutup.")
            } },
            confirmButton = { TextButton(onClick = { ui = UpdateUi.Hidden }) { Text("Sembunyikan") } },
            dismissButton = { TextButton(onClick = { downloads.discard(); ui = UpdateUi.Hidden }) { Text("Batalkan") } },
        )
        UpdateUi.Verifying -> AlertDialog(onDismissRequest = {}, title = { Text("Memeriksa APK") }, text = { CircularProgressIndicator() }, confirmButton = {})
        is UpdateUi.Ready -> AlertDialog(
            onDismissRequest = { ui = UpdateUi.Hidden }, title = { Text("Pembaruan siap dipasang") },
            text = { Text("Versi ${state.info.tag} telah diperiksa. Android akan meminta persetujuan pemasangan.") },
            confirmButton = { TextButton(onClick = {
                scope.launch {
                    try {
                        if (!downloads.canInstall()) {
                            context.startActivity(downloads.settingsIntent())
                        } else {
                            ui = UpdateUi.Verifying
                            val file = downloads.verify()
                            context.startActivity(downloads.installIntent(file))
                            ui = UpdateUi.Ready(state.info)
                        }
                    } catch (e: CancellationException) { throw e } catch (e: ActivityNotFoundException) {
                        ui = UpdateUi.Error("Pengaturan atau pemasang paket tidak tersedia di HP ini")
                    } catch (e: Exception) { ui = UpdateUi.Error(e.message ?: "APK tidak dapat dipasang") }
                }
            }) { Text(if (installAllowed) "Pasang" else "Izinkan pemasangan") } },
            dismissButton = { TextButton(onClick = { ui = UpdateUi.Hidden }) { Text("Nanti") } },
        )
        is UpdateUi.Error -> AlertDialog(
            onDismissRequest = { ui = UpdateUi.Hidden }, title = { Text("Pembaruan belum tersedia") },
            text = { Text(state.message) },
            confirmButton = { TextButton(onClick = { check++ }) { Text("Coba lagi") } },
            dismissButton = { TextButton(onClick = { ui = UpdateUi.Hidden }) { Text("Tutup") } },
        )
    }
}
