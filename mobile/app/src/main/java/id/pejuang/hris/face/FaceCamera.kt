package id.pejuang.hris.face

import android.graphics.Bitmap
import androidx.camera.core.CameraSelector
import androidx.camera.core.ImageAnalysis
import androidx.camera.core.ImageCapture
import androidx.camera.core.ImageCaptureException
import androidx.camera.core.ImageProxy
import androidx.camera.core.Preview
import androidx.camera.lifecycle.ProcessCameraProvider
import androidx.camera.view.PreviewView
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberUpdatedState
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import androidx.compose.ui.viewinterop.AndroidView
import androidx.core.content.ContextCompat
import androidx.lifecycle.compose.LocalLifecycleOwner
import com.google.mlkit.vision.common.InputImage
import com.google.mlkit.vision.face.FaceDetection
import com.google.mlkit.vision.face.FaceDetectorOptions
import kotlinx.coroutines.delay
import kotlinx.coroutines.suspendCancellableCoroutine
import java.util.concurrent.Executor
import java.util.concurrent.Executors
import kotlin.coroutines.resume
import kotlin.coroutines.resumeWithException

/**
 * Kamera depan dengan pemeriksaan liveness. Setelah gerakan [action] terdeteksi, mengambil [shots]
 * foto berurutan lalu memanggil [onCaptured] dengan JPEG yang sudah tegak.
 */
@Composable
fun FaceCamera(
    action: String,
    shots: Int,
    onCaptured: (List<ByteArray>) -> Unit,
    onError: (String) -> Unit,
    modifier: Modifier = Modifier,
) {
    val context = LocalContext.current
    val lifecycleOwner = LocalLifecycleOwner.current
    val tracker = remember(action) { LivenessTracker(action) }
    var state by remember(action) { mutableStateOf<LivenessState>(LivenessState.NoFace) }
    var capturing by remember(action) { mutableStateOf(false) }
    val currentOnCaptured by rememberUpdatedState(onCaptured)
    val currentOnError by rememberUpdatedState(onError)

    val previewView = remember { PreviewView(context).apply { scaleType = PreviewView.ScaleType.FILL_CENTER } }
    val imageCapture = remember {
        ImageCapture.Builder().setCaptureMode(ImageCapture.CAPTURE_MODE_MINIMIZE_LATENCY).build()
    }
    val executor = remember { Executors.newSingleThreadExecutor() }
    val detector = remember {
        FaceDetection.getClient(
            FaceDetectorOptions.Builder()
                .setPerformanceMode(FaceDetectorOptions.PERFORMANCE_MODE_FAST)
                .setClassificationMode(FaceDetectorOptions.CLASSIFICATION_MODE_ALL)
                .setMinFaceSize(0.25f)
                .build(),
        )
    }

    DisposableEffect(lifecycleOwner, action) {
        val future = ProcessCameraProvider.getInstance(context)
        var provider: ProcessCameraProvider? = null
        future.addListener({
            val cameraProvider = future.get()
            provider = cameraProvider
            val preview = Preview.Builder().build()
            preview.setSurfaceProvider(previewView.surfaceProvider)
            val analysis = ImageAnalysis.Builder()
                .setBackpressureStrategy(ImageAnalysis.STRATEGY_KEEP_ONLY_LATEST)
                .build()
            analysis.setAnalyzer(executor) { proxy ->
                val media = proxy.image
                if (media == null) {
                    proxy.close()
                    return@setAnalyzer
                }
                val input = InputImage.fromMediaImage(media, proxy.imageInfo.rotationDegrees)
                detector.process(input)
                    .addOnSuccessListener { faces ->
                        // Dijalankan di thread utama.
                        state = tracker.update(
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
                    .addOnCompleteListener { proxy.close() }
            }
            try {
                cameraProvider.unbindAll()
                cameraProvider.bindToLifecycle(lifecycleOwner, CameraSelector.DEFAULT_FRONT_CAMERA, preview, analysis, imageCapture)
            } catch (e: Exception) {
                currentOnError("Kamera depan tidak bisa dibuka.")
            }
        }, ContextCompat.getMainExecutor(context))
        onDispose {
            provider?.unbindAll()
        }
    }

    DisposableEffect(Unit) {
        onDispose {
            detector.close()
            executor.shutdown()
        }
    }

    LaunchedEffect(state) {
        if (state != LivenessState.Passed || capturing) return@LaunchedEffect
        capturing = true
        try {
            val photos = mutableListOf<ByteArray>()
            repeat(shots) { i ->
                if (i > 0) delay(600)
                photos += imageCapture.takeUprightJpeg(executor)
            }
            currentOnCaptured(photos)
        } catch (e: Exception) {
            currentOnError("Foto gagal diambil. Silakan ulangi.")
        }
    }

    val message = when (val s = state) {
        LivenessState.NoFace -> "Arahkan wajah ke kamera"
        LivenessState.ManyFaces -> "Pastikan hanya wajah Anda yang terlihat"
        is LivenessState.Waiting -> s.hint
        LivenessState.Passed -> if (shots > 1) "Tahan posisi, mengambil $shots foto..." else "Tahan posisi..."
    }

    Box(modifier = modifier.fillMaxSize()) {
        AndroidView(factory = { previewView }, modifier = Modifier.fillMaxSize())
        Text(
            text = message,
            color = Color.White,
            style = MaterialTheme.typography.titleMedium,
            textAlign = TextAlign.Center,
            modifier = Modifier
                .align(Alignment.BottomCenter)
                .fillMaxWidth()
                .background(Color(0x99000000))
                .padding(16.dp),
        )
    }
}

private suspend fun ImageCapture.takeUprightJpeg(executor: Executor): ByteArray =
    suspendCancellableCoroutine { cont ->
        takePicture(
            executor,
            object : ImageCapture.OnImageCapturedCallback() {
                override fun onCaptureSuccess(image: ImageProxy) {
                    try {
                        val bitmap: Bitmap = image.toBitmap()
                        val bytes = toUprightJpeg(bitmap, image.imageInfo.rotationDegrees)
                        bitmap.recycle()
                        cont.resume(bytes)
                    } catch (e: Exception) {
                        cont.resumeWithException(e)
                    } finally {
                        image.close()
                    }
                }

                override fun onError(exception: ImageCaptureException) {
                    cont.resumeWithException(exception)
                }
            },
        )
    }
