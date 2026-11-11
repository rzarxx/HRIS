package id.pejuang.hris.ui

import android.app.Activity
import android.view.WindowManager
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.imePadding
import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.statusBarsPadding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Lock
import androidx.compose.material3.Icon
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.platform.LocalView
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.ImeAction
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import id.pejuang.hris.data.Me

/** Melarang tangkapan layar selama layar berisi data pribadi tampil (MASTER.md §10.2). */
@Composable
fun SecureScreen() {
    val view = LocalView.current
    DisposableEffect(view) {
        val window = (view.context as? Activity)?.window
        window?.addFlags(WindowManager.LayoutParams.FLAG_SECURE)
        onDispose { window?.clearFlags(WindowManager.LayoutParams.FLAG_SECURE) }
    }
}

/** Tab Profil: avatar, nama, lalu kelompok Kontak, Akun, dan Pengaturan dalam kartu lembut. */
@Suppress("UNUSED_PARAMETER")
@Composable
fun ProfileContent(
    me: Me,
    version: String,
    onBack: () -> Unit,
    onPersonalData: () -> Unit,
    onChangePassword: () -> Unit,
    onCheckUpdate: (() -> Unit)?,
    onLogout: () -> Unit,
    modifier: Modifier = Modifier,
) {
    StatusBarIcons(dark = true)
    val verified = me.deviceStatus == "approved" && me.faceEnrolled
    Column(
        modifier.fillMaxSize().background(Hc.Canvas).verticalScroll(rememberScrollState()).statusBarsPadding()
            .padding(start = 16.dp, end = 16.dp, top = 8.dp, bottom = bottomContentSpace(20.dp)),
        horizontalAlignment = Alignment.CenterHorizontally,
    ) {
        Column(ContentWidth.fillMaxWidth(), verticalArrangement = Arrangement.spacedBy(20.dp)) {
            PageHeader("Profil")
            Column(Modifier.fillMaxWidth(), horizontalAlignment = Alignment.CenterHorizontally) {
                Avatar(me.fullName, size = 88.dp)
                Spacer(Modifier.height(14.dp))
                Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(6.dp), modifier = Modifier.padding(horizontal = 24.dp)) {
                    Text(
                        me.fullName,
                        style = figmaText(20, FontWeight.Bold, tracking = -0.2f),
                        textAlign = TextAlign.Center,
                        maxLines = 1,
                        overflow = TextOverflow.Ellipsis,
                        modifier = Modifier.weight(1f, fill = false),
                    )
                    if (verified) Icon(FigmaIcons.Verified, contentDescription = "Terverifikasi", tint = Hc.Brand, modifier = Modifier.size(20.dp))
                }
                Spacer(Modifier.height(4.dp))
                Text(
                    listOfNotNull(me.position, me.subdivision.name).joinToString(" · ").ifBlank { "Pejuang Yayasan" },
                    style = figmaText(14, color = Hc.InkSoft),
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis,
                )
            }
            ProfileGroup("Kontak") {
                ProfileRow(FigmaIcons.Sms, me.email.ifBlank { "Email belum tercatat" })
                RowDivider()
                ProfileRow(FigmaIcons.LocationTick, "${me.subdivision.name} · ${me.division.name}")
            }
            ProfileGroup("Akun") {
                ProfileRow(FigmaIcons.UserBold, "Data pribadi", onClick = onPersonalData)
                RowDivider()
                ProfileRow(HrisIcons.Smartphone, "Perangkat ini", trailing = deviceText(me.deviceStatus))
                RowDivider()
                ProfileRow(HrisIcons.FaceScan, "Data wajah", trailing = if (me.faceEnrolled) "Terdaftar" else "Belum terdaftar")
            }
            ProfileGroup("Pengaturan") {
                ProfileRow(FigmaIcons.Setting, "Ganti kata sandi", onClick = onChangePassword, trailing = if (me.mustChangePassword) "Wajib" else null)
                RowDivider()
                ProfileRow(FigmaIcons.Scroll, "Versi aplikasi", onClick = onCheckUpdate, trailing = "v$version")
                RowDivider()
                ProfileRow(FigmaIcons.Logout, "Keluar", onClick = onLogout, danger = true)
            }
        }
    }
}

private fun deviceText(status: String?) = when (status) {
    "approved" -> "Disetujui"
    "pending" -> "Menunggu SDM"
    else -> "Dicabut"
}

@Composable
private fun ProfileGroup(label: String, content: @Composable () -> Unit) {
    Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
        Text(label, style = figmaText(14, FontWeight.SemiBold, Hc.InkSoft), modifier = Modifier.padding(start = 4.dp))
        SectionCard(padding = 4.dp) { content() }
    }
}

@Composable
private fun ProfileRow(
    icon: ImageVector,
    text: String,
    onClick: (() -> Unit)? = null,
    trailing: String? = null,
    danger: Boolean = false,
) {
    Row(
        Modifier
            .fillMaxWidth()
            .clip(RoundedCornerShape(16.dp))
            .then(if (onClick != null) Modifier.clickable(role = Role.Button, onClick = onClick) else Modifier)
            .heightIn(min = 56.dp)
            .padding(horizontal = 16.dp, vertical = 10.dp),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(14.dp),
    ) {
        SoftIconTile(icon, 40.dp, if (danger) Tone.Danger else null)
        Text(
            text,
            style = figmaText(15, FontWeight.Medium, if (danger) Fg.Danger else Hc.Ink),
            modifier = Modifier.weight(1f),
            maxLines = 1,
            overflow = TextOverflow.Ellipsis,
        )
        if (trailing != null) Text(trailing, style = figmaText(13, color = Hc.InkSoft), maxLines = 1)
        if (onClick != null && !danger) Icon(FigmaIcons.ArrowRight, contentDescription = null, tint = Fg.Chevron, modifier = Modifier.size(16.dp))
    }
}


/** Data pribadi, mengikuti frame "MyProfile/PersonalData". Hanya-baca; perubahan lewat SDM. */
@Composable
fun PersonalDataContent(me: Me, onBack: () -> Unit) {
    Column(Modifier.fillMaxSize().background(Fg.PageProfile)) {
        FigmaHeader("Data Pribadi", onBack)
        Column(
            Modifier.weight(1f).verticalScroll(rememberScrollState()).padding(horizontal = 12.dp, vertical = 16.dp).navigationBarsPadding(),
        ) {
            FigmaCard(title = "Data pribadi saya", subtitle = "Diubah oleh SDM bila ada yang tidak sesuai", top = 24.dp, bottom = 24.dp, gap = 16.dp) {
                Box(Modifier.fillMaxWidth(), contentAlignment = Alignment.Center) {
                    Box(
                        Modifier.size(88.dp).clip(CircleShape).background(Hc.BrandSoft),
                        contentAlignment = Alignment.Center,
                    ) { Text(initials(me.fullName), style = figmaText(34, FontWeight.SemiBold, Hc.BrandDeep)) }
                }
                FigmaInput("Nama lengkap", me.fullName, {}, FigmaIcons.User, readOnly = true)
                FigmaInput("NIPY", me.nip, {}, FigmaIcons.Keyboard, readOnly = true)
                FigmaInput("Email", me.email.ifBlank { "-" }, {}, FigmaIcons.Sms, readOnly = true)
                FigmaInput("Jabatan", me.position ?: "-", {}, FigmaIcons.Keyboard, readOnly = true)
                FigmaInput("Divisi", me.division.name, {}, FigmaIcons.LocationTickLinear, readOnly = true)
                FigmaInput("Subdivisi", me.subdivision.name, {}, FigmaIcons.LocationTickLinear, readOnly = true)
            }
        }
    }
}

/** Isian layar ganti kata sandi; dipisah agar bisa dirender tanpa jaringan. */
data class PasswordForm(
    val current: String = "",
    val next: String = "",
    val repeat: String = "",
    val busy: Boolean = false,
    val error: String? = null,
    val done: Boolean = false,
)

/** Pesan isian sebelum dikirim ke server (aturan server: minimal 10 karakter). */
fun passwordProblem(form: PasswordForm): String? = when {
    form.next.isNotEmpty() && form.next.length < 10 -> "Minimal 10 karakter."
    form.repeat.isNotEmpty() && form.repeat != form.next -> "Kata sandi tidak sama."
    else -> null
}

@Composable
fun ChangePasswordContent(
    form: PasswordForm,
    onCurrent: (String) -> Unit,
    onNext: (String) -> Unit,
    onRepeat: (String) -> Unit,
    onSubmit: () -> Unit,
    onBack: () -> Unit,
    onFinished: () -> Unit,
) {
    val canSubmit = form.current.isNotEmpty() && form.next.length >= 10 && form.repeat == form.next && !form.busy
    Box(Modifier.fillMaxSize().background(Fg.PageProfile)) {
        Column(Modifier.fillMaxSize().imePadding()) {
            FigmaHeader("Ganti Kata Sandi", onBack)
            Column(Modifier.weight(1f).verticalScroll(rememberScrollState()).padding(horizontal = 12.dp, vertical = 16.dp)) {
                FigmaCard(
                    title = "Kata sandi baru",
                    subtitle = "Minimal 10 karakter. Setelah diganti, sesi di perangkat lain akan keluar.",
                    top = 24.dp,
                    bottom = 24.dp,
                    gap = 16.dp,
                ) {
                    val lock = Icons.Filled.Lock
                    FigmaInput(
                        "Kata sandi saat ini", form.current, onCurrent, lock, password = true,
                        keyboard = KeyboardOptions(keyboardType = KeyboardType.Password, imeAction = ImeAction.Next),
                    )
                    FigmaInput(
                        "Kata sandi baru", form.next, onNext, lock, password = true,
                        error = "Minimal 10 karakter.".takeIf { form.next.isNotEmpty() && form.next.length < 10 },
                        keyboard = KeyboardOptions(keyboardType = KeyboardType.Password, imeAction = ImeAction.Next),
                    )
                    FigmaInput(
                        "Ulangi kata sandi baru", form.repeat, onRepeat, lock, password = true,
                        error = "Kata sandi tidak sama.".takeIf { form.repeat.isNotEmpty() && form.repeat != form.next },
                        keyboard = KeyboardOptions(keyboardType = KeyboardType.Password, imeAction = ImeAction.Done),
                        onDone = { if (canSubmit) onSubmit() },
                    )
                    if (form.error != null) Text(form.error, style = figmaText(13, FontWeight.Medium, Fg.Danger, lineHeight = 1.3f))
                }
            }
            // Komponen "Bottom Button": bilah putih 76dp berisi tombol utama.
            Box(Modifier.fillMaxWidth().background(Fg.Card).navigationBarsPadding().padding(horizontal = 16.dp, vertical = 14.dp)) {
                FigmaPrimaryButton("Simpan kata sandi", onSubmit, enabled = canSubmit, loading = form.busy)
            }
        }
        if (form.done) {
            FigmaPopup(
                icon = FigmaIcons.Setting,
                title = "Kata sandi diganti",
                body = "Gunakan kata sandi baru saat masuk berikutnya. Sesi di perangkat lain sudah dikeluarkan.",
                primary = "Selesai",
                onPrimary = onFinished,
                onDismiss = onFinished,
            )
        }
    }
}

