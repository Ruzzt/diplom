"""
Собственная сиамская нейросеть для распознавания лиц.
Архитектура: Lightweight CNN (5 свёрточных блоков) + Triplet Loss
Датасет: Labeled Faces in the Wild (LFW)
Выход: 128-мерный эмбеддинг лица
"""

import os
import time
import random
import numpy as np
import torch
import torch.nn as nn
import torch.optim as optim
from torch.utils.data import Dataset, DataLoader
from torchvision import transforms
from sklearn.datasets import fetch_lfw_people
from sklearn.model_selection import train_test_split
from PIL import Image


# ======================== Архитектура модели ========================

class SiameseFaceNet(nn.Module):
    """
    Собственная CNN для извлечения дескрипторов лица.

    Архитектура:
        5 свёрточных блоков (Conv2d -> BatchNorm -> ReLU -> MaxPool)
        2 полносвязных слоя -> 128-мерный L2-нормализованный эмбеддинг

    Вход:  серое изображение 1x105x105
    Выход: вектор размерности 128
    """

    def __init__(self, embedding_dim=128):
        super(SiameseFaceNet, self).__init__()

        self.features = nn.Sequential(
            # Блок 1: 1x105x105 -> 32x52x52
            nn.Conv2d(1, 32, kernel_size=3, padding=1),
            nn.BatchNorm2d(32),
            nn.ReLU(inplace=True),
            nn.MaxPool2d(2, 2),

            # Блок 2: 32x52x52 -> 64x26x26
            nn.Conv2d(32, 64, kernel_size=3, padding=1),
            nn.BatchNorm2d(64),
            nn.ReLU(inplace=True),
            nn.MaxPool2d(2, 2),

            # Блок 3: 64x26x26 -> 128x13x13
            nn.Conv2d(64, 128, kernel_size=3, padding=1),
            nn.BatchNorm2d(128),
            nn.ReLU(inplace=True),
            nn.MaxPool2d(2, 2),

            # Блок 4: 128x13x13 -> 256x6x6
            nn.Conv2d(128, 256, kernel_size=3, padding=1),
            nn.BatchNorm2d(256),
            nn.ReLU(inplace=True),
            nn.MaxPool2d(2, 2),

            # Блок 5: 256x6x6 -> 256x3x3
            nn.Conv2d(256, 256, kernel_size=3, padding=1),
            nn.BatchNorm2d(256),
            nn.ReLU(inplace=True),
            nn.MaxPool2d(2, 2),
        )

        self.fc = nn.Sequential(
            nn.Linear(256 * 3 * 3, 512),
            nn.ReLU(inplace=True),
            nn.Dropout(0.5),
            nn.Linear(512, embedding_dim),
        )

    def forward(self, x):
        x = self.features(x)
        x = x.view(x.size(0), -1)
        x = self.fc(x)
        x = nn.functional.normalize(x, p=2, dim=1)
        return x


# ======================== Triplet Loss ========================

class TripletLoss(nn.Module):
    """
    Triplet Loss: L = max(d(a,p) - d(a,n) + margin, 0)

    Минимизирует расстояние между anchor и positive,
    максимизирует расстояние между anchor и negative.
    """

    def __init__(self, margin=0.3):
        super(TripletLoss, self).__init__()
        self.margin = margin

    def forward(self, anchor, positive, negative):
        pos_dist = torch.sum((anchor - positive) ** 2, dim=1)
        neg_dist = torch.sum((anchor - negative) ** 2, dim=1)
        loss = torch.clamp(pos_dist - neg_dist + self.margin, min=0.0)
        return loss.mean()


# ======================== Датасет ========================

class LFWTripletDataset(Dataset):
    """Генератор триплетов (anchor, positive, negative) из LFW."""

    def __init__(self, images, labels, transform=None, triplets_per_class=10):
        self.images = images
        self.labels = labels
        self.transform = transform
        self.triplets_per_class = triplets_per_class

        # Группируем индексы по людям
        self.label_to_indices = {}
        for idx, label in enumerate(labels):
            self.label_to_indices.setdefault(label, []).append(idx)

        # Только люди с >= 2 фото (для позитивных пар)
        self.valid_labels = [
            l for l, idx_list in self.label_to_indices.items()
            if len(idx_list) >= 2
        ]
        self.all_labels = list(self.label_to_indices.keys())

    def __len__(self):
        return len(self.valid_labels) * self.triplets_per_class

    def __getitem__(self, _):
        label = random.choice(self.valid_labels)
        indices = self.label_to_indices[label]

        anchor_idx, positive_idx = random.sample(indices, 2)

        neg_label = random.choice([l for l in self.all_labels if l != label])
        negative_idx = random.choice(self.label_to_indices[neg_label])

        anchor = self._load(anchor_idx)
        positive = self._load(positive_idx)
        negative = self._load(negative_idx)

        return anchor, positive, negative

    def _load(self, idx):
        arr = self.images[idx]
        # LFW возвращает float64 (0-255) -> uint8 для PIL
        img = Image.fromarray(arr.astype(np.uint8), mode='L')
        if self.transform:
            img = self.transform(img)
        return img


# ======================== Обучение ========================

def train_model():
    print("=" * 60)
    print("  Обучение собственной модели распознавания лиц")
    print("  Архитектура: SiameseFaceNet (CNN + Triplet Loss)")
    print("=" * 60)

    device = torch.device(
        'cuda' if torch.cuda.is_available() else
        'mps' if torch.backends.mps.is_available() else 'cpu'
    )
    print(f"\nУстройство: {device}")

    # ---------- Загрузка LFW ----------
    print("\nЗагрузка датасета LFW...")
    lfw = fetch_lfw_people(min_faces_per_person=2, resize=0.5)
    images = lfw.images          # (N, 62, 47), float64, grayscale
    labels = lfw.target
    names = lfw.target_names

    print(f"  Изображений: {len(images)}")
    print(f"  Людей:       {len(names)}")
    print(f"  Размер фото: {images[0].shape}")

    # ---------- Трансформации ----------
    transform_train = transforms.Compose([
        transforms.Resize((105, 105)),
        transforms.RandomHorizontalFlip(),
        transforms.RandomRotation(10),
        transforms.RandomAdjustSharpness(2, p=0.3),
        transforms.ToTensor(),
        transforms.Normalize(mean=[0.5], std=[0.5]),
    ])

    transform_val = transforms.Compose([
        transforms.Resize((105, 105)),
        transforms.ToTensor(),
        transforms.Normalize(mean=[0.5], std=[0.5]),
    ])

    # ---------- Train / Val split ----------
    train_idx, val_idx = train_test_split(
        range(len(images)), test_size=0.2, stratify=labels, random_state=42
    )

    train_ds = LFWTripletDataset(images[train_idx], labels[train_idx], transform_train)
    val_ds = LFWTripletDataset(images[val_idx], labels[val_idx], transform_val)

    train_loader = DataLoader(train_ds, batch_size=32, shuffle=True, num_workers=0)
    val_loader = DataLoader(val_ds, batch_size=32, shuffle=False, num_workers=0)

    # ---------- Модель, оптимизатор ----------
    model = SiameseFaceNet(embedding_dim=128).to(device)
    criterion = TripletLoss(margin=0.3)
    optimizer = optim.Adam(model.parameters(), lr=1e-3, weight_decay=1e-5)
    scheduler = optim.lr_scheduler.StepLR(optimizer, step_size=10, gamma=0.5)

    total_params = sum(p.numel() for p in model.parameters())
    print(f"\n  Параметров модели: {total_params:,}")

    # ---------- Цикл обучения ----------
    num_epochs = 30
    best_val_loss = float('inf')
    train_losses, val_losses = [], []

    print(f"\nОбучение ({num_epochs} эпох)...\n" + "-" * 55)

    t_start = time.time()

    for epoch in range(num_epochs):
        # --- Train ---
        model.train()
        epoch_loss, n_batches = 0.0, 0

        for anchor, positive, negative in train_loader:
            anchor = anchor.to(device)
            positive = positive.to(device)
            negative = negative.to(device)

            optimizer.zero_grad()
            loss = criterion(model(anchor), model(positive), model(negative))
            loss.backward()
            optimizer.step()

            epoch_loss += loss.item()
            n_batches += 1

        avg_train = epoch_loss / max(n_batches, 1)
        train_losses.append(avg_train)

        # --- Validation ---
        model.eval()
        v_loss, v_batches = 0.0, 0

        with torch.no_grad():
            for anchor, positive, negative in val_loader:
                anchor = anchor.to(device)
                positive = positive.to(device)
                negative = negative.to(device)

                loss = criterion(model(anchor), model(positive), model(negative))
                v_loss += loss.item()
                v_batches += 1

        avg_val = v_loss / max(v_batches, 1)
        val_losses.append(avg_val)

        scheduler.step()

        if avg_val < best_val_loss:
            best_val_loss = avg_val
            torch.save(model.state_dict(), 'custom_face_model.pth')

        if (epoch + 1) % 5 == 0 or epoch == 0:
            lr = scheduler.get_last_lr()[0]
            print(f"  Эпоха {epoch+1:3d}/{num_epochs}  |  "
                  f"Train: {avg_train:.4f}  |  Val: {avg_val:.4f}  |  LR: {lr:.6f}")

    elapsed = time.time() - t_start
    print("-" * 55)
    print(f"\nОбучение завершено за {elapsed:.1f} сек")
    print(f"Лучший Val Loss: {best_val_loss:.4f}")
    print(f"Модель сохранена: custom_face_model.pth")

    # Сохраняем историю
    np.savez(
        'training_history.npz',
        train_losses=train_losses,
        val_losses=val_losses,
        training_time=elapsed,
        total_params=total_params,
    )

    return model


if __name__ == '__main__':
    train_model()
