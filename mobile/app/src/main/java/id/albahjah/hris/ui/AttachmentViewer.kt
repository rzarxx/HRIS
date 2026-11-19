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
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.Icon
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
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
import kotlinx.coroutines.NonCancellable
import kotlinx.coroutines.awaitCancellation
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
    if (bounds.outWidth <= 0 || bounds.outHeight <= 0) return null
    while (max(bounds.outWidth, bounds.outHeight) / sample > IMAGE_MAX_SIDE) sample *= 2
    val options = BitmapFactory.Options().apply { inSampleSize = sample }
    return BitmapFactory.decodeByteArray(bytes, 0, bytes.size, options)?.asImageBitmap()
}

/**
 * PDF yang dibuka di dalam aplikasi lewat PdfRenderer bawaan Android, tanpa aplikasi pembaca lain.
 * Halaman dirender saat tampil di layar, satu per satu, karena PdfRenderer hanya boleh membuka satu
 * halaman dalam satu waktu.
 */
private class PdfDocument private constructor(private val file: File, private val descriptor: ParcelFileDescriptor, private val renderer: PdfRenderer) : Closeable {
    companion object {
        fun open(context: Context, bytes: ByteArray): PdfDocument {
            val file = File.createTempFile("lampiran", ".pdf", context.cacheDir)
            var descriptor: ParcelFileDescriptor? = null
            try {
                file.writeBytes(bytes)
                descriptor = ParcelFileDescriptor.open(file, ParcelFileDescriptor.MODE_READ_ONLY)
                return PdfDocument(file, descriptor, PdfRenderer(descriptor))
            } catch (e: Exception) {
                descriptor?.close()
                file.delete()
                throw e
            }
        }
    }
    val pageCount: Int = renderer.pageCount

    fun render(index: Int): ImageBitmap = synchronized(renderer) {
        renderer.openPage(index).use { page ->
            // Bound both axes, including unusually tall or wide PDF pages.
            val scale = minOf(RENDER_WIDTH.toFloat() / page.width, IMAGE_MAX_SIDE.toFloat() / page.height)
            val width = (page.width * scale).toInt().coerceAtLeast(1)
            val height = (page.height * scale).toInt().coerceAtLeast(1)
            val bitmap = Bitmap.createBitmap(width, height, Bitmap.Config.ARGB_8888)
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
    class Pdf(val doc: PdfDocument) : Loaded
}

/** Pratinjau dimuat otomatis lewat API bersesi, tanpa membuka aplikasi lain. */
@Composable
fun InlineAttachment(announcementId: String, attachment: AnnouncementAttachment, load: suspend () -> ByteArray, onExpand: () -> Unit) {
    Column(Modifier.fillMaxWidth(), verticalArrangement = Arrangement.spacedBy(6.dp)) {
        AttachmentContent(announcementId, attachment, load, inline = true)
        Row(Modifier.fillMaxWidth(), verticalAlignment = Alignment.CenterVertically) {
            Text(attachment.fileName, style = figmaText(12, color = Hc.InkSoft), modifier = Modifier.weight(1f), maxLines = 2)
            TextButton(onClick = onExpand) { Text("Perbesar") }
        }
    }
}

@Composable
fun AttachmentViewerScreen(repository: Repository, announcementId: String, attachment: AnnouncementAttachment, onBack: () -> Unit) {
    Column(Modifier.fillMaxSize().background(Fg.PageProfile)) {
        FigmaHeader(attachment.fileName, onBack)
        AttachmentContent(announcementId, attachment, load = { repository.attachment(announcementId, attachment.id) }, inline = false, modifier = Modifier.weight(1f).navigationBarsPadding())
    }
}

@Composable
fun AttachmentImage(bitmap: ImageBitmap, name: String, inline: Boolean) {
    val modifier = if (inline) Modifier.fillMaxWidth().heightIn(max = 600.dp).aspectRatio(bitmap.width.toFloat() / bitmap.height)
        else Modifier.fillMaxSize().padding(12.dp)
    Image(bitmap, contentDescription = name, contentScale = ContentScale.Fit, modifier = modifier)
}

@Composable
private fun AttachmentContent(announcementId: String, attachment: AnnouncementAttachment, load: suspend () -> ByteArray, inline: Boolean, modifier: Modifier = Modifier) {
    val context = LocalContext.current
    var loaded by remember(announcementId, attachment.id) { mutableStateOf<Loaded?>(null) }
    var error by remember(announcementId, attachment.id) { mutableStateOf<String?>(null) }
    var retry by remember(announcementId, attachment.id) { mutableStateOf(0) }
    LaunchedEffect(announcementId, attachment.id, retry) {
        var resource: Loaded? = null
        try {
            error = null
            val bytes = load()
            withContext(Dispatchers.IO) {
                if (attachment.contentType == "application/pdf") {
                    resource = Loaded.Pdf(PdfDocument.open(context, bytes))
                } else {
                    resource = Loaded.Image(decodeAttachmentImage(bytes) ?: error("Gambar tidak bisa dibaca"))
                }
            }
            loaded = resource
            awaitCancellation()
        } catch (e: CancellationException) {
            throw e
        } catch (e: Exception) {
            error = e.toApiError().message ?: "Lampiran belum bisa dibuka."
        } finally {
            // Also closes a renderer created just before cancellation of withContext.
            withContext(NonCancellable + Dispatchers.IO) { (resource as? Loaded.Pdf)?.doc?.close() }
        }
    }
    Column(modifier.fillMaxWidth()) {
        val shown = loaded
        when {
            error != null -> Column(Modifier.fillMaxWidth().padding(12.dp)) {
                Text(error ?: "Lampiran belum bisa dibuka.", style = figmaText(12, color = Fg.Danger))
                TextButton(onClick = { loaded = null; retry++ }) { Text("Coba lagi") }
            }
            shown == null -> Box(Modifier.fillMaxWidth().height(120.dp), contentAlignment = Alignment.Center) {
                CircularProgressIndicator(color = Hc.Brand, modifier = Modifier.size(28.dp))
            }
            shown is Loaded.Image -> {
                AttachmentImage(shown.bitmap, attachment.fileName, inline)
            }
            shown is Loaded.Pdf -> LazyColumn(
                if (inline) Modifier.fillMaxWidth().height(300.dp) else Modifier.fillMaxSize(),
                contentPadding = androidx.compose.foundation.layout.PaddingValues(4.dp),
                verticalArrangement = Arrangement.spacedBy(12.dp),
            ) {
                items(shown.doc.pageCount) { index ->
                    val page by produceState<Result<ImageBitmap>?>(null, shown, index) {
                        value = try {
                            Result.success(withContext(Dispatchers.IO) { shown.doc.render(index) })
                        } catch (e: CancellationException) { throw e }
                        catch (e: Exception) { Result.failure(e) }
                    }
                    val bitmap = page?.getOrNull()
                    Box(
                        Modifier.fillMaxWidth().heightIn(max = 1400.dp).aspectRatio(bitmap?.let { it.width.toFloat() / it.height } ?: 0.707f).clip(RoundedCornerShape(8.dp)).background(androidx.compose.ui.graphics.Color.White),
                        contentAlignment = Alignment.Center,
                    ) {
                        if (page?.isFailure == true) {
                            Text("Halaman ${index + 1} belum bisa ditampilkan.", style = figmaText(12, color = Fg.Danger))
                        } else if (bitmap == null) {
                            CircularProgressIndicator(color = Hc.Brand, strokeWidth = 2.dp, modifier = Modifier.size(28.dp))
                        } else {
                            Image(bitmap, contentDescription = "Halaman ${index + 1}", contentScale = ContentScale.Fit, modifier = Modifier.fillMaxSize())
                        }
                    }
                    Text("Halaman ${index + 1} dari ${shown.doc.pageCount}", style = figmaText(11, color = Hc.InkSoft))
                }
            }
        }
    }
}
