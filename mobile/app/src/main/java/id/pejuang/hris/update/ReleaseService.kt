package id.pejuang.hris.update

import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import kotlinx.serialization.json.Json
import kotlinx.serialization.json.jsonArray
import kotlinx.serialization.json.jsonObject
import kotlinx.serialization.json.jsonPrimitive
import okhttp3.OkHttpClient
import okhttp3.Request
import java.io.IOException
import java.net.URI
import java.util.concurrent.TimeUnit
import id.pejuang.hris.BuildConfig

/** Hanya membaca rilis publik stabil dan satu aset APK yang diterbitkan workflow proyek. */
class ReleaseService(private val client: OkHttpClient = OkHttpClient.Builder()
    .followRedirects(false).connectTimeout(10, TimeUnit.SECONDS).readTimeout(15, TimeUnit.SECONDS).build()) {
    suspend fun latest(currentVersion: String): ReleaseInfo? = withContext(Dispatchers.IO) {
        val current = AppVersion.parse(currentVersion) ?: throw IOException("Versi aplikasi tidak valid")
        val request = Request.Builder().url("https://api.github.com/repos/${BuildConfig.UPDATE_REPO}/releases/latest").header("Accept", "application/vnd.github+json")
            .header("X-GitHub-Api-Version", "2022-11-28").build()
        client.newCall(request).execute().use { response ->
            if (response.code == 404) return@withContext null
            if (!response.isSuccessful) throw IOException("GitHub tidak tersedia (${response.code})")
            val body = response.body ?: throw IOException("Respons GitHub kosong")
            val source = body.source()
            source.request(MAX_METADATA_BYTES + 1)
            if (source.buffer.size > MAX_METADATA_BYTES) throw IOException("Respons GitHub terlalu besar")
            val root = try { Json.parseToJsonElement(source.readUtf8()).jsonObject } catch (_: Exception) {
                throw IOException("Respons GitHub tidak valid")
            }
            try {
                if (root["draft"]?.jsonPrimitive?.content != "false" || root["prerelease"]?.jsonPrimitive?.content != "false") {
                    throw IOException("Rilis bukan versi stabil")
                }
                val tag = root.getValue("tag_name").jsonPrimitive.content
                if (!tag.startsWith("v")) throw IOException("Tag rilis tidak valid")
                val version = AppVersion.parse(tag) ?: throw IOException("Tag rilis tidak valid")
                if (version <= current) return@withContext null
                val assets = root.getValue("assets").jsonArray.map { it.jsonObject }
                    .filter { it["name"]?.jsonPrimitive?.content == APK_NAME && it["state"]?.jsonPrimitive?.content == "uploaded" }
                if (assets.size != 1) throw IOException("APK rilis tidak ditemukan atau tidak unik")
                val asset = assets.single()
                val url = asset.getValue("browser_download_url").jsonPrimitive.content
                val uri = URI(url)
                if (uri.scheme != "https" || uri.host != "github.com" || uri.port != -1 || uri.userInfo != null ||
                    uri.rawQuery != null || uri.rawFragment != null || uri.rawPath != "/${BuildConfig.UPDATE_REPO}/releases/download/$tag/$APK_NAME") {
                    throw IOException("Alamat APK tidak valid")
                }
                val digest = asset.getValue("digest").jsonPrimitive.content
                if (!Regex("^sha256:[0-9a-f]{64}$").matches(digest)) throw IOException("Checksum APK tidak tersedia")
                val size = asset.getValue("size").jsonPrimitive.content.toLong()
                if (size <= 0 || size > MAX_APK_BYTES) throw IOException("Ukuran APK tidak valid")
                ReleaseInfo(tag, url, digest.removePrefix("sha256:"), size)
            } catch (e: IOException) { throw e } catch (_: Exception) {
                throw IOException("Metadata rilis tidak lengkap atau tidak valid")
            }
        }
    }
    companion object {
        const val APK_NAME = "hris-pejuang-release.apk"
        const val MAX_APK_BYTES = 200L * 1024 * 1024
        private const val MAX_METADATA_BYTES = 256L * 1024
    }
}
