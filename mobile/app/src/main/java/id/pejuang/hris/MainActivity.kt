package id.pejuang.hris

import android.os.Bundle
import androidx.activity.ComponentActivity
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
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.unit.dp
import androidx.lifecycle.Lifecycle
import androidx.navigation.NavBackStackEntry
import androidx.navigation.NavHostController
import androidx.navigation.compose.NavHost
import androidx.navigation.compose.composable
import androidx.navigation.compose.currentBackStackEntryAsState
import androidx.navigation.compose.rememberNavController
import id.pejuang.hris.ui.BrandGradient
import id.pejuang.hris.ui.CaptureScreen
import id.pejuang.hris.ui.HistoryScreen
import id.pejuang.hris.ui.HomeScreen
import id.pejuang.hris.ui.HrisTheme
import id.pejuang.hris.ui.LoginScreen
import id.pejuang.hris.ui.LogoMark
import id.pejuang.hris.ui.StatusBarIcons
import id.pejuang.hris.update.UpdatePrompt

class MainActivity : ComponentActivity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        enableEdgeToEdge()
        super.onCreate(savedInstanceState)
        val app = application as HrisApp
        setContent {
            HrisTheme {
                var crash by remember { mutableStateOf(app.lastCrash) }
                val dismissCrash = {
                    crash = null
                    app.lastCrash = null
                }
                val nav = rememberNavController()
                NavHost(navController = nav, startDestination = "start") {
                    composable("start") {
                        Splash()
                        LaunchedEffect(Unit) {
                            nav.replaceWith(if (app.repository.resume()) "home" else "login")
                        }
                    }
                    composable("login") {
                        LoginScreen(app.repository, crash, dismissCrash) { nav.replaceWith("home") }
                    }
                    composable("home") { entry ->
                        HomeScreen(
                            repository = app.repository,
                            crash = crash,
                            onDismissCrash = dismissCrash,
                            onCapture = { purpose -> if (entry.isActive()) nav.navigate("capture/$purpose") },
                            onHistory = { if (entry.isActive()) nav.navigate("history") },
                            onLoggedOut = { nav.replaceWith("login") },
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
                    composable("history") { entry ->
                        HistoryScreen(app.repository) { if (entry.isActive()) nav.popBackStack() }
                    }
                }
                val activeEntry by nav.currentBackStackEntryAsState()
                val route = activeEntry?.destination?.route
                UpdatePrompt(allowDialog = route == "login" || route == "home")
            }
        }
    }
}

/** Mencegah ketukan ganda memicu navigasi dua kali (mis. kembali melewati beranda). */
private fun NavBackStackEntry.isActive() = lifecycle.currentState.isAtLeast(Lifecycle.State.RESUMED)

@Composable
private fun Splash() {
    StatusBarIcons(dark = false)
    Column(
        Modifier.fillMaxSize().background(BrandGradient),
        horizontalAlignment = Alignment.CenterHorizontally,
        verticalArrangement = Arrangement.Center,
    ) {
        LogoMark(size = 84.dp)
        Spacer(Modifier.height(20.dp))
        Text("HRIS Pejuang", style = MaterialTheme.typography.headlineMedium, color = Color.White)
        Spacer(Modifier.height(32.dp))
        CircularProgressIndicator(color = Color.White, strokeWidth = 2.dp, modifier = Modifier.size(24.dp))
    }
}

/** Pindah ke [route] dan hapus seluruh riwayat navigasi sebelumnya. */
private fun NavHostController.replaceWith(route: String) {
    navigate(route) {
        popUpTo(graph.id) { inclusive = true }
        launchSingleTop = true
    }
}
