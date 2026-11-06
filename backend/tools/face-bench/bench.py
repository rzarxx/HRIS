"""Uji banding model pengenalan wajah untuk D42.

Semua model memakai foto dan pasangan yang sama. SFace dan ArcFace memakai detektor YuNet yang sama,
jadi perbedaannya hanya dari model pengenal. dlib memakai detektor HOG bawaannya, sama seperti go-face.

Metrik utama untuk absen adalah TAR pada FAR = 0: berapa persen pasangan orang yang sama tetap
diterima bila ambang dipasang tepat di atas pasangan orang berbeda yang paling mirip. Angka itu
menunjukkan apakah ada ambang yang aman sekaligus bisa dipakai.

Pemakaian: python bench.py --images DIR --models DIR [--summary FILE]
Nama berkas menandai orangnya: bona.jpg dan bona2.jpg adalah orang yang sama.
"""

import argparse
import glob
import os
import re
import sys

import cv2
import numpy as np

# Posisi 5 titik wajah standar ArcFace pada gambar 112x112. Urutannya sama dengan keluaran YuNet.
ARCFACE_TEMPLATE = np.array(
    [[38.2946, 51.6963], [73.5318, 51.5014], [56.0252, 71.7366], [41.5493, 92.3655], [70.7299, 92.2041]],
    dtype=np.float32,
)


def person_of(path):
    return re.sub(r"\d+$", "", os.path.splitext(os.path.basename(path))[0])


def largest(faces):
    return max(faces, key=lambda f: f[2] * f[3])


class YuNet:
    def __init__(self, path):
        self.det = cv2.FaceDetectorYN.create(path, "", (320, 320), 0.7, 0.3, 5000)

    def detect(self, img):
        h, w = img.shape[:2]
        self.det.setInputSize((w, h))
        _, faces = self.det.detect(img)
        if faces is None or len(faces) == 0:
            return None
        return largest(faces)


class SFace:
    name = "SFace (OpenCV Zoo)"
    license = "Apache-2.0"

    def __init__(self, models, yunet):
        self.rec = cv2.FaceRecognizerSF.create(os.path.join(models, "sface.onnx"), "")
        self.yunet = yunet

    def embed(self, img):
        face = self.yunet.detect(img)
        if face is None:
            return None
        aligned = self.rec.alignCrop(img, face)
        v = self.rec.feature(aligned).flatten().astype(np.float64)
        return v / np.linalg.norm(v)

    @staticmethod
    def similarity(a, b):
        return float(np.dot(a, b))


class ArcFace:
    name = "ArcFace R50 (InsightFace buffalo_l)"
    license = "non-komersial, hanya pembanding"

    def __init__(self, models, yunet):
        import onnxruntime as ort

        self.sess = ort.InferenceSession(os.path.join(models, "w600k_r50.onnx"), providers=["CPUExecutionProvider"])
        self.input = self.sess.get_inputs()[0].name
        self.yunet = yunet

    def embed(self, img):
        face = self.yunet.detect(img)
        if face is None:
            return None
        landmarks = face[4:14].reshape(5, 2).astype(np.float32)
        matrix, _ = cv2.estimateAffinePartial2D(landmarks, ARCFACE_TEMPLATE, method=cv2.LMEDS)
        aligned = cv2.warpAffine(img, matrix, (112, 112), borderValue=0.0)
        blob = cv2.dnn.blobFromImage(aligned, 1.0 / 127.5, (112, 112), (127.5, 127.5, 127.5), swapRB=True)
        v = self.sess.run(None, {self.input: blob})[0].flatten().astype(np.float64)
        return v / np.linalg.norm(v)

    @staticmethod
    def similarity(a, b):
        return float(np.dot(a, b))


class Dlib:
    name = "dlib ResNet (model sekarang)"
    license = "domain publik"

    def __init__(self, models):
        import dlib

        self.det = dlib.get_frontal_face_detector()
        self.sp = dlib.shape_predictor(os.path.join(models, "shape_predictor_5_face_landmarks.dat"))
        self.rec = dlib.face_recognition_model_v1(os.path.join(models, "dlib_face_recognition_resnet_model_v1.dat"))

    def embed(self, img):
        rgb = cv2.cvtColor(img, cv2.COLOR_BGR2RGB)
        dets = self.det(rgb, 1)
        if len(dets) == 0:
            return None
        det = max(dets, key=lambda r: r.width() * r.height())
        shape = self.sp(rgb, det)
        return np.array(self.rec.compute_face_descriptor(rgb, shape), dtype=np.float64)

    @staticmethod
    def similarity(a, b):
        # Jarak Euclid dibalik agar semua model memakai "lebih besar = lebih mirip".
        return -float(np.linalg.norm(a - b))


def evaluate(model, paths):
    vectors = []
    skipped = []
    for p in paths:
        img = cv2.imread(p)
        v = model.embed(img) if img is not None else None
        if v is None:
            skipped.append(os.path.basename(p))
            continue
        vectors.append((person_of(p), os.path.basename(p), v))

    genuine, impostor = [], []
    closest = None
    for i in range(len(vectors)):
        for j in range(i + 1, len(vectors)):
            a, b = vectors[i], vectors[j]
            s = model.similarity(a[2], b[2])
            if a[0] == b[0]:
                genuine.append(s)
            else:
                impostor.append(s)
                if closest is None or s > closest[0]:
                    closest = (s, a[1], b[1])
    genuine, impostor = np.array(genuine), np.array(impostor)

    # TAR pada FAR = 0 dan FAR <= 1%.
    worst_impostor = impostor.max()
    tar_far0 = float((genuine > worst_impostor).mean())
    far1_threshold = np.quantile(impostor, 0.99)
    tar_far1 = float((genuine > far1_threshold).mean())

    # EER: titik saat salah terima sama dengan salah tolak.
    best = (1.0, None)
    for t in np.unique(np.concatenate([genuine, impostor])):
        far = (impostor >= t).mean()
        frr = (genuine < t).mean()
        gap = abs(far - frr)
        if gap < best[0]:
            best = (gap, (far + frr) / 2)
    eer = float(best[1])

    # Separasi: jarak rata-rata dua kelompok dibagi sebarannya (d-prime).
    dprime = float((genuine.mean() - impostor.mean()) / np.sqrt((genuine.var() + impostor.var()) / 2))
    return {
        "read": len(vectors),
        "skipped": skipped,
        "genuine": len(genuine),
        "impostor": len(impostor),
        "tar_far0": tar_far0,
        "tar_far1": tar_far1,
        "eer": eer,
        "dprime": dprime,
        "genuine_range": (float(genuine.min()), float(genuine.mean()), float(genuine.max())),
        "impostor_range": (float(impostor.min()), float(impostor.mean()), float(impostor.max())),
        "closest": closest,
    }


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--images", required=True)
    parser.add_argument("--models", required=True)
    parser.add_argument("--summary")
    args = parser.parse_args()

    paths = sorted(glob.glob(os.path.join(args.images, "*.jpg")))
    if len(paths) < 10:
        sys.exit(f"foto contoh kurang: {len(paths)}")

    yunet = YuNet(os.path.join(args.models, "yunet.onnx"))
    models = []
    for build in (lambda: Dlib(args.models), lambda: SFace(args.models, yunet), lambda: ArcFace(args.models, yunet)):
        try:
            models.append(build())
        except Exception as exc:  # noqa: BLE001 - satu model gagal dimuat tidak menghentikan yang lain
            print(f"model gagal dimuat: {exc}", file=sys.stderr)

    lines = [
        "## Uji banding model wajah",
        "",
        f"{len(paths)} foto contoh dari go-face-testdata. Skor dinormalkan agar lebih besar = lebih mirip.",
        "",
        "| Model | Lisensi | Foto terbaca | TAR @ FAR 0 | TAR @ FAR 1% | EER | d' |",
        "|---|---|---|---|---|---|---|",
    ]
    details = []
    for m in models:
        r = evaluate(m, paths)
        lines.append(
            f"| {m.name} | {m.license} | {r['read']}/{len(paths)} | {r['tar_far0']:.0%} | {r['tar_far1']:.0%} "
            f"| {r['eer']:.1%} | {r['dprime']:.2f} |"
        )
        g, i = r["genuine_range"], r["impostor_range"]
        details += [
            f"### {m.name}",
            f"- Pasangan: {r['genuine']} orang sama, {r['impostor']} orang berbeda. Terlewat: {', '.join(r['skipped']) or '-'}",
            f"- Orang sama: min {g[0]:.3f}, rata-rata {g[1]:.3f}, maks {g[2]:.3f}",
            f"- Orang berbeda: min {i[0]:.3f}, rata-rata {i[1]:.3f}, maks {i[2]:.3f}",
            f"- Orang berbeda paling mirip: {r['closest'][1]} dan {r['closest'][2]} ({r['closest'][0]:.3f})",
            "",
        ]
    lines += [
        "",
        "TAR @ FAR 0: persen pasangan orang sama yang diterima bila tidak ada orang berbeda yang boleh lolos. "
        "EER: titik saat salah terima sama dengan salah tolak (lebih kecil lebih baik). "
        "d': seberapa terpisah dua kelompok (lebih besar lebih baik).",
        "",
    ] + details
    report = "\n".join(lines)
    print(report)
    if args.summary:
        with open(args.summary, "a", encoding="utf-8") as f:
            f.write(report + "\n")


if __name__ == "__main__":
    main()
