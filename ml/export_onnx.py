"""Экспорт обученной модели в ONNX для инференса на Go.

Запуск:
    python ml/export_onnx.py --ckpt checkpoints/best.pt \
                             --out  checkpoints/face_embedding.onnx

Размер ONNX-файла попадёт в таблицу сравнения с face-api.js.
"""

from __future__ import annotations

import argparse
from pathlib import Path

import torch

from model import FaceEmbeddingNet


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--ckpt", type=Path, default=Path("checkpoints/best.pt"))
    parser.add_argument("--out", type=Path,
                        default=Path("checkpoints/face_embedding.onnx"))
    parser.add_argument("--opset", type=int, default=17)
    args = parser.parse_args()

    model = FaceEmbeddingNet()
    state = torch.load(args.ckpt, map_location="cpu")
    # strict=False — classifier head в инференсе не нужен
    model.load_state_dict(state["model"], strict=False)
    model.eval()

    dummy = torch.randn(1, 3, FaceEmbeddingNet.INPUT_SIZE, FaceEmbeddingNet.INPUT_SIZE)

    args.out.parent.mkdir(parents=True, exist_ok=True)

    common = {
        "input_names": ["input"],
        "output_names": ["embedding"],
        "dynamic_axes": {"input": {0: "batch"}, "embedding": {0: "batch"}},
        "opset_version": args.opset,
    }

    # В новых PyTorch (2.5+) dynamo=True требует onnxscript. Если его нет —
    # откатываемся на старый TorchScript-экспортёр, он работает без onnxscript.
    try:
        torch.onnx.export(model, dummy, str(args.out), dynamo=False, **common)
    except TypeError:
        # старые PyTorch не знают про параметр dynamo
        torch.onnx.export(model, dummy, str(args.out), **common)

    size_mb = args.out.stat().st_size / (1024 * 1024)
    print(f"[export] сохранено: {args.out}")
    print(f"[export] размер ONNX: {size_mb:.2f} MB")
    print(f"[export] input  shape: (B, 3, 112, 112), значения в [-1, 1]")
    print(f"[export] output shape: (B, 128), L2-нормализовано")

    # Быстрая sanity-проверка через onnxruntime, если установлен
    try:
        import numpy as np
        import onnxruntime as ort

        sess = ort.InferenceSession(str(args.out), providers=["CPUExecutionProvider"])
        x = np.random.randn(1, 3, 112, 112).astype(np.float32)
        y = sess.run(None, {"input": x})[0]
        norm = float(np.linalg.norm(y[0]))
        print(f"[export] onnxruntime sanity OK: output shape={y.shape}, ||y||={norm:.4f}")
    except ImportError:
        print("[export] onnxruntime не установлен — пропускаю sanity-проверку")


if __name__ == "__main__":
    main()
