package id.pejuang.hris.ui

import android.Manifest
import android.content.pm.PackageManager
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.material3.Button
import androidx.compose.material3.Checkbox
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.core.content.ContextCompat
import id.pejuang.hris.data.AttendanceChallenge
import id.pejuang.hris.data.ClockOutcome
import id.pejuang.hris.data.PURPOSE_CLOCK_IN
import id.pejuang.hris.data.PURPOSE_FACE_ENROLL
import id.pejuang.hris.data.Repository
import id.pejuang.hris.data.toApiError
import id.pejuang.hris.face.FaceCamera
import id.pejuang.hris.location.LocationReader
import kotlinx.coroutines.launch

private sealed interface Step {
    data object Consent : Step
    data object Preparing : Step
    data class Camera(val challenge: AttendanceChallenge) : Step
    data object Sending : Step
    data class Done(val outcome: ClockOutcome) : Step
    data class Failed(val message: String) : Step
}

private val permissions = arrayOf(Manifest.permission.CAMERA, Manifest.permission.ACCESS_FINE_LOCATION)

/** Alur daftar wajah (F7) dan absen masuk/pulang (F8): challenge, liveness, foto, lokasi, kirim. */
@Composable
fun CaptureScreen(purpose: String, repository: Repository, location: LocationReader, onClose: () -> Unit) {
    val context = LocalContext.current
    val scope = rememberCoroutineScope()
    val enrolling = purpose == PURPOSE_FACE_ENROLL
    var step by remember { mutableStateOf<Step>(if (enrolling) Step.Consent else Step.Preparing) }
    var attempt by remember { mutableIntStateOf(0) }
    var granted by remember {
        mutableStateOf(permissions.all { ContextCompat.checkSelfPermission(context, it) == PackageManager.PERMISSION_GRANTED })
    }
    val launcher = rememberLauncherForActivityResult(ActivityResultContracts.RequestMultiplePermissions()) { result ->
        granted = result.values.all { it }
    }

    LaunchedEffect(granted, step, attempt) {
        if (step != Step.Preparing) return@LaunchedEffect
        if (!granted) {
            launcher.launch(permissions)
            return@LaunchedEffect
        }
        step = try {
            Step.Camera(repository.challenge(purpose))
        } catch (e: Exception) {
            Step.Failed(e.toApiError().message ?: "Gagal memulai.")
        }
    }

    fun submit(challenge: AttendanceChallenge, photos: List<ByteArray>) {
        step = Step.Sending
        scope.launch {
            step = try {
                val fix = location.current()
                if (enrolling) {
                    repository.enroll(challenge, photos, fix)
                    Step.Done(ClockOutcome("Wajah terdaftar", "Sekarang Anda bisa absen dengan selfie.", false))
                } else {
                    Step.Done(repository.clock(purpose, challenge, photos.first(), fix))
                }
            } catch (e: Exception) {
                Step.Failed(e.toApiError().message ?: "Gagal mengirim.")
            }
        }
    }

    val title = when (purpose) {
        PURPOSE_FACE_ENROLL -> "Daftarkan wajah"
        PURPOSE_CLOCK_IN -> "Absen masuk"
        else -> "Absen pulang"
    }

    when (val s = step) {
        Step.Consent -> ConsentStep(
            onAccept = {
                scope.launch {
                    // Persetujuan lokasi dicatat terpisah; biometrik dicatat server saat daftar wajah.
                    runCatching { repository.consentLocation() }
                    step = Step.Preparing
                }
            },
            onCancel = onClose,
        )
        Step.Preparing, Step.Sending -> Screen(title) {
            if (!granted && s == Step.Preparing) {
                Text("Aplikasi butuh izin kamera dan lokasi untuk absen.")
                Button(onClick = { launcher.launch(permissions) }) { Text("Berikan izin") }
                OutlinedButton(onClick = onClose) { Text("Batal") }
            } else {
                Row(verticalAlignment = Alignment.CenterVertically) { CircularProgressIndicator() }
                Text(if (s == Step.Sending) "Mengirim..." else "Menyiapkan...")
            }
        }
        is Step.Camera -> FaceCamera(
            action = s.challenge.livenessAction,
            shots = if (enrolling) 3 else 1,
            onCaptured = { photos -> submit(s.challenge, photos) },
            onError = { message -> step = Step.Failed(message) },
            modifier = Modifier.fillMaxSize(),
        )
        is Step.Done -> Screen(title) {
            InfoCard(
                s.outcome.title,
                s.outcome.detail + if (s.outcome.needsReview) "\nAbsen ini akan ditinjau SDM." else "",
                if (s.outcome.needsReview) Amber else LightGreen,
            )
            Button(onClick = onClose, modifier = Modifier.fillMaxWidth()) { Text("Selesai") }
        }
        is Step.Failed -> Screen(title) {
            InfoCard("Belum berhasil", s.message, Red)
            Button(
                onClick = {
                    step = Step.Preparing
                    attempt++
                },
                modifier = Modifier.fillMaxWidth(),
            ) { Text("Coba lagi") }
            OutlinedButton(onClick = onClose, modifier = Modifier.fillMaxWidth()) { Text("Kembali") }
        }
    }
}

@Composable
private fun ConsentStep(onAccept: () -> Unit, onCancel: () -> Unit) {
    var biometric by remember { mutableStateOf(false) }
    var locationOk by remember { mutableStateOf(false) }
    Screen("Persetujuan data") {
        Text(
            "Untuk presensi, perusahaan memproses data wajah (biometrik) dan lokasi Anda sesuai UU Pelindungan " +
                "Data Pribadi. Foto wajah dipakai untuk mencocokkan identitas saat absen dan disimpan terenkripsi " +
                "paling lama 90 hari. Lokasi hanya diambil saat Anda menekan tombol absen.",
        )
        Row(verticalAlignment = Alignment.CenterVertically) {
            Checkbox(checked = biometric, onCheckedChange = { biometric = it })
            Text("Saya menyetujui pemrosesan data wajah.")
        }
        Row(verticalAlignment = Alignment.CenterVertically) {
            Checkbox(checked = locationOk, onCheckedChange = { locationOk = it })
            Text("Saya menyetujui pengambilan lokasi saat absen.")
        }
        Button(enabled = biometric && locationOk, onClick = onAccept, modifier = Modifier.fillMaxWidth()) {
            Text("Lanjut")
        }
        OutlinedButton(onClick = onCancel, modifier = Modifier.fillMaxWidth()) { Text("Batal") }
    }
}
