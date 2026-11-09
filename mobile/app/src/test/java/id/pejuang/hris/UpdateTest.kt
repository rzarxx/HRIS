package id.pejuang.hris.update

import kotlinx.coroutines.runBlocking
import okhttp3.OkHttpClient
import okhttp3.Protocol
import okhttp3.Response
import okhttp3.ResponseBody.Companion.toResponseBody
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test
import java.io.IOException

class UpdateTest {
    @Test fun versionsAreNumericAndStable() {
        assertTrue(AppVersion.parse("v1.10.0")!! > AppVersion.parse("1.9.99")!!)
        assertTrue(AppVersion.parse("1.0.0")!! > AppVersion.parse("0.99.99")!!)
        for (bad in listOf("1.2", "v1.2.3-beta", "1.02.3", "1.2.3.4", "v1.2.x", "999999999999.0.0")) assertNull(AppVersion.parse(bad))
    }

    private fun service(body: String, status: Int = 200): ReleaseService {
        val client = OkHttpClient.Builder().addInterceptor { chain ->
            Response.Builder().request(chain.request()).protocol(Protocol.HTTP_1_1).code(status).message("test")
                .body(body.toResponseBody()).build()
        }.build()
        return ReleaseService(client)
    }
    private fun release(url: String = "https://github.com/rzarxx/HRIS/releases/download/v1.2.0/hris-pejuang-release.apk", digest: String = "sha256:" + "a".repeat(64)) =
        """{"draft":false,"prerelease":false,"tag_name":"v1.2.0","assets":[{"name":"hris-pejuang-release.apk","state":"uploaded","browser_download_url":"$url","digest":"$digest","size":1234}]}"""

    @Test fun acceptsValidReleaseAndIgnoresOlder() = runBlocking {
        assertEquals("v1.2.0", service(release()).latest("1.1.9")?.tag)
        assertNull(service(release()).latest("1.2.0"))
        assertNull(service("{}", 404).latest("1.0.0"))
    }

    @Test fun rejectsUntrustedAssetAndMalformedMetadata() = runBlocking {
        val invalid = listOf(
            release(url = "https://attacker.invalid/hris-pejuang-release.apk"),
            release(url = "http://github.com/rzarxx/HRIS/releases/download/v1.2.0/hris-pejuang-release.apk"),
            release(digest = "sha256:bad"),
            "{broken",
            release().replace("\"prerelease\":false", "\"prerelease\":true"),
        )
        for (body in invalid) {
            try { service(body).latest("1.0.0"); throw AssertionError("metadata harus ditolak") }
            catch (_: IOException) { }
        }
    }
}
