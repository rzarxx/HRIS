package id.albahjah.hris.ui

import android.content.Context
import android.graphics.Bitmap
import android.graphics.BitmapFactory
import android.graphics.Color
import android.graphics.pdf.PdfRenderer
import android.os.ParcelFileDescriptor
import androidx.compose.foundation.Image
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.aspectRatio
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.Icon
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.produceState
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.ImageBitmap
import androidx.compose.ui.graphics.asImageBitmap
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import id.albahjah.hris.data.AnnouncementAttachment
import id.albahjah.hris.data.Repository
import id.albahjah.hris.data.toApiError
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import java.io.Closeable
import java.io.File
import kotlin.math.max

/** Lebar render halaman PDF dan batas sisi gambar lampiran, cukup tajam untuk layar HP. */
private const val RENDER_WIDTH = 1080
private const val IMAGE_MAX_SIDE = 2048

fun attachmentSizeLabel(bytes: Int): String =
    if (bytes >= 1024 * 1024) "%.1f MB".format(java.util.Locale.ROOT, bytes / 1024f / 1024f) else "${max(1, bytes / 1024)} KB"

/** Baris lampiran di pop-up dan detail pengumuman. */
@Composable
fun AttachmentRow(attachment: AnnouncementAttachment, onClick: () -> Unit) {
    Row(
        Modifier
            .fillMaxWidth()
            .heightIn(min = 52.dp)
            .clip(RoundedCornerShape(14.dp))
            .background(Hc.Field)
            .clickable(onClickLabel = "Buka lampiran", onClick = onClick)
            .padding(horizontal = 14.dp, vertical = 10.dp),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(12.dp),
    ) {
        Icon(FigmaIcons.Scroll, contentDescription = null, tint = Hc.Brand, modifier = Modifier.size(22.dp))
        Column(Modifier.weight(1f)) {
            Text(attachment.fileName, style = figmaText(14, FontWeight.Medium, Hc.Ink), maxLines = 1, overflow = TextOverflow.Ellipsis)
            Text(
                (if (attachment.contentType == "application/pdf") "PDF" else "Gambar") + " · " + attachmentSizeLabel(attachment.sizeBytes),
                style = figmaText(12, color = Hc.InkSoft),
            )
        }
        Text("Buka", style = figmaText(13, FontWeight.SemiBold, Hc.BrandDeep))
    }
}

/** Gambar lampiran diperkecil saat dibaca agar foto besar tidak menghabiskan memori HP. */
fun decodeAttachmentImage(bytes: ByteArray): ImageBitmap? {
    val bounds = BitmapFactory.Options().apply { inJustDecodeBounds = true }
    BitmapFactory.decodeByteArray(bytes, 0, bytes.size, bounds)
    var sample = 1
    while (max(bounds.outWidth, bounds.outHeight) / (sample * 2) >= IMAGE_MAX_SIDE) sample *= 2
    val options = BitmapFactory.Options().apply { inSampleSize = sample }
    return BitmapFactory.decodeByteArray(bytes, 0, bytes.size, options)?.asImageBitmap()
}

/**
 * PDF yang dibuka di dalam aplikasi lewat PdfRenderer bawaan Android, tanpa aplikasi pembaca lain.
 * Halaman dirender saat tampil di layar, satu per satu, karena PdfRenderer hanya boleh membuka satu
 * halaman dalam satu waktu.
 */
private class PdfDocument(context: Context, bytes: ByteArray) : Closeable {
    private val file: File = File.createTempFile("lampiran", ".pdf", context.cacheDir).apply { writeBytes(bytes) }
    private val descriptor = ParcelFileDescriptor.open(file, ParcelFileDescriptor.MODE_READ_ONLY)
    private val renderer = PdfRenderer(descriptor)
    val pageCount: Int get() = renderer.pageCount

    fun ratio(index: Int): Float = synchronized(renderer) {
        renderer.openPage(index).use { it.width.toFloat() / it.height }
    }

    fun render(index: Int): ImageBitmap = synchronized(renderer) {
        renderer.openPage(index).use { page ->
            val height = (RENDER_WIDTH.toFloat() * page.height / page.width).toInt().coerceAtLeast(1)
            val bitmap = Bitmap.createBitmap(RENDER_WIDTH, height, Bitmap.Config.ARGB_8888)
            bitmap.eraseColor(Color.WHITE)
            page.render(bitmap, null, null, PdfRenderer.Page.RENDER_MODE_FOR_DISPLAY)
            bitmap.asImageBitmap()
        }
    }

    override fun close() {
        synchronized(renderer) { renderer.close() }
        descriptor.close()
        file.delete()
    }
}

private sealed interface Loaded {
    data class Image(val bitmap: ImageBitmap) : Loaded
    class Pdf(val doc: PdfDocument, val ratios: List<Float>) : Loaded
}

/** Layar penuh untuk membaca lampiran pengumuman. */
@Composable
fun AttachmentViewerScreen(repository: Repository, announcementId: String, attachment: AnnouncementAttachment, onBack: () -> Unit) {
    val context = LocalContext.current
    var loaded by remember(attachment.id) { mutableStateOf<Loaded?>(null) }
    var error by remember(attachment.id) { mutableStateOf<String?>(null) }
    LaunchedEffect(attachment.id) {
        try {
            val bytes = repository.attachment(announcementId, attachment.id)
            loaded = withContext(Dispatchers.IO) {
                if (attachment.contentType == "application/pdf") {
                    val doc = PdfDocument(context, bytes)
                    Loaded.Pdf(doc, (0 until doc.pageCount).map(doc::ratio))
                } else {
                    Loaded.Image(decodeAttachmentImage(bytes) ?: error("Gambar tidak bisa dibaca"))
                }
            }
        } catch (e: CancellationException) {
            throw e
        } catch (e: Exception) {
            error = e.toApiError().message ?: "Lampiran belum bisa dibuka."
        }
    }
    DisposableEffect(attachment.id) {
        onDispose { (loaded as? Loaded.Pdf)?.doc?.close() }
    }
    Column(Modifier.fillMaxSize().background(Fg.PageProfile)) {
        FigmaHeader(attachment.fileName, onBack)
        val shown = loaded
        when {
            error != null -> CenterMessage(title = "Lampiran belum bisa dibuka", body = error, tone = Tone.Warning)
            shown == null -> CenterMessage(title = "Membuka lampiran", body = null, loading = true)
            shown is Loaded.Image -> Box(Modifier.weight(1f).fillMaxWidth().navigationBarsPadding().padding(12.dp), contentAlignment = Alignment.Center) {
                Image(shown.bitmap, contentDescription = attachment.fileName, contentScale = ContentScale.Fit, modifier = Modifier.fillMaxSize())
            }
            shown is Loaded.Pdf -> LazyColumn(
                Modifier.weight(1f).fillMaxWidth().navigationBarsPadding(),
                contentPadding = androidx.compose.foundation.layout.PaddingValues(12.dp),
                verticalArrangement = Arrangement.spacedBy(12.dp),
            ) {
                items(shown.ratios.size) { index ->
                    val page by produceState<ImageBitmap?>(null, shown, index) {
                        value = withContext(Dispatchers.IO) { shown.doc.render(index) }
                    }
                    Box(
                        Modifier.fillMaxWidth().aspectRatio(shown.ratios[index]).clip(RoundedCornerShape(8.dp)).background(androidx.compose.ui.graphics.Color.White),
                        contentAlignment = Alignment.Center,
                    ) {
                        val bitmap = page
                        if (bitmap == null) {
                            CircularProgressIndicator(color = Hc.Brand, strokeWidth = 2.dp, modifier = Modifier.size(28.dp))
                        } else {
                            Image(bitmap, contentDescription = "Halaman ${index + 1}", contentScale = ContentScale.FillWidth, modifier = Modifier.fillMaxSize())
                        }
                    }
                }
            }
        }
    }
}
