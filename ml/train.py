"""Обучение FaceEmbeddingNet.

Поддерживаются две стратегии (--mining):

  random      — случайные триплеты, классический TripletMarginLoss.
                Быстро сходится, но цепляет только лёгкие negatives.
                Раньше с ним получили 70% на LFW pairs.

  batch-hard  — PK-batch + batch-hard mining (Hermans et al. 2017).
                В каждом батче берём P людей по K фото, для каждого якоря
                выбираем самый дальний positive и самый близкий negative.
                Учит модель различать ПОХОЖИХ людей.
                Это путь B.

Запуск (по умолчанию — batch-hard):
    python train.py
    python train.py --mining random   # старый режим для сравнения
"""

from __future__ import annotations

import argparse
import json
import time
from pathlib import Path

import matplotlib

matplotlib.use("Agg")
import matplotlib.pyplot as plt
import torch
import torch.nn as nn
import torch.nn.functional as F
from torch.utils.data import DataLoader

from dataset import (PKBatchSampler, PKFaceDataset, TripletFaceDataset,
                     build_train_transform)
from model import FaceEmbeddingNet, count_parameters


def pick_device() -> torch.device:
    if torch.backends.mps.is_available():
        return torch.device("mps")
    if torch.cuda.is_available():
        return torch.device("cuda")
    return torch.device("cpu")


# ---------- метрики и loss-функции ----------

def pairwise_distances(embeddings: torch.Tensor) -> torch.Tensor:
    """Квадрат евклидова расстояния между всеми парами в батче.

    Для L2-нормализованных embeddings: ||a-b||^2 = 2 - 2*<a,b>.
    """
    dot = embeddings @ embeddings.t()
    dot = dot.clamp(-1.0, 1.0)
    dist_sq = (2.0 - 2.0 * dot).clamp(min=0.0)
    return dist_sq


def batch_hard_triplet_loss(embeddings: torch.Tensor, labels: torch.Tensor,
                            margin: float) -> tuple[torch.Tensor, dict]:
    """Batch-hard triplet loss в КВАДРАТНОМ расстоянии (без sqrt).

    Считаем в squared distance, чтобы избежать sqrt(0)→NaN в backward
    при близких embeddings одного класса. margin тогда тоже в squared-шкале:
    для нормализованных embeddings d^2 ∈ [0, 4], поэтому margin~0.2 здесь
    эквивалентен margin~0.45 в линейном расстоянии.
    """
    dist_sq = pairwise_distances(embeddings)  # уже >= 0 и без sqrt

    labels_eq = labels.unsqueeze(0) == labels.unsqueeze(1)
    eye = torch.eye(labels.size(0), dtype=torch.bool, device=labels.device)
    pos_mask = labels_eq & ~eye
    neg_mask = ~labels_eq

    dist_pos = dist_sq.masked_fill(~pos_mask, float("-inf"))
    hardest_pos, _ = dist_pos.max(dim=1)

    dist_neg = dist_sq.masked_fill(~neg_mask, float("inf"))
    hardest_neg, _ = dist_neg.min(dim=1)

    valid = torch.isfinite(hardest_pos) & torch.isfinite(hardest_neg)
    if not valid.any():
        return dist_sq.sum() * 0.0, {"frac_active": 0.0,
                                      "mean_pos": 0.0, "mean_neg": 0.0}

    hardest_pos = hardest_pos[valid]
    hardest_neg = hardest_neg[valid]
    losses = F.relu(hardest_pos - hardest_neg + margin)
    loss = losses.mean()

    info = {
        "frac_active": float((losses > 0).float().mean().item()),
        "mean_pos": float(hardest_pos.mean().item()),
        "mean_neg": float(hardest_neg.mean().item()),
    }
    return loss, info


def triplet_accuracy_batch(embeddings: torch.Tensor,
                           labels: torch.Tensor) -> float:
    """Доля якорей, у которых max d^2(a,p) < min d^2(a,n) внутри батча."""
    dist_sq = pairwise_distances(embeddings)
    labels_eq = labels.unsqueeze(0) == labels.unsqueeze(1)
    eye = torch.eye(labels.size(0), dtype=torch.bool, device=labels.device)
    pos_mask = labels_eq & ~eye
    neg_mask = ~labels_eq
    dp = dist_sq.masked_fill(~pos_mask, float("-inf")).max(dim=1).values
    dn = dist_sq.masked_fill(~neg_mask, float("inf")).min(dim=1).values
    valid = torch.isfinite(dp) & torch.isfinite(dn)
    if not valid.any():
        return 0.0
    return float((dp[valid] < dn[valid]).float().mean().item())


# ---------- режимы обучения ----------

def train_batch_hard(args: argparse.Namespace, device: torch.device,
                     ckpt_dir: Path) -> None:
    """Multi-task обучение: CrossEntropy (классификация по людям) + batch-hard
    triplet loss. CE предотвращает embedding-коллапс, который случается с
    чистым batch-hard в начале обучения (все эмбеддинги схлопываются в точку).
    """
    dataset = PKFaceDataset(
        root=args.data,
        min_images_per_class=args.k,
        transform=build_train_transform(),
    )
    batch_sampler = PKBatchSampler(
        dataset, p=args.p, k=args.k, num_iters=args.iters_per_epoch
    )
    loader = DataLoader(
        dataset,
        batch_sampler=batch_sampler,
        num_workers=args.workers,
        pin_memory=False,
    )

    n_classes = len(dataset.class_names)
    model = FaceEmbeddingNet(num_classes=n_classes).to(device)
    print(f"[train] параметров: {count_parameters(model):,} "
          f"(c classifier head на {n_classes} классов)")

    ce_loss = nn.CrossEntropyLoss()
    optimizer = torch.optim.Adam(model.parameters(), lr=args.lr,
                                 weight_decay=args.weight_decay)
    scheduler = torch.optim.lr_scheduler.CosineAnnealingLR(
        optimizer, T_max=args.epochs)

    history: dict[str, list[float]] = {"loss": [], "loss_ce": [], "loss_tri": [],
                                       "acc": [], "ce_acc": [], "lr": [],
                                       "frac_active": []}
    best_acc = 0.0

    print(f"[train] mining=batch-hard + CE, batch={args.p}×{args.k}={args.p * args.k}, "
          f"alpha_triplet={args.alpha}")

    for epoch in range(1, args.epochs + 1):
        model.train()
        t0 = time.time()
        sums = {"total": 0.0, "ce": 0.0, "tri": 0.0,
                "acc": 0.0, "ce_acc": 0.0, "active": 0.0}
        n_batches = 0

        for batch_idx, (images, labels) in enumerate(loader, 1):
            images = images.to(device, non_blocking=True)
            labels = labels.to(device, non_blocking=True)

            embeddings, logits = model.forward_train(images)
            loss_ce = ce_loss(logits, labels)
            loss_tri, info = batch_hard_triplet_loss(embeddings, labels, args.margin)
            loss = loss_ce + args.alpha * loss_tri

            optimizer.zero_grad(set_to_none=True)
            loss.backward()
            torch.nn.utils.clip_grad_norm_(model.parameters(), max_norm=5.0)
            optimizer.step()

            sums["total"] += float(loss.item())
            sums["ce"] += float(loss_ce.item())
            sums["tri"] += float(loss_tri.item())
            sums["acc"] += triplet_accuracy_batch(embeddings.detach(), labels)
            sums["ce_acc"] += float((logits.argmax(dim=1) == labels).float().mean().item())
            sums["active"] += info["frac_active"]
            n_batches += 1

            if batch_idx % args.log_every == 0:
                print(f"  epoch {epoch} batch {batch_idx}/{len(loader)} "
                      f"loss={sums['total']/n_batches:.4f} "
                      f"(ce={sums['ce']/n_batches:.3f} "
                      f"tri={sums['tri']/n_batches:.3f}) "
                      f"acc={sums['acc']/n_batches:.3f} "
                      f"ce_acc={sums['ce_acc']/n_batches:.3f} "
                      f"active={sums['active']/n_batches:.2f}")

        scheduler.step()
        n = max(n_batches, 1)
        ep = {k: v / n for k, v in sums.items()}
        dt = time.time() - t0

        history["loss"].append(ep["total"])
        history["loss_ce"].append(ep["ce"])
        history["loss_tri"].append(ep["tri"])
        history["acc"].append(ep["acc"])
        history["ce_acc"].append(ep["ce_acc"])
        history["lr"].append(optimizer.param_groups[0]["lr"])
        history["frac_active"].append(ep["active"])

        print(f"[epoch {epoch:3d}/{args.epochs}] loss={ep['total']:.4f} "
              f"acc={ep['acc']:.4f} ce_acc={ep['ce_acc']:.4f} "
              f"active={ep['active']:.3f} time={dt:.1f}s")

        _save_state(model, ckpt_dir, epoch, ep["acc"], best_acc, history)
        if ep["acc"] > best_acc:
            best_acc = ep["acc"]

    print(f"[train] обучение завершено. Лучшая triplet acc: {best_acc:.4f}")
    print(f"[train] чекпойнты в: {ckpt_dir.resolve()}")


def train_random(args: argparse.Namespace, device: torch.device,
                 ckpt_dir: Path) -> None:
    """Старый режим со случайными триплетами — для контрольного эксперимента."""
    model = FaceEmbeddingNet().to(device)
    print(f"[train] параметров: {count_parameters(model):,}")
    train_ds = TripletFaceDataset(
        root=args.data,
        min_images_per_class=args.k,
        transform=build_train_transform(),
        length=args.iters_per_epoch,
    )
    loader = DataLoader(train_ds, batch_size=args.batch, shuffle=False,
                        num_workers=args.workers, drop_last=True)

    criterion = nn.TripletMarginLoss(margin=args.margin, p=2)
    optimizer = torch.optim.Adam(model.parameters(), lr=args.lr,
                                 weight_decay=args.weight_decay)
    scheduler = torch.optim.lr_scheduler.CosineAnnealingLR(
        optimizer, T_max=args.epochs)

    history: dict[str, list[float]] = {"loss": [], "acc": [], "lr": []}
    best_acc = 0.0

    print(f"[train] mining=random, batch={args.batch}")

    for epoch in range(1, args.epochs + 1):
        model.train()
        t0 = time.time()
        total_loss, total_acc, n_batches = 0.0, 0.0, 0

        for batch_idx, (a, p, n) in enumerate(loader, 1):
            a = a.to(device); p = p.to(device); n = n.to(device)
            ea, ep_, en = model(a), model(p), model(n)
            loss = criterion(ea, ep_, en)
            optimizer.zero_grad(set_to_none=True)
            loss.backward()
            optimizer.step()

            total_loss += float(loss.item())
            dap = (ea - ep_).pow(2).sum(dim=1)
            dan = (ea - en).pow(2).sum(dim=1)
            total_acc += float((dap < dan).float().mean().item())
            n_batches += 1
            if batch_idx % args.log_every == 0:
                print(f"  epoch {epoch} batch {batch_idx}/{len(loader)} "
                      f"loss={total_loss/n_batches:.4f} acc={total_acc/n_batches:.4f}")

        scheduler.step()
        ep_loss = total_loss / max(n_batches, 1)
        ep_acc = total_acc / max(n_batches, 1)
        dt = time.time() - t0
        history["loss"].append(ep_loss)
        history["acc"].append(ep_acc)
        history["lr"].append(optimizer.param_groups[0]["lr"])
        print(f"[epoch {epoch:3d}/{args.epochs}] loss={ep_loss:.4f} "
              f"acc={ep_acc:.4f} time={dt:.1f}s")

        _save_state(model, ckpt_dir, epoch, ep_acc, best_acc, history)
        if ep_acc > best_acc:
            best_acc = ep_acc

    print(f"[train] обучение завершено. Лучшая точность: {best_acc:.4f}")


def _save_state(model: nn.Module, ckpt_dir: Path, epoch: int, acc: float,
                best_acc: float, history: dict) -> None:
    torch.save({"model": model.state_dict(), "epoch": epoch, "acc": acc},
               ckpt_dir / "last.pt")
    if acc > best_acc:
        torch.save({"model": model.state_dict(), "epoch": epoch, "acc": acc},
                   ckpt_dir / "best.pt")
        print(f"  -> новый лучший чекпойнт (acc={acc:.4f})")

    with open(ckpt_dir / "train_log.json", "w", encoding="utf-8") as f:
        json.dump(history, f, indent=2)
    _plot(history, ckpt_dir / "training.png")


def _plot(history: dict[str, list[float]], path: Path) -> None:
    has_active = "frac_active" in history and history["frac_active"]
    n_panels = 3 if has_active else 2
    fig, axes = plt.subplots(1, n_panels, figsize=(5 * n_panels, 4))
    axes[0].plot(history["loss"], color="tab:red")
    axes[0].set_title("Triplet loss")
    axes[0].set_xlabel("epoch")
    axes[0].grid(True, alpha=0.3)
    axes[1].plot(history["acc"], color="tab:blue")
    axes[1].set_title("Batch triplet acc")
    axes[1].set_xlabel("epoch")
    axes[1].set_ylim(0.4, 1.0)
    axes[1].grid(True, alpha=0.3)
    if has_active:
        axes[2].plot(history["frac_active"], color="tab:green")
        axes[2].set_title("Fraction of active triplets")
        axes[2].set_xlabel("epoch")
        axes[2].set_ylim(0.0, 1.0)
        axes[2].grid(True, alpha=0.3)
    fig.tight_layout()
    fig.savefig(path, dpi=110)
    plt.close(fig)


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--data", type=Path, default=Path("data/lfw_aligned"))
    parser.add_argument("--checkpoints", type=Path, default=Path("checkpoints"))
    parser.add_argument("--mining", choices=["batch-hard", "random"],
                        default="batch-hard")
    parser.add_argument("--epochs", type=int, default=40)
    parser.add_argument("--lr", type=float, default=5e-4)
    parser.add_argument("--weight-decay", type=float, default=5e-4)
    parser.add_argument("--margin", type=float, default=0.2,
                        help="margin в квадрате расстояния (squared L2). "
                             "Для нормализованных embeddings d^2 ∈ [0, 4], "
                             "0.2 ≈ 0.45 в линейной шкале")
    parser.add_argument("--iters-per-epoch", type=int, default=1500)
    parser.add_argument("--workers", type=int, default=2)
    parser.add_argument("--log-every", type=int, default=100)
    # PK-параметры (batch-hard)
    parser.add_argument("--p", type=int, default=8, help="людей в батче")
    parser.add_argument("--k", type=int, default=4, help="фото на человека в батче")
    # batch для режима random
    parser.add_argument("--batch", type=int, default=32)
    # multi-task: вес triplet loss относительно CE
    parser.add_argument("--alpha", type=float, default=0.5,
                        help="вес batch-hard triplet loss в multi-task (CE + α·triplet)")
    args = parser.parse_args()

    device = pick_device()
    print(f"[train] устройство: {device}")

    ckpt_dir = Path(args.checkpoints)
    ckpt_dir.mkdir(parents=True, exist_ok=True)

    if args.mining == "batch-hard":
        train_batch_hard(args, device, ckpt_dir)
    else:
        train_random(args, device, ckpt_dir)


if __name__ == "__main__":
    main()
