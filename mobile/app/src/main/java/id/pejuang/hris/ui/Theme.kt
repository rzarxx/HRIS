package id.pejuang.hris.ui

import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Shapes
import androidx.compose.material3.Typography
import androidx.compose.material3.lightColorScheme
import androidx.compose.runtime.Composable
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp

/** Warna dasar aplikasi. Hijau mengikuti aksen web admin (#15803D). */
object Hc {
    val Brand = Color(0xFF15803D)
    val BrandDark = Color(0xFF0E6430)
    val BrandDeep = Color(0xFF0A4A23)
    val BrandSoft = Color(0xFFE7F4EC)
    val BrandBright = Color(0xFF4ADE80)

    val Ink = Color(0xFF0F172A)
    val InkSoft = Color(0xFF475569)
    val InkMuted = Color(0xFF94A3B8)
    val Line = Color(0xFFE2E8F0)
    val Canvas = Color(0xFFF4F6F8)
    val Surface = Color.White
}

/** Peran warna untuk status: wadah lembut dan isi yang kontras. */
enum class Tone(val container: Color, val content: Color) {
    Success(Color(0xFFE7F4EC), Color(0xFF15803D)),
    Warning(Color(0xFFFFF4E0), Color(0xFFB45309)),
    Danger(Color(0xFFFDECEC), Color(0xFFB91C1C)),
    Info(Color(0xFFE8F0FE), Color(0xFF1D4ED8)),
    Neutral(Color(0xFFF1F5F9), Color(0xFF475569)),
}

private val colors = lightColorScheme(
    primary = Hc.Brand,
    onPrimary = Color.White,
    primaryContainer = Hc.BrandSoft,
    onPrimaryContainer = Hc.BrandDeep,
    secondary = Hc.BrandDark,
    background = Hc.Canvas,
    onBackground = Hc.Ink,
    surface = Hc.Surface,
    onSurface = Hc.Ink,
    surfaceVariant = Color(0xFFF1F5F9),
    onSurfaceVariant = Hc.InkSoft,
    outline = Hc.Line,
    outlineVariant = Hc.Line,
    error = Color(0xFFB91C1C),
)

private val typography = Typography(
    headlineMedium = TextStyle(fontSize = 26.sp, lineHeight = 32.sp, fontWeight = FontWeight.SemiBold, letterSpacing = (-0.2).sp),
    headlineSmall = TextStyle(fontSize = 22.sp, lineHeight = 28.sp, fontWeight = FontWeight.SemiBold),
    titleLarge = TextStyle(fontSize = 20.sp, lineHeight = 26.sp, fontWeight = FontWeight.SemiBold),
    titleMedium = TextStyle(fontSize = 16.sp, lineHeight = 22.sp, fontWeight = FontWeight.SemiBold),
    titleSmall = TextStyle(fontSize = 14.sp, lineHeight = 20.sp, fontWeight = FontWeight.SemiBold),
    bodyLarge = TextStyle(fontSize = 16.sp, lineHeight = 24.sp),
    bodyMedium = TextStyle(fontSize = 14.sp, lineHeight = 20.sp),
    bodySmall = TextStyle(fontSize = 12.sp, lineHeight = 16.sp),
    labelLarge = TextStyle(fontSize = 15.sp, lineHeight = 20.sp, fontWeight = FontWeight.SemiBold),
    labelMedium = TextStyle(fontSize = 12.sp, lineHeight = 16.sp, fontWeight = FontWeight.Medium),
    labelSmall = TextStyle(fontSize = 11.sp, lineHeight = 14.sp, fontWeight = FontWeight.Medium),
)

private val shapes = Shapes(
    extraSmall = RoundedCornerShape(8.dp),
    small = RoundedCornerShape(10.dp),
    medium = RoundedCornerShape(14.dp),
    large = RoundedCornerShape(20.dp),
    extraLarge = RoundedCornerShape(28.dp),
)

@Composable
fun HrisTheme(content: @Composable () -> Unit) {
    MaterialTheme(colorScheme = colors, typography = typography, shapes = shapes, content = content)
}
