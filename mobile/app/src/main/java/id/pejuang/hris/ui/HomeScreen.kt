package id.pejuang.hris.ui

import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.material3.Button
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import id.pejuang.hris.data.Me
import id.pejuang.hris.data.PURPOSE_CLOCK_IN
import id.pejuang.hris.data.PURPOSE_CLOCK_OUT
import id.pejuang.hris.data.PURPOSE_FACE_ENROLL
import id.pejuang.hris.data.Repository
import id.pejuang.hris.data.toApiError
import kotlinx.coroutines.launch

@Composable
fun HomeScreen(
    repository: Repository,
    onCapture: (String) -> Unit,
    onHistory: () -> Unit,
    onLoggedOut: () -> Unit,
) {
    val scope = rememberCoroutineScope()
    var me by remember { mutableStateOf<Me?>(null) }
    var error by remember { mutableStateOf<String?>(null) }
    var reload by remember { mutableIntStateOf(0) }

    LaunchedEffect(reload) {
        error = null
        try {
            me = repository.me()
        } catch (e: Exception) {
            val err = e.toApiError()
            if (err.code == "AUTH_TOKEN_INVALID" || !repository.loggedIn) {
                repository.logout()
                onLoggedOut()
            } else {
                error = err.message
            }
        }
    }

    Screen("HRIS Pejuang") {
        val current = me
        if (current == null) {
            if (error == null) CircularProgressIndicator() else {
                ErrorText(error)
                Button(onClick = { reload++ }) { Text("Coba lagi") }
            }
            return@Screen
        }

        InfoCard(
            current.fullName,
            listOfNotNull("NIPY ${current.nip}", current.position, "${current.subdivision.name} · ${current.division.name}")
                .joinToString("\n"),
        )

        val deviceApproved = current.deviceStatus == "approved"
        when (current.deviceStatus) {
            "approved" -> {}
            "pending" -> InfoCard("HP menunggu persetujuan", "SDM perlu menyetujui HP ini sebelum Anda bisa absen.", Amber)
            else -> InfoCard("HP tidak aktif", "HP ini sudah dicabut. Hubungi SDM.", Red)
        }
        if (current.mustChangePassword) {
            InfoCard("Ganti kata sandi", "Ganti kata sandi lewat web admin sebelum memakai aplikasi.", Amber)
        }

        val ready = deviceApproved && !current.mustChangePassword
        if (current.faceEnrolled) {
            InfoCard("Wajah terdaftar", "Absen memakai selfie dan gerakan wajah.", LightGreen)
        } else {
            InfoCard("Wajah belum terdaftar", "Daftarkan wajah di lokasi kantor sebelum absen pertama.", Amber)
            Button(enabled = ready, onClick = { onCapture(PURPOSE_FACE_ENROLL) }, modifier = Modifier.fillMaxWidth()) {
                Text("Daftarkan wajah")
            }
        }

        val canClock = ready && current.faceEnrolled
        Button(enabled = canClock, onClick = { onCapture(PURPOSE_CLOCK_IN) }, modifier = Modifier.fillMaxWidth()) {
            Text("Absen masuk")
        }
        Button(enabled = canClock, onClick = { onCapture(PURPOSE_CLOCK_OUT) }, modifier = Modifier.fillMaxWidth()) {
            Text("Absen pulang")
        }
        OutlinedButton(onClick = onHistory, modifier = Modifier.fillMaxWidth()) { Text("Riwayat absen") }
        OutlinedButton(onClick = { reload++ }, modifier = Modifier.fillMaxWidth()) { Text("Muat ulang") }
        OutlinedButton(
            onClick = {
                scope.launch {
                    repository.logout()
                    onLoggedOut()
                }
            },
            modifier = Modifier.fillMaxWidth(),
        ) { Text("Keluar") }
    }
}
