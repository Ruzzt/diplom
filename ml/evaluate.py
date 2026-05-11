"""Оценка модели на стандартных парах LFW.

Запуск:
    python ml/evaluate.py --ckpt checkpoints/best.pt \
                          --aligned data/lfw_aligned \
                          --pairs data/pairs.txt

Метрики, которые считаются:
- Accuracy (по оптимальному порогу)
- Best threshold
- EER (equal error rate)
- ROC AUC
- TPR @ FPR=0.01 (типичная рабочая точка верификации)
- Среднее время инференса одного лица
- Размер чекпойнта на диске

Эти числа можно прямо переносить в таблицу пояснительной записки.
"""

from __future__ import annotations

import argparse
import json
import time
from pathlib import Path

import matplotlib

matplotlib.use("Agg")
import matplotlib.pyplot as plt
import numpy as np
import torch
from sklearn.metrics import roc_curve, auc
from torch.utils.data import DataLoader

from dataset import PairsDataset, build_eval_transform
from model import FaceEmbeddingNet, count_parameters


def pick_device() -> torch.device:
    if torch.backends.mps.is_available():
        return torch.device("mps")
    if torch.cuda.is_available():
        return torch.device("cuda")
    return torch.device("cpu")


@torch.no_grad()
def compute_distances(model: FaceEmbeddingNet, loader: DataLoader,
                      device: torch.device) -> tuple[np.ndarray, np.ndarray, float]:
    model.eval()
    all_dist: list[float] = []
    all_labels: list[int] = []
    total_imgs = 0
    t_start = time.time()
    for img1, img2, label in loader:
        img1 = img1.to(device)
        img2 = img2.to(device)
        e1 = model(img1)
        e2 = model(img2)
        # L2-расстояние; embeddings уже нормализованы (||x||=1),
        # значит d^2 = 2 - 2*cos, то есть монотонная замена косинусу.
        dist = (e1 - e2).pow(2).sum(dim=1).sqrt()
        all_dist.extend(dist.cpu().tolist())
        all_labels.extend(label.tolist())
        total_imgs += img1.size(0) * 2
    dt = time.time() - t_start
    per_img_ms = (dt / max(total_imgs, 1)) * 1000.0
    return np.asarray(all_dist), np.asarray(all_labels), per_img_ms


def find_best_threshold(distances: np.ndarray, labels: np.ndarray) -> tuple[float, float]:
    """Перебор порога с шагом 0.01, выбираем по accuracy."""
    thresholds = np.arange(distances.min(), distances.max(), 0.01)
    best_acc, best_t = 0.0, 0.0
    for t in thresholds:
        pred = (distances < t).astype(int)
        acc = float((pred == labels).mean())
        if acc > best_acc:
            best_acc = acc
            best_t = float(t)
    return best_t, best_acc


def equal_error_rate(fpr: np.ndarray, tpr: np.ndarray) -> float:
    fnr = 1.0 - tpr
    i = int(np.nanargmin(np.abs(fpr - fnr)))
    return float((fpr[i] + fnr[i]) / 2)


def tpr_at_fpr(fpr: np.ndarray, tpr: np.ndarray, target: float) -> float:
    valid = fpr <= target
    if not valid.any():
        return 0.0
    return float(tpr[valid].max())


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--ckpt", type=Path, default=Path("checkpoints/best.pt"))
    parser.add_argument("--aligned", type=Path, default=Path("data/lfw_aligned"))
    parser.add_argument("--pairs", type=Path, default=Path("data/pairs.txt"))
    parser.add_argument("--batch", type=int, default=32)
    parser.add_argument("--out", type=Path, default=Path("checkpoints/eval.json"))
    args = parser.parse_args()

    device = pick_device()
    print(f"[eval] устройство: {device}")

    model = FaceEmbeddingNet().to(device)
    state = torch.load(args.ckpt, map_location=device)
    # strict=False — игнорируем веса classifier head, который есть только
    # в чекпойнтах после обучения, но не нужен для инференса
    missing, unexpected = model.load_state_dict(state["model"], strict=False)
    if unexpected:
        print(f"[eval] игнорирую веса (только для обучения): {unexpected}")
    if missing:
        print(f"[eval] ВНИМАНИЕ: пропущены веса: {missing}")
    n_params = count_parameters(model)
    ckpt_size_mb = args.ckpt.stat().st_size / (1024 * 1024)
    print(f"[eval] параметров: {n_params:,}, размер чекпойнта: {ckpt_size_mb:.2f} MB")

    ds = PairsDataset(aligned_root=args.aligned, pairs_file=args.pairs,
                      transform=build_eval_transform())
    loader = DataLoader(ds, batch_size=args.batch, shuffle=False, num_workers=2)

    distances, labels, per_img_ms = compute_distances(model, loader, device)

    # similarity = -distance (чем больше — тем больше уверенность, что одно лицо)
    scores = -distances
    fpr, tpr, _ = roc_curve(labels, scores)
    roc_auc = auc(fpr, tpr)

    best_t, best_acc = find_best_threshold(distances, labels)
    eer = equal_error_rate(fpr, tpr)
    tpr_001 = tpr_at_fpr(fpr, tpr, 0.01)

    results = {
        "params": n_params,
        "checkpoint_size_mb": round(ckpt_size_mb, 3),
        "pairs": int(len(labels)),
        "accuracy": round(best_acc, 4),
        "best_threshold": round(best_t, 4),
        "eer": round(eer, 4),
        "roc_auc": round(roc_auc, 4),
        "tpr_at_fpr_0.01": round(tpr_001, 4),
        "inference_ms_per_image": round(per_img_ms, 3),
        "device": str(device),
    }

    args.out.parent.mkdir(parents=True, exist_ok=True)
    with open(args.out, "w", encoding="utf-8") as f:
        json.dump(results, f, indent=2)

    # ROC-кривая для записки
    fig, ax = plt.subplots(figsize=(5, 5))
    ax.plot(fpr, tpr, label=f"AUC={roc_auc:.3f}")
    ax.plot([0, 1], [0, 1], "k--", alpha=0.4)
    ax.set_xlabel("False Positive Rate")
    ax.set_ylabel("True Positive Rate")
    ax.set_title("ROC — собственная модель на LFW")
    ax.grid(True, alpha=0.3)
    ax.legend()
    fig.tight_layout()
    fig.savefig(args.out.with_suffix(".roc.png"), dpi=120)
    plt.close(fig)

    print("\n=== РЕЗУЛЬТАТЫ ===")
    for k, v in results.items():
        print(f"  {k:24s} {v}")
    print(f"\nROC сохранён в: {args.out.with_suffix('.roc.png')}")
    print(f"JSON: {args.out}")


if __name__ == "__main__":
    main()
