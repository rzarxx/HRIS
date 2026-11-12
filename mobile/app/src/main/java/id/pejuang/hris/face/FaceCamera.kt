package id.pejuang.hris.face

import android.graphics.Bitmap
import android.graphics.Rect
import android.util.Size
import androidx.annotation.OptIn
import androidx.camera.core.CameraSelector
import androidx.camera.core.ExperimentalGetImage
import androidx.camera.core.ImageAnalysis
import androidx.camera.core.ImageCapture
import androidx.camera.core.ImageCaptureException
import androidx.camera.core.ImageProxy
import androidx.camera.core.Preview
import androidx.camera.core.resolutionselector.AspectRatioStrategy
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
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.rememberUpdatedState
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.viewinterop.AndroidView
import androidx.core.content.ContextCompat
import androidx.lifecycle.compose.LocalLifecycleOwner
import com.google.mlkit.vision.common.InputImage
import com.google.mlkit.vision.face.Face
import com.google.mlkit.vision.face.FaceDetection
import com.google.mlkit.vision.face.FaceDetector
import com.google.mlkit.vision.face.FaceDetectorOptions
import id.pejuang.hris.ui.CameraOverlay
import id.pejuang.hris.ui.CameraPhase
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch
import kotlinx.coroutines.suspendCancellableCoroutine
import kotlinx.coroutines.tasks.await
import kotlinx.coroutines.withContext
import java.util.concurrent.Executor
import java.util.concurrent.ExecutorService
import java.util.concurrent.Executors
import java.util.concurrent.atomic.AtomicBoolean
import kotlin.coroutines.resume
import kotlin.coroutines.resumeWithException

/** Hasil foto mentah dari kamera, sebelum ditegakkan dan diperkecil. */
private class RawShot(val bitmap: Bitmap, val rotation: Int)

/** Isi lapisan kamera: warna bingkai, pesan arahan, dan kemajuan menahan posisi (0 sampai 1). */
data class CameraMessage(val phase: CameraPhase, val text: String, val progress: Float = 0f)

/** Pesan yang ditampilkan di bawah bingkai wajah untuk tiap status liveness. */
fun cameraMessage(state: LivenessState, shots: Int, taken: Int, retrying: Boolean = false): CameraMessage = when (state) {
    LivenessState.NoFace -> CameraMessage(CameraPhase.Searching, FaceFit.Outside.hint)
    LivenessState.ManyFaces -> CameraMessage(CameraPhase.Problem, "Hanya satu wajah yang boleh terlihat")
    is LivenessState.Waiting -> CameraMessage(
        if (state.aligned) CameraPhase.Aligned else CameraPhase.Following,
        state.hint,
        state.progress,
    )
    LivenessState.Passed -> CameraMessage(
        CameraPhase.Capturing,
        when {
            retrying -> "Tahan posisi · foto diulang"
            shots > 1 -> "Tahan posisi · foto ${minOf(taken + 1, shots)} dari $shots"
            else -> "Tahan posisi..."
        },
        1f,
    )
}

// Analisis dan pratinjau sama-sama 4:3 agar kotak wajah dari frame analisis bisa dipetakan ke layar.
private val ratio43 = AspectRatioStrategy.RATIO_4_3_FALLBACK_AUTO_STRATEGY

/**
 * Kamera depan dengan pemeriksaan liveness. Wajah harus pas di oval (lihat [checkFit]); saat gerakan
 * [action] terlihat, frame itu disimpan sebagai foto gerakan. Setelah wajah pas dan lurus lagi,
 * mengambil [shots] foto, memeriksa wajah di tiap foto, lalu memanggil [onCaptured] dengan JPEG yang
 * sudah tegak dan foto gerakannya. Server membuktikan gerakan dari kedua foto itu (D50).
 */
@Composable
fun FaceCamera(
    title: String,
    action: String,
    shots: Int,
    onCaptured: (photos: List<ByteArray>, gesture: ByteArray) -> Unit,
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
    var retrying by remember(action) { mutableStateOf(false) }
    // Foto gerakan dari frame analisis. gestureGeneration naik setiap foto lama dibuang, agar hasil
    // kompresi yang terlambat tidak menimpa keadaan baru.
    var gesture by remember(action) { mutableStateOf<ByteArray?>(null) }
    var gestureGeneration by remember(action) { mutableIntStateOf(0) }
    val scope = rememberCoroutineScope()
    val currentOnCaptured by rememberUpdatedState(onCaptured)
    val currentOnError by rememberUpdatedState(onError)

    val previewView = remember { PreviewView(context).apply { scaleType = PreviewView.ScaleType.FILL_CENTER } }

    // Foto dibatasi sekitar 1280x960: cukup untuk wajah, dan tidak menghabiskan memori HP kelas bawah.
    val imageCapture = remember {
        ImageCapture.Builder()
            .setCaptureMode(ImageCapture.CAPTURE_MODE_MINIMIZE_LATENCY)
            .setResolutionSelector(
                ResolutionSelector.Builder()
                    .setAspectRatioStrategy(ratio43)
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
        // Ukuran minimum 0.2: wajah yang pas di oval sekitar 24-45% lebar frame, tergantung tinggi
        // layar, karena pratinjau memotong sisi frame. Ukuran wajah yang sebenarnya diperiksa checkFit.
        val detector = FaceDetection.getClient(
            FaceDetectorOptions.Builder()
                .setPerformanceMode(FaceDetectorOptions.PERFORMANCE_MODE_FAST)
                .setClassificationMode(FaceDetectorOptions.CLASSIFICATION_MODE_ALL)
                .setMinFaceSize(0.2f)
                .build(),
        )
        val analysis = ImageAnalysis.Builder()
            .setBackpressureStrategy(ImageAnalysis.STRATEGY_KEEP_ONLY_LATEST)
            .setResolutionSelector(
                ResolutionSelector.Builder()
                    .setAspectRatioStrategy(ratio43)
                    .setResolutionStrategy(
                        ResolutionStrategy(Size(640, 480), ResolutionStrategy.FALLBACK_RULE_CLOSEST_LOWER_THEN_HIGHER),
                    )
                    .build(),
            )
            .build()
        analysis.setAnalyzer(analysisExecutor) { proxy ->
            analyze(proxy, closed, detector) { faces, frameWidth, frameHeight, rotation, grab ->
                if (!passed) {
                    val viewWidth = previewView.width.toFloat()
                    val viewHeight = previewView.height.toFloat()
                    val oval = faceOval(viewWidth, viewHeight)
                    val samples = significantFaces(faces) { it.boundingBox.toFitRect() }.map { face ->
                        val onScreen = mapToView(face.boundingBox.toFitRect(), frameWidth, frameHeight, rotation, viewWidth, viewHeight, mirror = true)
                        FaceSample(
                            leftEyeOpen = face.leftEyeOpenProbability,
                            rightEyeOpen = face.rightEyeOpenProbability,
                            smiling = face.smilingProbability,
                            yaw = face.headEulerAngleY,
                            fit = if (viewWidth > 0f && viewHeight > 0f) {
                                checkFit(onScreen, oval, roll = face.headEulerAngleZ, pitch = face.headEulerAngleX)
                            } else {
                                FaceFit.Outside
                            },
                        )
                    }
                    val hadGesture = tracker.gestureSeen
                    val next = tracker.update(samples)
                    if (!tracker.gestureSeen) {
                        if (hadGesture || gesture != null) {
                            gesture = null
                            gestureGeneration++
                        }
                    } else if (!hadGesture) {
                        // Frame pertama saat kepala menoleh menjadi foto gerakan.
                        val generation = ++gestureGeneration
                        grab()?.let { bitmap ->
                            scope.launch {
                                val jpeg = withContext(Dispatchers.Default) {
                                    try {
                                        toUprightJpeg(bitmap, rotation)
                                    } finally {
                                        bitmap.recycle()
                                    }
                                }
                                if (generation == gestureGeneration) gesture = jpeg
                            }
                        }
                    }
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
                val preview = Preview.Builder()
                    .setResolutionSelector(ResolutionSelector.Builder().setAspectRatioStrategy(ratio43).build())
                    .build()
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
        // Detektor terpisah untuk foto hasil jepretan; wajah kecil ikut dicari agar orang kedua terlihat.
        val photoDetector = FaceDetection.getClient(
            FaceDetectorOptions.Builder()
                .setPerformanceMode(FaceDetectorOptions.PERFORMANCE_MODE_FAST)
                .setMinFaceSize(0.1f)
                .build(),
        )
        try {
            val photos = mutableListOf<ByteArray>()
            for (i in 0 until shots) {
                if (i > 0) delay(600)
                var retries = 0
                var photo: ByteArray? = null
                while (photo == null) {
                    val raw = imageCapture.takeRaw(ContextCompat.getMainExecutor(context))
                    val fits = try {
                        raw.fits(photoDetector, previewView.width.toFloat(), previewView.height.toFloat())
                    } catch (e: CancellationException) {
                        raw.bitmap.recycle()
                        throw e
                    } catch (e: Exception) {
                        // Detektor gagal: kirim saja, server tetap memeriksa wajah dan posisinya.
                        true
                    }
                    if (fits) {
                        photo = withContext(Dispatchers.Default) {
                            try {
                                toUprightJpeg(raw.bitmap, raw.rotation)
                            } finally {
                                raw.bitmap.recycle()
                            }
                        }
                    } else {
                        raw.bitmap.recycle()
                        if (retries >= PhotoRules.RETRIES) {
                            currentOnError("Wajah tidak pas di bingkai saat foto diambil. Ulangi dan tahan posisi sampai foto selesai.")
                            return@LaunchedEffect
                        }
                        retries++
                        retrying = true
                        delay(400)
                    }
                }
                retrying = false
                photos += checkNotNull(photo)
                taken = i + 1
            }
            // Foto gerakan dikompresi di latar sejak kepala menoleh; biasanya sudah siap di sini.
            var waited = 0
            while (gesture == null && waited < 2000) {
                delay(50)
                waited += 50
            }
            val gestureJpeg = gesture
            if (gestureJpeg == null) {
                currentOnError("Foto saat menoleh gagal diambil. Ulangi dan tolehkan kepala dengan jelas.")
                return@LaunchedEffect
            }
            currentOnCaptured(photos, gestureJpeg)
        } catch (e: CancellationException) {
            throw e
        } catch (e: OutOfMemoryError) {
            currentOnError("Memori HP tidak cukup untuk memproses foto. Tutup aplikasi lain lalu coba lagi.")
        } catch (e: Exception) {
            currentOnError("Foto gagal diambil. Silakan ulangi.")
        } finally {
            photoDetector.close()
        }
    }

    val message = cameraMessage(state, shots, taken, retrying)

    Box(modifier = modifier.fillMaxSize()) {
        AndroidView(factory = { previewView }, modifier = Modifier.fillMaxSize())
        CameraOverlay(
            title = title,
            message = message.text,
            phase = message.phase,
            gesture = tracker.prompt,
            onClose = onClose,
            progress = message.progress,
        )
    }
}

private fun Rect.toFitRect() = FitRect(left.toFloat(), top.toFloat(), right.toFloat(), bottom.toFloat())

/** Memeriksa wajah pada foto hasil jepretan dengan aturan [photoFits]. */
private suspend fun RawShot.fits(detector: FaceDetector, viewWidth: Float, viewHeight: Float): Boolean {
    val faces = detector.process(InputImage.fromBitmap(bitmap, rotation)).await()
    // Kotak dari ML Kit berada pada foto yang sudah ditegakkan.
    val sideways = rotation % 180 != 0
    val width = if (sideways) bitmap.height else bitmap.width
    val height = if (sideways) bitmap.width else bitmap.height
    return photoFits(faces.map { it.boundingBox.toFitRect() }, width, height, viewWidth, viewHeight, mirror = true)
}

@OptIn(ExperimentalGetImage::class)
private fun analyze(
    proxy: ImageProxy,
    closed: AtomicBoolean,
    detector: FaceDetector,
    onFaces: (faces: List<Face>, frameWidth: Int, frameHeight: Int, rotation: Int, grab: () -> Bitmap?) -> Unit,
) {
    val media = proxy.image
    if (closed.get() || media == null) {
        proxy.close()
        return
    }
    val frameWidth = proxy.width
    val frameHeight = proxy.height
    val rotation = proxy.imageInfo.rotationDegrees
    try {
        val input = InputImage.fromMediaImage(media, rotation)
        detector.process(input)
            .addOnCompleteListener { task ->
                // Dijalankan di thread utama. Frame baru ditutup setelah onFaces, karena grab()
                // mengambil bitmap dari frame ini (foto gerakan, D50).
                try {
                    if (task.isSuccessful && !closed.get()) {
                        onFaces(task.result, frameWidth, frameHeight, rotation) {
                            try {
                                proxy.toBitmap()
                            } catch (e: Exception) {
                                null
                            }
                        }
                    }
                } finally {
                    proxy.close()
                }
            }
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
