package id.pejuang.hris.location

import android.annotation.SuppressLint
import android.content.Context
import android.location.Location
import android.os.Build
import com.google.android.gms.location.LocationServices
import com.google.android.gms.location.Priority
import com.google.android.gms.tasks.CancellationTokenSource
import kotlinx.coroutines.tasks.await
import kotlin.math.roundToLong

/** Satu titik lokasi saat presensi. Koordinat dibulatkan 6 desimal sesuai kontrak (§5.4). */
data class Fix(
    val latitude: Double,
    val longitude: Double,
    val accuracyM: Double,
    val isMock: Boolean,
)

fun roundCoordinate(value: Double): Double = (value * 1_000_000).roundToLong() / 1_000_000.0

class LocationReader(context: Context) {
    private val client = LocationServices.getFusedLocationProviderClient(context)

    /** Izin lokasi harus sudah diberikan sebelum dipanggil. */
    @SuppressLint("MissingPermission")
    suspend fun current(): Fix {
        val source = CancellationTokenSource()
        val location: Location = client.getCurrentLocation(Priority.PRIORITY_HIGH_ACCURACY, source.token).await()
            ?: throw IllegalStateException("Lokasi belum didapat. Pastikan GPS menyala lalu coba lagi.")
        val mock = if (Build.VERSION.SDK_INT >= 31) {
            location.isMock
        } else {
            @Suppress("DEPRECATION")
            location.isFromMockProvider
        }
        return Fix(
            latitude = roundCoordinate(location.latitude),
            longitude = roundCoordinate(location.longitude),
            accuracyM = if (location.hasAccuracy()) location.accuracy.toDouble() else 9999.0,
            isMock = mock,
        )
    }
}
