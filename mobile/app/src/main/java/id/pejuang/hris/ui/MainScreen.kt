package id.pejuang.hris.ui

import androidx.activity.compose.BackHandler
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.statusBarsPadding
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Warning
import androidx.compose.runtime.Composable
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import dev.chrisbanes.haze.HazeState
import dev.chrisbanes.haze.hazeSource
import androidx.compose.ui.platform.LocalClipboardManager
import androidx.compose.ui.text.AnnotatedString
import androidx.compose.ui.unit.dp
import id.pejuang.hris.BuildConfig
import id.pejuang.hris.data.AttendanceRecord
import id.pejuang.hris.data.Me
import id.pejuang.hris.data.Repository
import id.pejuang.hris.data.toApiError
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.launch
import java.time.ZonedDateTime

/** Halaman di atas tab yang dibuka dari Profil. */
private enum class SubPage { PersonalData, Password }

/**
 * Kerangka aplikasi setelah masuk: memuat profil dan presensi sekali, lalu menampilkan lima tab
 * dengan navigasi bawah Figma.
 */
@Composable
fun MainScreen(
    repository: Repository,
    crash: String?,
    onDismissCrash: () -> Unit,
    onCapture: (String) -> Unit,
    onLoggedOut: () -> Unit,
) {
    val scope = rememberCoroutineScope()
    val clipboard = LocalClipboardManager.current
    var tab by rememberSaveable { mutableStateOf(MainTab.Home) }
    var sub by rememberSaveable { mutableStateOf<SubPage?>(null) }
    var askLogout by rememberSaveable { mutableStateOf(false) }
    var me by remember { mutableStateOf<Me?>(null) }
    var records by remember { mutableStateOf<List<AttendanceRecord>?>(null) }
    var historyError by remember { mutableStateOf<String?>(null) }
    var error by remember { mutableStateOf<String?>(null) }
    var loading by remember { mutableStateOf(true) }
    var reload by remember { mutableIntStateOf(0) }

    val logout: () -> Unit = {
        scope.launch {
            repository.logout()
            onLoggedOut()
        }
    }

    LaunchedEffect(reload) {
        loading = true
        error = null
        historyError = null
        try {
            me = repository.me()
            // Kegagalan riwayat tidak menghalangi presensi; tab Presensi menampilkan pesannya sendiri.
            try {
                records = repository.history()
            } catch (e: CancellationException) {
                throw e
            } catch (e: Exception) {
                historyError = e.toApiError().message
            }
        } catch (e: CancellationException) {
            throw e
        } catch (e: Exception) {
            val err = e.toApiError()
            if (err.code == "AUTH_TOKEN_INVALID" || !repository.loggedIn) {
                repository.logout()
                onLoggedOut()
            } else {
                error = err.message
            }
        } finally {
            loading = false
        }
    }

    val current = me
    if (current == null) {
        StatusBarIcons(dark = true)
        if (error != null) {
            CenterMessage(title = "Data belum bisa dimuat", body = error, icon = Icons.Filled.Warning, tone = Tone.Warning) {
                PrimaryButton("Coba lagi", onClick = { reload++ })
                SecondaryButton("Keluar", onClick = logout)
            }
        } else {
            CenterMessage(title = "Memuat data Anda", body = null, loading = true)
        }
        return
    }

    when (sub) {
        SubPage.PersonalData -> {
            BackHandler { sub = null }
            SecureScreen()
            PersonalDataContent(current) { sub = null }
            return
        }
        SubPage.Password -> {
            BackHandler { sub = null }
            ChangePasswordScreen(repository, onBack = { sub = null }, onChanged = { sub = null; reload++ })
            return
        }
        null -> Unit
    }

    if (tab != MainTab.Home) BackHandler { tab = MainTab.Home }
    val now = ZonedDateTime.now(Wib)
    Box(Modifier.fillMaxSize()) {
        MainTabs(
            tab = tab,
            onTab = { tab = it },
            me = current,
            records = records,
            historyError = historyError,
            now = now,
            refreshing = loading,
            crash = crash,
            onCopyCrash = { crash?.let { clipboard.setText(AnnotatedString(it)) } },
            onDismissCrash = onDismissCrash,
            onCapture = onCapture,
            onRefresh = { if (!loading) reload++ },
            onPersonalData = { sub = SubPage.PersonalData },
            onChangePassword = { sub = SubPage.Password },
            onLogout = { askLogout = true },
        )
        if (askLogout) {
            BackHandler { askLogout = false }
            FigmaPopup(
                icon = FigmaIcons.Logout,
                title = "Keluar dari aplikasi?",
                body = "Anda perlu masuk lagi dengan email dan kata sandi untuk presensi berikutnya.",
                primary = "Ya, keluar",
                onPrimary = { askLogout = false; logout() },
                secondary = "Batal",
                onSecondary = { askLogout = false },
                danger = true,
            )
        }
    }
}

/** Isi tab tanpa jaringan, dipakai juga oleh tes screenshot. */
@Composable
fun MainTabs(
    tab: MainTab,
    onTab: (MainTab) -> Unit,
    me: Me,
    records: List<AttendanceRecord>?,
    historyError: String?,
    now: ZonedDateTime,
    refreshing: Boolean,
    crash: String?,
    onCopyCrash: () -> Unit,
    onDismissCrash: () -> Unit,
    onCapture: (String) -> Unit,
    onRefresh: () -> Unit,
    onPersonalData: () -> Unit,
    onChangePassword: () -> Unit,
    onLogout: () -> Unit,
) {
    // Isi tab menjadi sumber blur; bottom bar kaca mengambang di atasnya.
    val haze = remember { HazeState() }
    Box(Modifier.fillMaxSize().background(Fg.Page)) {
        Box(Modifier.fillMaxSize().hazeSource(haze)) {
          CompositionLocalProvider(LocalBottomBarSpace provides glassBarSpace()) {
            when (tab) {
                MainTab.Home -> HomeContent(
                    me = me,
                    today = records?.firstOrNull { it.workDate == now.toLocalDate().toString() },
                    now = now,
                    refreshing = refreshing,
                    crash = crash,
                    onCopyCrash = onCopyCrash,
                    onDismissCrash = onDismissCrash,
                    onCapture = onCapture,
                    onHistory = { onTab(MainTab.Attendance) },
                    onRefresh = onRefresh,
                    onLogout = onLogout,
                )
                MainTab.Attendance -> AttendanceContent(me, records, historyError, now, onCapture, onRetry = onRefresh)
                MainTab.Requests -> ComingSoonContent(
                    title = "Pengajuan",
                    subtitle = "Cuti, izin, pinjaman, dan mutasi",
                    body = "Pengajuan dari aplikasi sedang disiapkan. Sementara ini ajukan lewat SDM. " +
                        "Riwayat dan alur persetujuannya akan tampil di sini.",
                )
                MainTab.Payslip -> {
                    SecureScreen()
                    ComingSoonContent(
                        title = "Slip ubar",
                        subtitle = "Rincian ubar per periode",
                        body = "Slip ubar akan tampil di sini setelah SDM menerbitkan payroll dari sistem.",
                    )
                }
                MainTab.Profile -> {
                    SecureScreen()
                    ProfileContent(
                        me = me,
                        version = BuildConfig.VERSION_NAME,
                        onBack = { onTab(MainTab.Home) },
                        onPersonalData = onPersonalData,
                        onChangePassword = onChangePassword,
                        onCheckUpdate = null,
                        onLogout = onLogout,
                    )
                }
            }
          }
        }
        GlassBottomBar(tab, onTab, haze, Modifier.align(Alignment.BottomCenter))
    }
}

/** Tab yang fiturnya belum tersedia di server (MASTER.md Fase 4 dan 5). */
@Composable
fun ComingSoonContent(title: String, subtitle: String, body: String) {
    Box(Modifier.fillMaxSize().background(Fg.Page)) {
        FigmaTabHeader(title, subtitle, illustration = false)
        Column(Modifier.align(Alignment.TopCenter).statusBarsPadding().then(ContentWidth).padding(start = 12.dp, end = 12.dp, top = 100.dp)) {
            FigmaCard(title = "Segera tersedia", subtitle = body, top = 16.dp, bottom = 16.dp) {}
        }
    }
}

/** Layar ganti kata sandi dengan pemanggilan API. */
@Composable
fun ChangePasswordScreen(repository: Repository, onBack: () -> Unit, onChanged: () -> Unit) {
    val scope = rememberCoroutineScope()
    var form by remember { mutableStateOf(PasswordForm()) }
    SecureScreen()
    ChangePasswordContent(
        form = form,
        onCurrent = { form = form.copy(current = it, error = null) },
        onNext = { form = form.copy(next = it, error = null) },
        onRepeat = { form = form.copy(repeat = it, error = null) },
        onSubmit = {
            if (!form.busy) {
                form = form.copy(busy = true, error = null)
                scope.launch {
                    form = try {
                        repository.changePassword(form.current, form.next)
                        form.copy(busy = false, done = true, current = "", next = "", repeat = "")
                    } catch (e: CancellationException) {
                        throw e
                    } catch (e: Exception) {
                        form.copy(busy = false, error = e.toApiError().message)
                    }
                }
            }
        },
        onBack = onBack,
        onFinished = onChanged,
    )
}
