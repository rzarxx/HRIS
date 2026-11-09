package id.pejuang.hris.ui

import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.foundation.interaction.collectIsFocusedAsState
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.BoxScope
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ColumnScope
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
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.BasicTextField
import androidx.compose.foundation.text.KeyboardActions
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.Icon
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.draw.shadow
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.Shadow
import androidx.compose.ui.graphics.SolidColor
import androidx.compose.ui.graphics.graphicsLayer
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.selected
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.PasswordVisualTransformation
import androidx.compose.ui.text.input.VisualTransformation
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import id.pejuang.hris.R

/**
 * Warna UI kit Figma. Ungu di kit diganti hijau aplikasi (#15803D) dengan peran yang sama; abu-abu
 * dan teks tetap mengikuti kit.
 */
object Fg {
    // Pengganti purple-500 (#7A5AF8) dan aksen ungu lain.
    val Accent = Hc.Brand
    // Pengganti gradasi header #795FFC → #7155FF.
    val HeaderTop = Color(0xFF168540)
    val HeaderBottom = Color(0xFF117A39)
    // Pengganti purple-200 (#D9D6FE) untuk teks dan bintang di atas header.
    val OnHeaderSoft = Color(0xFFBBF7D0)
    // Pengganti gradasi tombol utama #8862F2 → #7544FC → #5B2ED4, garis #A787FF, cincin #6938EF.
    val ButtonTop = Color(0xFF22A355)
    val ButtonMid = Color(0xFF178A42)
    val ButtonBottom = Color(0xFF0E6430)
    val ButtonBorder = Color(0xFF6CCB8E)
    val ButtonRing = Color(0xFF0E6430)
    val ButtonShadow = Color(0xFF07361A)
    // Pengganti ikon isian #9B8AFB.
    val FieldIcon = Color(0xFF5DB67F)

    val Page = Color(0xFFF1F3F8)
    val PageProfile = Color(0xFFF4F6F9)
    val Card = Color(0xFFFEFEFE)
    val Gray50 = Color(0xFFEBECEE)
    val Gray100 = Color(0xFFF9FAFB)
    val Gray200 = Color(0xFFEAECF0)
    val Gray300 = Color(0xFFD0D5DD)
    val Gray400 = Color(0xFF98A2B3)
    val Gray500 = Color(0xFF667085)
    val TextPrimary = Color(0xFF101828)
    val TextSecondary = Color(0xFF475467)
    val TextBody = Color(0xFF344054)
    val TextStrong = Color(0xFF161B23)
    val RowText = Color(0xFF4F5464)
    val Chevron = Color(0xFFB6C2D7)
    val Danger = Color(0xFFF14E4E)
    val NavBar = Color(0xFF1C2020)
    val Backdrop = Color(0x992D2D2D)
}

val FigmaHeaderGradient = Brush.verticalGradient(listOf(Fg.HeaderTop, Fg.HeaderBottom))

/** Teks Figma: Inter di kit, font sistem di aplikasi. */
fun figmaText(size: Int, weight: FontWeight = FontWeight.Normal, color: Color = Fg.TextPrimary, lineHeight: Float? = null, tracking: Float = 0f) =
    TextStyle(
        fontSize = size.sp,
        fontWeight = weight,
        color = color,
        lineHeight = lineHeight?.let { (size * it).sp } ?: androidx.compose.ui.unit.TextUnit.Unspecified,
        letterSpacing = tracking.sp,
    )

/** Tombol utama Figma: pil 48dp, gradasi, garis terang, dan cincin luar 1dp. */
@Composable
fun FigmaPrimaryButton(
    text: String,
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
    enabled: Boolean = true,
    loading: Boolean = false,
) {
    val active = enabled && !loading
    val shape = RoundedCornerShape(100.dp)
    Box(
        modifier
            .fillMaxWidth()
            .height(48.dp)
            .clip(shape)
            .background(if (enabled || loading) Fg.ButtonRing else Fg.Gray300)
            .padding(1.dp)
            .clip(shape)
            .background(
                if (enabled || loading) Brush.verticalGradient(0f to Fg.ButtonTop, 0.2883f to Fg.ButtonMid, 1f to Fg.ButtonBottom)
                else SolidColor(Fg.Gray200),
            )
            .border(1.dp, if (enabled || loading) Fg.ButtonBorder else Fg.Gray300, shape)
            .clickable(enabled = active, role = Role.Button, onClick = onClick),
        contentAlignment = Alignment.Center,
    ) {
        if (loading) {
            CircularProgressIndicator(color = Color.White, strokeWidth = 2.dp, modifier = Modifier.size(20.dp))
        } else {
            Text(
                text,
                style = figmaText(14, FontWeight.Medium, if (enabled) Color.White else Fg.Gray400, tracking = 0.1f).copy(
                    lineHeight = 20.sp,
                    shadow = if (enabled) Shadow(Fg.ButtonShadow, blurRadius = 16f, offset = androidx.compose.ui.geometry.Offset(0f, 1f)) else null,
                ),
                maxLines = 1,
            )
        }
    }
}

/** Tombol kedua Figma: pil bergaris warna aksen. */
@Composable
fun FigmaSecondaryButton(text: String, onClick: () -> Unit, modifier: Modifier = Modifier, color: Color = Fg.ButtonBottom) {
    Box(
        modifier
            .fillMaxWidth()
            .height(48.dp)
            .clip(RoundedCornerShape(100.dp))
            .border(BorderStroke(1.dp, color), RoundedCornerShape(100.dp))
            .clickable(role = Role.Button, onClick = onClick),
        contentAlignment = Alignment.Center,
    ) {
        Text(text, style = figmaText(14, FontWeight.Medium, color, tracking = 0.1f).copy(lineHeight = 20.sp), maxLines = 1)
    }
}

/** Tombol bundar 32dp di header, berisi ikon 16dp. */
@Composable
fun FigmaCircleButton(icon: ImageVector, description: String, background: Color, tint: Color, onClick: () -> Unit) {
    Box(
        Modifier.size(32.dp).clip(CircleShape).background(background).clickable(role = Role.Button, onClick = onClick)
            .semantics { contentDescription = description },
        contentAlignment = Alignment.Center,
    ) {
        Icon(icon, contentDescription = null, tint = tint, modifier = Modifier.size(16.dp))
    }
}

/** Header putih layar turunan (komponen "Header" di kit): tombol kembali, judul di tengah. */
@Composable
fun FigmaHeader(title: String, onBack: () -> Unit) {
    StatusBarIcons(dark = true)
    Column(Modifier.fillMaxWidth().background(Fg.Card).statusBarsPadding()) {
        Box(Modifier.fillMaxWidth().padding(horizontal = 23.5.dp).padding(top = 12.dp, bottom = 16.dp).height(32.dp)) {
            Box(Modifier.align(Alignment.CenterStart)) {
                FigmaCircleButton(FigmaIcons.ArrowLeftOutline, "Kembali", Fg.PageProfile, Fg.Accent, onBack)
            }
            Text(title, style = figmaText(18, FontWeight.SemiBold), modifier = Modifier.align(Alignment.Center), maxLines = 1)
        }
        Box(Modifier.fillMaxWidth().height(1.dp).background(Fg.Gray200))
    }
}

/** Kartu putih Figma: sudut 8dp, padding 16dp, judul dan keterangan. */
@Composable
fun FigmaCard(
    modifier: Modifier = Modifier,
    title: String? = null,
    subtitle: String? = null,
    top: Dp = 12.dp,
    bottom: Dp = 12.dp,
    gap: Dp = 12.dp,
    content: @Composable ColumnScope.() -> Unit,
) {
    Column(
        modifier.fillMaxWidth().clip(RoundedCornerShape(8.dp)).background(Fg.Card).padding(start = 16.dp, end = 16.dp, top = top, bottom = bottom),
        verticalArrangement = Arrangement.spacedBy(gap),
    ) {
        if (title != null) {
            Column(verticalArrangement = Arrangement.spacedBy(2.dp)) {
                Text(title, style = figmaText(14, FontWeight.SemiBold, lineHeight = 1.4f))
                if (subtitle != null) Text(subtitle, style = figmaText(12, color = Fg.TextSecondary, lineHeight = 1.4f))
            }
        }
        content()
    }
}

/** Kolom isian Figma ("Input Field/Text & Icon"): label 12, kotak 44dp, ikon 20dp. */
@Composable
fun FigmaInput(
    label: String,
    value: String,
    onChange: (String) -> Unit,
    icon: ImageVector,
    modifier: Modifier = Modifier,
    readOnly: Boolean = false,
    password: Boolean = false,
    error: String? = null,
    keyboard: KeyboardOptions = KeyboardOptions.Default,
    onDone: (() -> Unit)? = null,
) {
    val interaction = remember { MutableInteractionSource() }
    val focused by interaction.collectIsFocusedAsState()
    val border = when {
        error != null -> Fg.Danger
        focused && !readOnly -> Fg.Accent
        readOnly -> Fg.Gray300
        else -> Fg.Gray400
    }
    Column(modifier.fillMaxWidth(), verticalArrangement = Arrangement.spacedBy(4.dp)) {
        Text(label, style = figmaText(12, FontWeight.Medium, Fg.TextSecondary).copy(lineHeight = 17.sp))
        BasicTextField(
            value = value,
            onValueChange = onChange,
            readOnly = readOnly,
            singleLine = true,
            textStyle = figmaText(14, color = Fg.TextPrimary).copy(lineHeight = 17.sp),
            visualTransformation = if (password) PasswordVisualTransformation() else VisualTransformation.None,
            keyboardOptions = keyboard,
            keyboardActions = KeyboardActions(onDone = { onDone?.invoke() }),
            interactionSource = interaction,
            cursorBrush = SolidColor(Fg.Accent),
            decorationBox = { inner ->
                Row(
                    Modifier
                        .fillMaxWidth()
                        .height(44.dp)
                        .shadow(1.dp, RoundedCornerShape(8.dp), ambientColor = Color(0x0D101828), spotColor = Color(0x0D101828))
                        .clip(RoundedCornerShape(8.dp))
                        .background(Color.White)
                        .border(1.dp, border, RoundedCornerShape(8.dp))
                        .padding(12.dp),
                    verticalAlignment = Alignment.CenterVertically,
                    horizontalArrangement = Arrangement.spacedBy(10.dp),
                ) {
                    Icon(icon, contentDescription = null, tint = Fg.FieldIcon, modifier = Modifier.size(20.dp))
                    Box(Modifier.weight(1f)) { inner() }
                }
            },
        )
        if (error != null) Text(error, style = figmaText(12, color = Fg.Danger, lineHeight = 1.3f))
    }
}

/**
 * Popup pesan Figma ("Popup Message"): latar gelap, lembar putih dari bawah, ubin ikon 100dp yang
 * menonjol di atas lembar.
 */
@Composable
fun BoxScope.FigmaPopup(
    icon: ImageVector,
    title: String,
    body: String,
    primary: String,
    onPrimary: () -> Unit,
    secondary: String? = null,
    onSecondary: () -> Unit = {},
    onDismiss: () -> Unit = onSecondary,
    danger: Boolean = false,
) {
    Box(
        Modifier.matchParentSize().background(Fg.Backdrop)
            .clickable(interactionSource = remember { MutableInteractionSource() }, indication = null, onClick = onDismiss),
    )
    Box(Modifier.align(Alignment.BottomCenter).fillMaxWidth()) {
        Column(
            Modifier
                .padding(top = 53.dp)
                .fillMaxWidth()
                .clip(RoundedCornerShape(topStart = 16.dp, topEnd = 16.dp))
                .background(Color.White)
                .clickable(interactionSource = remember { MutableInteractionSource() }, indication = null) {}
                .navigationBarsPadding()
                .padding(start = 31.dp, end = 31.dp, top = 85.dp, bottom = 32.dp),
            verticalArrangement = Arrangement.spacedBy(24.dp),
        ) {
            Column(verticalArrangement = Arrangement.spacedBy(16.dp)) {
                Text(title, style = figmaText(20, FontWeight.SemiBold), textAlign = TextAlign.Center, modifier = Modifier.fillMaxWidth())
                Text(body, style = figmaText(13, FontWeight.Medium, Fg.TextSecondary, lineHeight = 1.3f), modifier = Modifier.fillMaxWidth())
            }
            Column(verticalArrangement = Arrangement.spacedBy(16.dp)) {
                FigmaPrimaryButton(primary, onPrimary)
                if (secondary != null) FigmaSecondaryButton(secondary, onSecondary)
            }
        }
        val tile = if (danger) Fg.Danger else Fg.Accent
        Box(Modifier.align(Alignment.TopCenter).offset(y = 95.dp).width(58.dp).height(11.dp).shadow(12.dp, RoundedCornerShape(6.dp), spotColor = tile, ambientColor = tile))
        Box(
            Modifier.align(Alignment.TopCenter).size(100.dp).clip(RoundedCornerShape(16.dp)).background(tile)
                .border(1.dp, if (danger) Color(0xFFF7A1A1) else Fg.ButtonBorder, RoundedCornerShape(16.dp)),
            contentAlignment = Alignment.Center,
        ) {
            Icon(icon, contentDescription = null, tint = Color.White, modifier = Modifier.size(48.dp))
        }
    }
}

/** Menu navigasi bawah, sesuai lima layar mobile di MASTER.md §10.2. */
enum class MainTab(val label: String, val icon: ImageVector, val selectedIcon: ImageVector) {
    Home("Beranda", FigmaIcons.NavHome, FigmaIcons.NavHomeBold),
    Attendance("Presensi", FigmaIcons.NavCalendar, FigmaIcons.NavCalendarBold),
    Requests("Pengajuan", FigmaIcons.NavNote, FigmaIcons.NavNoteBold),
    Payslip("Slip upah", FigmaIcons.NavReceipt, FigmaIcons.NavReceiptBold),
    Profile("Profil", FigmaIcons.User, FigmaIcons.UserBold),
}

/** Navbar Figma: latar #1C2020, ikon 24dp putih, garis 12×2dp di bawah ikon aktif. */
@Composable
fun FigmaBottomNav(selected: MainTab, onSelect: (MainTab) -> Unit) {
    Box(Modifier.fillMaxWidth().background(Fg.NavBar)) {
        // Pendaran elips #2A2A2A di tepi atas navbar.
        Box(
            Modifier.fillMaxWidth().height(18.dp)
                .background(Brush.verticalGradient(listOf(Color(0xFF2A2A2A), Fg.NavBar))),
        )
        Row(
            Modifier.fillMaxWidth().navigationBarsPadding().padding(horizontal = 20.dp).padding(top = 18.dp, bottom = 16.dp).height(40.dp),
            verticalAlignment = Alignment.Top,
        ) {
            MainTab.entries.forEach { tab ->
                val active = tab == selected
                Column(
                    Modifier
                        .weight(1f)
                        .clip(RoundedCornerShape(8.dp))
                        .clickable(role = Role.Tab) { onSelect(tab) }
                        .semantics {
                            contentDescription = tab.label
                            this.selected = active
                        },
                    horizontalAlignment = Alignment.CenterHorizontally,
                    verticalArrangement = Arrangement.spacedBy(8.dp),
                ) {
                    Icon(if (active) tab.selectedIcon else tab.icon, contentDescription = null, tint = Color(0xFFFEFEFE), modifier = Modifier.size(24.dp))
                    Box(Modifier.width(12.dp).height(2.dp).clip(RoundedCornerShape(2.dp)).background(if (active) Color.White else Color.Transparent))
                }
            }
        }
    }
}

/** Bintang empat sudut dari header Figma, berputar sesuai desain. */
@Composable
fun FigmaStar(x: Dp, y: Dp, size: Dp, rotation: Float, large: Boolean = false, color: Color = Fg.OnHeaderSoft) {
    Icon(
        if (large) FigmaIcons.StarInner else FigmaIcons.StarThin,
        contentDescription = null,
        tint = color,
        modifier = Modifier.offset(x = x, y = y).size(size).graphicsLayer(rotationZ = rotation),
    )
}

/**
 * Header hijau layar tab ("Attendant/Bg Header"): tinggi 189dp di bawah status bar, sudut bawah
 * 24dp, judul dan keterangan, serta ilustrasi jam bersayap.
 */
@Composable
fun FigmaTabHeader(title: String, subtitle: String, illustration: Boolean = true, extraHeight: Dp = 0.dp) {
    StatusBarIcons(dark = false)
    Box(
        Modifier
            .fillMaxWidth()
            .clip(RoundedCornerShape(bottomStart = 24.dp, bottomEnd = 24.dp))
            .background(FigmaHeaderGradient)
            .statusBarsPadding()
            .height(189.dp + extraHeight),
    ) {
        Column(Modifier.padding(start = 28.dp, top = 27.dp, end = 128.dp), verticalArrangement = Arrangement.spacedBy(1.dp)) {
            Text(title, style = figmaText(24, FontWeight.SemiBold, Color(0xFFFEFEFE)).copy(lineHeight = 32.sp), maxLines = 2)
            Text(subtitle, style = figmaText(14, FontWeight.Medium, Fg.OnHeaderSoft, tracking = 0.1f).copy(lineHeight = 20.sp), maxLines = 2)
        }
        if (illustration) {
            // Koordinat dari frame 390dp, dihitung dari tepi kanan agar tetap di kanan pada layar lain.
            Box(Modifier.align(Alignment.TopEnd).width(102.dp).height(120.dp)) {
                FigmaStar(x = 2.45.dp, y = 24.65.dp, size = 10.7.dp, rotation = 19.21f)
                FigmaStar(x = 4.4.dp + 0.9.dp, y = 65.8.dp + 0.9.dp, size = 7.15.dp, rotation = -18.39f)
                FigmaStar(x = 42.dp + 0.5.dp, y = 14.7.dp + 0.5.dp, size = 7.15.dp, rotation = 8.77f)
                FigmaStar(x = 2.7.dp + 0.5.dp, y = 51.3.dp + 0.5.dp, size = 3.56.dp, rotation = 19.89f)
                FigmaStar(x = 40.3.dp + 0.5.dp, y = 61.7.dp + 0.5.dp, size = 3.56.dp, rotation = 19.89f)
                FigmaStar(x = 36.dp + 0.5.dp, y = 25.dp + 0.5.dp, size = 3.56.dp, rotation = 19.89f)
                androidx.compose.foundation.Image(
                    painter = androidx.compose.ui.res.painterResource(R.drawable.figma_clock_wing),
                    contentDescription = null,
                    modifier = Modifier.offset(x = 16.4.dp, y = 23.25.dp).width(68.65.dp).height(57.3.dp),
                )
                FigmaStar(x = 49.dp + 3.75.dp, y = 12.dp + 3.75.dp, size = 20.5.dp, rotation = -30.21f, large = true)
            }
        }
    }
}

@Composable
fun FigmaSpacer(height: Dp) = Spacer(Modifier.height(height))
