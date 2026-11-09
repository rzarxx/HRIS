package id.pejuang.hris.ui

import androidx.annotation.RawRes
import androidx.compose.foundation.Canvas
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.offset
import androidx.compose.foundation.layout.size
import androidx.compose.runtime.Composable
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.drawscope.drawIntoCanvas
import androidx.compose.ui.graphics.nativeCanvas
import androidx.compose.ui.graphics.graphicsLayer
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import com.caverock.androidsvg.SVG
import id.pejuang.hris.R

/** Dekorasi banner asli dari desain Figma. */
@Composable
private fun Star(@RawRes id: Int, size: Dp, rotation: Float, right: Dp, top: Dp) {
    val context = LocalContext.current
    val picture = remember(id) { context.resources.openRawResource(id).use { SVG.getFromInputStream(it).renderToPicture() } }
    Box(Modifier.offset(x = -right, y = top), contentAlignment = Alignment.Center) {
        Canvas(Modifier.size(size).graphicsLayer(rotationZ = rotation)) {
            drawIntoCanvas { canvas ->
                val native = canvas.nativeCanvas
                val count = native.save()
                native.scale(this.size.width / picture.width, this.size.height / picture.height)
                native.drawPicture(picture)
                native.restoreToCount(count)
            }
        }
    }
}

@Composable
fun FigmaBannerStars() {
    // Box induk berukuran nol di sudut kanan atas banner.
    Star(R.raw.figma_star_large, 21.395.dp, -30.21f, 71.67.dp, 39.93.dp)
    Star(R.raw.figma_star_a, 16.969.dp, 19.21f, 52.71.dp, 18.32.dp)
    Star(R.raw.figma_star_b, 8.341.dp, -18.39f, 58.55.dp, 71.1.dp)
    Star(R.raw.figma_star_c, 8.341.dp, 8.77f, 33.07.dp, 17.59.dp)
    Star(R.raw.figma_star_d, 4.161.dp, 19.89f, 66.25.dp, 36.58.dp)
    Star(R.raw.figma_star_d, 4.161.dp, 19.89f, 39.25.dp, 72.58.dp)
    Star(R.raw.figma_star_d, 4.161.dp, 19.89f, 44.25.dp, 29.58.dp)
}
