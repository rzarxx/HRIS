package id.pejuang.hris

import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.safeDrawingPadding
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.Surface
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.ui.Modifier
import androidx.navigation.NavHostController
import androidx.navigation.compose.NavHost
import androidx.navigation.compose.composable
import androidx.navigation.compose.rememberNavController
import id.pejuang.hris.ui.CaptureScreen
import id.pejuang.hris.ui.HistoryScreen
import id.pejuang.hris.ui.HomeScreen
import id.pejuang.hris.ui.HrisTheme
import id.pejuang.hris.ui.LoginScreen
import id.pejuang.hris.ui.Screen

class MainActivity : ComponentActivity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        val app = application as HrisApp
        setContent {
            HrisTheme {
                Surface(modifier = Modifier.fillMaxSize().safeDrawingPadding()) {
                    val nav = rememberNavController()
                    NavHost(navController = nav, startDestination = "start") {
                        composable("start") {
                            Screen("HRIS Pejuang") { CircularProgressIndicator() }
                            LaunchedEffect(Unit) {
                                nav.replaceWith(if (app.repository.resume()) "home" else "login")
                            }
                        }
                        composable("login") {
                            LoginScreen(app.repository) { nav.replaceWith("home") }
                        }
                        composable("home") {
                            HomeScreen(
                                repository = app.repository,
                                onCapture = { purpose -> nav.navigate("capture/$purpose") },
                                onHistory = { nav.navigate("history") },
                                onLoggedOut = { nav.replaceWith("login") },
                            )
                        }
                        composable("capture/{purpose}") { entry ->
                            CaptureScreen(
                                purpose = entry.arguments?.getString("purpose").orEmpty(),
                                repository = app.repository,
                                location = app.location,
                                onClose = { nav.popBackStack() },
                            )
                        }
                        composable("history") {
                            HistoryScreen(app.repository) { nav.popBackStack() }
                        }
                    }
                }
            }
        }
    }
}

/** Pindah ke [route] dan hapus seluruh riwayat navigasi sebelumnya. */
private fun NavHostController.replaceWith(route: String) {
    navigate(route) {
        popUpTo(graph.id) { inclusive = true }
        launchSingleTop = true
    }
}
