"""Embedding acuan SFace dari OpenCV untuk TestSFaceMatchesOpenCV (D43).

Langkahnya sama dengan mesin Go di internal/face: gambar diperkecil agar sisi terpanjang paling banyak
640, dipad ke 640x640 untuk YuNet, wajah terbesar diambil, titiknya dikembalikan ke ukuran asli, lalu
alignCrop dan feature SFace pada gambar asli. Hasilnya JSON {nama berkas: [128 angka]}.

Pemakaian: python reference.py --images DIR --models DIR --out FILE
"""

import argparse
import glob
import json
import os

import cv2
import numpy as np

SIZE = 640


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--images", required=True)
    parser.add_argument("--models", required=True)
    parser.add_argument("--out", required=True)
    args = parser.parse_args()

    detector = cv2.FaceDetectorYN.create(os.path.join(args.models, "yunet.onnx"), "", (SIZE, SIZE), 0.7, 0.3, 5000)
    recognizer = cv2.FaceRecognizerSF.create(os.path.join(args.models, "sface.onnx"), "")

    out = {}
    for path in sorted(glob.glob(os.path.join(args.images, "*.jpg"))):
        img = cv2.imread(path, cv2.IMREAD_COLOR | cv2.IMREAD_IGNORE_ORIENTATION)
        h, w = img.shape[:2]
        scale = min(1.0, SIZE / max(w, h))
        small = img
        if scale < 1:
            small = cv2.resize(img, (max(int(round(w * scale)), 1), max(int(round(h * scale)), 1)), interpolation=cv2.INTER_LINEAR)
        padded = np.zeros((SIZE, SIZE, 3), dtype=np.uint8)
        padded[: small.shape[0], : small.shape[1]] = small
        _, faces = detector.detect(padded)
        if faces is None or len(faces) == 0:
            continue
        face = max(faces, key=lambda f: f[2] * f[3]).copy()
        face[:14] /= scale
        aligned = recognizer.alignCrop(img, face)
        feature = recognizer.feature(aligned).flatten().astype(np.float64)
        out[os.path.basename(path)] = (feature / np.linalg.norm(feature)).tolist()

    with open(args.out, "w", encoding="utf-8") as f:
        json.dump(out, f)
    print(f"{len(out)} embedding acuan ditulis ke {args.out}")


if __name__ == "__main__":
    main()
