@file:OptIn(androidx.compose.foundation.layout.ExperimentalLayoutApi::class)

package id.pejuang.hris.ui

import android.Manifest
import android.content.Context
import android.content.Intent
import android.content.pm.PackageManager
import android.net.Uri
import android.provider.Settings
import androidx.activity.compose.BackHandler
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.foundation.Canvas
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.statusBarsPadding
import androidx.compose.foundation.layout.WindowInsets
import androidx.compose.foundation.layout.consumeWindowInsets
import androidx.compose.foundation.layout.statusBars
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.CheckCircle
import androidx.compose.material.icons.filled.Close
import androidx.compose.material.icons.filled.Info
import androidx.compose.material.icons.filled.Lock
import androidx.compose.material.icons.filled.LocationOn
import androidx.compose.material3.Checkbox
import androidx.compose.material3.CheckboxDefaults
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
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
import androidx.compose.ui.draw.clip
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.geometry.Rect
import androidx.compose.ui.geometry.Size
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.Path
import androidx.compose.ui.graphics.PathFillType
import androidx.compose.ui.graphics.drawscope.Stroke
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import androidx.core.content.ContextCompat
import androidx.lifecycle.Lifecycle
import androidx.lifecycle.compose.LifecycleEventEffect
import id.pejuang.hris.data.AttendanceChallenge
import id.pejuang.hris.data.ClockOutcome
import id.pejuang.hris.data.PURPOSE_CLOCK_IN
import id.pejuang.hris.data.PURPOSE_FACE_ENROLL
import id.pejuang.hris.data.Repository
import id.pejuang.hris.data.toApiError
import id.pejuang.hris.face.FaceCamera
import id.pejuang.hris.location.LocationReader
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.launch
import java.time.ZonedDateTime

private sealed interface Step {
    data object Consent : Step
    data object Preparing : Step
    data class Camera(val challenge: AttendanceChallenge) : Step
    data object Sending : Step
    data class Done(val outcome: ClockOutcome, val at: ZonedDateTime) : Step
    data class Failed(val message: String) : Step
}

private val permissions = arrayOf(Manifest.permission.CAMERA, Manifest.permission.ACCESS_FINE_LOCATION)

private fun hasPermissions(context: Context) =
    permissions.all { ContextCompat.checkSelfPermission(context, it) == PackageManager.PERMISSION_GRANTED }

fun captureTitle(purpose: String) = when (purpose) {
    PURPOSE_FACE_ENROLL -> "Daftarkan wajah"
    PURPOSE_CLOCK_IN -> "Presensi masuk"
    else -> "Presensi pulang"
}

/** Alur daftar wajah (F7) dan presensi masuk/pulang (F8): challenge, liveness, foto, lokasi, kirim. */
@Composable
fun CaptureScreen(purpose: String, repository: Repository, location: LocationReader, onClose: () -> Unit) {
    val context = LocalContext.current
    val scope = rememberCoroutineScope()
    val enrolling = purpose == PURPOSE_FACE_ENROLL
    val title = captureTitle(purpose)
    var step by remember { mutableStateOf<Step>(if (enrolling) Step.Consent else Step.Preparing) }
    var attempt by remember { mutableIntStateOf(0) }
    var granted by remember { mutableStateOf(hasPermissions(context)) }
    var denied by remember { mutableStateOf(false) }
    var asked by remember { mutableStateOf(false) }
    val launcher = rememberLauncherForActivityResult(ActivityResultContracts.RequestMultiplePermissions()) { result ->
        granted = hasPermissions(context)
        denied = !result.values.all { it }
    }

    // Izin bisa diberikan dari pengaturan HP lalu kembali ke aplikasi.
    LifecycleEventEffect(Lifecycle.Event.ON_RESUME) { granted = hasPermissions(context) }

    LaunchedEffect(granted, step, attempt) {
        if (step != Step.Preparing) return@LaunchedEffect
        if (!granted) {
            if (!asked) {
                asked = true
                launcher.launch(permissions)
            }
            return@LaunchedEffect
        }
        step = try {
            Step.Camera(repository.challenge(purpose))
        } catch (e: CancellationException) {
            throw e
        } catch (e: Exception) {
            Step.Failed(e.toApiError().message ?: "Gagal memulai.")
        }
    }

    fun submit(challenge: AttendanceChallenge, photos: List<ByteArray>) {
        step = Step.Sending
        scope.launch {
            step = try {
                val fix = location.current()
                val outcome = if (enrolling) {
                    repository.enroll(challenge, photos, fix)
                    ClockOutcome("Wajah terdaftar", "Mulai sekarang Anda bisa presensi dengan selfie.", false)
                } else {
                    repository.clock(purpose, challenge, photos.first(), fix)
                }
                Step.Done(outcome, ZonedDateTime.now(Wib))
            } catch (e: CancellationException) {
                throw e
            } catch (e: Exception) {
                Step.Failed(e.toApiError().message ?: "Gagal mengirim.")
            }
        }
    }

    // Saat mengirim, tombol kembali diabaikan agar presensi tidak terputus di tengah jalan.
    BackHandler(enabled = step == Step.Sending) {}

    when (val s = step) {
        Step.Consent -> ConsentContent(
            onAccept = {
                step = Step.Preparing
                // Persetujuan lokasi dicatat terpisah; biometrik dicatat server saat daftar wajah.
                scope.launch { runCatching { repository.consentLocation() } }
            },
            onCancel = onClose,
        )
        Step.Preparing -> if (!granted) {
            PermissionContent(
                denied = denied,
                onRequest = { launcher.launch(permissions) },
                onOpenSettings = {
                    context.startActivity(
                        Intent(Settings.ACTION_APPLICATION_DETAILS_SETTINGS, Uri.fromParts("package", context.packageName, null))
                            .addFlags(Intent.FLAG_ACTIVITY_NEW_TASK),
                    )
                },
                onCancel = onClose,
            )
        } else {
            ProgressContent(title, "Menyiapkan kamera", "Meminta tantangan verifikasi dari server.", onClose)
        }
        is Step.Camera -> FaceCamera(
            title = title,
            action = s.challenge.livenessAction,
            shots = if (enrolling) 3 else 1,
            onCaptured = { photos -> submit(s.challenge, photos) },
            onError = { message -> step = Step.Failed(message) },
            onClose = onClose,
            modifier = Modifier.fillMaxSize(),
        )
        Step.Sending -> ProgressContent(
            title,
            if (enrolling) "Mendaftarkan wajah" else "Memverifikasi wajah",
            "Mengirim foto dan lokasi. Jangan tutup aplikasi.",
            onClose = null,
        )
        is Step.Done -> ResultContent(s.outcome, s.at, onClose)
        is Step.Failed -> FailedContent(
            title = title,
            message = s.message,
            onRetry = {
                step = Step.Preparing
                attempt++
            },
            onClose = onClose,
        )
    }
}

@Composable
fun ConsentContent(onAccept: () -> Unit, onCancel: () -> Unit) {
    var biometric by remember { mutableStateOf(false) }
    var locationOk by remember { mutableStateOf(false) }
    Column(Modifier.fillMaxSize().background(Hc.Canvas)) {
        TopBar("Persetujuan data", onBack = onCancel, navIcon = Icons.Filled.Close)
        Column(
            Modifier.weight(1f).verticalScroll(rememberScrollState()).padding(20.dp),
            verticalArrangement = Arrangement.spacedBy(16.dp),
        ) {
            IconBadge(HrisIcons.Shield, Tone.Success, size = 56.dp)
            Text("Sebelum mendaftarkan wajah", style = MaterialTheme.typography.headlineSmall, color = Hc.Ink)
            Text(
                "Untuk presensi, perusahaan memproses data wajah (biometrik) dan lokasi Anda sesuai UU Pelindungan Data Pribadi.",
                style = MaterialTheme.typography.bodyLarge,
                color = Hc.InkSoft,
            )
            SectionCard(padding = 4.dp) {
                Point(HrisIcons.FaceScan, "Dipakai untuk mencocokkan wajah", "Foto wajah hanya dipakai untuk memastikan yang presensi adalah Anda.")
                RowDivider()
                Point(Icons.Filled.Lock, "Disimpan terenkripsi", "Foto selfie dihapus otomatis paling lama 90 hari.")
                RowDivider()
                Point(Icons.Filled.LocationOn, "Lokasi hanya saat presensi", "Lokasi diambil saat Anda menekan tombol presensi, tidak dilacak terus-menerus.")
            }
            SectionCard(padding = 4.dp) {
                ConsentCheck(biometric, { biometric = it }, "Saya menyetujui pemrosesan data wajah saya.")
                HorizontalDivider(color = Hc.Line, modifier = Modifier.padding(horizontal = 16.dp))
                ConsentCheck(locationOk, { locationOk = it }, "Saya menyetujui pengambilan lokasi saat presensi.")
            }
        }
        Column(
            Modifier.background(Hc.Surface).navigationBarsPadding().padding(20.dp),
        ) {
            PrimaryButton("Setuju dan lanjutkan", onClick = onAccept, enabled = biometric && locationOk)
        }
    }
}

@Composable
private fun Point(icon: ImageVector, title: String, body: String) {
    Row(Modifier.padding(14.dp), horizontalArrangement = Arrangement.spacedBy(14.dp)) {
        IconBadge(icon, Tone.Neutral, size = 40.dp)
        Column(Modifier.weight(1f)) {
            Text(title, style = MaterialTheme.typography.titleSmall, color = Hc.Ink)
            Text(body, style = MaterialTheme.typography.bodyMedium, color = Hc.InkSoft)
        }
    }
}

@Composable
private fun ConsentCheck(checked: Boolean, onChange: (Boolean) -> Unit, text: String) {
    Row(
        Modifier
            .fillMaxWidth()
            .clip(MaterialTheme.shapes.medium)
            .clickable { onChange(!checked) }
            .padding(horizontal = 6.dp, vertical = 6.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Checkbox(
            checked = checked,
            onCheckedChange = onChange,
            colors = CheckboxDefaults.colors(checkedColor = Hc.Brand, uncheckedColor = Hc.InkMuted),
        )
        Text(text, style = MaterialTheme.typography.bodyMedium, color = Hc.Ink, modifier = Modifier.weight(1f))
    }
}

@Composable
fun PermissionContent(denied: Boolean, onRequest: () -> Unit, onOpenSettings: () -> Unit, onCancel: () -> Unit) {
    StatusBarIcons(dark = true)
    CenterMessage(
        title = "Izinkan kamera dan lokasi",
        body = "Kamera dipakai untuk selfie verifikasi wajah, lokasi untuk memastikan Anda berada di tempat kerja.",
        icon = HrisIcons.FaceScan,
        tone = Tone.Success,
        extra = {
            if (denied) {
                StatusBanner(
                    Tone.Warning,
                    Icons.Filled.Info,
                    "Izin belum diberikan",
                    "Bila tombol izin tidak memunculkan apa pun, buka pengaturan aplikasi lalu aktifkan Kamera dan Lokasi.",
                )
            }
        },
    ) {
        PrimaryButton("Berikan izin", onClick = onRequest)
        if (denied) SecondaryButton("Buka pengaturan aplikasi", onClick = onOpenSettings)
        TextButton(onClick = onCancel, modifier = Modifier.fillMaxWidth()) { Text("Batal", color = Hc.InkSoft) }
    }
}

@Composable
fun ProgressContent(title: String, heading: String, body: String, onClose: (() -> Unit)?) {
    Column(Modifier.fillMaxSize().background(Hc.Canvas)) {
        TopBar(title, onBack = onClose, navIcon = Icons.Filled.Close)
        CenterMessage(title = heading, body = body, loading = true, modifier = Modifier.weight(1f).consumeWindowInsets(WindowInsets.statusBars))
    }
}

@Composable
fun ResultContent(outcome: ClockOutcome, at: ZonedDateTime, onClose: () -> Unit) {
    StatusBarIcons(dark = true)
    CenterMessage(
        title = outcome.title,
        body = outcome.detail,
        icon = if (outcome.needsReview) Icons.Filled.Info else Icons.Filled.CheckCircle,
        tone = if (outcome.needsReview) Tone.Warning else Tone.Success,
        extra = {
            StatusChip("Pukul ${wibTime(at.toInstant().toString())} WIB", Tone.Neutral)
            if (outcome.needsReview) {
                Spacer(Modifier.height(16.dp))
                StatusBanner(
                    Tone.Warning,
                    Icons.Filled.Info,
                    "Perlu ditinjau SDM",
                    "Presensi tetap tercatat. SDM akan memeriksa lokasi atau verifikasi wajahnya.",
                )
            }
        },
    ) {
        PrimaryButton("Kembali ke beranda", onClick = onClose)
    }
}

@Composable
fun FailedContent(title: String, message: String, onRetry: () -> Unit, onClose: () -> Unit) {
    Column(Modifier.fillMaxSize().background(Hc.Canvas)) {
        TopBar(title, onBack = onClose, navIcon = Icons.Filled.Close)
        CenterMessage(
            title = "Belum berhasil",
            body = message,
            icon = Icons.Filled.Close,
            tone = Tone.Danger,
            modifier = Modifier.weight(1f).consumeWindowInsets(WindowInsets.statusBars),
        ) {
            PrimaryButton("Coba lagi", onClick = onRetry)
            SecondaryButton("Kembali", onClick = onClose)
        }
    }
}

enum class CameraPhase(val color: Color) {
    Searching(Color.White),
    Problem(Color(0xFFFBBF24)),
    Following(Color.White),
    Capturing(Hc.BrandBright),
}

/** Lapisan di atas pratinjau kamera: bingkai oval, judul, dan instruksi. */
@Composable
fun CameraOverlay(title: String, message: String, phase: CameraPhase, gesture: String, onClose: () -> Unit) {
    StatusBarIcons(dark = false)
    Box(Modifier.fillMaxSize()) {
        Canvas(Modifier.fillMaxSize()) {
            val oval = faceOval(size)
            val scrim = Path().apply {
                fillType = PathFillType.EvenOdd
                addRect(Rect(Offset.Zero, size))
                addOval(oval)
            }
            drawPath(scrim, Color(0xB3000000))
            drawOval(phase.color, oval.topLeft, oval.size, style = Stroke(width = 4.dp.toPx()))
        }

        Row(
            Modifier.fillMaxWidth().statusBarsPadding().padding(horizontal = 4.dp, vertical = 4.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            IconButton(onClick = onClose) { Icon(Icons.Filled.Close, contentDescription = "Tutup", tint = Color.White) }
            Text(title, style = MaterialTheme.typography.titleLarge, color = Color.White)
        }

        Column(
            Modifier
                .align(Alignment.BottomCenter)
                .fillMaxWidth()
                .navigationBarsPadding()
                .padding(horizontal = 24.dp, vertical = 28.dp),
            horizontalAlignment = Alignment.CenterHorizontally,
            verticalArrangement = Arrangement.spacedBy(14.dp),
        ) {
            Surface(shape = RoundedCornerShape(50), color = Color.White) {
                Row(
                    Modifier.padding(horizontal = 18.dp, vertical = 12.dp),
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    Box(
                        Modifier
                            .size(10.dp)
                            .clip(CircleShape)
                            .background(if (phase == CameraPhase.Searching || phase == CameraPhase.Following) Hc.Brand else phase.color),
                    )
                    Spacer(Modifier.width(10.dp))
                    Text(message, style = MaterialTheme.typography.titleMedium, color = Hc.Ink, textAlign = TextAlign.Center)
                }
            }
            Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                GuideChip("1", "Lihat lurus")
                GuideChip("2", gesture)
                GuideChip("3", "Lurus lagi")
            }
            Text(
                "Pastikan wajah cukup terang, tanpa masker atau kacamata hitam.",
                style = MaterialTheme.typography.bodySmall,
                color = Color.White.copy(alpha = 0.75f),
                textAlign = TextAlign.Center,
            )
        }
    }
}

@Composable
private fun GuideChip(number: String, text: String) {
    Row(
        Modifier.clip(RoundedCornerShape(50)).background(Color.White.copy(alpha = 0.16f)).padding(start = 4.dp, end = 10.dp, top = 4.dp, bottom = 4.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Box(Modifier.size(20.dp).clip(CircleShape).background(Color.White), contentAlignment = Alignment.Center) {
            Text(number, style = MaterialTheme.typography.labelSmall, color = Hc.Ink)
        }
        Spacer(Modifier.width(6.dp))
        Text(text, style = MaterialTheme.typography.labelMedium, color = Color.White, maxLines = 1)
    }
}

/** Oval wajah: 70% lebar layar, sedikit di atas tengah agar instruksi muat di bawahnya. */
private fun faceOval(size: Size): Rect {
    val w = size.width * 0.70f
    val h = w * 1.32f
    val cx = size.width / 2
    val cy = size.height * 0.40f
    return Rect(cx - w / 2, cy - h / 2, cx + w / 2, cy + h / 2)
}
