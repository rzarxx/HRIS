package id.pejuang.hris.ui

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.IntrinsicSize
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.imePadding
import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.statusBarsPadding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.selection.selectable
import androidx.compose.foundation.selection.selectableGroup
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Check
import androidx.compose.material.icons.filled.Close
import androidx.compose.material3.DatePickerDialog
import androidx.compose.material3.DateRangePicker
import androidx.compose.material3.DisplayMode
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.ModalBottomSheet
import androidx.compose.material3.SelectableDates
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.rememberDateRangePickerState
import androidx.compose.material3.rememberModalBottomSheetState
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import id.pejuang.hris.data.ApprovalItem
import id.pejuang.hris.data.ApprovalStep
import id.pejuang.hris.data.LeaveInput
import id.pejuang.hris.data.LeaveRequest
import id.pejuang.hris.data.LeaveType
import id.pejuang.hris.data.Repository
import id.pejuang.hris.data.toApiError
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.async
import kotlinx.coroutines.coroutineScope
import kotlinx.coroutines.launch
import java.time.LocalDate
import java.time.ZonedDateTime
import java.time.temporal.ChronoUnit

/** Data tab Pengajuan. Daftar null berarti belum pernah dimuat. */
data class RequestsData(
    val types: List<LeaveType>? = null,
    val leaves: List<LeaveRequest>? = null,
    /** Antrean persetujuan; null bila belum dimuat atau akun tidak punya izin approval:act. */
    val pending: List<ApprovalItem>? = null,
    val error: String? = null,
    val refreshing: Boolean = false,
)

enum class RequestsView { Mine, Inbox }

/** Keputusan yang sedang diisi: setujui atau tolak satu langkah persetujuan. */
data class Decision(val item: ApprovalItem, val approve: Boolean)

/** Isi dan aksi tab Pengajuan yang diteruskan [MainTabs] ke [RequestsContent]. */
class RequestsUi(
    val data: RequestsData = RequestsData(),
    val canApprove: Boolean = false,
    val view: RequestsView = RequestsView.Mine,
    val onView: (RequestsView) -> Unit = {},
    val onNew: () -> Unit = {},
    val onOpen: (LeaveRequest) -> Unit = {},
    val onDecide: (Decision) -> Unit = {},
    val onRefresh: () -> Unit = {},
)

/** Memuat dan memperbarui data pengajuan; dipegang [MainScreen] selama sesi. */
class RequestsController(private val repository: Repository, private val scope: CoroutineScope) {
    var data by mutableStateOf(RequestsData())
        private set

    fun refresh(canApprove: Boolean) {
        if (data.refreshing) return
        data = data.copy(refreshing = true, error = null)
        scope.launch {
            data = try {
                coroutineScope {
                    val types = async { repository.leaveTypes() }
                    val leaves = async { repository.leaves() }
                    // Antrean gagal dimuat tidak menghalangi pengajuan sendiri.
                    val pending = if (canApprove) async { attempt { repository.pendingApprovals() } } else null
                    RequestsData(
                        types = types.await(),
                        leaves = leaves.await(),
                        pending = pending?.await()?.getOrNull() ?: data.pending.takeIf { canApprove },
                    )
                }
            } catch (e: CancellationException) {
                throw e
            } catch (e: Exception) {
                data.copy(refreshing = false, error = e.toApiError().message)
            }
        }
    }

    suspend fun submit(input: LeaveInput): LeaveRequest {
        val created = repository.createLeave(input)
        data = data.copy(leaves = listOf(created) + data.leaves.orEmpty().filterNot { it.id == created.id })
        // Saldo berubah setelah pengajuan baru.
        attempt { repository.leaveTypes() }.getOrNull()?.let { data = data.copy(types = it) }
        return created
    }

    suspend fun cancel(id: String): LeaveRequest {
        val updated = repository.cancelLeave(id)
        data = data.copy(leaves = data.leaves?.map { if (it.id == id) updated else it })
        attempt { repository.leaveTypes() }.getOrNull()?.let { data = data.copy(types = it) }
        return updated
    }

    suspend fun decide(decision: Decision, note: String?) {
        repository.decide(decision.item.id, decision.approve, note)
        data = data.copy(pending = data.pending?.filterNot { it.id == decision.item.id })
    }

    suspend fun steps(approvalId: String): List<ApprovalStep> = repository.approval(approvalId).steps.orEmpty()
}

/** Seperti runCatching, tetapi pembatalan coroutine tetap diteruskan. */
suspend fun <T> attempt(block: suspend () -> T): Result<T> = try {
    Result.success(block())
} catch (e: CancellationException) {
    throw e
} catch (e: Exception) {
    Result.failure(e)
}

// ---------------------------------------------------------------------------------------------
// Tab Pengajuan

@Composable
fun RequestsContent(ui: RequestsUi, now: ZonedDateTime, modifier: Modifier = Modifier) {
    val d = ui.data
    val view = if (ui.canApprove) ui.view else RequestsView.Mine
    Box(modifier.fillMaxSize().background(Hc.Canvas)) {
        RefreshBox(refreshing = d.refreshing && d.leaves != null, onRefresh = ui.onRefresh, modifier = Modifier.fillMaxSize()) {
            LazyColumn(
                Modifier.fillMaxSize().statusBarsPadding(),
                contentPadding = PaddingValues(start = 16.dp, end = 16.dp, top = 8.dp, bottom = bottomContentSpace(20.dp)),
                verticalArrangement = Arrangement.spacedBy(12.dp),
                horizontalAlignment = Alignment.CenterHorizontally,
            ) {
                item(key = "judul") { PageHeader("Pengajuan", "Cuti dan izin, serta persetujuannya.", ContentWidth) }
                item(key = "saldo") {
                    Column(ContentWidth) { BalanceCard(d.types, now.toLocalDate(), ui.onNew) }
                }
                if (ui.canApprove) {
                    item(key = "pilihan") { ViewSwitch(view, d.pending?.size ?: 0, ui.onView, ContentWidth) }
                }
                if (d.error != null) {
                    item(key = "galat") {
                        FigmaCard(ContentWidth, title = "Data belum bisa dimuat", subtitle = d.error) {
                            FigmaSecondaryButton("Coba lagi", ui.onRefresh)
                        }
                    }
                }
                when (view) {
                    RequestsView.Mine -> {
                        val leaves = d.leaves
                        when {
                            leaves == null && d.error == null -> items(3, key = { "kerangka-$it" }) { SkeletonCard(ContentWidth, lines = 2) }
                            leaves == null -> Unit
                            leaves.isEmpty() -> item(key = "kosong") {
                                EmptyCard(
                                    "Belum ada pengajuan",
                                    "Ajukan cuti atau izin dari tombol di atas. Status dan alur persetujuannya tampil di sini.",
                                    ContentWidth,
                                )
                            }
                            else -> {
                                item(key = "judul-saya") { SectionTitle("Riwayat pengajuan", "${leaves.size} pengajuan", ContentWidth) }
                                items(leaves, key = { it.id }) { LeaveCard(it, now, { ui.onOpen(it) }, ContentWidth) }
                            }
                        }
                    }
                    RequestsView.Inbox -> {
                        val pending = d.pending
                        when {
                            pending == null && d.refreshing -> items(2, key = { "kerangka-p-$it" }) { SkeletonCard(ContentWidth, lines = 3) }
                            pending == null -> item(key = "antrean-galat") {
                                EmptyCard("Antrean belum bisa dimuat", "Tarik layar ke bawah untuk mencoba lagi.", ContentWidth)
                            }
                            pending.isEmpty() -> item(key = "antrean-kosong") {
                                EmptyCard("Semua sudah diputuskan", "Tidak ada pengajuan yang menunggu Anda. Pengajuan baru muncul di sini.", ContentWidth, done = true)
                            }
                            else -> {
                                item(key = "judul-antrean") { SectionTitle("Menunggu keputusan Anda", "Terlama di atas", ContentWidth) }
                                items(pending.sortedBy { it.createdAt }, key = { it.id }) { a ->
                                    ApprovalCard(a, now, onDecide = { approve -> ui.onDecide(Decision(a, approve)) }, modifier = ContentWidth)
                                }
                            }
                        }
                    }
                }
            }
        }
    }
}

@Composable
private fun SectionTitle(title: String, subtitle: String, modifier: Modifier) {
    Row(
        modifier.fillMaxWidth().padding(start = 4.dp, end = 4.dp, top = 12.dp, bottom = 2.dp),
        verticalAlignment = Alignment.Bottom,
        horizontalArrangement = Arrangement.spacedBy(8.dp),
    ) {
        Text(title, style = figmaText(16, FontWeight.SemiBold), modifier = Modifier.weight(1f).semantics { heading() })
        Text(subtitle, style = figmaText(12, color = Fg.TextSecondary), maxLines = 1)
    }
}

@Composable
private fun EmptyCard(title: String, body: String, modifier: Modifier, done: Boolean = false) {
    Column(
        modifier.fillMaxWidth().softShadow().clip(CardShape).background(Hc.Surface).padding(horizontal = 24.dp, vertical = 32.dp),
        horizontalAlignment = Alignment.CenterHorizontally,
        verticalArrangement = Arrangement.spacedBy(8.dp),
    ) {
        SoftIconTile(if (done) Icons.Filled.Check else FigmaIcons.NavNote, 64.dp)
        Text(title, style = figmaText(16, FontWeight.SemiBold), textAlign = TextAlign.Center)
        Text(body, style = figmaText(13, color = Fg.TextSecondary, lineHeight = 1.4f), textAlign = TextAlign.Center)
    }
}

/** Kartu saldo cuti per jenis bersaldo dan tombol pengajuan. */
@Composable
private fun BalanceCard(types: List<LeaveType>?, today: LocalDate, onNew: () -> Unit) {
    val balanced = types.orEmpty().filter { it.balance != null }
    val year = balanced.firstOrNull()?.balance?.year ?: today.year
    FigmaCard(
        title = if (types == null || balanced.isNotEmpty()) "Saldo cuti $year" else "Ajukan cuti atau izin",
        subtitle = "Hari kerja; akhir pekan dan hari libur tidak dihitung.",
        gap = 16.dp,
    ) {
        when {
            types == null -> Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                repeat(2) { SkeletonBlock(Modifier.weight(1f).height(96.dp), RoundedCornerShape(16.dp)) }
            }
            balanced.isNotEmpty() -> balanced.chunked(2).forEach { row ->
                Row(Modifier.fillMaxWidth().height(IntrinsicSize.Min), horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                    row.forEach { BalanceTile(it, Modifier.weight(1f).fillMaxHeight()) }
                    if (row.size == 1) Spacer(Modifier.weight(1f))
                }
            }
        }
        FigmaPrimaryButton("Ajukan cuti atau izin", onNew, enabled = !types.isNullOrEmpty())
        if (types != null && types.isEmpty()) {
            Text("Belum ada jenis cuti yang aktif. Hubungi SDM.", style = figmaText(12, color = Fg.TextSecondary), textAlign = TextAlign.Center, modifier = Modifier.fillMaxWidth())
        }
    }
}

@Composable
private fun BalanceTile(type: LeaveType, modifier: Modifier) {
    val b = type.balance ?: return
    val eligible = b.eligibleFrom?.let { runCatching { LocalDate.parse(it) }.getOrNull() }
    val waiting = b.entitledDays == 0 && eligible != null
    Column(
        modifier
            .clip(RoundedCornerShape(16.dp))
            .background(if (b.remainingDays > 0) Hc.BrandSoft else Hc.Field)
            .padding(horizontal = 14.dp, vertical = 14.dp)
            .semantics(mergeDescendants = true) {},
        verticalArrangement = Arrangement.spacedBy(2.dp),
    ) {
        Text(type.name, style = figmaText(12, FontWeight.Medium, if (b.remainingDays > 0) Hc.BrandDeep else Hc.InkSoft), maxLines = 1, overflow = TextOverflow.Ellipsis)
        Row(verticalAlignment = Alignment.Bottom) {
            Text("${b.remainingDays}", style = figmaText(28, FontWeight.Bold, if (b.remainingDays > 0) Hc.Ink else Fg.Gray500, tracking = -0.4f).copy(lineHeight = 34.sp))
            Text(" / ${b.entitledDays} hari", style = figmaText(13, color = Hc.InkSoft), modifier = Modifier.padding(bottom = 5.dp))
        }
        Text(
            when {
                waiting -> "Berhak mulai ${dateLabel(eligible!!)}"
                b.usedDays > 0 -> "Terpakai ${b.usedDays} hari, termasuk yang menunggu"
                else -> "Belum terpakai"
            },
            style = figmaText(11, color = if (waiting) Tone.Warning.content else Hc.InkSoft, lineHeight = 1.35f),
        )
    }
}

/** Pilihan "Pengajuan saya" atau "Persetujuan" untuk akun penyetuju. */
@Composable
private fun ViewSwitch(view: RequestsView, pending: Int, onView: (RequestsView) -> Unit, modifier: Modifier) {
    Row(
        modifier.fillMaxWidth().clip(RoundedCornerShape(100.dp)).background(Hc.Line).padding(4.dp).selectableGroup(),
        horizontalArrangement = Arrangement.spacedBy(4.dp),
    ) {
        SwitchItem("Pengajuan saya", null, view == RequestsView.Mine, Modifier.weight(1f)) { onView(RequestsView.Mine) }
        SwitchItem("Persetujuan", pending.takeIf { it > 0 }, view == RequestsView.Inbox, Modifier.weight(1f)) { onView(RequestsView.Inbox) }
    }
}

@Composable
private fun SwitchItem(label: String, count: Int?, selected: Boolean, modifier: Modifier, onClick: () -> Unit) {
    Row(
        modifier
            .heightIn(min = 44.dp)
            .then(if (selected) Modifier.softShadow(RoundedCornerShape(100.dp), 4.dp) else Modifier)
            .clip(RoundedCornerShape(100.dp))
            .background(if (selected) Hc.Surface else Color.Transparent)
            .selectable(selected = selected, role = Role.Tab, onClick = onClick)
            .padding(horizontal = 12.dp),
        horizontalArrangement = Arrangement.Center,
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Text(label, style = figmaText(14, if (selected) FontWeight.SemiBold else FontWeight.Medium, if (selected) Fg.TextPrimary else Fg.TextSecondary), maxLines = 1)
        if (count != null) {
            Spacer(Modifier.width(6.dp))
            CountPill(count)
        }
    }
}

/** Angka kecil berlatar merah, kontras cukup dengan teks putih (4,9:1). */
@Composable
fun CountPill(count: Int, modifier: Modifier = Modifier) {
    Box(
        modifier.heightIn(min = 20.dp).clip(RoundedCornerShape(100.dp)).background(Hc.Danger).padding(horizontal = 6.dp),
        contentAlignment = Alignment.Center,
    ) {
        Text(if (count > 99) "99+" else "$count", style = figmaText(11, FontWeight.Bold, Color.White).copy(lineHeight = 16.sp))
    }
}

/** Baris kemajuan pengajuan: siapa yang sedang memutuskan, atau hasil akhirnya. */
fun progressText(leave: LeaveRequest): String {
    val current = leave.approvals.firstOrNull { it.status == "pending" }
    return when (leave.status) {
        "pending" -> current?.let { "Menunggu ${approverRoleLabel(it.approverRole)}${it.approverName?.let { n -> " · $n" }.orEmpty()}" }
            ?: "Menunggu persetujuan"
        "approved" -> "Disetujui ${leave.decidedAt?.let(::wibDateTime) ?: ""}".trim()
        "rejected" -> leave.approvals.firstOrNull { it.status == "rejected" }?.note?.let { "Ditolak: $it" } ?: "Ditolak"
        "cancelled" -> "Dibatalkan"
        else -> leave.status
    }
}

@Composable
private fun LeaveCard(leave: LeaveRequest, now: ZonedDateTime, onClick: () -> Unit, modifier: Modifier) {
    val (status, tone) = requestStatusLabel(leave.status)
    FigmaCard(modifier, gap = 8.dp, onClick = onClick, onClickLabel = "Lihat detail") {
        Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(6.dp)) {
            Icon(FigmaIcons.CalendarOutline, contentDescription = null, tint = Fg.Accent, modifier = Modifier.size(16.dp))
            Text(leave.leaveType.name, style = figmaText(13, FontWeight.Medium, Fg.TextSecondary), modifier = Modifier.weight(1f), maxLines = 1, overflow = TextOverflow.Ellipsis)
            StatusChip(status, tone)
        }
        Text(dateRangeLabel(leave.startDate, leave.endDate), style = figmaText(16, FontWeight.SemiBold, Fg.TextStrong, lineHeight = 1.3f))
        Text("${leave.days} hari kerja · diajukan ${timeAgo(leave.createdAt, now)}", style = figmaText(12, color = Fg.TextSecondary))
        Row(
            Modifier.fillMaxWidth().padding(top = 4.dp).clip(RoundedCornerShape(12.dp)).background(Hc.Field).padding(horizontal = 12.dp, vertical = 10.dp),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(8.dp),
        ) {
            Dot(stepColor(if (leave.status == "pending") "pending" else leave.status), size = 8.dp)
            Text(progressText(leave), style = figmaText(12, FontWeight.Medium, Fg.TextBody), modifier = Modifier.weight(1f), maxLines = 2, overflow = TextOverflow.Ellipsis)
            Icon(FigmaIcons.ArrowRight, contentDescription = null, tint = Fg.Chevron, modifier = Modifier.size(14.dp))
        }
    }
}

@Composable
private fun ApprovalCard(item: ApprovalItem, now: ZonedDateTime, onDecide: (Boolean) -> Unit, modifier: Modifier) {
    val s = item.summary
    val name = item.requester.fullName
    FigmaCard(modifier) {
        Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(10.dp)) {
            Avatar(name, size = 44.dp)
            Column(Modifier.weight(1f)) {
                Text(name, style = figmaText(14, FontWeight.SemiBold), maxLines = 1, overflow = TextOverflow.Ellipsis)
                Text(
                    "${item.requester.subdivisionName} · ${timeAgo(item.createdAt, now)}",
                    style = figmaText(12, color = Fg.TextSecondary), maxLines = 1, overflow = TextOverflow.Ellipsis,
                )
            }
        }
        Column(verticalArrangement = Arrangement.spacedBy(4.dp)) {
            Text(
                "${requestKindLabel(item.requestType)} · langkah ${approverRoleLabel(item.approverRole)}",
                style = figmaText(12, FontWeight.Medium, Hc.BrandDark),
            )
            if (s.startDate != null && s.endDate != null) {
                Text(dateRangeLabel(s.startDate, s.endDate), style = figmaText(16, FontWeight.SemiBold, Fg.TextStrong))
                Text(listOfNotNull(s.leaveType, s.days?.let { "$it hari kerja" }).joinToString(" · "), style = figmaText(13, color = Fg.TextSecondary))
            }
            if (!s.reason.isNullOrBlank()) {
                Text(s.reason, style = figmaText(13, color = Fg.TextBody, lineHeight = 1.4f), maxLines = 3, overflow = TextOverflow.Ellipsis)
            }
        }
        Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
            FigmaSecondaryButton("Tolak", { onDecide(false) }, Modifier.weight(1f).semantics { contentDescription = "Tolak pengajuan $name" }, color = Fg.DangerText)
            FigmaPrimaryButton("Setujui", { onDecide(true) }, Modifier.weight(1f).semantics { contentDescription = "Setujui pengajuan $name" })
        }
    }
}

// ---------------------------------------------------------------------------------------------
// Timeline persetujuan (MASTER.md §10.2)

private fun stepColor(status: String): Color = when (status) {
    "approved", "submitted" -> Hc.Brand
    "rejected" -> Tone.Danger.content
    "pending" -> Tone.Warning.content
    else -> Fg.Gray400
}

private fun stepStatusText(step: ApprovalStep): String = when (step.status) {
    "approved" -> "Disetujui ${wibDateTime(step.actedAt)}"
    "rejected" -> "Ditolak ${wibDateTime(step.actedAt)}"
    "pending" -> "Sedang menunggu keputusan"
    "waiting" -> "Menunggu langkah sebelumnya"
    "skipped" -> "Dilewati"
    else -> step.status
}

/**
 * Timeline vertikal: titik "Diajukan", lalu tiap langkah dengan peran, nama penyetuju, status, dan
 * catatannya. Garis penghubung hijau sampai langkah terakhir yang sudah diputuskan.
 */
@Composable
fun ApprovalTimeline(steps: List<ApprovalStep>, requester: String, submittedAt: String, modifier: Modifier = Modifier) {
    Column(modifier.fillMaxWidth()) {
        val all = steps.sortedBy { it.stepOrder }
        TimelineRow(
            status = "submitted",
            title = "Diajukan",
            subtitle = requester,
            detail = wibDateTime(submittedAt),
            note = null,
            last = all.isEmpty(),
        )
        all.forEachIndexed { i, step ->
            TimelineRow(
                status = step.status,
                title = approverRoleLabel(step.approverRole),
                subtitle = step.approverName ?: if (step.approverRole == "sdm") "Tim SDM" else null,
                detail = stepStatusText(step),
                note = step.note,
                last = i == all.lastIndex,
            )
        }
    }
}

@Composable
private fun TimelineRow(status: String, title: String, subtitle: String?, detail: String, note: String?, last: Boolean) {
    val color = stepColor(status)
    Row(Modifier.fillMaxWidth().height(IntrinsicSize.Min).semantics(mergeDescendants = true) {}) {
        Column(Modifier.width(28.dp).fillMaxHeight(), horizontalAlignment = Alignment.CenterHorizontally) {
            StepMarker(status, color)
            if (!last) {
                Box(
                    Modifier.padding(vertical = 2.dp).width(2.dp).weight(1f).clip(RoundedCornerShape(1.dp))
                        .background(if (status == "approved" || status == "submitted") Hc.Brand.copy(alpha = 0.45f) else Hc.Line),
                )
            }
        }
        Column(Modifier.weight(1f).padding(start = 10.dp, bottom = if (last) 0.dp else 18.dp), verticalArrangement = Arrangement.spacedBy(2.dp)) {
            Row(verticalAlignment = Alignment.CenterVertically) {
                Text(title, style = figmaText(14, FontWeight.SemiBold), modifier = Modifier.weight(1f, fill = false))
                if (subtitle != null) {
                    Text(" · $subtitle", style = figmaText(13, color = Fg.TextSecondary), maxLines = 1, overflow = TextOverflow.Ellipsis)
                }
            }
            Text(detail, style = figmaText(12, FontWeight.Medium, if (status == "pending") color else Fg.TextSecondary))
            if (!note.isNullOrBlank()) {
                val skipped = status == "skipped"
                Text(
                    if (skipped) note else "“$note”",
                    style = figmaText(13, color = if (skipped) Fg.Gray500 else Fg.TextBody, lineHeight = 1.4f),
                    modifier = Modifier.padding(top = 4.dp).then(
                        if (skipped) Modifier else Modifier.clip(RoundedCornerShape(12.dp)).background(Hc.Field).padding(horizontal = 12.dp, vertical = 8.dp),
                    ),
                )
            }
        }
    }
}

@Composable
private fun StepMarker(status: String, color: Color) {
    val filled = status == "approved" || status == "rejected" || status == "submitted"
    Box(
        Modifier
            .size(24.dp)
            .clip(CircleShape)
            .background(if (filled) color else Color.White)
            .border(2.dp, color, CircleShape),
        contentAlignment = Alignment.Center,
    ) {
        when (status) {
            "approved", "submitted" -> Icon(Icons.Filled.Check, contentDescription = null, tint = Color.White, modifier = Modifier.size(14.dp))
            "rejected" -> Icon(Icons.Filled.Close, contentDescription = null, tint = Color.White, modifier = Modifier.size(14.dp))
            "pending" -> Box(Modifier.size(8.dp).clip(CircleShape).background(color))
            "skipped" -> Box(Modifier.width(10.dp).height(2.dp).background(color))
            else -> Unit
        }
    }
}

// ---------------------------------------------------------------------------------------------
// Detail pengajuan

@Composable
fun LeaveDetailContent(leave: LeaveRequest, requester: String, cancelling: Boolean, onBack: () -> Unit, onCancel: () -> Unit) {
    val (status, tone) = requestStatusLabel(leave.status)
    Column(Modifier.fillMaxSize().background(Hc.Canvas)) {
        FigmaHeader("Detail pengajuan", onBack)
        Column(
            Modifier.weight(1f).fillMaxWidth().verticalScroll(rememberScrollState())
                .navigationBarsPadding().padding(horizontal = 12.dp, vertical = 16.dp),
            verticalArrangement = Arrangement.spacedBy(12.dp),
            horizontalAlignment = Alignment.CenterHorizontally,
        ) {
            FigmaCard(ContentWidth, top = 16.dp, bottom = 16.dp, gap = 8.dp) {
                Row(verticalAlignment = Alignment.CenterVertically) {
                    Text(leave.leaveType.name, style = figmaText(13, FontWeight.Medium, Fg.TextSecondary), modifier = Modifier.weight(1f))
                    StatusChip(status, tone)
                }
                Text(dateRangeLabel(leave.startDate, leave.endDate), style = figmaText(20, FontWeight.SemiBold, Fg.TextStrong, lineHeight = 1.3f))
                Text("${leave.days} hari kerja", style = figmaText(13, color = Fg.TextSecondary))
                Box(Modifier.fillMaxWidth().padding(vertical = 4.dp).height(1.dp).background(Hc.Line))
                DetailValue("Alasan", leave.reason)
                DetailValue("Diajukan", wibDateTime(leave.createdAt))
                if (leave.decidedAt != null) DetailValue(if (leave.status == "cancelled") "Dibatalkan" else "Diputuskan", wibDateTime(leave.decidedAt))
            }
            FigmaCard(ContentWidth, title = "Alur persetujuan", subtitle = "Diperiksa berurutan dari atas ke bawah.", top = 16.dp, bottom = 16.dp, gap = 16.dp) {
                ApprovalTimeline(leave.approvals, requester, leave.createdAt)
            }
            if (leave.status == "pending") {
                Column(ContentWidth.fillMaxWidth(), verticalArrangement = Arrangement.spacedBy(8.dp)) {
                    FigmaPrimaryButton("Batalkan pengajuan", onCancel, loading = cancelling, danger = true)
                    Text(
                        "Pembatalan hanya bisa selama belum diputuskan. Saldo cuti kembali setelah dibatalkan.",
                        style = figmaText(12, color = Fg.TextSecondary, lineHeight = 1.4f),
                        textAlign = TextAlign.Center,
                        modifier = Modifier.fillMaxWidth(),
                    )
                }
            }
        }
    }
}

@Composable
private fun DetailValue(label: String, value: String) {
    Column(Modifier.fillMaxWidth(), verticalArrangement = Arrangement.spacedBy(2.dp)) {
        Text(label, style = figmaText(12, FontWeight.Medium, Fg.TextSecondary))
        Text(value, style = figmaText(14, color = Fg.TextBody, lineHeight = 1.4f))
    }
}

/** Halaman detail dengan konfirmasi pembatalan. */
@Composable
fun LeaveDetailScreen(controller: RequestsController, leave: LeaveRequest, onBack: () -> Unit, onMessage: (String) -> Unit) {
    val scope = rememberCoroutineScope()
    val haptic = rememberConfirmHaptic()
    var ask by remember { mutableStateOf(false) }
    var busy by remember { mutableStateOf(false) }
    Box(Modifier.fillMaxSize()) {
        LeaveDetailContent(leave, "Anda", busy, onBack, onCancel = { ask = true })
        if (ask) {
            FigmaPopup(
                icon = FigmaIcons.NavNoteBold,
                title = "Batalkan pengajuan?",
                body = "${leave.leaveType.name} ${dateRangeLabel(leave.startDate, leave.endDate)} dibatalkan dan tidak diproses lagi. Anda bisa mengajukan ulang kapan saja.",
                primary = "Ya, batalkan",
                onPrimary = {
                    ask = false
                    busy = true
                    scope.launch {
                        try {
                            controller.cancel(leave.id)
                            haptic()
                            onMessage("Pengajuan dibatalkan.")
                        } catch (e: CancellationException) {
                            throw e
                        } catch (e: Exception) {
                            onMessage(e.toApiError().message ?: "Pengajuan belum bisa dibatalkan.")
                        } finally {
                            busy = false
                        }
                    }
                },
                secondary = "Tidak",
                onSecondary = { ask = false },
                danger = true,
            )
        }
    }
}

// ---------------------------------------------------------------------------------------------
// Form cuti/izin

data class LeaveForm(
    val typeId: String? = null,
    val start: LocalDate? = null,
    val end: LocalDate? = null,
    val reason: String = "",
    val busy: Boolean = false,
    val errors: Map<String, String> = emptyMap(),
    val error: String? = null,
)

/** Pemeriksaan sebelum kirim, sama dengan aturan server agar galat muncul lebih awal. */
fun validateLeave(form: LeaveForm, today: LocalDate): Map<String, String> {
    val errors = mutableMapOf<String, String>()
    if (form.typeId == null) errors["leave_type_id"] = "Pilih jenis cuti."
    val start = form.start
    val end = form.end
    when {
        start == null || end == null -> errors["start_date"] = "Pilih tanggal mulai dan selesai."
        end.isBefore(start) -> errors["start_date"] = "Tanggal selesai tidak boleh sebelum tanggal mulai."
        start.year != end.year -> errors["start_date"] = "Pisahkan pengajuan yang melewati pergantian tahun."
        ChronoUnit.DAYS.between(start, end) + 1 > 92 -> errors["start_date"] = "Satu pengajuan paling lama 92 hari."
        start.isBefore(today.minusDays(30)) -> errors["start_date"] = "Tanggal mulai paling lambat 30 hari ke belakang."
    }
    if (form.reason.trim().length < 3) errors["reason"] = "Alasan wajib diisi, minimal 3 karakter."
    return errors
}

@Composable
fun LeaveFormContent(
    form: LeaveForm,
    types: List<LeaveType>,
    onType: (String) -> Unit,
    onPickDates: () -> Unit,
    onReason: (String) -> Unit,
    onSubmit: () -> Unit,
    onCancel: () -> Unit,
    modifier: Modifier = Modifier,
) {
    val selected = types.firstOrNull { it.id == form.typeId }
    val dateError = form.errors["start_date"] ?: form.errors["end_date"]
    Column(
        modifier.fillMaxWidth().verticalScroll(rememberScrollState()).padding(start = 20.dp, end = 20.dp, bottom = 24.dp),
        verticalArrangement = Arrangement.spacedBy(16.dp),
    ) {
        Column(verticalArrangement = Arrangement.spacedBy(2.dp)) {
            Text("Ajukan cuti atau izin", style = figmaText(20, FontWeight.SemiBold), modifier = Modifier.semantics { heading() })
            Text("Diperiksa atasan lalu SDM. Anda mendapat kabar di tab ini.", style = figmaText(13, color = Fg.TextSecondary, lineHeight = 1.4f))
        }

        Column(verticalArrangement = Arrangement.spacedBy(6.dp)) {
            Text("Jenis *", style = figmaText(12, FontWeight.Medium, Fg.TextSecondary))
            Column(Modifier.selectableGroup(), verticalArrangement = Arrangement.spacedBy(8.dp)) {
                types.forEach { t -> TypeOption(t, t.id == form.typeId) { onType(t.id) } }
            }
            form.errors["leave_type_id"]?.let { Text(it, style = figmaText(12, color = Fg.Danger, lineHeight = 1.3f)) }
        }

        FigmaPickerField(
            label = "Tanggal",
            value = if (form.start != null && form.end != null) dateRangeLabel(form.start, form.end) else null,
            placeholder = "Pilih tanggal mulai dan selesai",
            icon = FigmaIcons.CalendarOutline,
            onClick = onPickDates,
            error = dateError,
            required = true,
        )
        if (form.start != null && form.end != null && dateError == null) {
            val calendar = ChronoUnit.DAYS.between(form.start, form.end) + 1
            val b = selected?.balance
            val over = b != null && calendar > b.remainingDays
            InfoBox(
                buildString {
                    append("$calendar hari kalender. Akhir pekan dan hari libur tidak dihitung; jumlah hari kerja dipastikan server.")
                    if (selected != null && b != null) append("\nSisa ${selected.name}: ${b.remainingDays} hari.")
                },
                warning = over,
            )
        }

        FigmaTextArea(
            label = "Alasan",
            value = form.reason,
            onChange = onReason,
            placeholder = "Contoh: menghadiri pernikahan saudara di Cirebon.",
            error = form.errors["reason"],
            hint = "Dibaca atasan dan SDM.",
            required = true,
        )

        if (form.error != null) InfoBox(form.error, warning = true)

        Column(verticalArrangement = Arrangement.spacedBy(10.dp)) {
            FigmaPrimaryButton("Kirim pengajuan", onSubmit, loading = form.busy, enabled = types.isNotEmpty())
            FigmaSecondaryButton("Batal", onCancel)
        }
    }
}

@Composable
private fun InfoBox(text: String, warning: Boolean) {
    val tone = if (warning) Tone.Warning else Tone.Info
    Text(
        text,
        style = figmaText(12, FontWeight.Medium, tone.content, lineHeight = 1.45f),
        modifier = Modifier.fillMaxWidth().clip(RoundedCornerShape(14.dp)).background(tone.container).padding(horizontal = 14.dp, vertical = 12.dp),
    )
}

@Composable
private fun TypeOption(type: LeaveType, selected: Boolean, onClick: () -> Unit) {
    val b = type.balance
    val right = when {
        b != null -> "Sisa ${b.remainingDays} hari"
        type.quotaKind == "none" -> "Tanpa saldo"
        else -> ""
    }
    Row(
        Modifier
            .fillMaxWidth()
            .heightIn(min = 52.dp)
            .clip(RoundedCornerShape(14.dp))
            .background(if (selected) Hc.BrandSoft else Hc.Field)
            .then(if (selected) Modifier.border(2.dp, Hc.Brand, RoundedCornerShape(14.dp)) else Modifier)
            .selectable(selected = selected, role = Role.RadioButton, onClick = onClick)
            .padding(horizontal = 12.dp, vertical = 10.dp),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(12.dp),
    ) {
        Box(
            Modifier.size(20.dp).clip(CircleShape).border(2.dp, if (selected) Hc.Brand else Fg.Gray400, CircleShape),
            contentAlignment = Alignment.Center,
        ) {
            if (selected) Box(Modifier.size(10.dp).clip(CircleShape).background(Hc.Brand))
        }
        Column(Modifier.weight(1f)) {
            Text(type.name, style = figmaText(14, FontWeight.SemiBold))
            Text(if (type.paid) "Tetap dibayar" else "Tidak dibayar", style = figmaText(12, color = Fg.TextSecondary))
        }
        if (right.isNotEmpty()) {
            Text(right, style = figmaText(12, FontWeight.Medium, if (b != null && b.remainingDays == 0) Tone.Warning.content else Fg.TextSecondary))
        }
    }
}

/** Lembar bawah form cuti/izin dengan pemilih rentang tanggal Material3. */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun LeaveFormSheet(controller: RequestsController, today: LocalDate, onDismiss: () -> Unit, onSent: (String) -> Unit) {
    val scope = rememberCoroutineScope()
    val haptic = rememberConfirmHaptic()
    val sheet = rememberModalBottomSheetState(skipPartiallyExpanded = true)
    val types = controller.data.types.orEmpty().filter { it.isActive }
    var form by remember { mutableStateOf(LeaveForm(typeId = types.singleOrNull()?.id)) }
    var picking by remember { mutableStateOf(false) }

    fun close(then: () -> Unit) {
        scope.launch { sheet.hide() }.invokeOnCompletion { then() }
    }

    ModalBottomSheet(onDismissRequest = onDismiss, sheetState = sheet, containerColor = Color.White) {
        LeaveFormContent(
            form = form,
            types = types,
            onType = { form = form.copy(typeId = it, errors = form.errors - "leave_type_id", error = null) },
            onPickDates = { picking = true },
            onReason = { form = form.copy(reason = it, errors = form.errors - "reason", error = null) },
            onSubmit = submit@{
                if (form.busy) return@submit
                val errors = validateLeave(form, today)
                if (errors.isNotEmpty()) {
                    form = form.copy(errors = errors, error = null)
                    return@submit
                }
                val input = LeaveInput(form.typeId!!, form.start!!.toString(), form.end!!.toString(), form.reason.trim())
                form = form.copy(busy = true, errors = emptyMap(), error = null)
                scope.launch {
                    try {
                        val created = controller.submit(input)
                        haptic()
                        close { onSent("Pengajuan ${created.leaveType.name} terkirim. Menunggu persetujuan.") }
                    } catch (e: CancellationException) {
                        throw e
                    } catch (e: Exception) {
                        val err = e.toApiError()
                        form = form.copy(busy = false, errors = err.fields, error = if (err.fields.isEmpty()) err.message else null)
                    }
                }
            },
            onCancel = { close(onDismiss) },
            modifier = Modifier.imePadding(),
        )
    }

    if (picking) {
        LeaveDatePicker(form.start, form.end, today, onDismiss = { picking = false }) { start, end ->
            picking = false
            form = form.copy(start = start, end = end, errors = form.errors - "start_date" - "end_date", error = null)
        }
    }
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
private fun LeaveDatePicker(start: LocalDate?, end: LocalDate?, today: LocalDate, onDismiss: () -> Unit, onPick: (LocalDate, LocalDate) -> Unit) {
    val earliest = today.minusDays(30)
    val latest = LocalDate.of(today.year + 1, 12, 31)
    val state = rememberDateRangePickerState(
        initialSelectedStartDateMillis = start?.toPickerMillis(),
        initialSelectedEndDateMillis = end?.toPickerMillis(),
        yearRange = today.year..today.year + 1,
        initialDisplayMode = DisplayMode.Picker,
        selectableDates = object : SelectableDates {
            override fun isSelectableDate(utcTimeMillis: Long): Boolean {
                val d = pickerMillisToDate(utcTimeMillis)
                return !d.isBefore(earliest) && !d.isAfter(latest)
            }

            override fun isSelectableYear(year: Int): Boolean = year in today.year..today.year + 1
        },
    )
    DatePickerDialog(
        onDismissRequest = onDismiss,
        confirmButton = {
            TextButton(
                enabled = state.selectedStartDateMillis != null,
                onClick = pick@{
                    val s = state.selectedStartDateMillis?.let(::pickerMillisToDate) ?: return@pick
                    val e = state.selectedEndDateMillis?.let(::pickerMillisToDate) ?: s
                    onPick(s, e)
                },
            ) { Text("Pilih") }
        },
        dismissButton = { TextButton(onClick = onDismiss) { Text("Batal") } },
    ) {
        DateRangePicker(
            state = state,
            modifier = Modifier.weight(1f),
            title = {
                Text(
                    "Pilih tanggal mulai, lalu tanggal selesai. Untuk satu hari, ketuk sekali lalu Pilih.",
                    style = figmaText(13, color = Fg.TextSecondary, lineHeight = 1.4f),
                    modifier = Modifier.padding(start = 24.dp, end = 16.dp, top = 16.dp),
                )
            },
            showModeToggle = false,
        )
    }
}

// ---------------------------------------------------------------------------------------------
// Keputusan persetujuan

@Composable
fun DecisionContent(
    decision: Decision,
    note: String,
    error: String?,
    busy: Boolean,
    steps: List<ApprovalStep>?,
    onNote: (String) -> Unit,
    onSubmit: () -> Unit,
    onSwitch: () -> Unit,
    onCancel: () -> Unit,
    modifier: Modifier = Modifier,
) {
    val item = decision.item
    val s = item.summary
    val reject = !decision.approve
    Column(
        modifier.fillMaxWidth().verticalScroll(rememberScrollState()).padding(start = 20.dp, end = 20.dp, bottom = 24.dp),
        verticalArrangement = Arrangement.spacedBy(16.dp),
    ) {
        Text(
            if (reject) "Tolak pengajuan ${item.requester.fullName}?" else "Setujui pengajuan ${item.requester.fullName}?",
            style = figmaText(20, FontWeight.SemiBold, lineHeight = 1.3f),
            modifier = Modifier.semantics { heading() },
        )
        Column(
            Modifier.fillMaxWidth().clip(RoundedCornerShape(16.dp)).background(Hc.Field).padding(14.dp),
            verticalArrangement = Arrangement.spacedBy(2.dp),
        ) {
            Text(listOfNotNull(requestKindLabel(item.requestType), s.leaveType).joinToString(" · "), style = figmaText(12, FontWeight.Medium, Fg.TextSecondary))
            if (s.startDate != null && s.endDate != null) {
                Text(
                    "${dateRangeLabel(s.startDate, s.endDate)}${s.days?.let { " ($it hari kerja)" }.orEmpty()}",
                    style = figmaText(16, FontWeight.SemiBold, Fg.TextStrong),
                )
            }
            if (!s.reason.isNullOrBlank()) Text(s.reason, style = figmaText(13, color = Fg.TextBody, lineHeight = 1.4f))
            Text("${item.requester.subdivisionName} · diajukan ${wibDateTime(item.createdAt)}", style = figmaText(12, color = Fg.TextSecondary))
        }
        FigmaTextArea(
            label = if (reject) "Alasan penolakan" else "Catatan",
            value = note,
            onChange = onNote,
            placeholder = if (reject) "Contoh: bertepatan dengan agenda yayasan, mohon ajukan tanggal lain." else "Opsional",
            error = error,
            hint = if (reject) "Dibaca pemohon di aplikasi." else "Opsional, terlihat di riwayat persetujuan.",
            required = reject,
        )
        Column(verticalArrangement = Arrangement.spacedBy(10.dp)) {
            FigmaPrimaryButton(if (reject) "Tolak pengajuan" else "Setujui pengajuan", onSubmit, loading = busy, danger = reject)
            FigmaSecondaryButton("Batal", onCancel)
            Text(
                if (reject) "Ganti ke setujui" else "Ganti ke tolak",
                style = figmaText(13, FontWeight.Medium, Fg.TextSecondary),
                textAlign = TextAlign.Center,
                modifier = Modifier.fillMaxWidth().clip(RoundedCornerShape(8.dp))
                    .clickable(role = Role.Button, onClick = onSwitch).heightIn(min = 48.dp).padding(14.dp),
            )
        }
        Text("Riwayat persetujuan", style = figmaText(14, FontWeight.SemiBold), modifier = Modifier.semantics { heading() })
        if (steps == null) {
            SkeletonCard(lines = 3)
        } else {
            ApprovalTimeline(steps, item.requester.fullName, item.createdAt)
        }
    }
}

/** Lembar bawah untuk menyetujui atau menolak satu pengajuan. */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun DecisionSheet(controller: RequestsController, decision: Decision, onSwitch: (Decision) -> Unit, onDismiss: () -> Unit, onDone: (String) -> Unit) {
    val scope = rememberCoroutineScope()
    val haptic = rememberConfirmHaptic()
    val sheet = rememberModalBottomSheetState(skipPartiallyExpanded = true)
    val id = decision.item.id
    var note by remember(id) { mutableStateOf("") }
    var error by remember(id, decision.approve) { mutableStateOf<String?>(null) }
    var busy by remember(id) { mutableStateOf(false) }
    var steps by remember(id) { mutableStateOf<List<ApprovalStep>?>(null) }
    LaunchedEffect(id) {
        steps = attempt { controller.steps(id) }.getOrDefault(emptyList())
    }

    ModalBottomSheet(onDismissRequest = onDismiss, sheetState = sheet, containerColor = Color.White) {
        DecisionContent(
            decision = decision,
            note = note,
            error = error,
            busy = busy,
            steps = steps,
            onNote = { note = it; error = null },
            onSubmit = submit@{
                if (busy) return@submit
                if (!decision.approve && note.trim().length < 3) {
                    error = "Tulis alasan penolakan, minimal 3 karakter. Alasan ini dibaca pemohon."
                    return@submit
                }
                busy = true
                scope.launch {
                    try {
                        controller.decide(decision, note)
                        haptic()
                        val name = decision.item.requester.fullName
                        val message = if (decision.approve) "Pengajuan $name disetujui." else "Pengajuan $name ditolak."
                        scope.launch { sheet.hide() }.invokeOnCompletion { onDone(message) }
                    } catch (e: CancellationException) {
                        throw e
                    } catch (e: Exception) {
                        val err = e.toApiError()
                        error = err.fields["note"] ?: err.message
                        busy = false
                    }
                }
            },
            onSwitch = { onSwitch(decision.copy(approve = !decision.approve)) },
            onCancel = { scope.launch { sheet.hide() }.invokeOnCompletion { onDismiss() } },
            modifier = Modifier.imePadding(),
        )
    }
}

/** Bingkai lembar bawah tanpa jendela pop-up, untuk tes screenshot. */
@Composable
fun SheetPreview(content: @Composable () -> Unit) {
    Box(Modifier.fillMaxSize().background(Fg.Backdrop)) {
        Column(
            Modifier.align(Alignment.BottomCenter).fillMaxWidth()
                .clip(RoundedCornerShape(topStart = 28.dp, topEnd = 28.dp)).background(Color.White).navigationBarsPadding(),
            horizontalAlignment = Alignment.CenterHorizontally,
        ) {
            Box(Modifier.padding(vertical = 22.dp).width(32.dp).height(4.dp).clip(RoundedCornerShape(2.dp)).background(Fg.Gray300))
            content()
        }
    }
}
