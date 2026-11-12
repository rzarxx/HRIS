package id.pejuang.hris.security

/**
 * Hasil pemeriksaan HP sebelum presensi (MASTER.md §10.2 aturan 4). Sifatnya saran karena HP yang
 * dimodifikasi bisa menyembunyikannya; server tetap menolak bila salah satunya dilaporkan (§5.4).
 */
data class DeviceRisk(
    val rootDetected: Boolean,
    val emulatorDetected: Boolean,
    val fakeGpsAppDetected: Boolean,
) {
    val blocked: Boolean get() = rootDetected || emulatorDetected || fakeGpsAppDetected

    /** Pesan untuk Pejuang. Tidak menyebut cara melewatinya. */
    val message: String
        get() = when {
            fakeGpsAppDetected -> "Terpasang aplikasi pemalsu lokasi. Hapus aplikasi itu, lalu coba presensi lagi."
            rootDetected -> "HP ini sudah dimodifikasi (root), jadi presensi tidak bisa dilakukan dari HP ini. Hubungi SDM."
            emulatorDetected -> "Presensi hanya bisa dari HP sungguhan, bukan emulator."
            else -> ""
        }

    companion object {
        val CLEAN = DeviceRisk(rootDetected = false, emulatorDetected = false, fakeGpsAppDetected = false)
    }
}

/** Data build HP yang dipakai untuk mengenali emulator. */
data class BuildInfo(
    val fingerprint: String,
    val model: String,
    val manufacturer: String,
    val brand: String,
    val device: String,
    val product: String,
    val hardware: String,
)

/**
 * Aturan deteksi, terpisah dari Android agar bisa dites. Semua aturan sengaja konservatif: deteksi
 * keliru membuat Pejuang yang jujur tidak bisa absen, jadi tanda yang juga ada di HP murah biasa
 * (mis. `Build.TAGS` berisi `test-keys`) tidak dipakai.
 */
object DeviceRules {
    val SU_PATHS = listOf(
        "/system/bin/su", "/system/xbin/su", "/sbin/su", "/system/su", "/su/bin/su",
        "/system/bin/failsafe/su", "/system/sd/xbin/su", "/data/local/su", "/data/local/bin/su",
        "/data/local/xbin/su", "/system/app/Superuser.apk", "/system/app/SuperSU.apk",
    )

    val ROOT_PACKAGES = listOf(
        "com.topjohnwu.magisk", "eu.chainfire.supersu", "com.koushikdutta.superuser",
        "com.noshufou.android.su", "com.thirdparty.superuser", "me.weishu.kernelsu",
        "me.bmax.apatch",
    )

    /** Aplikasi pemalsu lokasi yang umum. Harus sama dengan `<queries>` di AndroidManifest.xml. */
    val FAKE_GPS_PACKAGES = listOf(
        "com.lexa.fakegps", "com.incorporateapps.fakegps.fre", "com.blogspot.newapphorizons.fakegps",
        "com.theappninjas.fakegpsjoystick", "com.theappninjas.gpsjoystick", "com.gsmartstudio.fakegps",
        "com.rosteam.gpsemulator", "com.lkr.fakelocation", "ru.gavrikov.mocklocations",
        "com.evezzon.fgl", "com.just4funtools.fakegpslocationprofessional",
        "com.marlon.floating.fake.location", "com.fakegps.mock",
    )

    fun rooted(fileExists: (String) -> Boolean, installed: (String) -> Boolean): Boolean =
        SU_PATHS.any(fileExists) || ROOT_PACKAGES.any(installed)

    fun fakeGpsApp(installed: (String) -> Boolean): Boolean = FAKE_GPS_PACKAGES.any(installed)

    fun emulator(b: BuildInfo): Boolean {
        val fingerprint = b.fingerprint.lowercase()
        val model = b.model.lowercase()
        val product = b.product.lowercase()
        val hardware = b.hardware.lowercase()
        return fingerprint.startsWith("generic") ||
            fingerprint.contains("/sdk_gphone") ||
            fingerprint.contains("emulator") ||
            model.contains("google_sdk") ||
            model.contains("emulator") ||
            model.contains("android sdk built for") ||
            hardware in setOf("goldfish", "ranchu", "vbox86") ||
            product.startsWith("sdk_gphone") ||
            product in setOf("sdk", "google_sdk", "sdk_x86", "sdk_google", "vbox86p", "emulator", "simulator") ||
            b.manufacturer.contains("Genymotion", ignoreCase = true) ||
            (b.brand.startsWith("generic") && b.device.startsWith("generic"))
    }
}
