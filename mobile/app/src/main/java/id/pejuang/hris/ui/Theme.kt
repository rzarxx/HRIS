package id.pejuang.hris.ui

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.WindowInsets
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.statusBars
import androidx.compose.foundation.layout.windowInsetsTopHeight
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Shapes
import androidx.compose.material3.Typography
import androidx.compose.material3.lightColorScheme
import androidx.compose.runtime.Composable
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.shadow
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.Shape
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.Font
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.Density
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import id.pejuang.hris.R

/**
 * Warna aplikasi, gaya Clean Minimalist / Soft UI (DESIGN.md §8). Hijau terang untuk isi tombol
 * dengan teks gelap di atasnya; hijau yang lebih tua hanya untuk teks agar kontras tetap ≥ 4,5:1.
 */
object Hc {
    /** Isi tombol utama dan indikator aktif. Teks di atasnya memakai [OnPrimary] (6,5:1). */
    val Primary = Color(0xFF22C55E)
    val OnPrimary = Color(0xFF052E16)
    /** Ikon, garis kemajuan, dan titik status hijau (bukan teks). */
    val Brand = Color(0xFF16A34A)
    /** Teks dan tautan hijau di atas putih (5,0:1). */
    val BrandDark = Color(0xFF15803D)
    /** Teks hijau di atas latar mint (6,5:1). */
    val BrandDeep = Color(0xFF166534)
    /** Latar lembut: chip, ubin ikon, banner. */
    val BrandSoft = Color(0xFFECFDF3)
    val BrandSoftStrong = Color(0xFFDCFCE7)
    val BrandBright = Color(0xFF4ADE80)

    val Ink = Color(0xFF101828)
    val InkSoft = Color(0xFF475467)
    val InkMuted = Color(0xFF98A2B3)
    val Line = Color(0xFFEAECF0)
    /** Isi kolom input tanpa garis. */
    val Field = Color(0xFFF2F4F7)
    val Canvas = Color(0xFFF7F8FA)
    val Surface = Color.White
    /** Tombol merah (tolak, keluar) dengan teks putih (5,0:1). */
    val Danger = Color(0xFFD92D20)
    /** Warna bayangan lembut. */
    val Shadow = Color(0xFF101828)
}

/** Peran warna untuk status: wadah lembut dan isi yang kontras (≥ 4,5:1 di atas wadahnya). */
enum class Tone(val container: Color, val content: Color) {
    Success(Color(0xFFECFDF3), Color(0xFF067647)),
    Warning(Color(0xFFFFFAEB), Color(0xFFB54708)),
    Danger(Color(0xFFFEF3F2), Color(0xFFB42318)),
    Info(Color(0xFFEFF8FF), Color(0xFF175CD3)),
    Neutral(Color(0xFFF2F4F7), Color(0xFF475467)),
}

/** Plus Jakarta Sans (OFL), dibundel agar sama di semua HP. */
val Jakarta = FontFamily(
    Font(R.font.plus_jakarta_sans_regular, FontWeight.Normal),
    Font(R.font.plus_jakarta_sans_medium, FontWeight.Medium),
    Font(R.font.plus_jakarta_sans_semibold, FontWeight.SemiBold),
    Font(R.font.plus_jakarta_sans_bold, FontWeight.Bold),
)

/** Sudut: kartu 20dp, kontrol 14dp. */
val CardShape = RoundedCornerShape(20.dp)
val ControlShape = RoundedCornerShape(14.dp)

/**
 * Bayangan lembut satu tingkat (Soft UI): menyebar dan sangat tipis, pengganti garis kartu.
 * Di Android 8 tanpa dukungan warna bayangan, tetap tampil sebagai bayangan abu tipis.
 */
fun Modifier.softShadow(shape: Shape = CardShape, elevation: Dp = 10.dp): Modifier =
    shadow(elevation, shape, clip = false, ambientColor = Hc.Shadow.copy(alpha = 0.05f), spotColor = Hc.Shadow.copy(alpha = 0.08f))

/** Lapisan selebar status bar berwarna latar halaman, dipasang di atas isi yang bisa digulir. */
@Composable
fun StatusBarScrim(color: Color = Hc.Canvas) {
    Box(Modifier.fillMaxWidth().windowInsetsTopHeight(WindowInsets.statusBars).background(color.copy(alpha = 0.96f)))
}

private val colors = lightColorScheme(
    primary = Hc.Primary,
    onPrimary = Hc.OnPrimary,
    primaryContainer = Hc.BrandSoft,
    onPrimaryContainer = Hc.BrandDeep,
    secondary = Hc.BrandDark,
    background = Hc.Canvas,
    onBackground = Hc.Ink,
    surface = Hc.Surface,
    onSurface = Hc.Ink,
    surfaceVariant = Hc.Field,
    onSurfaceVariant = Hc.InkSoft,
    outline = Hc.Line,
    outlineVariant = Hc.Line,
    error = Color(0xFFB42318),
)

private fun style(size: Int, line: Int, weight: FontWeight = FontWeight.Normal, tracking: Float = 0f) =
    TextStyle(fontFamily = Jakarta, fontSize = size.sp, lineHeight = line.sp, fontWeight = weight, letterSpacing = tracking.sp)

private val typography = Typography(
    headlineLarge = style(30, 38, FontWeight.Bold, -0.4f),
    headlineMedium = style(26, 32, FontWeight.Bold, -0.3f),
    headlineSmall = style(22, 28, FontWeight.SemiBold, -0.2f),
    titleLarge = style(20, 26, FontWeight.SemiBold, -0.1f),
    titleMedium = style(16, 22, FontWeight.SemiBold),
    titleSmall = style(14, 20, FontWeight.SemiBold),
    bodyLarge = style(16, 24),
    bodyMedium = style(14, 20),
    bodySmall = style(12, 16),
    labelLarge = style(15, 20, FontWeight.SemiBold),
    labelMedium = style(12, 16, FontWeight.Medium),
    labelSmall = style(11, 14, FontWeight.Medium),
)

private val shapes = Shapes(
    extraSmall = RoundedCornerShape(8.dp),
    small = RoundedCornerShape(12.dp),
    medium = ControlShape,
    large = CardShape,
    extraLarge = RoundedCornerShape(28.dp),
)

/**
 * Tema aplikasi. [fontScale] adalah ukuran huruf pilihan pengguna di Profil, dikalikan dengan
 * ukuran huruf sistem HP sehingga pengaturan aksesibilitas HP tetap berlaku.
 */
@Composable
fun HrisTheme(fontScale: Float = 1f, content: @Composable () -> Unit) {
    val base = LocalDensity.current
    CompositionLocalProvider(LocalDensity provides Density(base.density, base.fontScale * fontScale)) {
        MaterialTheme(colorScheme = colors, typography = typography, shapes = shapes, content = content)
    }
}
