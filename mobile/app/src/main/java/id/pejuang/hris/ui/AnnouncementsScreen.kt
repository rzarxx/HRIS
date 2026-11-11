package id.pejuang.hris.ui

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.BoxScope
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.Icon
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.semantics.LiveRegionMode
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.semantics.liveRegion
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import id.pejuang.hris.data.MyAnnouncement
import id.pejuang.hris.data.Repository
import id.pejuang.hris.data.toApiError
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch

/** Memuat pengumuman yang tayang dan mencatat buka/baca; dipegang [MainScreen] selama sesi. */
class AnnouncementsController(private val repository: Repository, private val scope: CoroutineScope) {
    /** Null berarti belum pernah dimuat. */
    var items by mutableStateOf<List<MyAnnouncement>?>(null)
        private set

    fun refresh() {
        scope.launch {
            // Gagal memuat pengumuman tidak menghalangi presensi; daftar lama tetap dipakai.
            attempt { repository.announcements() }.getOrNull()?.let { items = it }
        }
    }

    private fun replace(updated: MyAnnouncement) {
        items = items?.map { if (it.id == updated.id) updated else it }
    }

    suspend fun open(id: String): MyAnnouncement = repository.openAnnouncement(id).also(::replace)

    suspend fun read(id: String): MyAnnouncement = repository.readAnnouncement(id).also(::replace)

    val unread: Int get() = items.orEmpty().count { it.readAt == null }
}

/** Hitung mundur per detik dari [start]; dimulai ulang bila [key] atau [start] berubah. */
@Composable
fun rememberCountdown(key: Any?, start: Int): Int {
    var left by remember(key, start) { mutableIntStateOf(start.coerceAtLeast(0)) }
    LaunchedEffect(key, start) {
        while (left > 0) {
            delay(1000)
            left--
        }
    }
    return left
}

private fun readButtonLabel(wait: Int) = if (wait > 0) "Sudah baca ($wait)" else "Sudah baca"

/**
 * Membuka pengumuman di server sekali, lalu menghitung mundur dan mengirim "Sudah baca". Bila server
 * masih menolak (jam HP dan server berbeda), sisa jeda diambil ulang dari server.
 */
private class ReadFlow(
    val controller: AnnouncementsController,
    val scope: CoroutineScope,
    val haptic: () -> Unit,
) {
    var wait by mutableStateOf<Int?>(null)
    var busy by mutableStateOf(false)
    var error by mutableStateOf<String?>(null)

    suspend fun open(item: MyAnnouncement) {
        if (item.readAt != null) {
            wait = 0
            return
        }
        wait = try {
            controller.open(item.id).readWaitSeconds
        } catch (e: CancellationException) {
            throw e
        } catch (e: Exception) {
            error = e.toApiError().message
            item.minReadSeconds
        }
    }

    fun read(item: MyAnnouncement, onDone: () -> Unit) {
        if (busy) return
        busy = true
        error = null
        scope.launch {
            try {
                controller.read(item.id)
                haptic()
                onDone()
            } catch (e: CancellationException) {
                throw e
            } catch (e: Exception) {
                val err = e.toApiError()
                error = err.message
                if (err.code == "READ_TOO_SOON") wait = attempt { controller.open(item.id).readWaitSeconds }.getOrNull() ?: wait
            } finally {
                busy = false
            }
        }
    }
}

@Composable
private fun rememberReadFlow(controller: AnnouncementsController, item: MyAnnouncement): ReadFlow {
    val scope = rememberCoroutineScope()
    val haptic = rememberConfirmHaptic()
    val flow = remember(item.id) { ReadFlow(controller, scope, haptic) }
    LaunchedEffect(item.id) { flow.open(item) }
    return flow
}

// ---------------------------------------------------------------------------------------------
// Pop-up saat aplikasi dibuka

/** Pop-up pengumuman: latar gelap, kartu di tengah, tombol "Sudah baca" dengan hitung mundur. */
@Composable
fun BoxScope.AnnouncementPopupContent(
    item: MyAnnouncement,
    wait: Int?,
    busy: Boolean,
    error: String?,
    onRead: () -> Unit,
    onLater: () -> Unit,
) {
    Box(
        Modifier.matchParentSize().background(Fg.Backdrop)
            .clickable(interactionSource = remember { MutableInteractionSource() }, indication = null) {},
    )
    Column(
        Modifier
            .align(Alignment.Center)
            .then(ContentWidth)
            .padding(20.dp)
            .fillMaxWidth()
            .clip(RoundedCornerShape(24.dp))
            .background(Hc.Surface)
            .padding(24.dp),
        verticalArrangement = Arrangement.spacedBy(14.dp),
    ) {
        Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(10.dp)) {
            SoftIconTile(FigmaIcons.MessageText, 40.dp)
            Text("Pengumuman", style = figmaText(13, FontWeight.SemiBold, Hc.BrandDark), modifier = Modifier.weight(1f))
        }
        Text(item.title, style = figmaText(20, FontWeight.Bold, lineHeight = 1.3f), modifier = Modifier.semantics { heading() })
        Column(Modifier.heightIn(max = 320.dp).verticalScroll(rememberScrollState())) {
            Text(item.body, style = figmaText(15, color = Fg.TextBody, lineHeight = 1.55f))
        }
        if (error != null) Text(error, style = figmaText(12, color = Fg.Danger, lineHeight = 1.3f))
        Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
            val w = wait ?: item.minReadSeconds
            FigmaPrimaryButton(
                readButtonLabel(w),
                onRead,
                modifier = Modifier.semantics { liveRegion = LiveRegionMode.Polite },
                enabled = wait != null && w == 0,
                loading = busy,
            )
            FigmaSecondaryButton("Nanti saja", onLater, color = Hc.Ink)
        }
    }
}

/** Pop-up pengumuman yang terhubung ke server; [onClose] dipanggil setelah dibaca atau ditunda. */
@Composable
fun BoxScope.AnnouncementPopup(controller: AnnouncementsController, item: MyAnnouncement, onClose: (read: Boolean) -> Unit) {
    val flow = rememberReadFlow(controller, item)
    val wait = flow.wait?.let { rememberCountdown(item.id to it, it) }
    AnnouncementPopupContent(
        item = item,
        wait = wait,
        busy = flow.busy,
        error = flow.error,
        onRead = { flow.read(item) { onClose(true) } },
        onLater = { onClose(false) },
    )
}

// ---------------------------------------------------------------------------------------------
// Banner di Beranda

/** Kartu banner pengumuman di Beranda: latar mint lembut, ketuk untuk membaca selengkapnya. */
@Composable
fun AnnouncementBanner(item: MyAnnouncement, onClick: () -> Unit, modifier: Modifier = Modifier) {
    Row(
        modifier
            .fillMaxWidth()
            .clip(CardShape)
            .background(Hc.BrandSoft)
            .clickable(onClickLabel = "Baca pengumuman", onClick = onClick)
            .padding(16.dp),
        horizontalArrangement = Arrangement.spacedBy(14.dp),
    ) {
        Box(Modifier.size(40.dp).clip(CircleShape).background(Hc.Surface), contentAlignment = Alignment.Center) {
            Icon(FigmaIcons.MessageText, contentDescription = null, tint = Hc.Brand, modifier = Modifier.size(20.dp))
        }
        Column(Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(4.dp)) {
            Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(6.dp)) {
                Text("Pengumuman", style = figmaText(12, FontWeight.SemiBold, Hc.BrandDeep), modifier = Modifier.weight(1f))
                if (item.readAt == null) NewChip()
            }
            Text(item.title, style = figmaText(15, FontWeight.SemiBold, Hc.Ink, lineHeight = 1.3f), maxLines = 2, overflow = TextOverflow.Ellipsis)
            Text(item.body, style = figmaText(13, color = Hc.InkSoft, lineHeight = 1.4f), maxLines = 2, overflow = TextOverflow.Ellipsis)
        }
    }
}

@Composable
private fun NewChip() {
    Box(
        Modifier.clip(RoundedCornerShape(100.dp))
            .background(Hc.Danger)
            .padding(horizontal = 8.dp, vertical = 2.dp),
    ) {
        Text("Baru", style = figmaText(11, FontWeight.Bold, Color.White))
    }
}

// ---------------------------------------------------------------------------------------------
// Daftar dan detail

private fun scheduleText(item: MyAnnouncement): String {
    val from = wibDateTime(item.startsAt)
    return if (item.endsAt != null) "Tayang $from sampai ${wibDateTime(item.endsAt)}" else "Tayang sejak $from"
}

@Composable
fun AnnouncementListContent(list: List<MyAnnouncement>?, onBack: () -> Unit, onOpen: (MyAnnouncement) -> Unit) {
    Column(Modifier.fillMaxSize().background(Fg.PageProfile)) {
        FigmaHeader("Pengumuman", onBack)
        LazyColumn(
            Modifier.weight(1f).fillMaxWidth(),
            contentPadding = PaddingValues(start = 12.dp, end = 12.dp, top = 16.dp, bottom = bottomContentSpace(16.dp)),
            verticalArrangement = Arrangement.spacedBy(10.dp),
            horizontalAlignment = Alignment.CenterHorizontally,
        ) {
            when {
                list == null -> items(3) { SkeletonCard(ContentWidth, lines = 2) }
                list.isEmpty() -> item {
                    FigmaCard(ContentWidth, title = "Belum ada pengumuman", subtitle = "Pengumuman dari SDM yang sedang tayang tampil di sini.") {}
                }
                else -> items(list, key = { it.id }) { a ->
                    FigmaCard(ContentWidth, gap = 6.dp, onClick = { onOpen(a) }, onClickLabel = "Baca pengumuman") {
                        Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(6.dp)) {
                            Text(if (a.type == "popup") "Pop-up" else "Banner", style = figmaText(12, FontWeight.Medium, Fg.TextSecondary), modifier = Modifier.weight(1f))
                            if (a.readAt == null) NewChip() else StatusChip("Sudah dibaca", Tone.Success)
                        }
                        Text(a.title, style = figmaText(15, FontWeight.SemiBold, lineHeight = 1.3f))
                        Text(a.body, style = figmaText(13, color = Fg.TextBody, lineHeight = 1.4f), maxLines = 2, overflow = TextOverflow.Ellipsis)
                        Text(scheduleText(a), style = figmaText(11, color = Fg.Gray500))
                    }
                }
            }
        }
    }
}

@Composable
fun AnnouncementDetailContent(item: MyAnnouncement, wait: Int?, busy: Boolean, error: String?, onBack: () -> Unit, onRead: () -> Unit) {
    Column(Modifier.fillMaxSize().background(Fg.PageProfile)) {
        FigmaHeader("Pengumuman", onBack)
        Column(
            Modifier.weight(1f).fillMaxWidth().verticalScroll(rememberScrollState())
                .navigationBarsPadding().padding(horizontal = 12.dp, vertical = 16.dp),
            verticalArrangement = Arrangement.spacedBy(12.dp),
            horizontalAlignment = Alignment.CenterHorizontally,
        ) {
            FigmaCard(ContentWidth, top = 16.dp, bottom = 16.dp, gap = 10.dp) {
                Text(scheduleText(item), style = figmaText(12, color = Fg.TextSecondary))
                Text(item.title, style = figmaText(20, FontWeight.SemiBold, Fg.TextStrong, lineHeight = 1.3f), modifier = Modifier.semantics { heading() })
                Text(item.body, style = figmaText(15, color = Fg.TextBody, lineHeight = 1.55f))
            }
            Column(ContentWidth.fillMaxWidth(), verticalArrangement = Arrangement.spacedBy(8.dp)) {
                if (item.readAt != null) {
                    Row(
                        Modifier.fillMaxWidth().clip(RoundedCornerShape(14.dp)).background(Tone.Success.container)
                            .padding(14.dp),
                        verticalAlignment = Alignment.CenterVertically,
                    ) {
                        Text("Sudah dibaca ${wibDateTime(item.readAt)}", style = figmaText(13, FontWeight.Medium, Tone.Success.content))
                    }
                } else {
                    val w = wait ?: item.minReadSeconds
                    if (error != null) Text(error, style = figmaText(12, color = Fg.Danger, lineHeight = 1.3f))
                    FigmaPrimaryButton(
                        readButtonLabel(w),
                        onRead,
                        modifier = Modifier.semantics { liveRegion = LiveRegionMode.Polite },
                        enabled = wait != null && w == 0,
                        loading = busy,
                    )
                    Text(
                        "Tombol aktif setelah hitung mundur, agar isi sempat dibaca. SDM melihat siapa yang sudah membaca.",
                        style = figmaText(12, color = Fg.TextSecondary, lineHeight = 1.4f),
                        textAlign = TextAlign.Center,
                        modifier = Modifier.fillMaxWidth(),
                    )
                }
            }
        }
    }
}

/** Halaman detail yang terhubung ke server. */
@Composable
fun AnnouncementDetailScreen(controller: AnnouncementsController, item: MyAnnouncement, onBack: () -> Unit, onMessage: (String) -> Unit) {
    val flow = rememberReadFlow(controller, item)
    val wait = flow.wait?.let { rememberCountdown(item.id to it, it) }
    AnnouncementDetailContent(
        item = item,
        wait = wait,
        busy = flow.busy,
        error = flow.error,
        onBack = onBack,
        onRead = { flow.read(item) { onMessage("Terima kasih, pengumuman ditandai sudah dibaca.") } },
    )
}
