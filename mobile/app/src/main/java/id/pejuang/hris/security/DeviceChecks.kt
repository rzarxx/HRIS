package id.pejuang.hris.security

import android.content.Context
import android.content.pm.PackageManager
import android.os.Build
import java.io.File

/** Menjalankan [DeviceRules] di HP ini. */
class DeviceChecks(private val context: Context) {
    fun check(): DeviceRisk {
        val installed = { pkg: String -> isInstalled(pkg) }
        return DeviceRisk(
            rootDetected = DeviceRules.rooted({ path -> runCatching { File(path).exists() }.getOrDefault(false) }, installed),
            emulatorDetected = DeviceRules.emulator(
                BuildInfo(
                    fingerprint = Build.FINGERPRINT.orEmpty(),
                    model = Build.MODEL.orEmpty(),
                    manufacturer = Build.MANUFACTURER.orEmpty(),
                    brand = Build.BRAND.orEmpty(),
                    device = Build.DEVICE.orEmpty(),
                    product = Build.PRODUCT.orEmpty(),
                    hardware = Build.HARDWARE.orEmpty(),
                ),
            ),
            fakeGpsAppDetected = DeviceRules.fakeGpsApp(installed),
        )
    }

    private fun isInstalled(pkg: String): Boolean = try {
        if (Build.VERSION.SDK_INT >= 33) {
            context.packageManager.getPackageInfo(pkg, PackageManager.PackageInfoFlags.of(0))
        } else {
            @Suppress("DEPRECATION")
            context.packageManager.getPackageInfo(pkg, 0)
        }
        true
    } catch (e: PackageManager.NameNotFoundException) {
        false
    }
}
