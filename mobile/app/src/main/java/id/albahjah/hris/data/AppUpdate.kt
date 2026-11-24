package id.albahjah.hris.data

/** Tingkat pembaruan aplikasi dibanding versi di server (D63). */
enum class UpdateLevel {
    /** Sudah versi terbaru, atau versi belum diatur super admin. */
    None,

    /** Ada versi baru; aplikasi tetap bisa dipakai. */
    Available,

    /** Di bawah versi minimum: harus diperbarui sebelum bisa dipakai. */
    Required,
}

data class AppUpdateState(val level: UpdateLevel, val version: AppVersion)

/** Halaman aplikasi di Play Store, dipakai bila server tidak memberi tautan lain. */
const val DEFAULT_STORE_URL = "https://play.google.com/store/apps/details?id=id.albahjah.hris"

/** Pemeriksaan otomatis paling sering sekali per jam; pemeriksaan dari Profil selalu dijalankan. */
const val APP_VERSION_CHECK_MS = 60L * 60 * 1000

fun updateLevel(installedCode: Int, version: AppVersion): UpdateLevel = when {
    version.minCode > 0 && installedCode < version.minCode -> UpdateLevel.Required
    version.latestCode > installedCode -> UpdateLevel.Available
    else -> UpdateLevel.None
}

/** Isi header X-App-Version: "versionName+versionCode" (MASTER.md §7.2). */
fun appVersionHeader(name: String, code: Int): String = "$name+$code"
