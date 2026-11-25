package id.albahjah.hris

import android.content.ActivityNotFoundException
import android.content.Intent
import android.net.Uri
import android.os.Bundle
import android.widget.Toast
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.runtime.collectAsState
import com.google.android.play.core.appupdate.AppUpdateManagerFactory
import com.google.android.play.core.appupdate.AppUpdateOptions
import com.google.android.play.core.install.model.AppUpdateType
import com.google.android.play.core.install.model.UpdateAvailability
import id.albahjah.hris.data.DEFAULT_STORE_URL
import id.albahjah.hris.data.UpdateLevel
import id.albahjah.hris.ui.UpdateRequiredContent
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
import id.albahjah.hris.security.Biometric
import androidx.navigation.NavType
import androidx.navigation.navArgument
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
    private val appUpdateManager by lazy { AppUpdateManagerFactory.create(this) }
    private val updateLauncher = registerForActivityResult(ActivityResultContracts.StartIntentSenderForResult()) {}

    /**
     * Pembaruan aplikasi (D63): bila terpasang dari Play Store dan pembaruan tersedia, layar pembaruan bawaan Play
     * dibuka (unduh, pasang, dan aplikasi dimulai ulang). Selain itu tautan pembaruan dibuka di browser atau Play Store.
     */
    private fun startUpdate(storeUrl: String) {
        val fallback = { openStore(storeUrl.ifBlank { DEFAULT_STORE_URL }) }
        appUpdateManager.appUpdateInfo
            .addOnSuccessListener { info ->
                if (info.updateAvailability() == UpdateAvailability.UPDATE_AVAILABLE && info.isUpdateTypeAllowed(AppUpdateType.IMMEDIATE)) {
                    appUpdateManager.startUpdateFlowForResult(info, updateLauncher, AppUpdateOptions.newBuilder(AppUpdateType.IMMEDIATE).build())
                } else {
                    fallback()
                }
            }
            .addOnFailureListener { fallback() }
    }

    private fun openStore(url: String) {
        try {
            startActivity(Intent(Intent.ACTION_VIEW, Uri.parse(url)))
        } catch (e: ActivityNotFoundException) {
            Toast.makeText(this, "Buka Play Store untuk memperbarui aplikasi.", Toast.LENGTH_LONG).show()
        }
    }

    override fun onResume() {
        super.onResume()
        // Pembaruan Play yang terhenti di tengah jalan (mis. aplikasi ditutup) dilanjutkan.
        appUpdateManager.appUpdateInfo.addOnSuccessListener { info ->
            if (info.updateAvailability() == UpdateAvailability.DEVELOPER_TRIGGERED_UPDATE_IN_PROGRESS) {
                appUpdateManager.startUpdateFlowForResult(info, updateLauncher, AppUpdateOptions.newBuilder(AppUpdateType.IMMEDIATE).build())
            }
        }
    }

    override fun onStart() {
        super.onStart()
        // Versi dicek saat dibuka dan saat kembali ke depan, paling sering sekali per jam (D63).
        lifecycleScope.launch { app.repository.checkAppVersion() }
    }

    /** Masuk cepat (D64): sidik jari, wajah, atau PIN HP membuka NIPY dan kata sandi yang diingat. */
    private fun quickLoginPrompt(onResult: (Boolean) -> Unit) =
        Biometric.prompt(this, "Masuk ke HRIS Al Bahjah", "Gunakan sidik jari, wajah, atau PIN HP", onResult)

    private fun enableQuickLogin() =
        Biometric.prompt(this, "Aktifkan masuk cepat", "Konfirmasi dengan sidik jari, wajah, atau PIN HP") { ok ->
            if (ok) app.repository.savedLogin?.updateQuickLogin(true)
        }

    override fun onCreate(savedInstanceState: Bundle?) {
        enableEdgeToEdge()
        super.onCreate(savedInstanceState)
        val biometric = Biometric.available(this)
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
                            // Dari sini (aplikasi baru dibuka atau sesi berakhir) dialog masuk cepat langsung muncul (D64).
                            nav.replaceWith(if (app.repository.resume() == ResumeState.LoggedOut) "login?auto=true" else "home")
                        }
                    }
                    composable(
                        "login?auto={auto}",
                        arguments = listOf(navArgument("auto") { type = NavType.BoolType; defaultValue = false }),
                    ) { entry ->
                        LoginScreen(
                            repository = app.repository,
                            crash = crash,
                            onDismissCrash = dismissCrash,
                            onBiometric = if (biometric) ::quickLoginPrompt else null,
                            autoQuickLogin = entry.arguments?.getBoolean("auto") == true,
                        ) { nav.replaceWith("home") }
                    }
                    composable("home") { entry ->
                        MainScreen(
                            repository = app.repository,
                            crash = crash,
                            onDismissCrash = dismissCrash,
                            onCapture = { purpose -> if (entry.isActive()) nav.navigate("capture/$purpose") },
                            onLoggedOut = { nav.replaceWith("login") },
                            onEnableQuickLogin = if (biometric) ::enableQuickLogin else null,
                            onStartUpdate = ::startUpdate,
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
                // Di bawah versi minimum server menolak semua permintaan, jadi layar perbarui menutupi semuanya.
                val update = app.repository.appUpdate.collectAsState().value
                if (update?.level == UpdateLevel.Required) {
                    BackHandler { moveTaskToBack(true) }
                    UpdateRequiredContent(update.version, onUpdate = { startUpdate(update.version.storeUrl) })
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
