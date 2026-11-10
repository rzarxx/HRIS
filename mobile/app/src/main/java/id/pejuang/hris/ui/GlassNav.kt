package id.pejuang.hris.ui

import androidx.compose.animation.animateColorAsState
import androidx.compose.animation.core.animateDpAsState
import androidx.compose.animation.core.tween
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.BoxWithConstraints
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.WindowInsets
import androidx.compose.foundation.layout.asPaddingValues
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.navigationBars
import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.layout.offset
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Icon
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.compositionLocalOf
import androidx.compose.runtime.getValue
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.draw.shadow
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalInspectionMode
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.selected
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import dev.chrisbanes.haze.HazeState
import dev.chrisbanes.haze.HazeStyle
import dev.chrisbanes.haze.HazeTint
import dev.chrisbanes.haze.hazeEffect

/** Tinggi kapsul bottom bar kaca dan jaraknya dari tepi bawah layar. */
private val BarHeight = 64.dp
private val BarMargin = 12.dp

/**
 * Ruang kosong di bawah isi tab agar baris terakhir tidak tertutup bottom bar yang mengambang.
 * Nol di luar [MainTabs]; layar lalu memakai tinggi navigation bar sistem.
 */
val LocalBottomBarSpace = compositionLocalOf { 0.dp }

/** Jarak bawah untuk isi yang bisa digulir: di atas bottom bar kaca, atau di atas navigation bar sistem. */
@Composable
fun bottomContentSpace(extra: Dp = 0.dp): Dp {
    val bar = LocalBottomBarSpace.current
    return if (bar > 0.dp) bar + extra else WindowInsets.navigationBars.asPaddingValues().calculateBottomPadding() + extra
}

/** Total ruang yang dipakai bottom bar dari tepi bawah layar, termasuk navigation bar sistem. */
@Composable
fun glassBarSpace(): Dp = BarHeight + BarMargin * 2 + WindowInsets.navigationBars.asPaddingValues().calculateBottomPadding()

private val GlassStyle = HazeStyle(
    backgroundColor = Fg.Page,
    tint = HazeTint(Color.White.copy(alpha = 0.55f)),
    blurRadius = 24.dp,
    noiseFactor = 0.04f,
    // Android 8–11 tidak punya RenderEffect: pakai putih hampir pekat agar teks tetap terbaca.
    fallbackTint = HazeTint(Color.White.copy(alpha = 0.92f)),
)

/**
 * Bottom bar kapsul mengambang bergaya kaca (glassmorphism), mengaburkan isi di belakangnya.
 * Tab aktif ditandai pil hijau lembut yang bergeser dengan animasi singkat.
 */
@Composable
fun GlassBottomBar(
    selected: MainTab,
    onSelect: (MainTab) -> Unit,
    hazeState: HazeState,
    modifier: Modifier = Modifier,
    badges: Map<MainTab, Int> = emptyMap(),
) {
    val shape = RoundedCornerShape(BarHeight / 2)
    // Renderer pratinjau dan Paparazzi tidak mendukung RenderEffect; pakai tint pengganti di sana.
    val preview = LocalInspectionMode.current
    Box(
        modifier
            .fillMaxWidth()
            .navigationBarsPadding()
            .padding(horizontal = 20.dp, vertical = BarMargin),
        contentAlignment = Alignment.Center,
    ) {
        BoxWithConstraints(
            Modifier
                .widthIn(max = 480.dp)
                .fillMaxWidth()
                .height(BarHeight)
                .shadow(18.dp, shape, ambientColor = Color(0x14101828), spotColor = Color(0x29101828))
                .clip(shape)
                .hazeEffect(hazeState, GlassStyle) { if (preview) blurEnabled = false }
                // Kilau kaca: lebih terang di atas, memudar ke bawah.
                .background(Brush.verticalGradient(listOf(Color.White.copy(alpha = 0.35f), Color.White.copy(alpha = 0.06f))))
                .border(1.dp, Brush.verticalGradient(listOf(Color.White.copy(alpha = 0.95f), Color.White.copy(alpha = 0.35f))), shape),
        ) {
            val itemWidth = maxWidth / MainTab.entries.size
            val indicatorX by animateDpAsState(itemWidth * selected.ordinal, tween(280), label = "indikator tab")
            Box(
                Modifier
                    .offset(x = indicatorX)
                    .width(itemWidth)
                    .fillMaxHeight()
                    .padding(5.dp)
                    .clip(RoundedCornerShape(BarHeight / 2))
                    .background(Hc.Brand.copy(alpha = 0.12f))
                    .border(1.dp, Color.White.copy(alpha = 0.7f), RoundedCornerShape(BarHeight / 2)),
            )
            Row(Modifier.fillMaxSize(), verticalAlignment = Alignment.CenterVertically) {
                MainTab.entries.forEach { tab ->
                    GlassTabItem(tab, tab == selected, badges[tab] ?: 0, Modifier.weight(1f)) { onSelect(tab) }
                }
            }
        }
    }
}

@Composable
private fun GlassTabItem(tab: MainTab, active: Boolean, badge: Int, modifier: Modifier, onClick: () -> Unit) {
    val color by animateColorAsState(if (active) Hc.Brand else Fg.TextSecondary, tween(200), label = "warna tab")
    Column(
        modifier
            .fillMaxHeight()
            .clickable(
                interactionSource = remember { MutableInteractionSource() },
                indication = null,
                role = Role.Tab,
                onClick = onClick,
            )
            .semantics {
                contentDescription = if (badge > 0) "${tab.label}, $badge menunggu" else tab.label
                this.selected = active
            },
        horizontalAlignment = Alignment.CenterHorizontally,
        verticalArrangement = Arrangement.Center,
    ) {
        Box {
            Icon(if (active) tab.selectedIcon else tab.icon, contentDescription = null, tint = color, modifier = Modifier.size(22.dp))
            if (badge > 0) CountPill(badge, Modifier.align(Alignment.TopEnd).offset(x = 10.dp, y = (-6).dp))
        }
        Text(
            tab.label,
            style = figmaText(10, if (active) FontWeight.SemiBold else FontWeight.Medium, color).copy(lineHeight = 13.sp),
            maxLines = 1,
            overflow = TextOverflow.Ellipsis,
            modifier = Modifier.padding(top = 3.dp, start = 2.dp, end = 2.dp),
        )
    }
}
