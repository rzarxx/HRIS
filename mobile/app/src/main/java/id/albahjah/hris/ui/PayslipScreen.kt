package id.albahjah.hris.ui

import android.os.Build
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.statusBarsPadding
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
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
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import id.albahjah.hris.data.Payslip
import id.albahjah.hris.data.PayslipItem
import id.albahjah.hris.data.PayslipSummary
import id.albahjah.hris.data.Repository
import id.albahjah.hris.data.toApiError
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext

/** Memuat daftar slip ubar (D68); dipegang [MainScreen] selama sesi agar lencana tab ikut segar. */
class PayslipController(private val repository: Repository, private val scope: CoroutineScope) {
    /** Null berarti belum pernah dimuat. */
    var items by mutableStateOf<List<PayslipSummary>?>(null)
        private set
    var error by mutableStateOf<String?>(null)
        private set
    var loading by mutableStateOf(false)
        private set

    fun refresh() {
        scope.launch { load() }
    }

    /** Gagal memuat tidak menghalangi fitur lain; daftar lama tetap dipakai. */
    suspend fun load() {
        loading = true
        try {
            items = repository.payslips()
            error = null
        } catch (e: CancellationException) {
            throw e
        } catch (e: Exception) {
            error = e.toApiError().message ?: "Slip ubar belum bisa dimuat."
        } finally {
            loading = false
        }
    }

    /** Membuka slip menandai notifikasinya dibaca di server; tanda "Baru" di daftar ikut hilang. */
    fun markRead(id: String) {
        items = items?.map { if (it.id == id) it.copy(unread = false) else it }
    }

    val unread: Int get() = items.orEmpty().count { it.unread }

    /** Slip terbaru yang belum dibuka, untuk banner Beranda. */
    val newest: PayslipSummary? get() = items?.firstOrNull { it.unread }
}

// ---------------------------------------------------------------------------------------------
// Daftar

/** Tab Slip: daftar slip per periode, terbaru di atas. Tanpa jaringan, dipakai juga oleh tes screenshot. */
@Composable
fun PayslipListContent(slips: List<PayslipSummary>?, error: String?, onOpen: (PayslipSummary) -> Unit, onRetry: () -> Unit) {
    LazyColumn(
        Modifier.fillMaxSize().background(Hc.Canvas).statusBarsPadding(),
        contentPadding = PaddingValues(start = 16.dp, end = 16.dp, top = 8.dp, bottom = bottomContentSpace(20.dp)),
        verticalArrangement = Arrangement.spacedBy(12.dp),
        horizontalAlignment = Alignment.CenterHorizontally,
    ) {
        item { PageHeader("Slip ubar", "Rincian ubar per periode", ContentWidth) }
        when {
            slips == null && error != null -> item {
                FigmaCard(ContentWidth, title = "Slip ubar belum bisa dimuat", subtitle = error) {
                    FigmaSecondaryButton("Coba lagi", onClick = onRetry)
                }
            }
            slips == null -> items(3) { SkeletonCard(ContentWidth, lines = 2) }
            slips.isEmpty() -> item {
                FigmaCard(ContentWidth) {
                    SoftIconTile(FigmaIcons.NavReceipt, 56.dp)
                    Text("Belum ada slip ubar", style = figmaText(16, FontWeight.SemiBold))
                    Text(
                        "Slip ubar tampil di sini setelah SDM menyetujui ubar periode Anda. Anda akan melihat pemberitahuan di Beranda.",
                        style = figmaText(14, color = Hc.InkSoft, lineHeight = 1.5f),
                    )
                }
            }
            else -> items(slips, key = { it.id }) { slip -> PayslipRow(slip) { onOpen(slip) } }
        }
    }
}

@Composable
private fun PayslipRow(slip: PayslipSummary, onClick: () -> Unit) {
    FigmaCard(ContentWidth, gap = 8.dp, top = 16.dp, bottom = 16.dp, onClick = onClick, onClickLabel = "Buka slip") {
        Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(12.dp)) {
            SoftIconTile(FigmaIcons.NavReceipt, 44.dp)
            Column(Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(2.dp)) {
                Text(periodName(slip.year, slip.month), style = figmaText(16, FontWeight.SemiBold))
                Text(dateRangeLabel(slip.startDate, slip.endDate), style = figmaText(12, color = Hc.InkSoft))
            }
            if (slip.unread) PillLabel("Baru", Hc.Danger, Color.White)
            else if (slip.status == "paid") StatusChip("Dibayar", Tone.Success)
        }
        Row(Modifier.fillMaxWidth(), verticalAlignment = Alignment.CenterVertically) {
            Text("Ubar bersih", style = figmaText(13, color = Hc.InkSoft), modifier = Modifier.weight(1f))
            Text(rupiah(slip.net), style = figmaText(17, FontWeight.Bold))
        }
    }
}

@Composable
private fun PillLabel(text: String, background: Color, color: Color) {
    Box(Modifier.clip(RoundedCornerShape(100.dp)).background(background).padding(horizontal = 8.dp, vertical = 2.dp)) {
        Text(text, style = figmaText(11, FontWeight.Bold, color))
    }
}

// ---------------------------------------------------------------------------------------------
// Rincian

/** Isi rincian slip tanpa jaringan, dipakai juga oleh tes screenshot. */
@Composable
fun PayslipDetailContent(
    slip: Payslip,
    onBack: () -> Unit,
    onViewPdf: () -> Unit,
    onDownload: () -> Unit,
    onShare: () -> Unit,
    busy: Boolean = false,
) {
    Column(Modifier.fillMaxSize().background(Fg.PageProfile)) {
        FigmaHeader("Slip ubar", onBack)
        Column(
            Modifier.weight(1f).fillMaxWidth().verticalScroll(rememberScrollState())
                .navigationBarsPadding().padding(horizontal = 16.dp, vertical = 12.dp),
            verticalArrangement = Arrangement.spacedBy(12.dp),
            horizontalAlignment = Alignment.CenterHorizontally,
        ) {
            // Ringkasan: periode dan ubar bersih yang besar.
            FigmaCard(ContentWidth, gap = 6.dp) {
                Text(periodName(slip.year, slip.month), style = figmaText(20, FontWeight.SemiBold), modifier = Modifier.semantics { heading() })
                Text("Presensi ${dateRangeLabel(slip.startDate, slip.endDate)}", style = figmaText(13, color = Hc.InkSoft))
                Text("Ubar bersih", style = figmaText(13, color = Hc.InkSoft), modifier = Modifier.padding(top = 8.dp))
                Text(rupiah(slip.net), style = figmaText(30, FontWeight.Bold, Hc.BrandDeep, tracking = -0.4f))
                if (slip.netClamped) {
                    Text("Potongan melebihi pendapatan, ubar bersih dibulatkan ke Rp 0.", style = figmaText(12, color = Tone.Warning.content))
                }
                Text("No. ${slip.documentNumber}", style = figmaText(11, color = Fg.Gray500))
            }

            FigmaCard(ContentWidth, title = "Presensi", gap = 10.dp) {
                val a = slip.attendance
                Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                    StatTile("Hadir", "${a.presentDays}", Modifier.weight(1f))
                    StatTile("Telat", "${a.lateCount}", Modifier.weight(1f))
                    StatTile("Alpha", "${a.absentDays}", Modifier.weight(1f))
                    StatTile("Cuti", "${a.leaveDays}", Modifier.weight(1f))
                }
                if (a.lateMinutes > 0) Text("Total telat ${a.lateMinutes} menit", style = figmaText(12, color = Hc.InkSoft))
            }

            ItemsCard("Pendapatan", slip.items.filter { it.kind == "earning" }, slip.gross, negative = false)
            ItemsCard("Potongan", slip.items.filter { it.kind == "deduction" }, slip.deductions, negative = true)

            Column(ContentWidth.fillMaxWidth(), verticalArrangement = Arrangement.spacedBy(8.dp)) {
                FigmaPrimaryButton("Lihat PDF", onClick = onViewPdf, enabled = !busy)
                Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                    FigmaSecondaryButton("Unduh", onClick = { if (!busy) onDownload() }, modifier = Modifier.weight(1f))
                    FigmaSecondaryButton("Bagikan", onClick = { if (!busy) onShare() }, modifier = Modifier.weight(1f))
                }
                Text(
                    "Slip ini rahasia. Bagikan hanya bila diperlukan, mis. untuk pengajuan ke bank. Hubungi SDM bila ada yang tidak sesuai.",
                    style = figmaText(12, color = Hc.InkSoft, lineHeight = 1.45f),
                    textAlign = TextAlign.Center,
                    modifier = Modifier.fillMaxWidth().padding(top = 4.dp),
                )
            }
        }
    }
}

@Composable
private fun StatTile(label: String, value: String, modifier: Modifier = Modifier) {
    Column(
        modifier.clip(RoundedCornerShape(14.dp)).background(Hc.Field).padding(horizontal = 10.dp, vertical = 10.dp),
        verticalArrangement = Arrangement.spacedBy(2.dp),
        horizontalAlignment = Alignment.CenterHorizontally,
    ) {
        Text(value, style = figmaText(18, FontWeight.Bold))
        Text(label, style = figmaText(11, FontWeight.Medium, Hc.InkSoft))
    }
}

/** Keterangan baris, mis. "Lembur (4 x Rp 20.000)". */
fun payslipItemLabel(item: PayslipItem): String {
    val qty = item.quantity?.toBigDecimalOrNull()?.stripTrailingZeros()?.toPlainString()
    return if (qty != null && item.rate != null) "${item.name} ($qty x ${rupiah(item.rate)})" else item.name
}

@Composable
private fun ItemsCard(title: String, items: List<PayslipItem>, total: String, negative: Boolean) {
    FigmaCard(ContentWidth, title = title, gap = 10.dp) {
        if (items.isEmpty()) {
            Text("Tidak ada", style = figmaText(14, color = Hc.InkSoft))
        }
        items.forEach { item ->
            Row(Modifier.fillMaxWidth(), verticalAlignment = Alignment.Top, horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                Text(payslipItemLabel(item), style = figmaText(14, color = Fg.TextBody, lineHeight = 1.35f), modifier = Modifier.weight(1f))
                Text(rupiah(item.amount), style = figmaText(14, FontWeight.Medium, if (negative) Fg.DangerText else Hc.Ink))
            }
        }
        Box(Modifier.fillMaxWidth().padding(top = 2.dp).background(Hc.Line).padding(top = 1.dp))
        Row(Modifier.fillMaxWidth(), verticalAlignment = Alignment.CenterVertically) {
            Text("Total ${title.lowercase()}", style = figmaText(14, FontWeight.SemiBold), modifier = Modifier.weight(1f))
            Text(rupiah(total), style = figmaText(15, FontWeight.Bold, if (negative) Fg.DangerText else Hc.Ink))
        }
    }
}

/**
 * Rincian slip dengan jaringan: memuat slip (tersimpan untuk offline), lalu menyediakan lihat, unduh,
 * dan bagikan PDF. [SecureScreen] mencegah tangkapan layar karena slip memuat data ubar.
 */
@Composable
fun PayslipDetailScreen(
    repository: Repository,
    summary: PayslipSummary,
    onBack: () -> Unit,
    onOpened: () -> Unit,
    onViewPdf: (Payslip) -> Unit,
    onMessage: (String) -> Unit,
) {
    SecureScreen()
    val context = LocalContext.current
    val scope = rememberCoroutineScope()
    var slip by remember(summary.id) { mutableStateOf<Payslip?>(null) }
    var error by remember(summary.id) { mutableStateOf<String?>(null) }
    var retry by remember(summary.id) { mutableIntStateOf(0) }
    var busy by remember { mutableStateOf(false) }
    var pending by remember { mutableStateOf<ByteArray?>(null) }

    LaunchedEffect(summary.id, retry) {
        error = null
        try {
            slip = repository.payslip(summary.id)
            onOpened()
        } catch (e: CancellationException) {
            throw e
        } catch (e: Exception) {
            error = e.toApiError().message ?: "Slip belum bisa dimuat."
        }
    }

    // Android 8 dan 9: pengguna memilih lokasi simpan lewat pemilih sistem.
    val createDocument = rememberLauncherForActivityResult(ActivityResultContracts.CreateDocument("application/pdf")) { uri ->
        val bytes = pending
        pending = null
        if (uri != null && bytes != null) {
            scope.launch {
                runCatching { withContext(Dispatchers.IO) { writeToUri(context, uri, bytes) } }
                    .onSuccess { onMessage("Slip ubar tersimpan.") }
                    .onFailure { onMessage("Slip ubar belum bisa disimpan.") }
            }
        }
    }

    val current = slip
    if (current == null) {
        Column(Modifier.fillMaxSize().background(Fg.PageProfile)) {
            FigmaHeader("Slip ubar", onBack)
            Box(Modifier.weight(1f).fillMaxWidth().padding(16.dp), contentAlignment = Alignment.TopCenter) {
                if (error != null) {
                    FigmaCard(ContentWidth, title = "Slip belum bisa dimuat", subtitle = error) {
                        FigmaSecondaryButton("Coba lagi", onClick = { retry++ })
                    }
                } else {
                    SkeletonCard(ContentWidth, lines = 4)
                }
            }
        }
        return
    }

    val fileName = payslipFileName(current.year, current.month, current.nip)
    fun withPdf(action: suspend (ByteArray) -> Unit) {
        if (busy) return
        busy = true
        scope.launch {
            try {
                action(repository.payslipPdf(current.id))
            } catch (e: CancellationException) {
                throw e
            } catch (e: Exception) {
                onMessage(e.toApiError().message ?: "PDF slip belum bisa dimuat.")
            } finally {
                busy = false
            }
        }
    }

    PayslipDetailContent(
        slip = current,
        busy = busy,
        onBack = onBack,
        onViewPdf = { onViewPdf(current) },
        onDownload = {
            withPdf { bytes ->
                if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.Q) {
                    withContext(Dispatchers.IO) { saveToDownloads(context, fileName, bytes) }
                    onMessage("Slip ubar tersimpan di folder Unduhan ($fileName).")
                } else {
                    pending = bytes
                    createDocument.launch(fileName)
                }
            }
        },
        onShare = {
            withPdf { bytes ->
                val uri = withContext(Dispatchers.IO) { prepareShare(context, fileName, bytes) }
                sharePdf(context, uri, title = "Slip ubar ${periodName(current.year, current.month)}")
            }
        },
    )
}

/** PDF slip layar penuh dengan penampil bawaan aplikasi. */
@Composable
fun PayslipPdfScreen(repository: Repository, slip: Payslip, onBack: () -> Unit) {
    SecureScreen()
    Column(Modifier.fillMaxSize().background(Fg.PageProfile)) {
        FigmaHeader("Slip ${periodName(slip.year, slip.month)}", onBack)
        PdfContent(
            key = "payslip-pdf:${slip.id}",
            name = payslipFileName(slip.year, slip.month, slip.nip),
            load = { repository.payslipPdf(slip.id) },
            modifier = Modifier.weight(1f).navigationBarsPadding(),
        )
    }
}
