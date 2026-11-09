package id.pejuang.hris.ui

import android.app.Activity
import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.Canvas
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ColumnScope
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.RowScope
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
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
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.geometry.CornerRadius
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.Path
import androidx.compose.ui.graphics.StrokeCap
import androidx.compose.ui.graphics.StrokeJoin
import androidx.compose.ui.graphics.drawscope.Stroke
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.platform.LocalView
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import androidx.core.view.WindowCompat

val BrandGradient = Brush.verticalGradient(listOf(Hc.Brand, Hc.BrandDeep))

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

/** Logo aplikasi: bingkai pindai wajah dengan tanda centang, sama dengan ikon peluncur. */
@Composable
fun LogoMark(size: Dp = 56.dp, background: Color = Color.White, foreground: Color = Hc.Brand) {
    Canvas(Modifier.size(size)) {
        val s = this.size.minDimension
        drawRoundRect(background, cornerRadius = CornerRadius(s * 0.28f))
        val stroke = Stroke(width = s * 0.075f, cap = StrokeCap.Round, join = StrokeJoin.Round)
        val a = s * 0.24f
        val b = s * 0.76f
        val arm = s * 0.13f
        val r = s * 0.07f
        fun corner(x: Float, y: Float, dx: Float, dy: Float) {
            val p = Path().apply {
                moveTo(x, y + dy * arm)
                lineTo(x, y + dy * r)
                quadraticBezierTo(x, y, x + dx * r, y)
                lineTo(x + dx * arm, y)
            }
            drawPath(p, foreground, style = stroke)
        }
        corner(a, a, 1f, 1f)
        corner(b, a, -1f, 1f)
        corner(b, b, -1f, -1f)
        corner(a, b, 1f, -1f)
        val check = Path().apply {
            moveTo(s * 0.37f, s * 0.51f)
            lineTo(s * 0.46f, s * 0.60f)
            lineTo(s * 0.64f, s * 0.42f)
        }
        drawPath(check, foreground, style = Stroke(width = s * 0.085f, cap = StrokeCap.Round, join = StrokeJoin.Round))
    }
}

/** Bilah judul putih untuk layar turunan. */
@Composable
fun TopBar(title: String, onBack: (() -> Unit)?, navIcon: ImageVector = Icons.AutoMirrored.Filled.ArrowBack) {
    StatusBarIcons(dark = true)
    Column(Modifier.background(Hc.Surface).statusBarsPadding()) {
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
        HorizontalDivider(color = Hc.Line)
    }
}

/** Kartu putih bergaris tipis, wadah utama konten. */
@Composable
fun SectionCard(
    modifier: Modifier = Modifier,
    padding: Dp = 16.dp,
    content: @Composable ColumnScope.() -> Unit,
) {
    Surface(
        modifier = modifier.fillMaxWidth(),
        shape = MaterialTheme.shapes.large,
        color = Hc.Surface,
        border = BorderStroke(1.dp, Hc.Line),
        shadowElevation = 1.dp,
    ) {
        Column(Modifier.padding(padding), content = content)
    }
}

@Composable
fun SectionLabel(text: String, modifier: Modifier = Modifier) {
    Text(
        text.uppercase(),
        style = MaterialTheme.typography.labelMedium,
        color = Hc.InkSoft,
        modifier = modifier.padding(start = 4.dp, top = 8.dp, bottom = 2.dp),
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
        modifier = modifier.clip(CircleShape).background(tone.container).padding(horizontal = 10.dp, vertical = 4.dp),
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
    Surface(modifier = modifier.fillMaxWidth(), shape = MaterialTheme.shapes.medium, color = tone.container) {
        Row(Modifier.padding(14.dp), horizontalArrangement = Arrangement.spacedBy(12.dp)) {
            Icon(icon, contentDescription = null, tint = tone.content, modifier = Modifier.size(22.dp))
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
    shape: androidx.compose.ui.graphics.Shape = MaterialTheme.shapes.medium,
) {
    Button(
        onClick = onClick,
        enabled = enabled && !loading,
        modifier = modifier.fillMaxWidth().height(52.dp),
        shape = shape,
        colors = ButtonDefaults.buttonColors(
            containerColor = Hc.Brand,
            contentColor = Color.White,
            disabledContainerColor = if (loading) Hc.Brand else Hc.Line,
            disabledContentColor = if (loading) Color.White else Hc.InkMuted,
        ),
    ) {
        if (loading) {
            CircularProgressIndicator(color = Color.White, strokeWidth = 2.dp, modifier = Modifier.size(20.dp))
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
    OutlinedButton(
        onClick = onClick,
        modifier = modifier.fillMaxWidth().height(52.dp),
        shape = MaterialTheme.shapes.medium,
        border = BorderStroke(1.dp, Hc.Line),
        colors = ButtonDefaults.outlinedButtonColors(containerColor = Hc.Surface, contentColor = Hc.Ink),
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
            .padding(horizontal = 16.dp, vertical = 14.dp),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(14.dp),
    ) {
        IconBadge(icon, Tone.Neutral, size = 40.dp)
        Column(Modifier.weight(1f)) {
            Text(title, style = MaterialTheme.typography.titleSmall, color = Hc.Ink)
            if (subtitle != null) Text(subtitle, style = MaterialTheme.typography.bodySmall, color = Hc.InkSoft)
        }
        trailing?.invoke()
        if (onClick != null) {
            Icon(Icons.AutoMirrored.Filled.KeyboardArrowRight, contentDescription = null, tint = Hc.InkMuted)
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
    Column(modifier.fillMaxSize().background(Hc.Canvas).navigationBarsPadding().padding(24.dp)) {
        Column(
            modifier = Modifier.weight(1f).fillMaxWidth(),
            horizontalAlignment = Alignment.CenterHorizontally,
            verticalArrangement = Arrangement.Center,
        ) {
            when {
                loading -> CircularProgressIndicator(color = Hc.Brand, strokeWidth = 3.dp, modifier = Modifier.size(48.dp))
                icon != null -> Box(
                    Modifier.size(104.dp).clip(CircleShape).background(tone.container.copy(alpha = 0.6f)),
                    contentAlignment = Alignment.Center,
                ) { IconBadge(icon, tone, size = 76.dp) }
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
    HorizontalDivider(color = Hc.Line, modifier = Modifier.padding(start = 70.dp))
}

/** Titik bulat kecil berwarna. */
@Composable
fun Dot(color: Color, size: Dp = 8.dp) {
    Box(Modifier.size(size).clip(CircleShape).background(color))
}
