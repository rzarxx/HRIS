package id.pejuang.hris.data

import android.content.Context
import java.security.MessageDigest
import java.util.UUID

/**
 * Status masuk aplikasi. Access token hanya di memori; refresh token di [SecureStore] (MASTER.md
 * §5.9). Alamat server dan ID instalasi disimpan biasa karena bukan rahasia.
 */
class Session(context: Context, private val secure: SecureStore) {
    private val prefs = context.getSharedPreferences("hris_session", Context.MODE_PRIVATE)

    @Volatile
    var accessToken: String? = null
        private set

    var refreshToken: String?
        get() = secure.get(KEY_REFRESH)
        private set(value) = secure.put(KEY_REFRESH, value)

    var deviceId: String?
        get() = prefs.getString(KEY_DEVICE, null)
        private set(value) {
            prefs.edit().putString(KEY_DEVICE, value).apply()
        }

    var serverUrl: String?
        get() = prefs.getString(KEY_SERVER, null)
        set(value) {
            prefs.edit().putString(KEY_SERVER, value).apply()
        }

    val loggedIn: Boolean get() = refreshToken != null

    /**
     * Hash ID instalasi acak, bukan IMEI (§5.9). Memasang ulang aplikasi membuat ID baru, sehingga
     * HP tercatat sebagai perangkat baru dan perlu disetujui lagi.
     */
    val deviceHash: String
        get() {
            val id = prefs.getString(KEY_INSTALLATION, null) ?: UUID.randomUUID().toString().also {
                prefs.edit().putString(KEY_INSTALLATION, it).apply()
            }
            return sha256Hex(id.toByteArray(Charsets.UTF_8))
        }

    @Synchronized
    fun save(tokens: MobileTokens) {
        accessToken = tokens.accessToken
        refreshToken = tokens.refreshToken
        deviceId = tokens.deviceId
    }

    @Synchronized
    fun clear() {
        accessToken = null
        refreshToken = null
    }

    private companion object {
        const val KEY_REFRESH = "refresh_token"
        const val KEY_DEVICE = "device_id"
        const val KEY_SERVER = "server_url"
        const val KEY_INSTALLATION = "installation_id"
    }
}

fun sha256Hex(data: ByteArray): String =
    MessageDigest.getInstance("SHA-256").digest(data).joinToString("") { "%02x".format(it) }
