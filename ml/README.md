# Собственная модель распознавания лиц

Эта папка содержит код для обучения собственной CNN-модели распознавания лиц
с нуля на PyTorch — для сравнения с готовой моделью `@vladmandic/face-api`
(ResNet-34), которая используется в основной системе.

## Содержание

| Файл | Назначение |
|------|------------|
| `model.py` | Архитектура CNN (MobileNet-подобная, ~350K параметров, embedding 128-D) |
| `dataset.py` | Загрузка LFW + сэмплер триплетов + пары для оценки |
| `align.py` | Выравнивание лиц через MTCNN перед обучением |
| `train.py` | Обучение с Triplet Margin Loss |
| `evaluate.py` | Метрики на стандартных парах LFW (accuracy, EER, ROC AUC) |
| `export_onnx.py` | Экспорт в ONNX для инференса из Go |

## Установка зависимостей

```bash
cd ml
python3 -m venv .venv
source .venv/bin/activate
pip install --upgrade pip
pip install -r requirements.txt
```

Под Apple Silicon (M2) PyTorch ставится с поддержкой MPS из коробки.
Проверка устройства:

```bash
python -c "import torch; print('mps:', torch.backends.mps.is_available())"
```

## Подготовка датасета LFW

Сайт UMass (`vis-www.cs.umass.edu/lfw/`) часто бывает недоступен, поэтому
используем скрипт `download_lfw.py`, который качает с альтернативных источников
(HuggingFace без регистрации или Kaggle).

### Способ 1 — HuggingFace (без регистрации, рекомендую)

```bash
python download_lfw.py --source hf
```

### Способ 2 — Kaggle (если HF не открывается)

1. Заведи бесплатный аккаунт на kaggle.com
2. Account → Settings → API → **Create New API Token** → скачается `kaggle.json`
3. Положи его в `~/.kaggle/kaggle.json` и сделай `chmod 600 ~/.kaggle/kaggle.json`
4. Запусти:

   ```bash
   python download_lfw.py --source kaggle
   ```

После скачивания должна получиться структура:

```
ml/data/lfw_raw/
    Aaron_Eckhart/
        Aaron_Eckhart_0001.jpg
    Aaron_Peirsol/
        Aaron_Peirsol_0001.jpg
        Aaron_Peirsol_0002.jpg
    ...
ml/data/pairs.txt
```

### Выравнивание лиц

MTCNN детектирует лицо, обрезает и сохраняет 112×112:

```bash
python align.py --src data/lfw_raw --dst data/lfw_aligned
```

На M2 это занимает ~10–15 минут на полный LFW.

## Обучение

```bash
python train.py --data data/lfw_aligned --epochs 30 --batch 32
```

Параметры по умолчанию подобраны под MacBook Air M2 / 8 GB:
- `--batch 32` — безопасно для 8 GB unified memory (можно поднять до 64 на 16 GB)
- `--iters-per-epoch 2000` — сколько триплетов сэмплируется за эпоху
- `--epochs 30` — обычно достаточно для сходимости на отфильтрованном LFW
- `--min-images 5` — используем только людей с ≥5 фото (иначе триплеты бесполезны)

Время одной эпохи на MPS: ориентировочно 3–6 минут, всё обучение — 2–4 часа.
Если MPS даёт ошибки на какой-то операции — добавь `PYTORCH_ENABLE_MPS_FALLBACK=1`:

```bash
PYTORCH_ENABLE_MPS_FALLBACK=1 python train.py
```

Результаты сохраняются в `checkpoints/`:
- `best.pt` — лучший по точности на триплетах
- `last.pt` — последняя эпоха
- `train_log.json` — история loss/accuracy (для графиков в записке)
- `training.png` — готовый график

## Оценка на стандартных парах LFW

```bash
python evaluate.py --ckpt checkpoints/best.pt \
                   --aligned data/lfw_aligned \
                   --pairs data/pairs.txt
```

В консоль выводится сводка, а в `checkpoints/eval.json` сохраняются все цифры,
готовые для таблицы в пояснительной записке:

- **Accuracy** при оптимальном пороге
- **EER** (equal error rate)
- **ROC AUC**
- **TPR @ FPR=0.01** (рабочая точка верификации)
- **Время инференса** одного лица (мс)
- **Размер чекпойнта** (MB)

Также сохраняется `checkpoints/eval.roc.png` с ROC-кривой.

## Экспорт в ONNX

```bash
python export_onnx.py --ckpt checkpoints/best.pt \
                      --out checkpoints/face_embedding.onnx
```

Получаем `face_embedding.onnx` (~1–2 MB), который потом грузится из Go
через `onnxruntime-go` рядом с существующим face-api.

## Сравнение с face-api.js

| Метрика | face-api.js (ResNet-34) | Своя модель | Где смотреть |
|---------|-------------------------|-------------|--------------|
| Параметры | ~22 M | ~0.35 M | `evaluate.py` → `params` |
| Размер | ~6 MB | ~1.5 MB | `evaluate.py` → `checkpoint_size_mb` |
| Accuracy LFW | ~99% (паспорт) | заполнить после обучения | `evaluate.py` → `accuracy` |
| Время инференса | ~? мс | заполнить | `evaluate.py` → `inference_ms_per_image` |

Сравнительная таблица для записки заполняется после прогона `evaluate.py`.

## Интеграция в Go-бэкенд

После экспорта в ONNX модель подключается к существующей системе:
- На фронте захватывается изображение лица (canvas → base64).
- На бэке Go-хэндлер принимает картинку, прогоняет через ONNX-модель,
  получает 128-D embedding.
- Сравнение с сохранёнными embeddings — той же функцией `euclideanDistance`,
  что и для face-api.

Этот шаг делается отдельно (новый хэндлер рядом с `handlers/auth.go`).
