"""Архитектура собственной CNN для распознавания лиц.

Модель учится отображать изображение лица 112x112x3 в 128-мерный
L2-нормализованный embedding. Сравнение лиц — евклидово расстояние
(или 1 - cos_sim, что эквивалентно для нормализованных векторов).

Размер модели: ~350K параметров (vs ~22M у ResNet-34 в face-api).
"""

from __future__ import annotations

import torch
import torch.nn as nn
import torch.nn.functional as F


class ConvBlock(nn.Module):
    def __init__(self, in_ch: int, out_ch: int, kernel: int = 3, stride: int = 1):
        super().__init__()
        self.conv = nn.Conv2d(in_ch, out_ch, kernel, stride=stride,
                              padding=kernel // 2, bias=False)
        self.bn = nn.BatchNorm2d(out_ch)
        self.act = nn.ReLU(inplace=True)

    def forward(self, x: torch.Tensor) -> torch.Tensor:
        return self.act(self.bn(self.conv(x)))


class DepthwiseSeparable(nn.Module):
    """Depthwise separable conv — основа MobileNet, даёт компактность."""

    def __init__(self, in_ch: int, out_ch: int, stride: int = 1):
        super().__init__()
        self.dw = nn.Conv2d(in_ch, in_ch, 3, stride=stride, padding=1,
                            groups=in_ch, bias=False)
        self.bn1 = nn.BatchNorm2d(in_ch)
        self.pw = nn.Conv2d(in_ch, out_ch, 1, bias=False)
        self.bn2 = nn.BatchNorm2d(out_ch)
        self.act = nn.ReLU(inplace=True)

    def forward(self, x: torch.Tensor) -> torch.Tensor:
        x = self.act(self.bn1(self.dw(x)))
        x = self.act(self.bn2(self.pw(x)))
        return x


class FaceEmbeddingNet(nn.Module):
    """Лёгкая CNN, выдающая 128-D L2-нормализованный embedding.

    Вход:  (B, 3, 112, 112) — RGB, значения в [-1, 1].
    Выход: (B, 128)         — L2-норма = 1.

    Для предотвращения embedding-коллапса при обучении с batch-hard triplet
    loss добавлен опциональный classification head. Его веса используются
    только во время тренировки и в production не сохраняются.
    """

    EMBEDDING_DIM = 128
    INPUT_SIZE = 112

    def __init__(self, embedding_dim: int = EMBEDDING_DIM,
                 num_classes: int | None = None):
        super().__init__()
        self.stem = ConvBlock(3, 32, kernel=3, stride=2)   # 112 -> 56
        self.block1 = DepthwiseSeparable(32, 64, stride=1)
        self.block2 = DepthwiseSeparable(64, 64, stride=2)  # 56 -> 28
        self.block3 = DepthwiseSeparable(64, 128, stride=1)
        self.block4 = DepthwiseSeparable(128, 128, stride=2)  # 28 -> 14
        self.block5 = DepthwiseSeparable(128, 128, stride=1)
        self.block6 = DepthwiseSeparable(128, 256, stride=2)  # 14 -> 7
        self.block7 = DepthwiseSeparable(256, 256, stride=1)

        self.pool = nn.AdaptiveAvgPool2d(1)
        self.fc = nn.Linear(256, embedding_dim)
        self.bn_emb = nn.BatchNorm1d(embedding_dim)

        self.classifier = nn.Linear(embedding_dim, num_classes) if num_classes else None

    def features(self, x: torch.Tensor) -> torch.Tensor:
        x = self.stem(x)
        x = self.block1(x)
        x = self.block2(x)
        x = self.block3(x)
        x = self.block4(x)
        x = self.block5(x)
        x = self.block6(x)
        x = self.block7(x)
        x = self.pool(x).flatten(1)
        x = self.fc(x)
        x = self.bn_emb(x)
        return x  # без нормализации, "сырой" embedding

    def forward(self, x: torch.Tensor) -> torch.Tensor:
        x = self.features(x)
        x = F.normalize(x, p=2, dim=1)
        return x

    def forward_train(self, x: torch.Tensor) -> tuple[torch.Tensor, torch.Tensor | None]:
        """При обучении возвращает (нормализованный embedding, logits для CE).
        Logits считаются от ненормализованных features — это стандарт."""
        feats = self.features(x)
        embedding = F.normalize(feats, p=2, dim=1)
        logits = self.classifier(feats) if self.classifier is not None else None
        return embedding, logits


def count_parameters(model: nn.Module) -> int:
    return sum(p.numel() for p in model.parameters() if p.requires_grad)


if __name__ == "__main__":
    net = FaceEmbeddingNet()
    print(f"Параметров: {count_parameters(net):,}")
    dummy = torch.randn(2, 3, 112, 112)
    out = net(dummy)
    print(f"Вход:  {tuple(dummy.shape)}")
    print(f"Выход: {tuple(out.shape)}  (норма={out.norm(dim=1).tolist()})")
