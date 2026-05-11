"""Загрузка LFW-датасета и сэмплер триплетов.

Структура ожидаемого датасета (после ml/align.py):

    data/lfw_aligned/
        Person_A/
            img1.jpg
            img2.jpg
        Person_B/
            ...

Сэмплер выдаёт триплеты (anchor, positive, negative), где anchor и positive
из одного класса, negative — из другого. Для обучения с Triplet Loss.
"""

from __future__ import annotations

import random
from pathlib import Path
from typing import Iterator, Tuple

import torch
from PIL import Image
from torch.utils.data import Dataset, Sampler
from torchvision import transforms


def build_train_transform() -> transforms.Compose:
    """Аугментации для обучения. Мягкий уровень: горизонтальный флип
    и небольшой ColorJitter. Сильные аугментации (RandomCrop, RandomRotation,
    RandomErasing) дестабилизировали batch-hard triplet и провоцировали
    embedding-коллапс — поэтому убраны."""
    return transforms.Compose([
        transforms.Resize((112, 112)),
        transforms.RandomHorizontalFlip(),
        transforms.ColorJitter(brightness=0.15, contrast=0.15),
        transforms.ToTensor(),
        transforms.Normalize(mean=[0.5, 0.5, 0.5], std=[0.5, 0.5, 0.5]),
    ])


def build_eval_transform() -> transforms.Compose:
    return transforms.Compose([
        transforms.Resize((112, 112)),
        transforms.ToTensor(),
        transforms.Normalize(mean=[0.5, 0.5, 0.5], std=[0.5, 0.5, 0.5]),
    ])


class TripletFaceDataset(Dataset):
    """Сэмплирует триплеты (anchor, positive, negative).

    Берёт только людей, у которых >= min_images_per_class фотографий.
    На каждой итерации __getitem__ возвращает один свежий триплет
    (онлайн-сэмплинг — нет фиксированного списка триплетов).
    """

    def __init__(self, root: str | Path, min_images_per_class: int = 5,
                 transform: transforms.Compose | None = None,
                 length: int | None = None,
                 seed: int = 42):
        self.root = Path(root)
        if not self.root.is_dir():
            raise FileNotFoundError(
                f"Папка датасета не найдена: {self.root}. "
                "Сначала запусти ml/align.py."
            )

        self.transform = transform or build_train_transform()
        self.rng = random.Random(seed)

        classes: dict[str, list[Path]] = {}
        for person_dir in sorted(self.root.iterdir()):
            if not person_dir.is_dir():
                continue
            images = sorted(
                p for p in person_dir.iterdir()
                if p.suffix.lower() in {".jpg", ".jpeg", ".png"}
            )
            if len(images) >= min_images_per_class:
                classes[person_dir.name] = images

        if len(classes) < 2:
            raise RuntimeError(
                f"Недостаточно классов после фильтра min={min_images_per_class}. "
                f"Найдено {len(classes)} классов с фото в {self.root}."
            )

        self.class_names = list(classes.keys())
        self.class_to_images = classes
        total_imgs = sum(len(v) for v in classes.values())
        self._length = length or total_imgs
        print(f"[Dataset] классов: {len(self.class_names)}, "
              f"всего фото: {total_imgs}, итераций/эпоху: {self._length}")

    def __len__(self) -> int:
        return self._length

    def _load(self, path: Path) -> torch.Tensor:
        img = Image.open(path).convert("RGB")
        return self.transform(img)

    def __getitem__(self, idx: int) -> Tuple[torch.Tensor, torch.Tensor, torch.Tensor]:
        anchor_class = self.rng.choice(self.class_names)
        while True:
            negative_class = self.rng.choice(self.class_names)
            if negative_class != anchor_class:
                break

        anchor_path, positive_path = self.rng.sample(self.class_to_images[anchor_class], 2)
        negative_path = self.rng.choice(self.class_to_images[negative_class])

        return self._load(anchor_path), self._load(positive_path), self._load(negative_path)


class PKFaceDataset(Dataset):
    """Плоский датасет лиц с метками классов. Используется вместе с
    PKBatchSampler для batch-hard triplet mining.

    Каждый __getitem__ возвращает (image, class_id).
    """

    def __init__(self, root: str | Path, min_images_per_class: int = 4,
                 transform: transforms.Compose | None = None):
        self.root = Path(root)
        if not self.root.is_dir():
            raise FileNotFoundError(f"Папка датасета не найдена: {self.root}")
        self.transform = transform or build_train_transform()

        classes: dict[str, list[Path]] = {}
        for person_dir in sorted(self.root.iterdir()):
            if not person_dir.is_dir():
                continue
            images = sorted(
                p for p in person_dir.iterdir()
                if p.suffix.lower() in {".jpg", ".jpeg", ".png"}
            )
            if len(images) >= min_images_per_class:
                classes[person_dir.name] = images

        if len(classes) < 2:
            raise RuntimeError(
                f"Недостаточно классов (min_images={min_images_per_class}): "
                f"найдено {len(classes)}."
            )

        self.class_names: list[str] = list(classes.keys())
        self.class_to_id: dict[str, int] = {n: i for i, n in enumerate(self.class_names)}
        self.samples: list[tuple[Path, int]] = []
        self.class_to_sample_idx: dict[int, list[int]] = {}
        for name, imgs in classes.items():
            cid = self.class_to_id[name]
            self.class_to_sample_idx[cid] = []
            for img in imgs:
                self.class_to_sample_idx[cid].append(len(self.samples))
                self.samples.append((img, cid))

        print(f"[PKDataset] классов: {len(self.class_names)}, "
              f"всего фото: {len(self.samples)}, "
              f"min_per_class: {min_images_per_class}")

    def __len__(self) -> int:
        return len(self.samples)

    def __getitem__(self, idx: int) -> Tuple[torch.Tensor, int]:
        path, label = self.samples[idx]
        img = Image.open(path).convert("RGB")
        return self.transform(img), label


class PKBatchSampler(Sampler[list[int]]):
    """Сэмплер «P людей × K фото» для batch-hard triplet mining.

    Каждый батч содержит ровно K экземпляров каждого из P случайно выбранных
    классов (batch_size = P * K). Внутри одного батча у каждого якоря есть
    хотя бы K-1 позитивов и (P-1)*K негативов — этого хватает, чтобы найти
    самый трудный позитив и самый трудный негатив.
    """

    def __init__(self, dataset: PKFaceDataset, p: int, k: int,
                 num_iters: int, seed: int = 42):
        if k < 2:
            raise ValueError("K должен быть >= 2 (нужны позитивы внутри батча)")
        eligible = [cid for cid, idxs in dataset.class_to_sample_idx.items()
                    if len(idxs) >= k]
        if len(eligible) < p:
            raise ValueError(
                f"Недостаточно классов с K={k} фото: {len(eligible)} < P={p}"
            )
        self.dataset = dataset
        self.p = p
        self.k = k
        self.num_iters = num_iters
        self.eligible = eligible
        self.rng = random.Random(seed)

    def __iter__(self) -> Iterator[list[int]]:
        for _ in range(self.num_iters):
            classes = self.rng.sample(self.eligible, self.p)
            batch: list[int] = []
            for cid in classes:
                pool = self.dataset.class_to_sample_idx[cid]
                batch.extend(self.rng.sample(pool, self.k))
            yield batch

    def __len__(self) -> int:
        return self.num_iters


class PairsDataset(Dataset):
    """Загружает пары лиц из стандартного файла LFW pairs.txt.

    Формат pairs.txt описан на сайте LFW. В каждой строке либо тройка
    "Name idxA idxB" (одна и та же персона), либо четвёрка
    "NameA idxA NameB idxB" (разные персоны).
    """

    def __init__(self, aligned_root: str | Path, pairs_file: str | Path,
                 transform: transforms.Compose | None = None):
        self.root = Path(aligned_root)
        self.transform = transform or build_eval_transform()
        self.pairs: list[tuple[Path, Path, int]] = []

        with open(pairs_file, "r", encoding="utf-8") as f:
            header = f.readline().strip().split()
            for line in f:
                parts = line.strip().split()
                if not parts:
                    continue
                if len(parts) == 3:
                    name, a, b = parts
                    p1 = self._img_path(name, int(a))
                    p2 = self._img_path(name, int(b))
                    label = 1
                elif len(parts) == 4:
                    name1, a, name2, b = parts
                    p1 = self._img_path(name1, int(a))
                    p2 = self._img_path(name2, int(b))
                    label = 0
                else:
                    continue
                if p1.exists() and p2.exists():
                    self.pairs.append((p1, p2, label))

        print(f"[PairsDataset] пар: {len(self.pairs)} "
              f"(positive={sum(1 for _, _, l in self.pairs if l == 1)}, "
              f"negative={sum(1 for _, _, l in self.pairs if l == 0)})")

    def _img_path(self, name: str, idx: int) -> Path:
        return self.root / name / f"{name}_{idx:04d}.jpg"

    def __len__(self) -> int:
        return len(self.pairs)

    def __getitem__(self, idx: int):
        p1, p2, label = self.pairs[idx]
        img1 = self.transform(Image.open(p1).convert("RGB"))
        img2 = self.transform(Image.open(p2).convert("RGB"))
        return img1, img2, label
