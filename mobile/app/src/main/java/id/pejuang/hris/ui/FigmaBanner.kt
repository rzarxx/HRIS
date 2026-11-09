package id.pejuang.hris.ui

import androidx.annotation.DrawableRes
import androidx.compose.foundation.Image
import androidx.compose.foundation.layout.offset
import androidx.compose.foundation.layout.size
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.graphicsLayer
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import id.pejuang.hris.R

/** Dekorasi banner dari aset SVG Figma, dirender menjadi PNG pada tiga kali ukuran asli. */
@Composable
private fun Star(@DrawableRes id: Int, size: Dp, rotation: Float, right: Dp, top: Dp) {
    Image(
        painter = painterResource(id),
        contentDescription = null,
        modifier = Modifier.offset(x = -right, y = top).size(size).graphicsLayer(rotationZ = rotation),
    )
}

@Composable
fun FigmaBannerStars() {
    Star(R.drawable.figma_star_large, 21.395.dp, -30.21f, 71.67.dp, 39.93.dp)
    Star(R.drawable.figma_star_a, 16.969.dp, 19.21f, 52.71.dp, 18.32.dp)
    Star(R.drawable.figma_star_b, 8.341.dp, -18.39f, 58.55.dp, 71.1.dp)
    Star(R.drawable.figma_star_c, 8.341.dp, 8.77f, 33.07.dp, 17.59.dp)
    Star(R.drawable.figma_star_d, 4.161.dp, 19.89f, 66.25.dp, 36.58.dp)
    Star(R.drawable.figma_star_d, 4.161.dp, 19.89f, 39.25.dp, 72.58.dp)
    Star(R.drawable.figma_star_d, 4.161.dp, 19.89f, 44.25.dp, 29.58.dp)
}
