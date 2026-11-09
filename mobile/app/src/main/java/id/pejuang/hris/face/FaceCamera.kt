package id.pejuang.hris.face

import android.graphics.Bitmap
import android.util.Size
import androidx.annotation.OptIn
import androidx.camera.core.CameraSelector
import androidx.camera.core.ExperimentalGetImage
import androidx.camera.core.ImageAnalysis
import androidx.camera.core.ImageCapture
import androidx.camera.core.ImageCaptureException
import androidx.camera.core.ImageProxy
import androidx.camera.core.Preview
import androidx.camera.core.resolutionselector.ResolutionSelector
import androidx.camera.core.resolutionselector.ResolutionStrategy
import androidx.camera.lifecycle.ProcessCameraProvider
import androidx.camera.view.PreviewView
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberUpdatedState
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.viewinterop.AndroidView
import androidx.core.content.ContextCompat
import androidx.lifecycle.compose.LocalLifecycleOwner
import com.google.mlkit.vision.common.InputImage
import com.google.mlkit.vision.face.FaceDetection
import com.google.mlkit.vision.face.FaceDetector
import com.google.mlkit.vision.face.FaceDetectorOptions
import id.pejuang.hris.ui.CameraOverlay
import id.pejuang.hris.ui.CameraPhase
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.delay
import kotlinx.coroutines.suspendCancellableCoroutine
import kotlinx.coroutines.withContext
import java.util.concurrent.Executor
import java.util.concurrent.ExecutorService
import java.util.concurrent.Executors
import java.util.concurrent.atomic.AtomicBoolean
import kotlin.coroutines.resume
import kotlin.coroutines.resumeWithException

/** Hasil foto mentah dari kamera, sebelum ditegakkan dan diperkecil. */
private class RawShot(val bitmap: Bitmap, val rotation: Int)

/** Pesan yang ditampilkan di bawah bingkai wajah untuk tiap status liveness. */
fun cameraMessage(state: LivenessState, shots: Int, taken: Int): Pair<CameraPhase, String> = when (state) {
    LivenessState.NoFace -> CameraPhase.Searching to "Posisikan wajah di dalam bingkai"
    LivenessState.ManyFaces -> CameraPhase.Problem to "Hanya satu wajah yang boleh terlihat"
    is LivenessState.Waiting -> CameraPhase.Following to state.hint
    LivenessState.Passed -> CameraPhase.Capturing to
        if (shots > 1) "Tahan posisi · foto ${minOf(taken + 1, shots)} dari $shots" else "Tahan posisi..."
}

/**
 * Kamera depan dengan pemeriksaan liveness. Setelah gerakan [action] selesai dan wajah lurus lagi,
 * mengambil [shots] foto lalu memanggil [onCaptured] dengan JPEG yang sudah tegak.
 */
@Composable
fun FaceCamera(
    title: String,
    action: String,
    shots: Int,
    onCaptured: (List<ByteArray>) -> Unit,
    onError: (String) -> Unit,
    onClose: () -> Unit,
    modifier: Modifier = Modifier,
) {
    val context = LocalContext.current
    val lifecycleOwner = LocalLifecycleOwner.current
    val tracker = remember(action) { LivenessTracker(action) }
    var state by remember(action) { mutableStateOf<LivenessState>(LivenessState.NoFace) }
    // Setelah lolos, status dibekukan agar pengambilan foto tidak dibatalkan bila wajah bergerak.
    var passed by remember(action) { mutableStateOf(false) }
    var taken by remember(action) { mutableIntStateOf(0) }
    val currentOnCaptured by rememberUpdatedState(onCaptured)
    val currentOnError by rememberUpdatedState(onError)

    val previewView = remember { PreviewView(context).apply { scaleType = PreviewView.ScaleType.FILL_CENTER } }

    // Foto dibatasi sekitar 1280x960: cukup untuk wajah, dan tidak menghabiskan memori HP kelas bawah.
    val imageCapture = remember {
        ImageCapture.Builder()
            .setCaptureMode(ImageCapture.CAPTURE_MODE_MINIMIZE_LATENCY)
            .setResolutionSelector(
                ResolutionSelector.Builder()
                    .setResolutionStrategy(
                        ResolutionStrategy(Size(1280, 960), ResolutionStrategy.FALLBACK_RULE_CLOSEST_LOWER_THEN_HIGHER),
                    )
                    .build(),
            )
            .build()
    }

    DisposableEffect(lifecycleOwner, action) {
        // Satu executor dan detektor per sesi kamera. Keduanya baru ditutup setelah kamera dilepas,
        // karena CameraX masih bisa mengirim frame sampai unbind selesai.
        val analysisExecutor: ExecutorService = Executors.newSingleThreadExecutor()
        val closed = AtomicBoolean(false)
        val detector = FaceDetection.getClient(
            FaceDetectorOptions.Builder()
                .setPerformanceMode(FaceDetectorOptions.PERFORMANCE_MODE_FAST)
                .setClassificationMode(FaceDetectorOptions.CLASSIFICATION_MODE_ALL)
                .setMinFaceSize(0.25f)
                .build(),
        )
        val analysis = ImageAnalysis.Builder()
            .setBackpressureStrategy(ImageAnalysis.STRATEGY_KEEP_ONLY_LATEST)
            .build()
        analysis.setAnalyzer(analysisExecutor) { proxy ->
            analyze(proxy, closed, detector) { faces ->
                if (!passed) {
                    val next = tracker.update(faces)
                    state = next
                    if (next == LivenessState.Passed) passed = true
                }
            }
        }

        var provider: ProcessCameraProvider? = null
        val future = ProcessCameraProvider.getInstance(context)
        future.addListener({
            if (closed.get()) return@addListener
            try {
                val cameraProvider = future.get()
                provider = cameraProvider
                val preview = Preview.Builder().build()
                preview.setSurfaceProvider(previewView.surfaceProvider)
                cameraProvider.unbindAll()
                cameraProvider.bindToLifecycle(lifecycleOwner, CameraSelector.DEFAULT_FRONT_CAMERA, preview, analysis, imageCapture)
            } catch (e: Exception) {
                currentOnError("Kamera depan tidak bisa dibuka. Tutup aplikasi lain yang memakai kamera, lalu coba lagi.")
            }
        }, ContextCompat.getMainExecutor(context))

        onDispose {
            closed.set(true)
            analysis.clearAnalyzer()
            runCatching { provider?.unbindAll() }
            detector.close()
            analysisExecutor.shutdown()
        }
    }

    LaunchedEffect(passed) {
        if (!passed) return@LaunchedEffect
        try {
            val photos = mutableListOf<ByteArray>()
            repeat(shots) { i ->
                if (i > 0) delay(600)
                val raw = imageCapture.takeRaw(ContextCompat.getMainExecutor(context))
                photos += withContext(Dispatchers.Default) {
                    try {
                        toUprightJpeg(raw.bitmap, raw.rotation)
                    } finally {
                        raw.bitmap.recycle()
                    }
                }
                taken = i + 1
            }
            currentOnCaptured(photos)
        } catch (e: CancellationException) {
            throw e
        } catch (e: OutOfMemoryError) {
            currentOnError("Memori HP tidak cukup untuk memproses foto. Tutup aplikasi lain lalu coba lagi.")
        } catch (e: Exception) {
            currentOnError("Foto gagal diambil. Silakan ulangi.")
        }
    }

    val (phase, message) = cameraMessage(state, shots, taken)

    Box(modifier = modifier.fillMaxSize()) {
        AndroidView(factory = { previewView }, modifier = Modifier.fillMaxSize())
        CameraOverlay(title = title, message = message, phase = phase, gesture = tracker.prompt, onClose = onClose)
    }
}

@OptIn(ExperimentalGetImage::class)
private fun analyze(
    proxy: ImageProxy,
    closed: AtomicBoolean,
    detector: FaceDetector,
    onFaces: (List<FaceSample>) -> Unit,
) {
    val media = proxy.image
    if (closed.get() || media == null) {
        proxy.close()
        return
    }
    try {
        val input = InputImage.fromMediaImage(media, proxy.imageInfo.rotationDegrees)
        detector.process(input)
            .addOnSuccessListener { faces ->
                // Dijalankan di thread utama.
                if (!closed.get()) {
                    onFaces(
                        faces.map {
                            FaceSample(
                                leftEyeOpen = it.leftEyeOpenProbability,
                                rightEyeOpen = it.rightEyeOpenProbability,
                                smiling = it.smilingProbability,
                                yaw = it.headEulerAngleY,
                            )
                        },
                    )
                }
            }
            .addOnCompleteListener { proxy.close() }
    } catch (e: Exception) {
        // Detektor sudah ditutup saat layar ditinggalkan: abaikan frame ini.
        proxy.close()
    }
}

/** Mengambil foto; callback berjalan di [executor] (thread utama) agar tidak ditolak saat layar ditutup. */
private suspend fun ImageCapture.takeRaw(executor: Executor): RawShot =
    suspendCancellableCoroutine { cont ->
        takePicture(
            executor,
            object : ImageCapture.OnImageCapturedCallback() {
                override fun onCaptureSuccess(image: ImageProxy) {
                    try {
                        val shot = RawShot(image.toBitmap(), image.imageInfo.rotationDegrees)
                        if (cont.isActive) cont.resume(shot) else shot.bitmap.recycle()
                    } catch (e: Throwable) {
                        if (cont.isActive) cont.resumeWithException(e)
                    } finally {
                        image.close()
                    }
                }

                override fun onError(exception: ImageCaptureException) {
                    if (cont.isActive) cont.resumeWithException(exception)
                }
            },
        )
    }
