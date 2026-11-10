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
import androidx.compose.foundation.layout.heightIn
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
import androidx.compose.material.icons.filled.Home
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
    pendingApprovals: Int = 0,
    onRequests: () -> Unit = {},
) {
    StatusBarIcons(dark = true)
    val ready = me.deviceStatus == "approved" && !me.mustChangePassword
    val canClock = ready && me.faceEnrolled
    val clockedIn = today?.clockInAt != null
    val clockedOut = today?.clockOutAt != null
    val (statusText, statusTone) = statusLabel(today?.status)

    Column(Modifier.fillMaxSize().background(Hc.Canvas)) {
        // Profil dan tombol tindakan tetap berada di bagian atas saat konten digulir.
        Row(
            Modifier.fillMaxWidth().background(Hc.Surface).statusBarsPadding().padding(start = 12.dp, end = 4.dp, top = 12.dp, bottom = 16.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Box(Modifier.size(44.dp).clip(CircleShape).background(Hc.BrandSoft), contentAlignment = Alignment.Center) {
                Text(initials(me.fullName), style = MaterialTheme.typography.titleMedium, color = Hc.BrandDeep)
            }
            Spacer(Modifier.width(9.dp))
            Column(Modifier.weight(1f)) {
                Text(me.fullName, style = MaterialTheme.typography.titleMedium, color = Hc.Ink, maxLines = 1, overflow = TextOverflow.Ellipsis)
                Text(
                    listOfNotNull(me.position, me.subdivision.name).joinToString(" · ").ifBlank { "Pejuang Yayasan" },
                    style = MaterialTheme.typography.labelMedium, color = Hc.BrandDark, maxLines = 1, overflow = TextOverflow.Ellipsis,
                )
            }
            IconButton(onClick = onRefresh) {
                if (refreshing) CircularProgressIndicator(color = Hc.Brand, strokeWidth = 2.dp, modifier = Modifier.size(20.dp))
                else Icon(Icons.Filled.Refresh, contentDescription = "Muat ulang", tint = Hc.Brand)
            }
            IconButton(onClick = onLogout) { Icon(HrisIcons.ClockOut, contentDescription = "Keluar", tint = Hc.Brand) }
        }
        Column(
            Modifier.weight(1f).fillMaxWidth().verticalScroll(rememberScrollState())
                .padding(start = 12.dp, end = 12.dp, top = 16.dp, bottom = bottomContentSpace(16.dp)),
            verticalArrangement = Arrangement.spacedBy(16.dp),
        ) {
            Box(Modifier.fillMaxWidth().heightIn(min = 96.dp).clip(RoundedCornerShape(12.dp)).background(Hc.Brand)) {
                Column(Modifier.align(Alignment.CenterStart).padding(start = 16.dp, end = 110.dp, top = 16.dp, bottom = 16.dp), verticalArrangement = Arrangement.spacedBy(2.dp)) {
                    Text("Ringkasan kerja", style = MaterialTheme.typography.titleMedium, color = Color.White, maxLines = 1)
                    Text("Presensi dan kegiatan Anda", style = MaterialTheme.typography.bodySmall, color = Color.White.copy(alpha = 0.9f), maxLines = 1)
                }
                Box(Modifier.align(Alignment.TopEnd)) { FigmaBannerStars() }
                androidx.compose.foundation.Image(
                    painter = androidx.compose.ui.res.painterResource(id.pejuang.hris.R.drawable.figma_summary),
                    contentDescription = null,
                    contentScale = androidx.compose.ui.layout.ContentScale.Crop,
                    modifier = Modifier.align(Alignment.TopEnd).offset(x = 20.dp, y = 13.dp).width(101.dp).height(85.dp),
                )
            }

            if (crash != null) CrashNotice(crash, onCopyCrash, onDismissCrash)
            when (me.deviceStatus) {
                "approved" -> {}
                "pending" -> StatusBanner(Tone.Warning, HrisIcons.Smartphone, "HP menunggu persetujuan", "SDM perlu menyetujui HP ini sebelum Anda bisa presensi. Muat ulang setelah disetujui.")
                else -> StatusBanner(Tone.Danger, HrisIcons.Smartphone, "HP tidak aktif", "HP ini sudah dicabut. Hubungi SDM.")
            }
            if (me.mustChangePassword) StatusBanner(Tone.Warning, Icons.Filled.Lock, "Ganti kata sandi dulu", "Buka menu Profil, lalu pilih Ganti kata sandi.")
            if (!me.faceEnrolled) {
                StatusBanner(Tone.Info, HrisIcons.FaceScan, "Daftarkan wajah Anda", "Sekali saja, di lokasi kantor, sebelum presensi pertama.") {
                    BannerAction("Daftarkan sekarang", enabled = ready) { onCapture(PURPOSE_FACE_ENROLL) }
                }
            }

            HomeSection("Presensi hari ini", longDateLabel(now.toLocalDate())) {
                Row(Modifier.fillMaxWidth(), verticalAlignment = Alignment.CenterVertically) {
                    Text("Status kehadiran", style = MaterialTheme.typography.titleSmall, color = Hc.Ink, modifier = Modifier.weight(1f))
                    StatusChip(statusText, statusTone)
                }
                Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                    HomeTime("Masuk", wibTime(today?.clockInAt), Modifier.weight(1f))
                    HomeTime("Pulang", wibTime(today?.clockOutAt), Modifier.weight(1f))
                }
                Text(today?.locationName ?: today?.let { workModeLabel(it.workMode) } ?: "Presensi belum tercatat", style = MaterialTheme.typography.bodySmall, color = Hc.InkSoft)
            }

            HomeSection("Presensi", "Catat kehadiran sesuai jadwal kerja") {
                Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                    HomeAction("Presensi masuk", if (clockedIn) "Tercatat ${wibTime(today?.clockInAt)}" else "Mulai hari kerja", HrisIcons.ClockIn, canClock, Modifier.weight(1f)) { onCapture(PURPOSE_CLOCK_IN) }
                    HomeAction("Presensi pulang", if (clockedOut) "Tercatat ${wibTime(today?.clockOutAt)}" else "Akhiri hari kerja", HrisIcons.ClockOut, canClock, Modifier.weight(1f)) { onCapture(PURPOSE_CLOCK_OUT) }
                }
                if (!canClock) Text("Presensi aktif setelah HP disetujui dan wajah terdaftar.", style = MaterialTheme.typography.bodySmall, color = Hc.InkSoft)
            }

            HomeSection("Layanan Pejuang", "Informasi dan riwayat kehadiran") {
                ListRow(HrisIcons.History, "Riwayat presensi", "31 hari terakhir", onClick = onHistory)
                RowDivider()
                ListRow(FigmaIcons.NavNote, "Cuti dan izin", "Ajukan dan pantau persetujuan", onClick = onRequests) {
                    if (pendingApprovals > 0) StatusChip("$pendingApprovals menunggu Anda", Tone.Warning)
                }
                RowDivider()
                ListRow(HrisIcons.FaceScan, "Data wajah", "Untuk verifikasi saat presensi") {
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
            Text("HRIS Pejuang · versi ${BuildConfig.VERSION_NAME}", style = MaterialTheme.typography.labelSmall, color = Hc.InkMuted, modifier = Modifier.fillMaxWidth(), textAlign = androidx.compose.ui.text.style.TextAlign.Center)
        }
    }
}

@Composable
private fun HomeSection(title: String, subtitle: String, content: @Composable androidx.compose.foundation.layout.ColumnScope.() -> Unit) {
    Surface(modifier = Modifier.fillMaxWidth(), shape = RoundedCornerShape(8.dp), color = Hc.Surface) {
        Column(Modifier.padding(horizontal = 16.dp, vertical = 12.dp), verticalArrangement = Arrangement.spacedBy(12.dp)) {
            Column(verticalArrangement = Arrangement.spacedBy(2.dp)) {
                Text(title, style = MaterialTheme.typography.titleSmall, color = Hc.Ink)
                Text(subtitle, style = MaterialTheme.typography.bodySmall, color = Hc.InkSoft)
            }
            content()
        }
    }
}

@Composable
private fun HomeTime(label: String, value: String, modifier: Modifier = Modifier) {
    Column(modifier.clip(RoundedCornerShape(12.dp)).background(Color(0xFFF9FAFB)).padding(12.dp)) {
        Text(label, style = MaterialTheme.typography.bodySmall, color = Hc.InkSoft)
        Text(value, style = MaterialTheme.typography.titleMedium, color = Hc.Ink)
    }
}

@Composable
private fun HomeAction(title: String, subtitle: String, icon: ImageVector, enabled: Boolean, modifier: Modifier = Modifier, onClick: () -> Unit) {
    Surface(onClick = onClick, enabled = enabled, modifier = modifier, shape = RoundedCornerShape(12.dp), color = Color(0xFFF9FAFB), border = BorderStroke(1.dp, Hc.Line)) {
        Column(Modifier.padding(12.dp), verticalArrangement = Arrangement.spacedBy(4.dp)) {
            IconBadge(icon, Tone.Success, size = 32.dp)
            Text(title, style = MaterialTheme.typography.titleSmall, color = if (enabled) Hc.Ink else Hc.InkMuted, maxLines = 1)
            Text(subtitle, style = MaterialTheme.typography.bodySmall, color = Hc.InkSoft, maxLines = 1, overflow = TextOverflow.Ellipsis)
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
