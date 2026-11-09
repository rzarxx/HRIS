package id.pejuang.hris

import android.app.Application
import id.pejuang.hris.data.Repository
import id.pejuang.hris.data.SecureStore
import id.pejuang.hris.data.Session
import id.pejuang.hris.location.LocationReader

/** Wadah dependensi sederhana untuk seluruh aplikasi. */
class HrisApp : Application() {
    lateinit var session: Session
        private set
    lateinit var repository: Repository
        private set
    lateinit var location: LocationReader
        private set

    override fun onCreate() {
        super.onCreate()
        session = Session(this, SecureStore(this))
        repository = Repository(session)
        location = LocationReader(this)
    }
}
