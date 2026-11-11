package id.pejuang.hris.ui

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
import androidx.compose.foundation.layout.RowScope
import androidx.compose.foundation.layout.Spacer
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
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.SolidColor
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.KeyboardCapitalization
import androidx.compose.ui.text.input.PasswordVisualTransformation
import androidx.compose.ui.text.input.VisualTransformation
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.TextUnit
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp

/**
 * Nama warna lama dari kit Figma, kini menunjuk ke token Soft UI di [Hc] agar aplikasi hanya punya
 * satu palet. Pakai [Hc] untuk kode baru.
 */
object Fg {
    val Accent = Hc.Brand
    val OnHeaderSoft = Hc.BrandSoftStrong
    val FieldIcon = Hc.InkMuted

    val Page = Hc.Canvas
    val PageProfile = Hc.Canvas
    val Card = Hc.Surface
    val Gray50 = Hc.Line
    val Gray100 = Color(0xFFF9FAFB)
    val Gray200 = Hc.Line
    val Gray300 = Color(0xFFD0D5DD)
    val Gray400 = Hc.InkMuted
    val Gray500 = Color(0xFF667085)
    val TextPrimary = Hc.Ink
    val TextSecondary = Hc.InkSoft
    val TextBody = Color(0xFF344054)
    val TextStrong = Hc.Ink
    val RowText = Color(0xFF344054)
    val Chevron = Color(0xFFD0D5DD)
    /** Teks galat di bawah isian (5,7:1 di atas putih). */
    val Danger = Color(0xFFB42318)
    val DangerText = Color(0xFFB42318)
    val Backdrop = Color(0x660C111D)
}

/** Gaya teks dengan Plus Jakarta Sans. [lineHeight] adalah kelipatan ukuran huruf. */
fun figmaText(size: Int, weight: FontWeight = FontWeight.Normal, color: Color = Hc.Ink, lineHeight: Float? = null, tracking: Float = 0f) =
    TextStyle(
        fontFamily = Jakarta,
        fontSize = size.sp,
        fontWeight = weight,
        color = color,
        lineHeight = lineHeight?.let { (size * it).sp } ?: TextUnit.Unspecified,
        letterSpacing = tracking.sp,
    )

/**
 * Tombol utama: datar, hijau terang dengan teks gelap (6,5:1), sudut 14dp, tinggi 52dp. Varian
 * [danger] merah dengan teks putih untuk menolak, membatalkan, atau keluar.
 */
@Composable
fun FigmaPrimaryButton(
    text: String,
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
    enabled: Boolean = true,
    loading: Boolean = false,
    danger: Boolean = false,
) {
    val active = enabled && !loading
    val fill = when {
        !enabled && !loading -> Hc.Field
        danger -> Hc.Danger
        else -> Hc.Primary
    }
    val ink = when {
        !enabled && !loading -> Hc.InkMuted
        danger -> Color.White
        else -> Hc.OnPrimary
    }
    Box(
        modifier
            .fillMaxWidth()
            .heightIn(min = 52.dp)
            .clip(ControlShape)
            .background(fill)
            .clickable(enabled = active, role = Role.Button, onClick = onClick)
            .padding(horizontal = 20.dp, vertical = 14.dp),
        contentAlignment = Alignment.Center,
    ) {
        if (loading) {
            CircularProgressIndicator(color = ink, strokeWidth = 2.dp, modifier = Modifier.size(20.dp))
        } else {
            Text(text, style = figmaText(15, FontWeight.SemiBold, ink).copy(lineHeight = 20.sp), maxLines = 1, overflow = TextOverflow.Ellipsis)
        }
    }
}

/**
 * Tombol kedua: tonal lembut tanpa garis. Mint untuk aksi hijau (bawaan), merah muda bila [color]
 * adalah [Fg.DangerText] (mis. "Tolak"), abu untuk warna lain (mis. "Batal").
 */
@Composable
fun FigmaSecondaryButton(text: String, onClick: () -> Unit, modifier: Modifier = Modifier, color: Color = Hc.BrandDeep) {
    val fill = when (color) {
        Fg.DangerText -> Tone.Danger.container
        Hc.BrandDeep -> Hc.BrandSoft
        else -> Hc.Field
    }
    Box(
        modifier
            .fillMaxWidth()
            .heightIn(min = 52.dp)
            .clip(ControlShape)
            .background(fill)
            .clickable(role = Role.Button, onClick = onClick)
            .padding(horizontal = 20.dp, vertical = 14.dp),
        contentAlignment = Alignment.Center,
    ) {
        Text(text, style = figmaText(15, FontWeight.SemiBold, color).copy(lineHeight = 20.sp), maxLines = 1, overflow = TextOverflow.Ellipsis)
    }
}

/** Tombol bundar 44dp berisi ikon 20dp, untuk kembali atau aksi di header. */
@Composable
fun FigmaCircleButton(icon: ImageVector, description: String, background: Color, tint: Color, onClick: () -> Unit) {
    Box(
        Modifier.size(44.dp).softShadow(CircleShape, 6.dp).clip(CircleShape).background(background)
            .clickable(role = Role.Button, onClick = onClick)
            .semantics { contentDescription = description },
        contentAlignment = Alignment.Center,
    ) {
        Icon(icon, contentDescription = null, tint = tint, modifier = Modifier.size(20.dp))
    }
}

/** Header halaman turunan: tombol kembali bulat putih dan judul di tengah, di atas latar halaman. */
@Composable
fun FigmaHeader(title: String, onBack: () -> Unit) {
    StatusBarIcons(dark = true)
    Box(
        Modifier.fillMaxWidth().background(Hc.Canvas).statusBarsPadding()
            .padding(horizontal = 16.dp).padding(top = 8.dp, bottom = 8.dp).heightIn(min = 48.dp),
    ) {
        Box(Modifier.align(Alignment.CenterStart)) {
            FigmaCircleButton(FigmaIcons.ArrowLeftOutline, "Kembali", Hc.Surface, Hc.Ink, onBack)
        }
        Text(
            title,
            style = figmaText(17, FontWeight.SemiBold),
            modifier = Modifier.align(Alignment.Center).padding(horizontal = 56.dp).semantics { heading() },
            maxLines = 1,
            overflow = TextOverflow.Ellipsis,
        )
    }
}

/**
 * Judul halaman tab: teks besar di atas latar halaman, tanpa blok warna (Clean Minimalist).
 * Dipakai sebagai item pertama daftar agar ikut tergulir.
 */
@Composable
fun PageHeader(title: String, subtitle: String? = null, modifier: Modifier = Modifier, trailing: (@Composable RowScope.() -> Unit)? = null) {
    StatusBarIcons(dark = true)
    Row(
        modifier.fillMaxWidth().padding(start = 4.dp, end = 4.dp, top = 20.dp, bottom = 8.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Column(Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(4.dp)) {
            Text(title, style = figmaText(26, FontWeight.Bold, tracking = -0.3f).copy(lineHeight = 32.sp), modifier = Modifier.semantics { heading() })
            if (subtitle != null) Text(subtitle, style = figmaText(14, color = Hc.InkSoft, lineHeight = 1.4f))
        }
        trailing?.invoke(this)
    }
}

/** Kartu putih lembut: sudut 20dp, bayangan tipis, tanpa garis. [onClick] membuat seluruh kartu bisa diketuk. */
@Composable
fun FigmaCard(
    modifier: Modifier = Modifier,
    title: String? = null,
    subtitle: String? = null,
    top: Dp = 20.dp,
    bottom: Dp = 20.dp,
    gap: Dp = 14.dp,
    onClick: (() -> Unit)? = null,
    onClickLabel: String? = null,
    content: @Composable ColumnScope.() -> Unit,
) {
    Column(
        modifier
            .fillMaxWidth()
            .softShadow()
            .clip(CardShape)
            .background(Hc.Surface)
            .then(if (onClick != null) Modifier.clickable(onClickLabel = onClickLabel, onClick = onClick) else Modifier)
            .padding(start = 20.dp, end = 20.dp, top = top, bottom = bottom),
        verticalArrangement = Arrangement.spacedBy(gap),
    ) {
        if (title != null) {
            Column(verticalArrangement = Arrangement.spacedBy(4.dp)) {
                Text(title, style = figmaText(16, FontWeight.SemiBold, lineHeight = 1.35f))
                if (subtitle != null) Text(subtitle, style = figmaText(13, color = Hc.InkSoft, lineHeight = 1.45f))
            }
        }
        content()
    }
}

/** Bingkai isian terisi: abu lembut tanpa garis, cincin hijau saat fokus, cincin merah saat galat. */
private fun Modifier.fieldFrame(focused: Boolean, error: Boolean, readOnly: Boolean = false): Modifier {
    val ring = when {
        error -> Fg.Danger
        focused && !readOnly -> Hc.Brand
        else -> null
    }
    return clip(ControlShape)
        .background(if (focused && !readOnly) Hc.Surface else Hc.Field)
        .then(if (ring != null) Modifier.border(if (error) 1.5.dp else 2.dp, ring, ControlShape) else Modifier)
}

@Composable
private fun FieldLabel(label: String, required: Boolean = false) {
    Text(if (required) "$label *" else label, style = figmaText(13, FontWeight.Medium, Hc.InkSoft).copy(lineHeight = 18.sp))
}

/** Isian satu baris dengan ikon. */
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
    Column(modifier.fillMaxWidth(), verticalArrangement = Arrangement.spacedBy(6.dp)) {
        FieldLabel(label)
        BasicTextField(
            value = value,
            onValueChange = onChange,
            readOnly = readOnly,
            singleLine = true,
            textStyle = figmaText(15, color = if (readOnly) Hc.InkSoft else Hc.Ink).copy(lineHeight = 20.sp),
            visualTransformation = if (password) PasswordVisualTransformation() else VisualTransformation.None,
            keyboardOptions = keyboard,
            keyboardActions = KeyboardActions(onDone = { onDone?.invoke() }),
            interactionSource = interaction,
            cursorBrush = SolidColor(Hc.Brand),
            decorationBox = { inner ->
                Row(
                    Modifier.fillMaxWidth().heightIn(min = 52.dp).fieldFrame(focused, error != null, readOnly).padding(horizontal = 14.dp),
                    verticalAlignment = Alignment.CenterVertically,
                    horizontalArrangement = Arrangement.spacedBy(10.dp),
                ) {
                    Icon(icon, contentDescription = null, tint = if (focused) Hc.Brand else Hc.InkMuted, modifier = Modifier.size(20.dp))
                    Box(Modifier.weight(1f)) { inner() }
                }
            },
        )
        if (error != null) Text(error, style = figmaText(12, color = Fg.Danger, lineHeight = 1.35f))
    }
}

/** Isian teks banyak baris dengan penghitung karakter dan contoh isian. */
@Composable
fun FigmaTextArea(
    label: String,
    value: String,
    onChange: (String) -> Unit,
    modifier: Modifier = Modifier,
    placeholder: String = "",
    error: String? = null,
    hint: String? = null,
    maxLength: Int = 500,
    required: Boolean = false,
) {
    val interaction = remember { MutableInteractionSource() }
    val focused by interaction.collectIsFocusedAsState()
    Column(modifier.fillMaxWidth(), verticalArrangement = Arrangement.spacedBy(6.dp)) {
        FieldLabel(label, required)
        BasicTextField(
            value = value,
            onValueChange = { onChange(it.take(maxLength)) },
            minLines = 3,
            maxLines = 6,
            textStyle = figmaText(15, color = Hc.Ink, lineHeight = 1.45f),
            keyboardOptions = KeyboardOptions(capitalization = KeyboardCapitalization.Sentences),
            interactionSource = interaction,
            cursorBrush = SolidColor(Hc.Brand),
            decorationBox = { inner ->
                Box(Modifier.fillMaxWidth().heightIn(min = 96.dp).fieldFrame(focused, error != null).padding(14.dp)) {
                    if (value.isEmpty() && placeholder.isNotEmpty()) {
                        Text(placeholder, style = figmaText(15, color = Fg.Gray500, lineHeight = 1.45f))
                    }
                    inner()
                }
            },
        )
        Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.spacedBy(8.dp)) {
            Text(
                (error ?: hint).orEmpty(),
                style = figmaText(12, color = if (error != null) Fg.Danger else Hc.InkSoft, lineHeight = 1.35f),
                modifier = Modifier.weight(1f),
            )
            Text("${value.length}/$maxLength", style = figmaText(12, color = Fg.Gray500))
        }
    }
}

/** Kolom pilihan yang membuka dialog (mis. pemilih tanggal), tampil seperti [FigmaInput]. */
@Composable
fun FigmaPickerField(
    label: String,
    value: String?,
    placeholder: String,
    icon: ImageVector,
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
    error: String? = null,
    required: Boolean = false,
) {
    Column(modifier.fillMaxWidth(), verticalArrangement = Arrangement.spacedBy(6.dp)) {
        FieldLabel(label, required)
        Row(
            Modifier
                .fillMaxWidth()
                .heightIn(min = 52.dp)
                .fieldFrame(focused = false, error = error != null)
                .clickable(role = Role.Button, onClickLabel = label, onClick = onClick)
                .padding(horizontal = 14.dp),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(10.dp),
        ) {
            Icon(icon, contentDescription = null, tint = if (value != null) Hc.Brand else Hc.InkMuted, modifier = Modifier.size(20.dp))
            Text(
                value ?: placeholder,
                style = figmaText(15, if (value != null) FontWeight.Medium else FontWeight.Normal, if (value != null) Hc.Ink else Fg.Gray500),
                modifier = Modifier.weight(1f),
                maxLines = 1,
                overflow = TextOverflow.Ellipsis,
            )
            Icon(FigmaIcons.ArrowRight, contentDescription = null, tint = Fg.Chevron, modifier = Modifier.size(16.dp))
        }
        if (error != null) Text(error, style = figmaText(12, color = Fg.Danger, lineHeight = 1.35f))
    }
}

/** Ubin ikon bulat lembut (mint, atau merah muda untuk peringatan). */
@Composable
fun SoftIconTile(icon: ImageVector, size: Dp = 48.dp, tone: Tone? = null) {
    Box(
        Modifier.size(size).clip(CircleShape).background(tone?.container ?: Hc.BrandSoft),
        contentAlignment = Alignment.Center,
    ) {
        Icon(icon, contentDescription = null, tint = tone?.content ?: Hc.BrandDeep, modifier = Modifier.size(size * 0.46f))
    }
}

/** Lembar konfirmasi dari bawah: latar redup, ubin ikon, judul, isi, dan tombol. */
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
    Column(
        Modifier
            .align(Alignment.BottomCenter)
            .fillMaxWidth()
            .clip(RoundedCornerShape(topStart = 28.dp, topEnd = 28.dp))
            .background(Hc.Surface)
            .clickable(interactionSource = remember { MutableInteractionSource() }, indication = null) {}
            .navigationBarsPadding()
            .padding(start = 24.dp, end = 24.dp, top = 12.dp, bottom = 24.dp),
        horizontalAlignment = Alignment.CenterHorizontally,
        verticalArrangement = Arrangement.spacedBy(16.dp),
    ) {
        Box(Modifier.width(36.dp).height(4.dp).clip(RoundedCornerShape(2.dp)).background(Fg.Gray300))
        SoftIconTile(icon, 56.dp, if (danger) Tone.Danger else null)
        Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
            Text(title, style = figmaText(20, FontWeight.Bold, lineHeight = 1.3f), textAlign = TextAlign.Center, modifier = Modifier.fillMaxWidth())
            Text(body, style = figmaText(14, color = Hc.InkSoft, lineHeight = 1.5f), textAlign = TextAlign.Center, modifier = Modifier.fillMaxWidth())
        }
        Column(Modifier.padding(top = 4.dp), verticalArrangement = Arrangement.spacedBy(10.dp)) {
            FigmaPrimaryButton(primary, onPrimary, danger = danger)
            if (secondary != null) FigmaSecondaryButton(secondary, onSecondary, color = Hc.Ink)
        }
    }
}

/** Menu navigasi bawah, sesuai lima layar mobile di MASTER.md §10.2. */
enum class MainTab(val label: String, val icon: ImageVector, val selectedIcon: ImageVector) {
    Home("Beranda", FigmaIcons.NavHome, FigmaIcons.NavHomeBold),
    Attendance("Presensi", FigmaIcons.NavCalendar, FigmaIcons.NavCalendarBold),
    Requests("Pengajuan", FigmaIcons.NavNote, FigmaIcons.NavNoteBold),
    Payslip("Slip ubar", FigmaIcons.NavReceipt, FigmaIcons.NavReceiptBold),
    Profile("Profil", FigmaIcons.User, FigmaIcons.UserBold),
}

@Composable
fun FigmaSpacer(height: Dp) = Spacer(Modifier.height(height))
