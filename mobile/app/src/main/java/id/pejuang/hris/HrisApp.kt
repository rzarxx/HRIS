package id.pejuang.hris

import android.app.Application
import id.pejuang.hris.data.AppPrefs
import id.pejuang.hris.data.Repository
import id.pejuang.hris.data.SecureStore
import id.pejuang.hris.data.Session
import id.pejuang.hris.location.LocationReader
import id.pejuang.hris.security.DeviceIdentity

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

    override fun onCreate() {
        super.onCreate()
        lastCrash = CrashLog.take(this)
        CrashLog.install(this)
        session = Session(this, SecureStore(this))
        prefs = AppPrefs(this)
        repository = Repository(session, prefs, DeviceIdentity(this))
        location = LocationReader(this)
    }
}
