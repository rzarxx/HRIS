import org.jetbrains.kotlin.gradle.dsl.JvmTarget
import java.net.URI

plugins {
    id("com.android.application")
    id("org.jetbrains.kotlin.android")
    id("org.jetbrains.kotlin.plugin.compose")
    id("org.jetbrains.kotlin.plugin.serialization")
    // Render layar ke PNG di tes unit (tanpa emulator) untuk ditinjau di CI.
    id("app.cash.paparazzi")
}

// Alamat API untuk build rilis Play Store (Android Studio) bawaan ke https://api.albahjah.or.id.
// Saat dibuild di GitHub Actions (release.yml), ditimpa oleh -Phris.apiBaseUrl=$HRIS_RELEASE_API_BASE_URL (domain .net).
// Build debug memakai https://api.albahjah.net sebagai isian awal dan tetap bisa diganti di layar masuk.
val customApiBaseUrl = (findProperty("hris.apiBaseUrl") as String?)?.trim()
val releaseApiBaseUrl = if (!customApiBaseUrl.isNullOrEmpty()) customApiBaseUrl else "https://api.albahjah.or.id"
val debugApiBaseUrl = if (!customApiBaseUrl.isNullOrEmpty()) customApiBaseUrl else "https://api.albahjah.net"
val releaseVersionName = (findProperty("hris.versionName") as String?) ?: "1.0.0"
val releaseVersionCode = (findProperty("hris.versionCode") as String?)?.let {
    requireNotNull(it.toIntOrNull()) { "hris.versionCode harus berupa bilangan bulat" }
} ?: 1000000
val playCloudProjectNumber = (findProperty("hris.playCloudProjectNumber") as String?)?.let {
    requireNotNull(it.toLongOrNull()) { "hris.playCloudProjectNumber harus berupa bilangan bulat" }
} ?: 0L
require(Regex("^(0|[1-9][0-9]*)\\.(0|[1-9][0-9]*)\\.(0|[1-9][0-9]*)$").matches(releaseVersionName)) { "hris.versionName harus berupa versi stabil x.y.z" }
require(releaseVersionCode > 0) { "hris.versionCode harus positif" }
require(playCloudProjectNumber >= 0L) { "hris.playCloudProjectNumber tidak boleh negatif" }
for (url in listOf(releaseApiBaseUrl, debugApiBaseUrl)) {
    val uri = URI(url)
    require(uri.scheme == "https" && !uri.host.isNullOrBlank() && uri.userInfo == null &&
        uri.query == null && uri.fragment == null && !url.contains('"') && !url.contains('\\')) {
        "hris.apiBaseUrl harus berupa URL HTTPS tanpa kredensial, query, atau fragment"
    }
}

android {
    namespace = "id.pejuang.hris"
    compileSdk = 35

    defaultConfig {
        applicationId = "id.pejuang.hris"
        minSdk = 26
        // Naikkan mengikuti batas target API Google Play sebelum rilis.
        targetSdk = 35
        versionCode = releaseVersionCode
        versionName = releaseVersionName
        buildConfigField("long", "PLAY_CLOUD_PROJECT_NUMBER", "${playCloudProjectNumber}L")
        testInstrumentationRunner = "androidx.test.runner.AndroidJUnitRunner"
        // Hanya prosesor ARM (semua HP Android yang dituju). Pustaka x86 hanya terpakai emulator dan
        // menambah sekitar 20 MB.
        ndk {
            abiFilters += listOf("arm64-v8a", "armeabi-v7a")
        }
    }

    buildTypes {
        debug {
            applicationIdSuffix = ".debug"
            buildConfigField("String", "API_BASE_URL", "\"$debugApiBaseUrl\"")
            buildConfigField("boolean", "SERVER_EDITABLE", "true")
        }
        release {
            isMinifyEnabled = false
            buildConfigField("String", "API_BASE_URL", "\"$releaseApiBaseUrl\"")
            buildConfigField("boolean", "SERVER_EDITABLE", "false")
        }
    }

    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_17
        targetCompatibility = JavaVersion.VERSION_17
    }

    buildFeatures {
        compose = true
        buildConfig = true
    }

    packaging {
        resources.excludes += "/META-INF/{AL2.0,LGPL2.1}"
        // Pustaka native dikompresi dan diekstrak saat pemasangan, cara yang didukung semua pemasang.
        // Diduga pemasang bawaan Infinix XOS (Android 16) menolak pustaka tanpa kompresi sebagai
        // "paket tidak valid"; dugaan ini belum dipastikan dengan kode galat adb.
        jniLibs.useLegacyPackaging = true
    }
}

kotlin {
    compilerOptions {
        jvmTarget.set(JvmTarget.JVM_17)
    }
}

dependencies {
    val composeBom = platform("androidx.compose:compose-bom:2024.12.01")
    implementation(composeBom)
    implementation("androidx.compose.ui:ui")
    implementation("androidx.compose.foundation:foundation")
    implementation("androidx.compose.material3:material3")
    implementation("androidx.compose.material:material-icons-core")
    implementation("androidx.compose.ui:ui-tooling-preview")
    debugImplementation("androidx.compose.ui:ui-tooling")

    implementation("androidx.core:core-ktx:1.15.0")
    implementation("androidx.activity:activity-compose:1.9.3")
    implementation("androidx.lifecycle:lifecycle-viewmodel-compose:2.8.7")
    implementation("androidx.lifecycle:lifecycle-runtime-compose:2.8.7")
    implementation("androidx.navigation:navigation-compose:2.8.5")
    // Blur latar (glassmorphism) untuk bottom bar; Android 12+ memakai RenderEffect, versi lama memakai tint.
    implementation("dev.chrisbanes.haze:haze:1.2.2")

    implementation("com.squareup.retrofit2:retrofit:2.11.0")
    implementation("com.squareup.retrofit2:converter-kotlinx-serialization:2.11.0")
    implementation("com.squareup.okhttp3:okhttp:4.12.0")
    implementation("org.jetbrains.kotlinx:kotlinx-serialization-json:1.7.3")
    implementation("org.jetbrains.kotlinx:kotlinx-coroutines-play-services:1.9.0")

    val camerax = "1.4.1"
    implementation("androidx.camera:camera-camera2:$camerax")
    implementation("androidx.camera:camera-lifecycle:$camerax")
    implementation("androidx.camera:camera-view:$camerax")
    // Model deteksi wajah dibundel di APK agar jalan juga di HP tanpa layanan Google (§5.4.1).
    implementation("com.google.mlkit:face-detection:16.1.7")
    implementation("com.google.android.gms:play-services-location:21.3.0")
    implementation("com.google.android.play:integrity:1.4.0")

    testImplementation("junit:junit:4.13.2")
}
