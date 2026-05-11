"""Выравнивание лиц через MTCNN.

Берёт сырой LFW (data/lfw_raw/Person_Name/*.jpg), детектит лицо, обрезает
и сохраняет 112x112 в data/lfw_aligned/Person_Name/. Этот шаг важен:
без выравнивания качество распознавания резко падает.

Использует facenet-pytorch.MTCNN (детектор лиц + landmarks).

Запуск:
    python ml/align.py --src data/lfw_raw --dst data/lfw_aligned
"""

from __future__ import annotations

import argparse
from pathlib import Path

import torch
from PIL import Image
from tqdm import tqdm

from facenet_pytorch import MTCNN


def pick_device(force_cpu: bool = False) -> torch.device:
    # MTCNN использует adaptive_avg_pool2d с произвольными scale-факторами,
    # а на MPS эта операция падает, если входной размер не делится на выходной
    # нацело (известный баг PyTorch). Поэтому по умолчанию — CPU.
    if force_cpu:
        return torch.device("cpu")
    if torch.cuda.is_available():
        return torch.device("cuda")
    return torch.device("cpu")


def align_dataset(src: Path, dst: Path, image_size: int = 112, margin: int = 14,
                  device_override: torch.device | None = None) -> None:
    device = device_override or pick_device()
    print(f"[align] устройство: {device}")

    mtcnn = MTCNN(
        image_size=image_size,
        margin=margin,
        post_process=False,
        device=device,
        keep_all=False,
        select_largest=True,
    )

    persons = sorted(p for p in src.iterdir() if p.is_dir())
    if not persons:
        raise SystemExit(f"В {src} нет папок с фото. Распакуй LFW сначала.")

    dst.mkdir(parents=True, exist_ok=True)
    saved, skipped = 0, 0

    for person_dir in tqdm(persons, desc="people"):
        out_dir = dst / person_dir.name
        out_dir.mkdir(exist_ok=True)
        for img_path in person_dir.iterdir():
            if img_path.suffix.lower() not in {".jpg", ".jpeg", ".png"}:
                continue
            out_path = out_dir / img_path.with_suffix(".jpg").name
            if out_path.exists():
                continue
            try:
                img = Image.open(img_path).convert("RGB")
            except Exception:
                skipped += 1
                continue
            face = mtcnn(img, save_path=str(out_path))
            if face is None:
                skipped += 1
            else:
                saved += 1

    print(f"[align] сохранено: {saved}, пропущено (лицо не найдено): {skipped}")


def main() -> None:
    parser = argparse.ArgumentParser(description="Align LFW faces with MTCNN")
    parser.add_argument("--src", type=Path, default=Path("data/lfw_raw"))
    parser.add_argument("--dst", type=Path, default=Path("data/lfw_aligned"))
    parser.add_argument("--image-size", type=int, default=112)
    parser.add_argument("--margin", type=int, default=14)
    parser.add_argument("--device", choices=["auto", "cpu", "mps", "cuda"],
                        default="auto",
                        help="по умолчанию CPU — на MPS MTCNN падает")
    args = parser.parse_args()

    if not args.src.is_dir():
        raise SystemExit(f"Не найдено: {args.src}. Скачай LFW и распакуй сюда.")

    device_override = None
    if args.device != "auto":
        device_override = torch.device(args.device)

    align_dataset(args.src, args.dst, args.image_size, args.margin, device_override)


if __name__ == "__main__":
    main()
