package id.pejuang.hris.ui

import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.layout.offset
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.statusBarsPadding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Info
import androidx.compose.material.icons.filled.Lock
import androidx.compose.material.icons.filled.Refresh
import androidx.compose.material.icons.filled.Warning
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
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
import androidx.compose.ui.draw.alpha
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.platform.LocalClipboardManager
import androidx.compose.ui.text.AnnotatedString
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import id.pejuang.hris.BuildConfig
import id.pejuang.hris.data.AttendanceRecord
import id.pejuang.hris.data.Me
import id.pejuang.hris.data.PURPOSE_CLOCK_IN
import id.pejuang.hris.data.PURPOSE_CLOCK_OUT
import id.pejuang.hris.data.PURPOSE_FACE_ENROLL
import id.pejuang.hris.data.Repository
import id.pejuang.hris.data.toApiError
import kotlinx.coroutines.launch
import java.time.LocalDate
import java.time.ZonedDateTime

@Composable
fun HomeScreen(
    repository: Repository,
    crash: String?,
    onDismissCrash: () -> Unit,
    onCapture: (String) -> Unit,
    onHistory: () -> Unit,
    onLoggedOut: () -> Unit,
) {
    val scope = rememberCoroutineScope()
    val clipboard = LocalClipboardManager.current
    var me by remember { mutableStateOf<Me?>(null) }
    var today by remember { mutableStateOf<AttendanceRecord?>(null) }
    var error by remember { mutableStateOf<String?>(null) }
    var loading by remember { mutableStateOf(true) }
    var reload by remember { mutableIntStateOf(0) }

    LaunchedEffect(reload) {
        loading = true
        error = null
        try {
            me = repository.me()
            // Riwayat hanya untuk kartu "hari ini"; kegagalannya tidak menghalangi absen.
            val date = LocalDate.now(Wib).toString()
            today = runCatching { repository.history().firstOrNull { it.workDate == date } }.getOrNull()
        } catch (e: Exception) {
            val err = e.toApiError()
            if (err.code == "AUTH_TOKEN_INVALID" || !repository.loggedIn) {
                repository.logout()
                onLoggedOut()
            } else {
                error = err.message
            }
        } finally {
            loading = false
        }
    }

    val current = me
    when {
        current != null -> HomeContent(
            me = current,
            today = today,
            now = ZonedDateTime.now(Wib),
            refreshing = loading,
            crash = crash,
            onCopyCrash = { crash?.let { clipboard.setText(AnnotatedString(it)) } },
            onDismissCrash = onDismissCrash,
            onCapture = onCapture,
            onHistory = onHistory,
            onRefresh = { if (!loading) reload++ },
            onLogout = {
                scope.launch {
                    repository.logout()
                    onLoggedOut()
                }
            },
        )
        error != null -> {
            StatusBarIcons(dark = true)
            CenterMessage(
                title = "Data belum bisa dimuat",
                body = error,
                icon = Icons.Filled.Warning,
                tone = Tone.Warning,
            ) {
                PrimaryButton("Coba lagi", onClick = { reload++ })
                SecondaryButton("Keluar", onClick = {
                    scope.launch {
                        repository.logout()
                        onLoggedOut()
                    }
                })
            }
        }
        else -> {
            StatusBarIcons(dark = true)
            CenterMessage(title = "Memuat data Anda", body = null, loading = true)
        }
    }
}

@Composable
fun HomeContent(
    me: Me,
    today: AttendanceRecord?,
    now: ZonedDateTime,
    refreshing: Boolean,
    crash: String?,
    onCopyCrash: () -> Unit,
    onDismissCrash: () -> Unit,
    onCapture: (String) -> Unit,
    onHistory: () -> Unit,
    onRefresh: () -> Unit,
    onLogout: () -> Unit,
) {
    StatusBarIcons(dark = false)
    val deviceApproved = me.deviceStatus == "approved"
    val ready = deviceApproved && !me.mustChangePassword
    val canClock = ready && me.faceEnrolled
    val clockedIn = today?.clockInAt != null
    val clockedOut = today?.clockOutAt != null
    val (statusText, statusTone) = statusLabel(today?.status)

    Column(
        Modifier
            .fillMaxSize()
            .background(Hc.Canvas)
            .verticalScroll(rememberScrollState()),
    ) {
        // Header
        Column(
            Modifier
                .fillMaxWidth()
                .clip(RoundedCornerShape(bottomStart = 28.dp, bottomEnd = 28.dp))
                .background(BrandGradient)
                .statusBarsPadding()
                .padding(start = 20.dp, end = 8.dp, top = 12.dp, bottom = 92.dp),
        ) {
            Row(verticalAlignment = Alignment.CenterVertically) {
                Box(
                    Modifier.size(46.dp).clip(CircleShape).background(Color.White.copy(alpha = 0.18f)),
                    contentAlignment = Alignment.Center,
                ) {
                    Text(initials(me.fullName), style = MaterialTheme.typography.titleMedium, color = Color.White)
                }
                Spacer(Modifier.width(12.dp))
                Column(Modifier.weight(1f)) {
                    Text(greeting(now), style = MaterialTheme.typography.bodyMedium, color = Color.White.copy(alpha = 0.8f))
                    Text(
                        me.fullName,
                        style = MaterialTheme.typography.titleLarge,
                        color = Color.White,
                        maxLines = 1,
                        overflow = TextOverflow.Ellipsis,
                    )
                }
                IconButton(onClick = onRefresh) {
                    if (refreshing) {
                        CircularProgressIndicator(color = Color.White, strokeWidth = 2.dp, modifier = Modifier.size(20.dp))
                    } else {
                        Icon(Icons.Filled.Refresh, contentDescription = "Muat ulang", tint = Color.White)
                    }
                }
                IconButton(onClick = onLogout) {
                    Icon(HrisIcons.ClockOut, contentDescription = "Keluar", tint = Color.White)
                }
            }
            Spacer(Modifier.height(18.dp))
            Text(longDateLabel(now.toLocalDate()), style = MaterialTheme.typography.titleSmall, color = Color.White)
            Text(
                listOfNotNull(me.position, me.subdivision.name).joinToString(" · "),
                style = MaterialTheme.typography.bodySmall,
                color = Color.White.copy(alpha = 0.75f),
                maxLines = 1,
                overflow = TextOverflow.Ellipsis,
            )
        }

        Column(
            Modifier.padding(horizontal = 20.dp).offset(y = (-72).dp),
            verticalArrangement = Arrangement.spacedBy(12.dp),
        ) {
            // Presensi hari ini
            SectionCard(padding = 18.dp) {
                Row(verticalAlignment = Alignment.CenterVertically) {
                    Text("Presensi hari ini", style = MaterialTheme.typography.titleMedium, color = Hc.Ink, modifier = Modifier.weight(1f))
                    StatusChip(statusText, statusTone)
                }
                Spacer(Modifier.height(16.dp))
                Row(Modifier.fillMaxWidth(), verticalAlignment = Alignment.CenterVertically) {
                    TimeCell("Masuk", wibTime(today?.clockInAt), HrisIcons.ClockIn, clockedIn, Modifier.weight(1f))
                    Box(Modifier.width(1.dp).height(44.dp).background(Hc.Line))
                    TimeCell("Pulang", wibTime(today?.clockOutAt), HrisIcons.ClockOut, clockedOut, Modifier.weight(1f).padding(start = 16.dp))
                }
                val note = today?.let { t ->
                    val worked = t.workedMinutes
                    when {
                        t.lateMinutes > 0 -> "Terlambat ${t.lateMinutes} menit"
                        worked != null && clockedOut -> "Jam kerja ${durationLabel(worked)}"
                        else -> t.locationName ?: workModeLabel(t.workMode)
                    }
                }
                if (note != null) {
                    Spacer(Modifier.height(12.dp))
                    Text(note, style = MaterialTheme.typography.bodySmall, color = Hc.InkSoft)
                }
            }

            if (crash != null) CrashNotice(crash, onCopyCrash, onDismissCrash)

            when (me.deviceStatus) {
                "approved" -> {}
                "pending" -> StatusBanner(
                    Tone.Warning,
                    HrisIcons.Smartphone,
                    "HP menunggu persetujuan",
                    "SDM perlu menyetujui HP ini sebelum Anda bisa absen. Muat ulang setelah disetujui.",
                )
                else -> StatusBanner(Tone.Danger, HrisIcons.Smartphone, "HP tidak aktif", "HP ini sudah dicabut. Hubungi SDM.")
            }
            if (me.mustChangePassword) {
                StatusBanner(
                    Tone.Warning,
                    Icons.Filled.Lock,
                    "Ganti kata sandi dulu",
                    "Ganti kata sandi lewat web admin, lalu muat ulang aplikasi.",
                )
            }
            if (!me.faceEnrolled) {
                StatusBanner(
                    Tone.Info,
                    HrisIcons.FaceScan,
                    "Daftarkan wajah Anda",
                    "Sekali saja, di lokasi kantor, sebelum absen pertama.",
                ) {
                    BannerAction("Daftarkan sekarang", enabled = ready) { onCapture(PURPOSE_FACE_ENROLL) }
                }
            }

            SectionLabel("Absen")
            Row(horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                // Tombol yang relevan saat ini dibuat menonjol.
                val inFirst = !clockedIn
                ActionTile(
                    title = "Absen masuk",
                    subtitle = if (clockedIn) "Tercatat ${wibTime(today?.clockInAt)}" else "Mulai hari kerja",
                    icon = HrisIcons.ClockIn,
                    emphasized = inFirst,
                    enabled = canClock,
                    onClick = { onCapture(PURPOSE_CLOCK_IN) },
                    modifier = Modifier.weight(1f),
                )
                ActionTile(
                    title = "Absen pulang",
                    subtitle = if (clockedOut) "Tercatat ${wibTime(today?.clockOutAt)}" else "Akhiri hari kerja",
                    icon = HrisIcons.ClockOut,
                    emphasized = !inFirst && !clockedOut,
                    enabled = canClock,
                    onClick = { onCapture(PURPOSE_CLOCK_OUT) },
                    modifier = Modifier.weight(1f),
                )
            }
            if (!canClock) {
                Row(verticalAlignment = Alignment.CenterVertically, modifier = Modifier.padding(horizontal = 4.dp)) {
                    Icon(Icons.Filled.Info, contentDescription = null, tint = Hc.InkMuted, modifier = Modifier.size(16.dp))
                    Spacer(Modifier.width(6.dp))
                    Text(
                        "Absen aktif setelah HP disetujui dan wajah terdaftar.",
                        style = MaterialTheme.typography.bodySmall,
                        color = Hc.InkSoft,
                    )
                }
            }

            SectionLabel("Lainnya")
            SectionCard(padding = 0.dp) {
                ListRow(HrisIcons.History, "Riwayat absen", "31 hari terakhir", onClick = onHistory)
                RowDivider()
                ListRow(HrisIcons.FaceScan, "Data wajah", "Untuk verifikasi saat absen") {
                    if (me.faceEnrolled) StatusChip("Terdaftar", Tone.Success) else StatusChip("Belum", Tone.Warning)
                }
                RowDivider()
                ListRow(HrisIcons.Smartphone, "Perangkat ini", "NIPY ${me.nip}") {
                    when (me.deviceStatus) {
                        "approved" -> StatusChip("Disetujui", Tone.Success)
                        "pending" -> StatusChip("Menunggu", Tone.Warning)
                        else -> StatusChip("Dicabut", Tone.Danger)
                    }
                }
            }

            Text(
                "HRIS Pejuang · versi ${BuildConfig.VERSION_NAME}",
                style = MaterialTheme.typography.labelSmall,
                color = Hc.InkMuted,
                modifier = Modifier.fillMaxWidth().navigationBarsPadding().padding(top = 8.dp),
                textAlign = androidx.compose.ui.text.style.TextAlign.Center,
            )
        }
    }
}

@Composable
private fun TimeCell(label: String, time: String, icon: ImageVector, done: Boolean, modifier: Modifier = Modifier) {
    Row(modifier, verticalAlignment = Alignment.CenterVertically) {
        IconBadge(icon, if (done) Tone.Success else Tone.Neutral, size = 40.dp)
        Spacer(Modifier.width(12.dp))
        Column {
            Text(label, style = MaterialTheme.typography.bodySmall, color = Hc.InkSoft)
            Text(time, style = MaterialTheme.typography.titleLarge, color = if (done) Hc.Ink else Hc.InkMuted)
        }
    }
}

@Composable
private fun BannerAction(text: String, enabled: Boolean, onClick: () -> Unit) {
    Surface(
        onClick = onClick,
        enabled = enabled,
        shape = MaterialTheme.shapes.small,
        color = if (enabled) Tone.Info.content else Hc.Line,
        modifier = Modifier.alpha(if (enabled) 1f else 0.7f),
    ) {
        Text(
            text,
            style = MaterialTheme.typography.labelLarge,
            color = if (enabled) Color.White else Hc.InkMuted,
            modifier = Modifier.padding(horizontal = 14.dp, vertical = 8.dp),
        )
    }
}

/** Kartu aksi besar untuk absen masuk/pulang. */
@Composable
private fun ActionTile(
    title: String,
    subtitle: String,
    icon: ImageVector,
    emphasized: Boolean,
    enabled: Boolean,
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
) {
    val container = if (emphasized) Hc.Brand else Hc.Surface
    val content = if (emphasized) Color.White else Hc.Ink
    Surface(
        onClick = onClick,
        enabled = enabled,
        shape = MaterialTheme.shapes.large,
        color = container,
        border = if (emphasized) null else BorderStroke(1.dp, Hc.Line),
        shadowElevation = if (emphasized) 4.dp else 1.dp,
        modifier = modifier.height(132.dp).alpha(if (enabled) 1f else 0.5f),
    ) {
        Column(Modifier.fillMaxSize().padding(16.dp), verticalArrangement = Arrangement.SpaceBetween) {
            Box(
                Modifier
                    .size(42.dp)
                    .clip(CircleShape)
                    .background(if (emphasized) Color.White.copy(alpha = 0.18f) else Hc.BrandSoft),
                contentAlignment = Alignment.Center,
            ) {
                Icon(icon, contentDescription = null, tint = if (emphasized) Color.White else Hc.Brand, modifier = Modifier.size(22.dp))
            }
            Column {
                Text(title, style = MaterialTheme.typography.titleMedium, color = content)
                Text(
                    subtitle,
                    style = MaterialTheme.typography.bodySmall,
                    color = content.copy(alpha = 0.75f),
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis,
                )
            }
        }
    }
}
