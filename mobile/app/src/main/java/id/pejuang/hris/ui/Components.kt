package id.pejuang.hris.ui

import android.app.Activity
import android.os.Build
import android.view.HapticFeedbackConstants
import androidx.compose.animation.core.RepeatMode
import androidx.compose.animation.core.animateFloat
import androidx.compose.animation.core.infiniteRepeatable
import androidx.compose.animation.core.rememberInfiniteTransition
import androidx.compose.animation.core.tween
import androidx.compose.foundation.Image
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.BoxScope
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ColumnScope
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.RowScope
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.statusBarsPadding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowBack
import androidx.compose.material.icons.automirrored.filled.KeyboardArrowRight
import androidx.compose.material.icons.filled.Warning
import androidx.compose.material3.Button
import androidx.compose.material3.ButtonDefaults
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.material3.pulltorefresh.PullToRefreshBox
import androidx.compose.material3.pulltorefresh.PullToRefreshDefaults
import androidx.compose.material3.pulltorefresh.rememberPullToRefreshState
import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.graphicsLayer
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.platform.LocalView
import androidx.compose.ui.res.painterResource
import id.pejuang.hris.R
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import androidx.core.view.WindowCompat

/** Mengatur warna ikon status bar: terang di atas header hijau, gelap di atas latar putih. */
@Composable
fun StatusBarIcons(dark: Boolean) {
    val view = LocalView.current
    DisposableEffect(dark) {
        val window = (view.context as? Activity)?.window
        if (window != null) WindowCompat.getInsetsController(window, view).isAppearanceLightStatusBars = dark
        onDispose {}
    }
}

/** Logo PUTI Al Bahjah, dipakai di layar pembuka dan layar masuk. */
@Composable
fun AppLogo(size: Dp = 64.dp) {
    Image(painterResource(R.drawable.logo_albahjah), contentDescription = "Logo PUTI Al Bahjah", modifier = Modifier.size(size))
}

/** Bilah judul putih untuk layar turunan. */
@Composable
fun TopBar(title: String, onBack: (() -> Unit)?, navIcon: ImageVector = Icons.AutoMirrored.Filled.ArrowBack) {
    StatusBarIcons(dark = true)
    Column(Modifier.background(Hc.Canvas).statusBarsPadding()) {
        Row(
            modifier = Modifier.fillMaxWidth().height(56.dp).padding(horizontal = 4.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            if (onBack != null) {
                IconButton(onClick = onBack) { Icon(navIcon, contentDescription = "Kembali", tint = Hc.Ink) }
            } else {
                Spacer(Modifier.width(12.dp))
            }
            Text(title, style = MaterialTheme.typography.titleLarge, color = Hc.Ink, maxLines = 1, overflow = TextOverflow.Ellipsis)
        }
    }
}

/** Kartu putih lembut tanpa garis, wadah utama konten. */
@Composable
fun SectionCard(
    modifier: Modifier = Modifier,
    padding: Dp = 16.dp,
    shape: androidx.compose.ui.graphics.Shape = CardShape,
    content: @Composable ColumnScope.() -> Unit,
) {
    Column(
        modifier.fillMaxWidth().softShadow(shape).clip(shape).background(Hc.Surface).padding(padding),
        content = content,
    )
}

@Composable
fun SectionLabel(text: String, modifier: Modifier = Modifier) {
    Text(
        text,
        style = MaterialTheme.typography.titleSmall,
        color = Hc.InkSoft,
        modifier = modifier.padding(start = 4.dp, top = 12.dp, bottom = 4.dp),
    )
}

/** Ikon dalam lingkaran berwarna sesuai [tone]. */
@Composable
fun IconBadge(icon: ImageVector, tone: Tone, size: Dp = 44.dp) {
    Box(
        modifier = Modifier.size(size).clip(CircleShape).background(tone.container),
        contentAlignment = Alignment.Center,
    ) {
        Icon(icon, contentDescription = null, tint = tone.content, modifier = Modifier.size(size * 0.5f))
    }
}

@Composable
fun StatusChip(text: String, tone: Tone, modifier: Modifier = Modifier) {
    Box(
        modifier = modifier.clip(CircleShape).background(tone.container).padding(horizontal = 10.dp, vertical = 5.dp),
    ) {
        Text(text, style = MaterialTheme.typography.labelMedium, color = tone.content, maxLines = 1)
    }
}

/** Pemberitahuan berwarna dengan ikon, judul, isi, dan aksi opsional. */
@Composable
fun StatusBanner(
    tone: Tone,
    icon: ImageVector,
    title: String,
    body: String,
    modifier: Modifier = Modifier,
    action: (@Composable RowScope.() -> Unit)? = null,
) {
    Surface(modifier = modifier.fillMaxWidth(), shape = CardShape, color = tone.container) {
        Row(Modifier.padding(16.dp), horizontalArrangement = Arrangement.spacedBy(12.dp)) {
            Box(Modifier.size(36.dp).clip(CircleShape).background(Color.White.copy(alpha = 0.7f)), contentAlignment = Alignment.Center) {
                Icon(icon, contentDescription = null, tint = tone.content, modifier = Modifier.size(20.dp))
            }
            Column(Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(2.dp)) {
                Text(title, style = MaterialTheme.typography.titleSmall, color = Hc.Ink)
                Text(body, style = MaterialTheme.typography.bodyMedium, color = Hc.InkSoft)
                if (action != null) {
                    Row(Modifier.padding(top = 6.dp), horizontalArrangement = Arrangement.spacedBy(8.dp), content = action)
                }
            }
        }
    }
}

@Composable
fun PrimaryButton(
    text: String,
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
    enabled: Boolean = true,
    loading: Boolean = false,
    icon: ImageVector? = null,
    shape: androidx.compose.ui.graphics.Shape = ControlShape,
) {
    Button(
        onClick = onClick,
        enabled = enabled && !loading,
        modifier = modifier.fillMaxWidth().height(52.dp),
        shape = shape,
        colors = ButtonDefaults.buttonColors(
            containerColor = Hc.Primary,
            contentColor = Hc.OnPrimary,
            disabledContainerColor = if (loading) Hc.Primary else Hc.Field,
            disabledContentColor = if (loading) Hc.OnPrimary else Hc.InkMuted,
        ),
        elevation = null,
    ) {
        if (loading) {
            CircularProgressIndicator(color = Hc.OnPrimary, strokeWidth = 2.dp, modifier = Modifier.size(20.dp))
        } else {
            if (icon != null) {
                Icon(icon, contentDescription = null, modifier = Modifier.size(20.dp))
                Spacer(Modifier.width(8.dp))
            }
            Text(text, style = MaterialTheme.typography.labelLarge)
        }
    }
}

@Composable
fun SecondaryButton(text: String, onClick: () -> Unit, modifier: Modifier = Modifier) {
    Button(
        onClick = onClick,
        modifier = modifier.fillMaxWidth().height(52.dp),
        shape = ControlShape,
        colors = ButtonDefaults.buttonColors(containerColor = Hc.Field, contentColor = Hc.Ink),
        elevation = null,
    ) {
        Text(text, style = MaterialTheme.typography.labelLarge)
    }
}

/** Baris daftar dengan ikon, judul, keterangan, dan isi kanan. */
@Composable
fun ListRow(
    icon: ImageVector,
    title: String,
    subtitle: String?,
    onClick: (() -> Unit)? = null,
    trailing: (@Composable () -> Unit)? = null,
) {
    Row(
        modifier = Modifier
            .fillMaxWidth()
            .then(if (onClick != null) Modifier.clickable(onClick = onClick) else Modifier)
            .heightIn(min = 56.dp)
            .padding(horizontal = 16.dp, vertical = 12.dp),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(14.dp),
    ) {
        SoftIconTile(icon, size = 40.dp)
        Column(Modifier.weight(1f)) {
            Text(title, style = MaterialTheme.typography.titleSmall, color = Hc.Ink)
            if (subtitle != null) Text(subtitle, style = MaterialTheme.typography.bodySmall, color = Hc.InkSoft)
        }
        trailing?.invoke()
        if (onClick != null) {
            Icon(Icons.AutoMirrored.Filled.KeyboardArrowRight, contentDescription = null, tint = Fg.Chevron)
        }
    }
}

/**
 * Layar pesan di tengah: hasil, kosong, galat, atau memuat. Tombol menempel di bawah agar mudah
 * dijangkau ibu jari.
 */
@Composable
fun CenterMessage(
    title: String,
    body: String?,
    modifier: Modifier = Modifier,
    icon: ImageVector? = null,
    tone: Tone = Tone.Neutral,
    loading: Boolean = false,
    extra: (@Composable ColumnScope.() -> Unit)? = null,
    actions: (@Composable ColumnScope.() -> Unit)? = null,
) {
    // Latar dipasang sebelum inset agar area status bar tidak memperlihatkan latar window.
    Column(modifier.fillMaxSize().background(Hc.Canvas).statusBarsPadding().navigationBarsPadding().padding(24.dp)) {
        Column(
            modifier = Modifier.weight(1f).fillMaxWidth(),
            horizontalAlignment = Alignment.CenterHorizontally,
            verticalArrangement = Arrangement.Center,
        ) {
            when {
                loading -> CircularProgressIndicator(color = Hc.Brand, strokeWidth = 3.dp, modifier = Modifier.size(48.dp))
                icon != null -> Box(
                    Modifier.size(112.dp).clip(CircleShape).background(tone.container.copy(alpha = 0.5f)),
                    contentAlignment = Alignment.Center,
                ) { IconBadge(icon, tone, size = 80.dp) }
            }
            Spacer(Modifier.height(24.dp))
            Text(title, style = MaterialTheme.typography.headlineSmall, color = Hc.Ink, textAlign = TextAlign.Center)
            if (body != null) {
                Spacer(Modifier.height(8.dp))
                Text(body, style = MaterialTheme.typography.bodyLarge, color = Hc.InkSoft, textAlign = TextAlign.Center)
            }
            if (extra != null) {
                Spacer(Modifier.height(20.dp))
                extra()
            }
        }
        if (actions != null) Column(verticalArrangement = Arrangement.spacedBy(10.dp), content = actions)
    }
}

/** Kartu ringkasan crash dari pemakaian sebelumnya. */
@Composable
fun CrashNotice(report: String, onCopy: () -> Unit, onDismiss: () -> Unit) {
    StatusBanner(
        tone = Tone.Warning,
        icon = Icons.Filled.Warning,
        title = "Aplikasi sempat tertutup tiba-tiba",
        body = "Salin laporan ini dan kirim ke tim agar penyebabnya bisa diperbaiki.\n" +
            report.lineSequence().drop(1).firstOrNull().orEmpty(),
    ) {
        Text(
            "Salin laporan",
            style = MaterialTheme.typography.labelLarge,
            color = Tone.Warning.content,
            modifier = Modifier.clip(RoundedCornerShape(8.dp)).clickable(onClick = onCopy).padding(4.dp),
        )
        Text(
            "Tutup",
            style = MaterialTheme.typography.labelLarge.copy(fontWeight = FontWeight.Medium),
            color = Hc.InkSoft,
            modifier = Modifier.clip(RoundedCornerShape(8.dp)).clickable(onClick = onDismiss).padding(4.dp),
        )
    }
}

/** Garis tipis pemisah antarbaris di dalam [SectionCard]. */
@Composable
fun RowDivider() {
    HorizontalDivider(color = Hc.Line, modifier = Modifier.padding(start = 70.dp, end = 16.dp))
}

/** Titik bulat kecil berwarna. */
@Composable
fun Dot(color: Color, size: Dp = 8.dp) {
    Box(Modifier.size(size).clip(CircleShape).background(color))
}

/** Getar singkat tanda tindakan berhasil (presensi, kirim, putuskan). Tidak ada efek di HP tanpa motor getar. */
@Composable
fun rememberConfirmHaptic(): () -> Unit {
    val view = LocalView.current
    return remember(view) {
        val haptic: () -> Unit = {
            val type = if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.R) HapticFeedbackConstants.CONFIRM else HapticFeedbackConstants.VIRTUAL_KEY
            view.performHapticFeedback(type)
        }
        haptic
    }
}

/** Kotak abu-abu berdenyut pelan sebagai tanda isi sedang dimuat. */
@Composable
fun SkeletonBlock(modifier: Modifier = Modifier, shape: androidx.compose.ui.graphics.Shape = RoundedCornerShape(6.dp)) {
    val pulse = rememberInfiniteTransition(label = "kerangka")
    val level by pulse.animateFloat(
        initialValue = 0.55f,
        targetValue = 1f,
        animationSpec = infiniteRepeatable(tween(900), RepeatMode.Reverse),
        label = "kerangka alfa",
    )
    Box(modifier.graphicsLayer { alpha = level }.clip(shape).background(Fg.Gray200))
}

/** Kartu kerangka seukuran kartu daftar, untuk menggantikan spinner saat daftar dimuat. */
@Composable
fun SkeletonCard(modifier: Modifier = Modifier, lines: Int = 2) {
    Column(
        modifier.fillMaxWidth().clip(CardShape).background(Hc.Surface).padding(20.dp)
            .semantics { contentDescription = "Memuat" },
        verticalArrangement = Arrangement.spacedBy(10.dp),
    ) {
        SkeletonBlock(Modifier.fillMaxWidth(0.45f).height(14.dp))
        repeat(lines) { i -> SkeletonBlock(Modifier.fillMaxWidth(if (i == lines - 1) 0.7f else 1f).height(12.dp)) }
    }
}

/**
 * Tarik ke bawah untuk memuat ulang. Indikator muncul di bawah status bar, di atas isi yang
 * digulir di dalam [content].
 */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun RefreshBox(refreshing: Boolean, onRefresh: () -> Unit, modifier: Modifier = Modifier, content: @Composable BoxScope.() -> Unit) {
    val state = rememberPullToRefreshState()
    PullToRefreshBox(
        isRefreshing = refreshing,
        onRefresh = onRefresh,
        modifier = modifier,
        state = state,
        indicator = {
            PullToRefreshDefaults.Indicator(
                state = state,
                isRefreshing = refreshing,
                color = Hc.Brand,
                containerColor = Color.White,
                modifier = Modifier.align(Alignment.TopCenter).statusBarsPadding(),
            )
        },
        content = content,
    )
}

/** Lingkaran inisial nama. */
@Composable
fun Avatar(name: String, size: Dp = 40.dp) {
    Box(Modifier.size(size).clip(CircleShape).background(Hc.BrandSoft), contentAlignment = Alignment.Center) {
        Text(initials(name), style = MaterialTheme.typography.titleSmall, color = Hc.BrandDeep)
    }
}

/** Footer aplikasi: teks dari super admin (lewat /v1/branding), atau teks bawaan selama belum termuat. */
@Composable
fun AppFooter(text: String?, modifier: Modifier = Modifier, version: String? = null) {
    Column(modifier.fillMaxWidth().padding(vertical = 8.dp), horizontalAlignment = Alignment.CenterHorizontally) {
        Text(text ?: defaultFooter(), style = MaterialTheme.typography.labelSmall, color = Fg.Gray500, textAlign = TextAlign.Center)
        if (version != null) {
            Text("Versi $version", style = MaterialTheme.typography.labelSmall, color = Fg.Gray500, textAlign = TextAlign.Center)
        }
    }
}
