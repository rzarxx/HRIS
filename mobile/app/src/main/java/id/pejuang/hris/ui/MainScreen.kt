package id.pejuang.hris.ui

import androidx.activity.compose.BackHandler
import androidx.compose.animation.Crossfade
import androidx.compose.animation.core.tween
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.statusBarsPadding
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Warning
import androidx.compose.material3.SnackbarHost
import androidx.compose.material3.SnackbarHostState
import androidx.compose.material3.Text
import androidx.compose.ui.text.font.FontWeight
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
import id.pejuang.hris.data.FontSize
import id.pejuang.hris.data.Me
import id.pejuang.hris.data.MyAnnouncement
import id.pejuang.hris.data.Repository
import id.pejuang.hris.data.toApiError
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.launch
import java.time.ZonedDateTime

/** Halaman di atas tab yang dibuka dari Profil. */
private enum class SubPage { PersonalData, Password, FontSize }

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
    val requests = remember(repository) { RequestsController(repository, scope) }
    var requestsView by rememberSaveable { mutableStateOf(RequestsView.Mine) }
    var openLeaveId by rememberSaveable { mutableStateOf<String?>(null) }
    var leaveForm by rememberSaveable { mutableStateOf(false) }
    var deciding by remember { mutableStateOf<Decision?>(null) }
    val announcements = remember(repository) { AnnouncementsController(repository, scope) }
    var openAnnouncementId by rememberSaveable { mutableStateOf<String?>(null) }
    var announcementList by rememberSaveable { mutableStateOf(false) }
    // Pop-up yang ditunda ("Nanti saja") tidak muncul lagi sampai aplikasi dibuka ulang.
    var postponed by remember { mutableStateOf(setOf<String>()) }
    val snackbar = remember { SnackbarHostState() }
    val notify: (String) -> Unit = { message -> scope.launch { snackbar.showSnackbar(message) } }

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
        SubPage.FontSize -> {
            BackHandler { sub = null }
            FontSizeContent(repository.prefs.fontSize, onPick = repository.prefs::updateFontSize, onBack = { sub = null })
            return
        }
        null -> Unit
    }

    val canApprove = "approval:act" in current.permissions
    // Data pengajuan dimuat bersama profil, lalu diperbarui setiap tab Pengajuan dibuka (lencana
    // persetujuan ikut segar tanpa notifikasi push).
    LaunchedEffect(reload, current.employeeId) {
        requests.refresh(canApprove)
        announcements.refresh()
        repository.refreshBranding()
    }
    LaunchedEffect(tab) { if (tab == MainTab.Requests) requests.refresh(canApprove) }

    if (tab != MainTab.Home) BackHandler { tab = MainTab.Home }
    val now = ZonedDateTime.now(Wib)
    val openLeave = openLeaveId?.let { id -> requests.data.leaves?.firstOrNull { it.id == id } }
    val openAnnouncement = openAnnouncementId?.let { id -> announcements.items?.firstOrNull { it.id == id } }
    val onSubPage = openLeave != null || openAnnouncement != null || announcementList
    // Pop-up pengumuman muncul satu per satu saat aplikasi dibuka, di atas halaman tab.
    val popup = if (onSubPage || leaveForm || deciding != null || askLogout) null
    else announcements.items?.firstOrNull { it.type == "popup" && it.readAt == null && it.id !in postponed }
    Box(Modifier.fillMaxSize()) {
        if (openLeave != null) {
            BackHandler { openLeaveId = null }
            LeaveDetailScreen(
                controller = requests,
                leave = openLeave,
                onBack = { openLeaveId = null },
                onMessage = { openLeaveId = null; notify(it) },
            )
        } else if (openAnnouncement != null) {
            BackHandler { openAnnouncementId = null }
            AnnouncementDetailScreen(
                controller = announcements,
                item = openAnnouncement,
                onBack = { openAnnouncementId = null },
                onMessage = notify,
            )
        } else if (announcementList) {
            BackHandler { announcementList = false }
            AnnouncementListContent(announcements.items, onBack = { announcementList = false }, onOpen = { openAnnouncementId = it.id })
        } else MainTabs(
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
            footer = repository.prefs.footer,
            fontSize = repository.prefs.fontSize,
            onFontSize = { sub = SubPage.FontSize },
            requests = RequestsUi(
                data = requests.data,
                canApprove = canApprove,
                view = requestsView,
                onView = { requestsView = it },
                onNew = { leaveForm = true },
                onOpen = { openLeaveId = it.id },
                onDecide = { deciding = it },
                onRefresh = { requests.refresh(canApprove) },
            ),
            announcements = announcements.items.orEmpty(),
            onAnnouncement = { openAnnouncementId = it.id },
            onAnnouncements = { announcementList = true },
        )
        if (popup != null) {
            AnnouncementPopup(announcements, popup) { read ->
                if (read) notify("Terima kasih, pengumuman ditandai sudah dibaca.") else postponed = postponed + popup.id
            }
        }
        if (leaveForm) {
            LeaveFormSheet(
                controller = requests,
                today = now.toLocalDate(),
                onDismiss = { leaveForm = false },
                onSent = { leaveForm = false; requestsView = RequestsView.Mine; notify(it) },
            )
        }
        deciding?.let { d ->
            DecisionSheet(
                controller = requests,
                decision = d,
                onSwitch = { deciding = it },
                onDismiss = { deciding = null },
                onDone = { deciding = null; notify(it) },
            )
        }
        SnackbarHost(
            snackbar,
            Modifier.align(Alignment.BottomCenter).then(
                // glassBarSpace sudah termasuk navigation bar sistem.
                if (!onSubPage) Modifier.padding(bottom = glassBarSpace()) else Modifier.navigationBarsPadding().padding(bottom = 16.dp),
            ),
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
    requests: RequestsUi = RequestsUi(),
    announcements: List<MyAnnouncement> = emptyList(),
    onAnnouncement: (MyAnnouncement) -> Unit = {},
    onAnnouncements: () -> Unit = {},
    footer: String? = null,
    fontSize: FontSize = FontSize.Normal,
    onFontSize: () -> Unit = {},
) {
    // Isi tab menjadi sumber blur; bottom bar kaca mengambang di atasnya.
    val haze = remember { HazeState() }
    val pending = if (requests.canApprove) requests.data.pending?.size ?: 0 else 0
    Box(Modifier.fillMaxSize().background(Fg.Page)) {
        Box(Modifier.fillMaxSize().hazeSource(haze)) {
          CompositionLocalProvider(LocalBottomBarSpace provides glassBarSpace()) {
            // Peralihan tab singkat agar perpindahan tidak terasa melompat (MASTER.md §10.2: minim animasi).
            Crossfade(targetState = tab, animationSpec = tween(160), label = "tab") { shown ->
            when (shown) {
                MainTab.Home -> RefreshBox(refreshing, onRefresh, Modifier.fillMaxSize()) {
                    HomeContent(
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
                        pendingApprovals = pending,
                        onRequests = { onTab(MainTab.Requests) },
                        announcements = announcements,
                        onAnnouncement = onAnnouncement,
                        onAnnouncements = onAnnouncements,
                        onPayslip = { onTab(MainTab.Payslip) },
                        footer = footer,
                    )
                }
                MainTab.Attendance -> RefreshBox(refreshing && records != null, onRefresh, Modifier.fillMaxSize()) {
                    AttendanceContent(me, records, historyError, now, onCapture, onRetry = onRefresh)
                }
                MainTab.Requests -> RequestsContent(requests, now)
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
                        fontSize = fontSize,
                        onFontSize = onFontSize,
                        footer = footer,
                    )
                }
            }
            }
          }
        }
        // Isi yang digulir tidak menembus jam dan ikon status bar.
        StatusBarScrim()
        GlassBottomBar(tab, onTab, haze, Modifier.align(Alignment.BottomCenter), badges = mapOf(MainTab.Requests to pending))
    }
}

/** Tab yang fiturnya belum tersedia di server (MASTER.md Fase 4 dan 5). */
@Composable
fun ComingSoonContent(title: String, subtitle: String, body: String) {
    Box(Modifier.fillMaxSize().background(Hc.Canvas)) {
        Column(
            Modifier.align(Alignment.TopCenter).statusBarsPadding().then(ContentWidth).padding(start = 16.dp, end = 16.dp, top = 8.dp),
            verticalArrangement = Arrangement.spacedBy(12.dp),
        ) {
            PageHeader(title, subtitle)
            FigmaCard {
                SoftIconTile(FigmaIcons.NavReceipt, 56.dp)
                Text("Segera tersedia", style = figmaText(16, FontWeight.SemiBold))
                Text(body, style = figmaText(14, color = Hc.InkSoft, lineHeight = 1.5f))
            }
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
