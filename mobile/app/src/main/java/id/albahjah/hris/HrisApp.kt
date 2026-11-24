package id.albahjah.hris

import android.app.Application
import androidx.compose.runtime.mutableStateOf
import id.albahjah.hris.data.AppPrefs
import id.albahjah.hris.data.LocalCache
import id.albahjah.hris.data.Repository
import id.albahjah.hris.data.SecureStore
import id.albahjah.hris.data.Session
import id.albahjah.hris.location.LocationReader
import id.albahjah.hris.security.DeviceIdentity

/** Wadah dependensi sederhana untuk seluruh aplikasi. */
class HrisApp : Application() {
    lateinit var session: Session
        private set
    lateinit var repository: Repository
        private set
    lateinit var location: LocationReader
        private set
    lateinit var prefs: AppPrefs
        private set

    /** Ringkasan crash dari pemakaian sebelumnya; ditampilkan sekali lalu dibuang. */
    var lastCrash: String? = null

    /**
     * Status kunci aplikasi (D62) disimpan di sini, bukan di Activity, agar memutar layar tidak mengunci
     * ulang. [started] false berarti proses baru dibuka; [stoppedAt] waktu terakhir masuk latar (elapsedRealtime).
     */
    val locked = mutableStateOf(false)
    var started = false
    var stoppedAt: Long? = null

    override fun onCreate() {
        super.onCreate()
        lastCrash = CrashLog.take(this)
        CrashLog.install(this)
        session = Session(this, SecureStore(this))
        prefs = AppPrefs(this)
        repository = Repository(session, prefs, DeviceIdentity(this), LocalCache(this))
        location = LocationReader(this)
    }
}
