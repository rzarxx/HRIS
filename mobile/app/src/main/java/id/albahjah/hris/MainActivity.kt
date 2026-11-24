package id.albahjah.hris

import android.os.Bundle
import android.os.SystemClock
import androidx.activity.compose.setContent
import androidx.activity.enableEdgeToEdge
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.size
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.unit.dp
import androidx.compose.foundation.layout.Box
import androidx.activity.compose.BackHandler
import androidx.fragment.app.FragmentActivity
import id.albahjah.hris.security.AppLock
import id.albahjah.hris.ui.LockContent
import androidx.lifecycle.Lifecycle
import androidx.lifecycle.lifecycleScope
import kotlinx.coroutines.launch
import androidx.navigation.NavBackStackEntry
import androidx.navigation.NavHostController
import androidx.navigation.compose.NavHost
import androidx.navigation.compose.composable
import androidx.navigation.compose.rememberNavController
import id.albahjah.hris.data.ResumeState
import id.albahjah.hris.ui.CaptureScreen
import id.albahjah.hris.ui.HrisTheme
import id.albahjah.hris.ui.LoginScreen
import id.albahjah.hris.ui.AppLogo
import id.albahjah.hris.ui.Hc
import id.albahjah.hris.ui.MainScreen
import id.albahjah.hris.ui.StatusBarIcons

// FragmentActivity karena dialog sidik jari (BiometricPrompt) membutuhkannya.
class MainActivity : FragmentActivity() {
    private val app get() = application as HrisApp

    override fun onStart() {
        super.onStart()
        val now = SystemClock.elapsedRealtime()
        // Memutar layar tidak tercatat sebagai ke latar, jadi dianggap baru saja tampil.
        val since = if (app.started) app.stoppedAt ?: now else null
        if (AppLock.shouldLock(app.prefs.appLock, app.repository.loggedIn, since, now)) app.locked.value = true
        app.started = true
        app.stoppedAt = null
    }

    override fun onStop() {
        super.onStop()
        if (!isChangingConfigurations) app.stoppedAt = SystemClock.elapsedRealtime()
    }

    private fun unlock() = AppLock.prompt(this, "Buka HRIS Al Bahjah", "Gunakan sidik jari, wajah, atau PIN HP") { ok ->
        if (ok) app.locked.value = false
    }

    private fun enableLock() = AppLock.prompt(this, "Aktifkan kunci aplikasi", "Konfirmasi dengan sidik jari, wajah, atau PIN HP") { ok ->
        if (ok) app.prefs.updateAppLock(true)
    }

    override fun onCreate(savedInstanceState: Bundle?) {
        enableEdgeToEdge()
        super.onCreate(savedInstanceState)
        val lockAvailable = AppLock.available(this)
        setContent {
            HrisTheme(fontScale = app.prefs.fontScale) {
                var crash by remember { mutableStateOf(app.lastCrash) }
                val dismissCrash = {
                    crash = null
                    app.lastCrash = null
                }
                val nav = rememberNavController()
                Box(Modifier.fillMaxSize()) {
                NavHost(navController = nav, startDestination = "start") {
                    composable("start") {
                        Splash()
                        LaunchedEffect(Unit) {
                            // Offline tetap ke beranda dengan data cache; hanya sesi yang ditolak server ke layar masuk (D62).
                            nav.replaceWith(if (app.repository.resume() == ResumeState.LoggedOut) "login" else "home")
                        }
                    }
                    composable("login") {
                        LoginScreen(app.repository, crash, dismissCrash) { nav.replaceWith("home") }
                    }
                    composable("home") { entry ->
                        MainScreen(
                            repository = app.repository,
                            crash = crash,
                            onDismissCrash = dismissCrash,
                            onCapture = { purpose -> if (entry.isActive()) nav.navigate("capture/$purpose") },
                            onLoggedOut = { nav.replaceWith("login") },
                            onEnableAppLock = if (lockAvailable) ::enableLock else null,
                        )
                    }
                    composable("capture/{purpose}") { entry ->
                        CaptureScreen(
                            purpose = entry.arguments?.getString("purpose").orEmpty(),
                            repository = app.repository,
                            location = app.location,
                            onClose = { if (entry.isActive()) nav.popBackStack() },
                        )
                    }
                }
                // Kunci menutupi isi; sesi dan data tetap dimuat di belakangnya (D62).
                if (app.locked.value && app.repository.loggedIn) {
                    BackHandler { moveTaskToBack(true) }
                    LaunchedEffect(Unit) { unlock() }
                    LockContent(
                        onUnlock = ::unlock,
                        onUsePassword = {
                            app.locked.value = false
                            lifecycleScope.launch {
                                app.repository.logout()
                                nav.replaceWith("login")
                            }
                        },
                    )
                }
                }
            }
        }
    }
}

/** Mencegah ketukan ganda memicu navigasi dua kali (mis. kembali melewati beranda). */
private fun NavBackStackEntry.isActive() = lifecycle.currentState.isAtLeast(Lifecycle.State.RESUMED)

@Composable
private fun Splash() {
    StatusBarIcons(dark = true)
    Column(
        Modifier.fillMaxSize().background(Hc.Canvas),
        horizontalAlignment = Alignment.CenterHorizontally,
        verticalArrangement = Arrangement.Center,
    ) {
        AppLogo(size = 112.dp)
        Spacer(Modifier.height(24.dp))
        Text(stringResource(R.string.app_name), style = MaterialTheme.typography.headlineSmall, color = Hc.Ink)
        Spacer(Modifier.height(32.dp))
        CircularProgressIndicator(color = Hc.Brand, strokeWidth = 2.dp, modifier = Modifier.size(24.dp))
    }
}

/** Pindah ke [route] dan hapus seluruh riwayat navigasi sebelumnya. */
private fun NavHostController.replaceWith(route: String) {
    navigate(route) {
        popUpTo(graph.id) { inclusive = true }
        launchSingleTop = true
    }
}
