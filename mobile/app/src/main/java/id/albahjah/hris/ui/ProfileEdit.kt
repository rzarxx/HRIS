package id.albahjah.hris.ui

import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.PickVisualMediaRequest
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.imePadding
import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.selection.selectable
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Lock
import androidx.compose.material3.DatePicker
import androidx.compose.material3.DatePickerDialog
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.SelectableDates
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.rememberDatePickerState
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
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.unit.dp
import id.albahjah.hris.data.Child
import id.albahjah.hris.data.Me
import id.albahjah.hris.data.MyFamily
import id.albahjah.hris.data.Repository
import id.albahjah.hris.data.toApiError
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import java.time.LocalDate
import java.time.ZonedDateTime

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
                    ContactForm(phone = localPhone(updated.phone), email = updated.email.orEmpty(), message = "Kontak diperbarui.")
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

const val MAX_CHILDREN_SENT = 10

/** Isian satu anak; urutan anak mengikuti posisinya di daftar. */
data class ChildDraft(
    val name: String = "",
    val gender: String? = null,
    val birthPlace: String = "",
    val birthDate: String? = null,
    val relation: String? = null,
    val education: String? = null,
    val school: String = "",
)

data class FamilyForm(
    val hasChildren: Boolean? = null,
    val count: String = "",
    val children: List<ChildDraft> = emptyList(),
    val loading: Boolean = false,
    val busy: Boolean = false,
    val errors: Map<String, String> = emptyMap(),
    val message: String? = null,
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

/** Isian menjadi body PUT /v1/me/children; anak ke- diambil dari urutan di daftar. */
fun FamilyForm.toFamily(): MyFamily = MyFamily(
    hasChildren = hasChildren,
    childrenCount = count.trim().toIntOrNull(),
    children = children.mapIndexed { i, c ->
        Child(
            childOrder = i + 1,
            fullName = c.name.trim(),
            gender = c.gender,
            birthPlace = c.birthPlace.trim().ifEmpty { null },
            birthDate = c.birthDate,
            relation = c.relation,
            education = c.education,
            schoolName = c.school.trim().ifEmpty { null },
        )
    },
)

/** Pemeriksaan sebelum dikirim; aturan sama dengan server. */
fun familyProblems(form: FamilyForm): Map<String, String> {
    val errors = mutableMapOf<String, String>()
    val count = form.count.trim()
    if (count.isNotEmpty() && (count.toIntOrNull() == null || count.toInt() !in 0..30)) errors["children_count"] = "Jumlah anak 0 sampai 30."
    if (form.hasChildren == false && (form.children.isNotEmpty() || (count.toIntOrNull() ?: 0) > 0)) {
        errors["has_children"] = "Pilih \"Ya\" pada memiliki anak bila ada data anak."
    }
    val n = count.toIntOrNull()
    if (n != null && n < form.children.size) errors["children_count"] = "Jumlah anak perlu sama atau lebih dari data anak yang diisi."
    form.children.forEachIndexed { i, c ->
        if (c.name.trim().length !in 2..100) errors["children.$i.full_name"] = "Nama anak 2 sampai 100 karakter."
    }
    return errors
}

@Composable
private fun OptionPill(text: String, selected: Boolean, onClick: () -> Unit) {
    val shape = RoundedCornerShape(12.dp)
    Box(
        Modifier
            .heightIn(min = 40.dp)
            .clip(shape)
            .background(if (selected) Hc.BrandSoft else Hc.Field)
            .then(if (selected) Modifier.border(2.dp, Hc.Brand, shape) else Modifier)
            .selectable(selected = selected, role = Role.RadioButton, onClick = onClick)
            .padding(horizontal = 14.dp, vertical = 10.dp),
        contentAlignment = Alignment.Center,
    ) {
        Text(text, style = figmaText(14, FontWeight.Medium, if (selected) Hc.BrandDeep else Hc.Ink))
    }
}

@OptIn(ExperimentalLayoutApi::class)
@Composable
private fun OptionGroup(label: String, options: List<Pair<String, String>>, selected: String?, onPick: (String?) -> Unit) {
    Column(verticalArrangement = Arrangement.spacedBy(6.dp)) {
        Text(label, style = figmaText(13, FontWeight.Medium, Hc.InkSoft))
        FlowRow(horizontalArrangement = Arrangement.spacedBy(8.dp), verticalArrangement = Arrangement.spacedBy(8.dp)) {
            // Mengetuk pilihan yang sama mengosongkannya.
            options.forEach { (value, text) -> OptionPill(text, selected == value) { onPick(if (selected == value) null else value) } }
        }
    }
}

/** Data anak milik sendiri: hanya terlihat oleh Pejuang dan SDM. */
@Composable
fun ChildrenContent(
    form: FamilyForm,
    onBack: () -> Unit,
    onChange: (FamilyForm) -> Unit = {},
    onPickDate: (Int) -> Unit = {},
    onSave: () -> Unit = {},
) {
    Column(Modifier.fillMaxSize().background(Fg.PageProfile)) {
        FigmaHeader("Data Anak", onBack)
        if (form.loading) {
            CenterMessage(title = "Memuat data anak", body = null, loading = true)
            return
        }
        Column(
            Modifier.weight(1f).verticalScroll(rememberScrollState()).imePadding().padding(horizontal = 12.dp, vertical = 16.dp).navigationBarsPadding(),
            verticalArrangement = Arrangement.spacedBy(12.dp),
        ) {
            FigmaCard(title = "Keluarga", subtitle = "Hanya bisa dilihat oleh Anda dan SDM.", gap = 16.dp) {
                OptionGroup("Memiliki anak", listOf("ya" to "Ya", "tidak" to "Tidak"), form.hasChildren?.let { if (it) "ya" else "tidak" }) {
                    onChange(form.copy(hasChildren = it?.let { v -> v == "ya" }, errors = form.errors - "has_children", message = null))
                }
                form.errors["has_children"]?.let { Text(it, style = figmaText(12, color = Fg.Danger, lineHeight = 1.35f)) }
                FigmaInput(
                    "Jumlah anak", form.count,
                    { onChange(form.copy(count = it.filter(Char::isDigit).take(2), errors = form.errors - "children_count", message = null)) },
                    FigmaIcons.User,
                    error = form.errors["children_count"],
                    keyboard = KeyboardOptions(keyboardType = KeyboardType.Number),
                )
            }

            form.children.forEachIndexed { i, child ->
                val update: (ChildDraft) -> Unit = { c -> onChange(form.copy(children = form.children.toMutableList().also { it[i] = c }, message = null)) }
                FigmaCard(title = "Anak ke-${i + 1}", gap = 14.dp) {
                    FigmaInput(
                        "Nama lengkap", child.name, { update(child.copy(name = it)) }, FigmaIcons.User,
                        error = form.errors["children.$i.full_name"],
                    )
                    OptionGroup("Jenis kelamin", ChildGenders, child.gender) { update(child.copy(gender = it)) }
                    FigmaInput("Tempat lahir", child.birthPlace, { update(child.copy(birthPlace = it)) }, FigmaIcons.LocationTickLinear)
                    Column(verticalArrangement = Arrangement.spacedBy(6.dp)) {
                        Text("Tanggal lahir", style = figmaText(13, FontWeight.Medium, Hc.InkSoft))
                        Row(
                            Modifier.fillMaxWidth().heightIn(min = 52.dp).clip(RoundedCornerShape(14.dp)).background(Hc.Field)
                                .clickable(onClickLabel = "Pilih tanggal lahir") { onPickDate(i) }.padding(horizontal = 14.dp),
                            verticalAlignment = Alignment.CenterVertically,
                            horizontalArrangement = Arrangement.spacedBy(10.dp),
                        ) {
                            Icon(FigmaIcons.CalendarOutline, contentDescription = null, tint = Hc.InkMuted, modifier = Modifier.size(20.dp))
                            Text(
                                child.birthDate?.let { runCatching { dateLabel(LocalDate.parse(it)) }.getOrNull() } ?: "Pilih tanggal",
                                style = figmaText(15, color = if (child.birthDate == null) Hc.InkMuted else Hc.Ink),
                            )
                        }
                        form.errors["children.$i.birth_date"]?.let { Text(it, style = figmaText(12, color = Fg.Danger)) }
                    }
                    OptionGroup("Status anak", ChildRelations, child.relation) { update(child.copy(relation = it)) }
                    OptionGroup("Jenjang pendidikan", ChildEducations, child.education) { update(child.copy(education = it)) }
                    FigmaInput("Nama sekolah", child.school, { update(child.copy(school = it)) }, FigmaIcons.Keyboard)
                    TextButton(
                        onClick = { onChange(form.copy(children = form.children.filterIndexed { j, _ -> j != i }, errors = emptyMap(), message = null)) },
                        modifier = Modifier.fillMaxWidth(),
                    ) { Text("Hapus data anak ini", style = figmaText(14, FontWeight.Medium, Fg.Danger)) }
                }
            }

            if (form.children.size < MAX_CHILDREN_SENT) {
                SecondaryButton("Tambah data anak", onClick = {
                    onChange(form.copy(hasChildren = true, children = form.children + ChildDraft(), message = null))
                })
            }
            form.message?.let { Text(it, style = figmaText(13, FontWeight.Medium, Hc.BrandDeep)) }
            form.errors["general"]?.let { Text(it, style = figmaText(13, color = Fg.Danger, lineHeight = 1.4f)) }
            PrimaryButton("Simpan data anak", onClick = onSave, loading = form.busy)
        }
    }
}

/** Layar Data anak dengan pemanggilan API. */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun ChildrenScreen(repository: Repository, onBack: () -> Unit) {
    val scope = rememberCoroutineScope()
    var form by remember { mutableStateOf(FamilyForm(loading = true)) }
    var picking by remember { mutableStateOf<Int?>(null) }
    LaunchedEffect(Unit) {
        form = try {
            FamilyForm.from(repository.myFamily())
        } catch (e: CancellationException) {
            throw e
        } catch (e: Exception) {
            FamilyForm(errors = mapOf("general" to (e.toApiError().message ?: "Data anak belum bisa dimuat.")))
        }
    }
    SecureScreen()
    ChildrenContent(
        form = form,
        onBack = onBack,
        onChange = { form = it },
        onPickDate = { picking = it },
        onSave = save@{
            if (form.busy) return@save
            val problems = familyProblems(form)
            if (problems.isNotEmpty()) {
                form = form.copy(errors = problems)
                return@save
            }
            form = form.copy(busy = true, errors = emptyMap(), message = null)
            scope.launch {
                form = try {
                    FamilyForm.from(repository.setMyFamily(form.toFamily())).copy(message = "Data anak tersimpan.")
                } catch (e: CancellationException) {
                    throw e
                } catch (e: Exception) {
                    val err = e.toApiError()
                    form.copy(busy = false, errors = err.fields.ifEmpty { mapOf("general" to (err.message ?: "Data anak belum tersimpan.")) })
                }
            }
        },
    )
    val index = picking
    if (index != null && index < form.children.size) {
        val today = ZonedDateTime.now(Wib).toLocalDate()
        val state = rememberDatePickerState(
            initialSelectedDateMillis = form.children[index].birthDate?.let { runCatching { LocalDate.parse(it) }.getOrNull() }?.toPickerMillis(),
            yearRange = (today.year - 60)..today.year,
            selectableDates = object : SelectableDates {
                override fun isSelectableDate(utcTimeMillis: Long): Boolean = !pickerMillisToDate(utcTimeMillis).isAfter(today)
            },
        )
        DatePickerDialog(
            onDismissRequest = { picking = null },
            confirmButton = {
                TextButton(enabled = state.selectedDateMillis != null, onClick = {
                    val date = state.selectedDateMillis?.let(::pickerMillisToDate)?.toString()
                    form = form.copy(children = form.children.toMutableList().also { it[index] = it[index].copy(birthDate = date) })
                    picking = null
                }) { Text("Pilih") }
            },
            dismissButton = { TextButton(onClick = { picking = null }) { Text("Batal") } },
        ) {
            DatePicker(state = state, showModeToggle = false)
        }
    }
}
