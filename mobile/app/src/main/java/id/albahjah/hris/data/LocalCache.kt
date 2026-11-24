package id.albahjah.hris.data

import android.content.Context
import android.security.keystore.KeyGenParameterSpec
import android.security.keystore.KeyProperties
import kotlinx.serialization.KSerializer
import java.io.File
import java.security.KeyStore
import javax.crypto.Cipher
import javax.crypto.KeyGenerator
import javax.crypto.SecretKey
import javax.crypto.spec.GCMParameterSpec

/** Data dari cache beserta waktu terakhir diperbarui dari server (epoch milidetik). */
data class Cached<T>(val value: T, val savedAt: Long)

/**
 * Salinan lokal data yang sudah pernah dimuat (D62), agar aplikasi tetap bisa dibuka dan dibaca saat
 * internet putus. Isinya data pribadi, jadi setiap berkas dienkripsi AES-GCM dengan kunci di Android
 * Keystore dan disimpan di folder privat aplikasi. Dihapus saat keluar.
 */
class LocalCache(context: Context) {
    private val dir = File(context.filesDir, "cache-v1").apply { mkdirs() }
    private val blobs = File(dir, "blobs").apply { mkdirs() }

    private fun key(): SecretKey {
        val keyStore = KeyStore.getInstance(KEYSTORE).apply { load(null) }
        (keyStore.getEntry(ALIAS, null) as? KeyStore.SecretKeyEntry)?.let { return it.secretKey }
        val generator = KeyGenerator.getInstance(KeyProperties.KEY_ALGORITHM_AES, KEYSTORE)
        generator.init(
            KeyGenParameterSpec.Builder(ALIAS, KeyProperties.PURPOSE_ENCRYPT or KeyProperties.PURPOSE_DECRYPT)
                .setBlockModes(KeyProperties.BLOCK_MODE_GCM)
                .setEncryptionPaddings(KeyProperties.ENCRYPTION_PADDING_NONE)
                .setKeySize(256)
                .build(),
        )
        return generator.generateKey()
    }

    private fun seal(plain: ByteArray): ByteArray {
        val cipher = Cipher.getInstance(TRANSFORMATION)
        cipher.init(Cipher.ENCRYPT_MODE, key())
        return cipher.iv + cipher.doFinal(plain)
    }

    private fun open(sealed: ByteArray): ByteArray {
        val cipher = Cipher.getInstance(TRANSFORMATION)
        cipher.init(Cipher.DECRYPT_MODE, key(), GCMParameterSpec(128, sealed, 0, IV_SIZE))
        return cipher.doFinal(sealed, IV_SIZE, sealed.size - IV_SIZE)
    }

    /** Nama berkas aman dari kunci cache: hanya huruf, angka, titik, minus, garis bawah. */
    private fun safe(key: String): String = sha256Hex(key.toByteArray(Charsets.UTF_8))

    /** Menulis atomik: berkas sementara lalu diganti, agar cache tidak rusak bila aplikasi tertutup. */
    private fun write(file: File, bytes: ByteArray) {
        val tmp = File(file.parentFile, file.name + ".tmp")
        tmp.writeBytes(seal(bytes))
        if (!tmp.renameTo(file)) {
            file.delete()
            tmp.renameTo(file)
        }
    }

    private fun read(file: File): ByteArray? {
        if (!file.exists()) return null
        return try {
            open(file.readBytes())
        } catch (e: Exception) {
            // Kunci berganti atau berkas rusak: buang saja, nanti dimuat ulang dari server.
            file.delete()
            null
        }
    }

    @Synchronized
    fun <T> put(key: String, serializer: KSerializer<T>, value: T) {
        runCatching { write(File(dir, safe(key)), apiJson.encodeToString(serializer, value).toByteArray(Charsets.UTF_8)) }
    }

    @Synchronized
    fun <T> get(key: String, serializer: KSerializer<T>): Cached<T>? {
        val file = File(dir, safe(key))
        val raw = read(file) ?: return null
        return runCatching { Cached(apiJson.decodeFromString(serializer, raw.toString(Charsets.UTF_8)), file.lastModified()) }.getOrNull()
    }

    /** Berkas besar (lampiran, foto) dengan batas total; yang paling lama tidak dipakai dibuang lebih dulu. */
    @Synchronized
    fun putBlob(key: String, bytes: ByteArray) {
        if (bytes.size > MAX_BLOBS_BYTES) return
        runCatching {
            write(File(blobs, safe(key)), bytes)
            trimBlobs()
        }
    }

    @Synchronized
    fun getBlob(key: String): ByteArray? {
        val file = File(blobs, safe(key))
        return read(file)?.also { file.setLastModified(System.currentTimeMillis()) }
    }

    private fun trimBlobs() {
        val files = blobs.listFiles()?.filter { it.isFile } ?: return
        var total = files.sumOf { it.length() }
        for (f in files.sortedBy { it.lastModified() }) {
            if (total <= MAX_BLOBS_BYTES) break
            total -= f.length()
            f.delete()
        }
    }

    /** Menghapus semua cache, dipanggil saat keluar agar data tidak tertinggal untuk akun lain. */
    @Synchronized
    fun clear() {
        dir.listFiles()?.forEach { if (it.isFile) it.delete() }
        blobs.listFiles()?.forEach { it.delete() }
    }

    private companion object {
        const val KEYSTORE = "AndroidKeyStore"
        const val ALIAS = "hris_cache"
        const val TRANSFORMATION = "AES/GCM/NoPadding"
        const val IV_SIZE = 12
        const val MAX_BLOBS_BYTES = 50L * 1024 * 1024
    }
}
