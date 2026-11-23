package id.albahjah.hris.ui

import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.PickVisualMediaRequest
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.imePadding
import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Lock
import androidx.compose.material3.Icon
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.ImageBitmap
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.unit.dp
import id.albahjah.hris.data.Me
import id.albahjah.hris.data.MyFamily
import id.albahjah.hris.data.Repository
import id.albahjah.hris.data.toApiError
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import java.time.LocalDate

// ---------------------------------------------------------------------------------------------
// Data pribadi: foto profil, nomor WA, email

/** Nomor WA 62812… ditampilkan sebagai 0812… agar mudah dibaca. */
fun localPhone(phone: String?): String = when {
    phone.isNullOrBlank() -> ""
    phone.startsWith("62") -> "0" + phone.removePrefix("62")
    else -> phone
}

/** Isian kontak di layar Data pribadi; dipisah agar bisa dirender tanpa jaringan. */
data class ContactForm(
    val phone: String = "",
    val email: String = "",
    val password: String = "",
    val busy: Boolean = false,
    val photoBusy: Boolean = false,
    val errors: Map<String, String> = emptyMap(),
    val message: String? = null,
)

fun ContactForm.emailChanged(me: Me) = email.trim() != (me.email ?: "")
fun ContactForm.changed(me: Me) = emailChanged(me) || phone.trim() != localPhone(me.phone)

/** Data pribadi: foto dan kontak bisa diubah sendiri; data kepegawaian tetap diubah SDM. */
@Composable
fun PersonalDataContent(
    me: Me,
    onBack: () -> Unit,
    photo: ImageBitmap? = null,
    form: ContactForm = ContactForm(phone = localPhone(me.phone), email = me.email.orEmpty()),
    onPhone: (String) -> Unit = {},
    onEmail: (String) -> Unit = {},
    onPassword: (String) -> Unit = {},
    onSave: () -> Unit = {},
    onPickPhoto: () -> Unit = {},
    onRemovePhoto: () -> Unit = {},
    onCancelEmail: () -> Unit = {},
) {
    Column(Modifier.fillMaxSize().background(Fg.PageProfile)) {
        FigmaHeader("Data Pribadi", onBack)
        Column(
            Modifier.weight(1f).verticalScroll(rememberScrollState()).imePadding().padding(horizontal = 12.dp, vertical = 16.dp).navigationBarsPadding(),
            verticalArrangement = Arrangement.spacedBy(12.dp),
        ) {
            FigmaCard(title = "Foto profil", subtitle = "Tampil di aplikasi dan web SDM.", gap = 14.dp) {
                Box(Modifier.fillMaxWidth(), contentAlignment = Alignment.Center) {
                    Box(Modifier.clickable(enabled = !form.photoBusy, onClickLabel = "Ganti foto", onClick = onPickPhoto)) {
                        Avatar(me.fullName, size = 96.dp, photo = photo)
                    }
                }
                SecondaryButton(
                    when {
                        form.photoBusy -> "Mengunggah foto..."
                        me.hasPhoto -> "Ganti foto dari galeri"
                        else -> "Pilih foto dari galeri"
                    },
                    onClick = { if (!form.photoBusy) onPickPhoto() },
                )
                if (me.hasPhoto && !form.photoBusy) {
                    TextButton(onClick = onRemovePhoto, modifier = Modifier.fillMaxWidth()) {
                        Text("Hapus foto", style = figmaText(14, FontWeight.Medium, Fg.Danger))
                    }
                }
                form.errors["photo"]?.let { Text(it, style = figmaText(12, color = Fg.Danger, lineHeight = 1.35f)) }
            }

            FigmaCard(title = "Kontak", subtitle = "Nomor WA dan email juga bisa dipakai untuk masuk.", gap = 16.dp) {
                FigmaInput(
                    "Nomor WA", form.phone, onPhone, HrisIcons.Smartphone,
                    error = form.errors["phone"],
                    keyboard = KeyboardOptions(keyboardType = KeyboardType.Phone),
                )
                FigmaInput(
                    "Email", form.email, onEmail, FigmaIcons.Sms,
                    error = form.errors["email"],
                    keyboard = KeyboardOptions(keyboardType = KeyboardType.Email),
                )
                if (form.emailChanged(me)) {
                    FigmaInput(
                        "Kata sandi saat ini", form.password, onPassword, Icons.Filled.Lock,
                        password = true,
                        error = form.errors["current_password"],
                        keyboard = KeyboardOptions(keyboardType = KeyboardType.Password),
                    )
                    Text("Diperlukan karena email juga dipakai untuk masuk.", style = figmaText(12, color = Hc.InkSoft, lineHeight = 1.35f))
                }
                me.pendingEmail?.let { pending ->
                    // Email baru berlaku setelah tautan di email itu dibuka (D59).
                    Text(
                        "Menunggu konfirmasi ke $pending. Buka tautan di email itu dalam 24 jam; sampai saat itu email lama tetap dipakai.",
                        style = figmaText(13, FontWeight.Medium, Tone.Warning.content, lineHeight = 1.35f),
                    )
                    TextButton(onClick = onCancelEmail, enabled = !form.busy) {
                        Text("Batalkan ganti email", style = figmaText(14, FontWeight.Medium, Fg.Danger))
                    }
                }
                form.message?.let { Text(it, style = figmaText(13, FontWeight.Medium, Hc.BrandDeep)) }
                PrimaryButton("Simpan kontak", onClick = onSave, enabled = form.changed(me), loading = form.busy)
            }

            FigmaCard(title = "Data kepegawaian", subtitle = "Diubah oleh SDM bila ada yang tidak sesuai.", gap = 16.dp) {
                FigmaInput("Nama lengkap", me.fullName, {}, FigmaIcons.User, readOnly = true)
                FigmaInput("NIPY", me.nip, {}, FigmaIcons.Keyboard, readOnly = true)
                FigmaInput("Jabatan", me.position ?: "-", {}, FigmaIcons.Keyboard, readOnly = true)
                FigmaInput("Divisi", me.division.name, {}, FigmaIcons.LocationTickLinear, readOnly = true)
                FigmaInput("Subdivisi", me.subdivision.name, {}, FigmaIcons.LocationTickLinear, readOnly = true)
            }
        }
    }
}

/** Layar Data pribadi dengan pemanggilan API; [onUpdated] dipanggil dengan profil terbaru. */
@Composable
fun PersonalDataScreen(repository: Repository, me: Me, photo: ImageBitmap?, onBack: () -> Unit, onUpdated: (Me) -> Unit) {
    val scope = rememberCoroutineScope()
    val context = LocalContext.current
    var form by remember(me.employeeId) { mutableStateOf(ContactForm(phone = localPhone(me.phone), email = me.email.orEmpty())) }
    val picker = rememberLauncherForActivityResult(ActivityResultContracts.PickVisualMedia()) { uri ->
        if (uri == null) return@rememberLauncherForActivityResult
        form = form.copy(photoBusy = true, errors = emptyMap(), message = null)
        scope.launch {
            form = try {
                val jpeg = withContext(Dispatchers.Default) { prepareProfilePhoto(context, uri) }
                onUpdated(repository.setPhoto(jpeg))
                form.copy(photoBusy = false, message = "Foto profil diperbarui.")
            } catch (e: CancellationException) {
                throw e
            } catch (e: Exception) {
                val err = e.toApiError()
                form.copy(photoBusy = false, errors = mapOf("photo" to (err.fields["photo"] ?: err.message ?: "Foto belum bisa diunggah.")))
            }
        }
    }
    SecureScreen()
    PersonalDataContent(
        me = me,
        onBack = onBack,
        photo = photo,
        form = form,
        onPhone = { form = form.copy(phone = it, errors = form.errors - "phone", message = null) },
        onEmail = { form = form.copy(email = it, errors = form.errors - "email", message = null) },
        onPassword = { form = form.copy(password = it, errors = form.errors - "current_password") },
        onPickPhoto = { picker.launch(PickVisualMediaRequest(ActivityResultContracts.PickVisualMedia.ImageOnly)) },
        onRemovePhoto = {
            form = form.copy(photoBusy = true, errors = emptyMap(), message = null)
            scope.launch {
                form = try {
                    repository.deletePhoto()
                    onUpdated(me.copy(hasPhoto = false, photoUpdatedAt = null))
                    form.copy(photoBusy = false, message = "Foto profil dihapus.")
                } catch (e: CancellationException) {
                    throw e
                } catch (e: Exception) {
                    form.copy(photoBusy = false, errors = mapOf("photo" to (e.toApiError().message ?: "Foto belum bisa dihapus.")))
                }
            }
        },
        onCancelEmail = {
            form = form.copy(busy = true, errors = emptyMap(), message = null)
            scope.launch {
                form = try {
                    repository.cancelEmailChange()
                    onUpdated(me.copy(pendingEmail = null))
                    form.copy(busy = false, message = "Ganti email dibatalkan.")
                } catch (e: CancellationException) {
                    throw e
                } catch (e: Exception) {
                    form.copy(busy = false, errors = mapOf("email" to (e.toApiError().message ?: "Belum bisa dibatalkan.")))
                }
            }
        },
        onSave = save@{
            if (form.busy) return@save
            val emailChanged = form.emailChanged(me)
            if (emailChanged && form.password.isEmpty()) {
                form = form.copy(errors = mapOf("current_password" to "Masukkan kata sandi saat ini untuk mengganti email."))
                return@save
            }
            form = form.copy(busy = true, errors = emptyMap(), message = null)
            scope.launch {
                form = try {
                    val updated = repository.updateContact(form.phone.trim(), form.email.trim(), form.password.takeIf { emailChanged })
                    onUpdated(updated)
                    val message = if (emailChanged && updated.pendingEmail != null && updated.pendingEmail != me.pendingEmail) {
                        "Tautan konfirmasi dikirim ke ${updated.pendingEmail}. Email baru berlaku setelah tautan dibuka."
                    } else {
                        "Kontak diperbarui."
                    }
                    ContactForm(phone = localPhone(updated.phone), email = updated.email.orEmpty(), message = message)
                } catch (e: CancellationException) {
                    throw e
                } catch (e: Exception) {
                    val err = e.toApiError()
                    form.copy(busy = false, errors = err.fields.ifEmpty { mapOf("phone" to (err.message ?: "Kontak belum tersimpan.")) })
                }
            }
        },
    )
}

// ---------------------------------------------------------------------------------------------
// Data anak

val ChildGenders = listOf("L" to "Laki-laki", "P" to "Perempuan")
val ChildRelations = listOf("kandung" to "Kandung", "angkat" to "Angkat", "tiri" to "Tiri")
val ChildEducations = listOf(
    "belum_sekolah" to "Belum sekolah",
    "paud" to "PAUD",
    "tk_ra" to "TK/RA",
    "sd_mi" to "SD/MI",
    "smp_mts" to "SMP/MTs",
    "sma_sederajat" to "SMA sederajat",
    "perguruan_tinggi" to "Perguruan tinggi",
    "pondok_pesantren" to "Pondok pesantren",
)

/** Data satu anak untuk ditampilkan; urutan mengikuti anak ke-. */
data class ChildDraft(
    val name: String = "",
    val gender: String? = null,
    val birthPlace: String = "",
    val birthDate: String? = null,
    val relation: String? = null,
    val education: String? = null,
    val school: String = "",
)

/** Isi layar Data anak. Data anak hanya diubah SDM (D56), jadi layar ini hanya menampilkan. */
data class FamilyForm(
    val hasChildren: Boolean? = null,
    val count: String = "",
    val children: List<ChildDraft> = emptyList(),
    val loading: Boolean = false,
    val error: String? = null,
) {
    companion object
}

fun FamilyForm.Companion.from(f: MyFamily) = FamilyForm(
    hasChildren = f.hasChildren,
    count = f.childrenCount?.toString().orEmpty(),
    children = f.children.sortedBy { it.childOrder }.map {
        ChildDraft(it.fullName, it.gender, it.birthPlace.orEmpty(), it.birthDate, it.relation, it.education, it.schoolName.orEmpty())
    },
)

private fun List<Pair<String, String>>.labelOf(value: String?): String = firstOrNull { it.first == value }?.second ?: "-"

@Composable
private fun InfoRow(label: String, value: String) {
    Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.spacedBy(12.dp)) {
        Text(label, style = figmaText(14, color = Hc.InkSoft), modifier = Modifier.weight(0.45f))
        Text(value.ifBlank { "-" }, style = figmaText(14, FontWeight.Medium, Hc.Ink, lineHeight = 1.35f), modifier = Modifier.weight(0.55f))
    }
}

/** Data anak milik sendiri: hanya terlihat oleh Pejuang dan SDM, dan hanya SDM yang mengubahnya. */
@Composable
fun ChildrenContent(form: FamilyForm, onBack: () -> Unit) {
    Column(Modifier.fillMaxSize().background(Fg.PageProfile)) {
        FigmaHeader("Data Anak", onBack)
        if (form.loading) {
            CenterMessage(title = "Memuat data anak", body = null, loading = true)
            return
        }
        Column(
            Modifier.weight(1f).verticalScroll(rememberScrollState()).padding(horizontal = 12.dp, vertical = 16.dp).navigationBarsPadding(),
            verticalArrangement = Arrangement.spacedBy(12.dp),
        ) {
            Row(
                Modifier.fillMaxWidth().clip(RoundedCornerShape(14.dp)).background(Hc.BrandSoft).padding(14.dp),
                horizontalArrangement = Arrangement.spacedBy(10.dp),
            ) {
                Icon(Icons.Filled.Lock, contentDescription = null, tint = Hc.BrandDeep, modifier = Modifier.size(18.dp))
                Text(
                    "Data anak dikelola SDM. Bila ada yang perlu diubah atau ditambah, silakan hubungi SDM.",
                    style = figmaText(13, color = Hc.BrandDeep, lineHeight = 1.4f),
                )
            }
            form.error?.let { Text(it, style = figmaText(13, color = Fg.Danger, lineHeight = 1.4f)) }
            FigmaCard(title = "Keluarga", subtitle = "Hanya bisa dilihat oleh Anda dan SDM.", gap = 12.dp) {
                InfoRow("Memiliki anak", when (form.hasChildren) { true -> "Ya"; false -> "Tidak"; null -> "-" })
                InfoRow("Jumlah anak", form.count)
            }
            form.children.forEachIndexed { i, child ->
                FigmaCard(title = "Anak ke-${i + 1}", gap = 12.dp) {
                    InfoRow("Nama lengkap", child.name)
                    InfoRow("Jenis kelamin", ChildGenders.labelOf(child.gender))
                    InfoRow("Tempat lahir", child.birthPlace)
                    InfoRow("Tanggal lahir", child.birthDate?.let { runCatching { dateLabel(LocalDate.parse(it)) }.getOrNull() } ?: "-")
                    InfoRow("Status anak", ChildRelations.labelOf(child.relation))
                    InfoRow("Jenjang pendidikan", ChildEducations.labelOf(child.education))
                    InfoRow("Nama sekolah", child.school)
                }
            }
            if (form.children.isEmpty() && form.error == null) {
                Text("Belum ada data anak yang tercatat.", style = figmaText(14, color = Hc.InkMuted))
            }
        }
    }
}

/** Layar Data anak dengan pemanggilan API (hanya baca). */
@Composable
fun ChildrenScreen(repository: Repository, onBack: () -> Unit) {
    var form by remember { mutableStateOf(FamilyForm(loading = true)) }
    LaunchedEffect(Unit) {
        form = try {
            FamilyForm.from(repository.myFamily())
        } catch (e: CancellationException) {
            throw e
        } catch (e: Exception) {
            FamilyForm(error = e.toApiError().message ?: "Data anak belum bisa dimuat.")
        }
    }
    SecureScreen()
    ChildrenContent(form = form, onBack = onBack)
}
