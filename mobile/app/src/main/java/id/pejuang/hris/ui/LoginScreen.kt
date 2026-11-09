package id.pejuang.hris.ui

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.imePadding
import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.statusBarsPadding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.KeyboardActions
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Email
import androidx.compose.material.icons.filled.Lock
import androidx.compose.material.icons.filled.Settings
import androidx.compose.material.icons.filled.Warning
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.OutlinedTextFieldDefaults
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.platform.LocalClipboardManager
import androidx.compose.ui.text.AnnotatedString
import androidx.compose.ui.text.input.ImeAction
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.input.PasswordVisualTransformation
import androidx.compose.ui.text.input.VisualTransformation
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import id.pejuang.hris.BuildConfig
import id.pejuang.hris.data.Repository
import id.pejuang.hris.data.toApiError
import kotlinx.coroutines.launch

/** Isian layar masuk; dipisah dari [LoginScreen] agar bisa dirender tanpa jaringan. */
data class LoginForm(
    val server: String = "",
    val email: String = "",
    val password: String = "",
    val mfa: String = "",
    val askMfa: Boolean = false,
    val busy: Boolean = false,
    val error: String? = null,
)

@Composable
fun LoginScreen(repository: Repository, crash: String?, onDismissCrash: () -> Unit, onLoggedIn: () -> Unit) {
    val scope = rememberCoroutineScope()
    val clipboard = LocalClipboardManager.current
    var server by rememberSaveable { mutableStateOf(repository.serverUrl) }
    var email by rememberSaveable { mutableStateOf("") }
    var password by rememberSaveable { mutableStateOf("") }
    var mfa by rememberSaveable { mutableStateOf("") }
    var askMfa by rememberSaveable { mutableStateOf(false) }
    var busy by rememberSaveable { mutableStateOf(false) }
    var error by rememberSaveable { mutableStateOf<String?>(null) }

    LoginContent(
        form = LoginForm(server, email, password, mfa, askMfa, busy, error),
        showServer = BuildConfig.SERVER_EDITABLE,
        version = BuildConfig.VERSION_NAME,
        crash = crash,
        onCopyCrash = { crash?.let { clipboard.setText(AnnotatedString(it)) } },
        onDismissCrash = onDismissCrash,
        onServer = { server = it },
        onEmail = { email = it },
        onPassword = { password = it },
        onMfa = { mfa = it.filter(Char::isDigit).take(6) },
        onSubmit = {
            if (busy) return@LoginContent
            busy = true
            error = null
            scope.launch {
                try {
                    repository.login(server, email, password, if (askMfa) mfa else null)
                    onLoggedIn()
                } catch (e: Exception) {
                    val err = e.toApiError()
                    if (err.code == "AUTH_MFA_REQUIRED" || err.code == "AUTH_MFA_INVALID") askMfa = true
                    error = err.message
                } finally {
                    busy = false
                }
            }
        },
    )
}

@Composable
fun LoginContent(
    form: LoginForm,
    showServer: Boolean,
    version: String,
    crash: String?,
    onCopyCrash: () -> Unit,
    onDismissCrash: () -> Unit,
    onServer: (String) -> Unit,
    onEmail: (String) -> Unit,
    onPassword: (String) -> Unit,
    onMfa: (String) -> Unit,
    onSubmit: () -> Unit,
) {
    StatusBarIcons(dark = false)
    var showPassword by rememberSaveable { mutableStateOf(false) }
    val canSubmit = form.email.isNotBlank() && form.password.isNotEmpty() &&
        (!showServer || form.server.isNotBlank()) && (!form.askMfa || form.mfa.length == 6)

    Column(
        Modifier
            .fillMaxSize()
            .background(Hc.BrandDeep)
            .imePadding()
            .verticalScroll(rememberScrollState()),
    ) {
        Box(
            Modifier
                .fillMaxWidth()
                .background(BrandGradient)
                .statusBarsPadding()
                .padding(start = 32.dp, end = 32.dp, top = 24.dp, bottom = 28.dp),
        ) {
            Column {
                LogoMark(size = 60.dp)
                Spacer(Modifier.height(20.dp))
                Text("HRIS Pejuang", style = MaterialTheme.typography.headlineMedium, color = Color.White)
                Spacer(Modifier.height(4.dp))
                Text(
                    "Presensi dengan verifikasi wajah dan lokasi.",
                    style = MaterialTheme.typography.bodyMedium,
                    color = Color.White.copy(alpha = 0.82f),
                )
            }
        }

        Column(
            Modifier.fillMaxWidth()
                .clip(RoundedCornerShape(topStart = 24.dp, topEnd = 24.dp))
                .background(Hc.Surface)
                .padding(start = 32.dp, end = 32.dp, top = 40.dp, bottom = 24.dp),
            verticalArrangement = Arrangement.spacedBy(12.dp),
        ) {
            Column {
                Text("Masuk", style = MaterialTheme.typography.headlineSmall, color = Hc.Ink, modifier = Modifier.fillMaxWidth(), textAlign = TextAlign.Center)
                Spacer(Modifier.height(4.dp))
                Text(
                    "Masuk ke akun Pejuang Anda",
                    style = MaterialTheme.typography.bodyMedium,
                    color = Hc.InkSoft,
                    modifier = Modifier.fillMaxWidth(),
                    textAlign = TextAlign.Center,
                )
                Spacer(Modifier.height(20.dp))
                Column(verticalArrangement = Arrangement.spacedBy(14.dp)) {
                    Field(
                        value = form.email,
                        onChange = onEmail,
                        label = "Email",
                        icon = Icons.Filled.Email,
                        keyboard = KeyboardOptions(keyboardType = KeyboardType.Email, imeAction = ImeAction.Next),
                    )
                    Field(
                        value = form.password,
                        onChange = onPassword,
                        label = "Kata sandi",
                        icon = Icons.Filled.Lock,
                        keyboard = KeyboardOptions(
                            keyboardType = KeyboardType.Password,
                            imeAction = if (form.askMfa) ImeAction.Next else ImeAction.Done,
                        ),
                        hidden = !showPassword,
                        onDone = { if (canSubmit) onSubmit() },
                        trailing = {
                            TextButton(onClick = { showPassword = !showPassword }) {
                                Text(if (showPassword) "Sembunyikan" else "Lihat", color = Hc.Brand)
                            }
                        },
                    )
                    if (form.askMfa) {
                        Field(
                            value = form.mfa,
                            onChange = onMfa,
                            label = "Kode autentikator (6 digit)",
                            icon = HrisIcons.Shield,
                            keyboard = KeyboardOptions(keyboardType = KeyboardType.NumberPassword, imeAction = ImeAction.Done),
                            onDone = { if (canSubmit) onSubmit() },
                        )
                    }
                    if (form.error != null) {
                        StatusBanner(Tone.Danger, Icons.Filled.Warning, "Belum bisa masuk", form.error)
                    }
                    PrimaryButton(
                        text = "Masuk",
                        onClick = onSubmit,
                        enabled = canSubmit,
                        loading = form.busy,
                        modifier = Modifier.padding(top = 4.dp),
                        shape = RoundedCornerShape(100.dp),
                    )
                }
            }

            if (showServer) {
                SectionCard {
                    Text("Mode pengembang", style = MaterialTheme.typography.titleSmall, color = Hc.Ink)
                    Text(
                        "Alamat API server develop. Hanya muncul di build debug.",
                        style = MaterialTheme.typography.bodySmall,
                        color = Hc.InkSoft,
                    )
                    Spacer(Modifier.height(12.dp))
                    Field(
                        value = form.server,
                        onChange = onServer,
                        label = "Alamat server",
                        icon = Icons.Filled.Settings,
                        placeholder = "http://192.168.1.10:8080",
                        keyboard = KeyboardOptions(keyboardType = KeyboardType.Uri, imeAction = ImeAction.Done),
                    )
                }
            }

            if (crash != null) CrashNotice(crash, onCopyCrash, onDismissCrash)

            Text(
                "Versi $version · Data Anda dilindungi sesuai UU PDP",
                style = MaterialTheme.typography.labelSmall,
                color = Hc.InkMuted,
                textAlign = TextAlign.Center,
                modifier = Modifier.fillMaxWidth().navigationBarsPadding().padding(top = 8.dp),
            )
        }
    }
}


@Composable
private fun Field(
    value: String,
    onChange: (String) -> Unit,
    label: String,
    icon: ImageVector,
    keyboard: KeyboardOptions,
    hidden: Boolean = false,
    placeholder: String? = null,
    onDone: (() -> Unit)? = null,
    trailing: (@Composable () -> Unit)? = null,
) {
    OutlinedTextField(
        value = value,
        onValueChange = onChange,
        label = { Text(label) },
        placeholder = { if (placeholder != null) Text(placeholder, color = Hc.InkMuted) },
        leadingIcon = { Icon(icon, contentDescription = null, tint = Hc.InkMuted) },
        trailingIcon = trailing,
        singleLine = true,
        visualTransformation = if (hidden) PasswordVisualTransformation() else VisualTransformation.None,
        keyboardOptions = keyboard,
        keyboardActions = KeyboardActions(onDone = { onDone?.invoke() }),
        shape = MaterialTheme.shapes.medium,
        colors = OutlinedTextFieldDefaults.colors(
            focusedBorderColor = Hc.Brand,
            unfocusedBorderColor = Hc.Line,
            focusedLabelColor = Hc.Brand,
            cursorColor = Hc.Brand,
            focusedContainerColor = Hc.Surface,
            unfocusedContainerColor = Hc.Surface,
        ),
        modifier = Modifier.fillMaxWidth(),
    )
}
