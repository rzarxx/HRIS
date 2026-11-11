package id.pejuang.hris.ui

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.IntrinsicSize
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.statusBarsPadding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Lock
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.alpha
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import id.pejuang.hris.BuildConfig
import id.pejuang.hris.data.AttendanceRecord
import id.pejuang.hris.data.Me
import id.pejuang.hris.data.MyAnnouncement
import id.pejuang.hris.data.PURPOSE_FACE_ENROLL
import java.time.ZonedDateTime

/**
 * Beranda (Clean Minimalist): sapaan, satu kartu presensi hari ini dengan satu tombol besar
 * (MASTER.md §10.2), pengumuman, lalu akses cepat. Muat ulang lewat tarik layar.
 */
@Suppress("UNUSED_PARAMETER")
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
    announcements: List<MyAnnouncement> = emptyList(),
    onAnnouncement: (MyAnnouncement) -> Unit = {},
    onAnnouncements: () -> Unit = {},
    onPayslip: () -> Unit = {},
) {
    StatusBarIcons(dark = true)
    val ready = me.deviceStatus == "approved" && !me.mustChangePassword
    val unread = announcements.count { it.readAt == null }

    Column(
        Modifier.fillMaxSize().background(Hc.Canvas).verticalScroll(rememberScrollState()).statusBarsPadding()
            .padding(start = 16.dp, end = 16.dp, top = 16.dp, bottom = bottomContentSpace(20.dp)),
        horizontalAlignment = Alignment.CenterHorizontally,
    ) {
      Column(ContentWidth.fillMaxWidth(), verticalArrangement = Arrangement.spacedBy(16.dp)) {
        Row(Modifier.fillMaxWidth().padding(horizontal = 4.dp, vertical = 4.dp), verticalAlignment = Alignment.CenterVertically) {
            Column(Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(2.dp)) {
                Text("${greeting(now)},", style = figmaText(14, color = Hc.InkSoft))
                Text(me.fullName, style = figmaText(22, FontWeight.Bold, tracking = -0.2f).copy(lineHeight = 28.sp), maxLines = 1, overflow = TextOverflow.Ellipsis)
                Text(
                    listOfNotNull(me.position, me.subdivision.name).joinToString(" · ").ifBlank { "Pejuang Yayasan" },
                    style = figmaText(13, color = Hc.InkSoft), maxLines = 1, overflow = TextOverflow.Ellipsis,
                )
            }
            Avatar(me.fullName, size = 48.dp)
        }

        if (crash != null) CrashNotice(crash, onCopyCrash, onDismissCrash)
        when (me.deviceStatus) {
            "approved" -> {}
            "pending" -> StatusBanner(Tone.Warning, HrisIcons.Smartphone, "HP menunggu persetujuan", "SDM perlu menyetujui HP ini sebelum Anda bisa presensi. Tarik layar ke bawah setelah disetujui.")
            else -> StatusBanner(Tone.Danger, HrisIcons.Smartphone, "HP tidak aktif", "HP ini sudah dicabut. Hubungi SDM.")
        }
        if (me.mustChangePassword) StatusBanner(Tone.Warning, Icons.Filled.Lock, "Ganti kata sandi dulu", "Buka menu Profil, lalu pilih Ganti kata sandi.")
        if (!me.faceEnrolled) {
            StatusBanner(Tone.Info, HrisIcons.FaceScan, "Daftarkan wajah Anda", "Sekali saja, di lokasi kantor, sebelum presensi pertama.") {
                BannerAction("Daftarkan sekarang", enabled = ready) { onCapture(PURPOSE_FACE_ENROLL) }
            }
        }

        TodayCard(me, today, now, onCapture)

        // Banner tampil sampai berakhir; yang belum dibaca lebih dulu (urutan dari server).
        announcements.filter { it.type == "banner" }.take(2).forEach { AnnouncementBanner(it, onClick = { onAnnouncement(it) }) }

        Text("Akses cepat", style = figmaText(16, FontWeight.SemiBold), modifier = Modifier.padding(start = 4.dp, top = 4.dp))
        Column(verticalArrangement = Arrangement.spacedBy(12.dp)) {
            Row(Modifier.fillMaxWidth().height(IntrinsicSize.Min), horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                QuickTile(HrisIcons.History, "Riwayat presensi", "31 hari terakhir", 0, Modifier.weight(1f).fillMaxHeight(), onHistory)
                QuickTile(FigmaIcons.NavNote, "Cuti dan izin", if (pendingApprovals > 0) "Menunggu keputusan Anda" else "Ajukan dan pantau", pendingApprovals, Modifier.weight(1f).fillMaxHeight(), onRequests)
            }
            Row(Modifier.fillMaxWidth().height(IntrinsicSize.Min), horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                QuickTile(FigmaIcons.MessageText, "Pengumuman", if (unread > 0) "Ada yang belum dibaca" else "Informasi dari SDM", unread, Modifier.weight(1f).fillMaxHeight(), onAnnouncements)
                QuickTile(FigmaIcons.NavReceipt, "Slip ubar", "Rincian per periode", 0, Modifier.weight(1f).fillMaxHeight(), onPayslip)
            }
        }

        SectionCard(padding = 4.dp) {
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
        Text(
            "HRIS Pejuang · versi ${BuildConfig.VERSION_NAME}",
            style = MaterialTheme.typography.labelSmall,
            color = Fg.Gray500,
            modifier = Modifier.fillMaxWidth(),
            textAlign = TextAlign.Center,
        )
      }
    }
}

/** Kartu utama Beranda: status hari ini, jam masuk dan pulang, dan satu tombol presensi. */
@Composable
private fun TodayCard(me: Me, today: AttendanceRecord?, now: ZonedDateTime, onCapture: (String) -> Unit) {
    val (statusText, statusTone) = statusLabel(today?.status)
    val action = clockAction(me, today)
    FigmaCard(gap = 16.dp) {
        Row(verticalAlignment = Alignment.CenterVertically) {
            Column(Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(2.dp)) {
                Text("Presensi hari ini", style = figmaText(16, FontWeight.SemiBold))
                Text(longDateLabel(now.toLocalDate()), style = figmaText(13, color = Hc.InkSoft))
            }
            StatusChip(statusText, statusTone)
        }
        Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.spacedBy(12.dp)) {
            TimeTile("Masuk", wibTime(today?.clockInAt), Modifier.weight(1f))
            TimeTile("Pulang", wibTime(today?.clockOutAt), Modifier.weight(1f))
        }
        Text(
            today?.locationName ?: today?.let { workModeLabel(it.workMode) } ?: "Belum ada presensi hari ini",
            style = figmaText(13, color = Hc.InkSoft),
        )
        FigmaPrimaryButton(action.label, onClick = { action.purpose?.let(onCapture) }, enabled = action.purpose != null)
        if (action.note != null) {
            Text(action.note, style = figmaText(12, color = Hc.InkSoft, lineHeight = 1.4f), textAlign = TextAlign.Center, modifier = Modifier.fillMaxWidth())
        }
    }
}

@Composable
private fun TimeTile(label: String, value: String, modifier: Modifier = Modifier) {
    Column(
        modifier.clip(RoundedCornerShape(16.dp)).background(Hc.Field).padding(horizontal = 16.dp, vertical = 14.dp)
            .semantics(mergeDescendants = true) {},
        verticalArrangement = Arrangement.spacedBy(2.dp),
    ) {
        Text(label, style = figmaText(12, FontWeight.Medium, Hc.InkSoft))
        Text(value, style = figmaText(24, FontWeight.Bold, if (value == "--:--") Hc.InkMuted else Hc.Ink, tracking = -0.3f).copy(lineHeight = 30.sp))
    }
}

/** Ubin akses cepat: ikon mint, judul, keterangan, dan jumlah bila ada yang menunggu. */
@Composable
private fun QuickTile(icon: ImageVector, title: String, subtitle: String, count: Int, modifier: Modifier, onClick: () -> Unit) {
    Column(
        modifier
            .heightIn(min = 112.dp)
            .softShadow()
            .clip(CardShape)
            .background(Hc.Surface)
            .clickable(role = Role.Button, onClick = onClick)
            .semantics { contentDescription = if (count > 0) "$title, $count menunggu" else title }
            .padding(16.dp),
        verticalArrangement = Arrangement.spacedBy(10.dp),
    ) {
        Row(verticalAlignment = Alignment.Top) {
            SoftIconTile(icon, 40.dp)
            Box(Modifier.weight(1f), contentAlignment = Alignment.TopEnd) { if (count > 0) CountPill(count) }
        }
        Column(verticalArrangement = Arrangement.spacedBy(2.dp)) {
            Text(title, style = figmaText(14, FontWeight.SemiBold), maxLines = 1, overflow = TextOverflow.Ellipsis)
            Text(subtitle, style = figmaText(12, color = Hc.InkSoft, lineHeight = 1.35f), maxLines = 2, overflow = TextOverflow.Ellipsis)
        }
    }
}

@Composable
private fun BannerAction(text: String, enabled: Boolean, onClick: () -> Unit) {
    Surface(
        onClick = onClick,
        enabled = enabled,
        shape = ControlShape,
        color = if (enabled) Tone.Info.content else Hc.Line,
        modifier = Modifier.alpha(if (enabled) 1f else 0.7f),
    ) {
        Text(
            text,
            style = MaterialTheme.typography.labelLarge,
            color = if (enabled) Color.White else Hc.InkMuted,
            modifier = Modifier.padding(horizontal = 16.dp, vertical = 10.dp),
        )
    }
}
