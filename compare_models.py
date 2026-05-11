"""
Сравнительный анализ двух моделей распознавания лиц:
  1) SiameseFaceNet — собственная модель (CNN + Triplet Loss)
  2) FaceNet (InceptionResnetV1) — аналог face-api.js

Датасет оценки: LFW Pairs (стандартный бенчмарк)
Метрики: Accuracy, AUC, EER, FAR, FRR, время инференса, размер модели
Результат: графики в папке comparison_results/
"""

import os
import time
import numpy as np
import torch
import torch.nn as nn
from torchvision import transforms
from sklearn.datasets import fetch_lfw_pairs
from sklearn.metrics import roc_curve, auc
from PIL import Image

import matplotlib
matplotlib.use('Agg')
import matplotlib.pyplot as plt
matplotlib.rcParams['font.family'] = 'DejaVu Sans'

from train_custom_model import SiameseFaceNet

OUTPUT_DIR = 'comparison_results'


# ======================== Загрузка моделей ========================

def load_custom_model(path='custom_face_model.pth', device='cpu'):
    model = SiameseFaceNet(embedding_dim=128)
    model.load_state_dict(torch.load(path, map_location=device, weights_only=True))
    model.to(device).eval()
    return model


def load_facenet(device='cpu'):
    from facenet_pytorch import InceptionResnetV1
    model = InceptionResnetV1(pretrained='vggface2').to(device).eval()
    return model


# ======================== Извлечение эмбеддингов ========================

def embeddings_custom(model, images, device, batch_size=64):
    tfm = transforms.Compose([
        transforms.Resize((105, 105)),
        transforms.ToTensor(),
        transforms.Normalize([0.5], [0.5]),
    ])
    return _extract(model, images, device, tfm, mode='L', batch_size=batch_size)


def embeddings_facenet(model, images, device, batch_size=64):
    tfm = transforms.Compose([
        transforms.Resize((160, 160)),
        transforms.ToTensor(),
        transforms.Normalize([0.5]*3, [0.5]*3),
    ])
    return _extract(model, images, device, tfm, mode='RGB', batch_size=batch_size)


def _extract(model, images, device, tfm, mode, batch_size):
    all_emb = []
    t0 = time.time()
    for i in range(0, len(images), batch_size):
        batch = images[i:i+batch_size]
        tensors = [tfm(Image.fromarray(img.astype(np.uint8)).convert(mode)) for img in batch]
        inp = torch.stack(tensors).to(device)
        with torch.no_grad():
            emb = model(inp)
        all_emb.append(emb.cpu().numpy())
    elapsed = time.time() - t0
    return np.vstack(all_emb), elapsed


# ======================== Метрики ========================

def compute_metrics(distances, labels, n_thresh=2000):
    """FAR, FRR, Accuracy, ROC, AUC, EER по массиву расстояний и меток."""
    thresholds = np.linspace(0, float(distances.max()) * 1.2, n_thresh)

    fars, frrs, accs = [], [], []
    for th in thresholds:
        pred = (distances < th).astype(int)
        tp = int(np.sum((pred == 1) & (labels == 1)))
        fp = int(np.sum((pred == 1) & (labels == 0)))
        tn = int(np.sum((pred == 0) & (labels == 0)))
        fn = int(np.sum((pred == 0) & (labels == 1)))

        fars.append(fp / max(fp + tn, 1))
        frrs.append(fn / max(fn + tp, 1))
        accs.append((tp + tn) / len(labels))

    fars = np.array(fars)
    frrs = np.array(frrs)
    accs = np.array(accs)

    best_i = int(np.argmax(accs))
    eer_i = int(np.argmin(np.abs(fars - frrs)))

    fpr, tpr, _ = roc_curve(labels, -distances)
    roc_auc = auc(fpr, tpr)

    return {
        'accuracy':       accs[best_i],
        'threshold':      thresholds[best_i],
        'far':            fars[best_i],
        'frr':            frrs[best_i],
        'eer':            (fars[eer_i] + frrs[eer_i]) / 2,
        'eer_threshold':  thresholds[eer_i],
        'auc':            roc_auc,
        'fpr':            fpr,
        'tpr':            tpr,
        'thresholds':     thresholds,
        'fars':           fars,
        'frrs':           frrs,
        'accs':           accs,
    }


# ======================== Графики ========================

def plot_all(cm, fm, cd, fd, labels, train_losses, val_losses):
    os.makedirs(OUTPUT_DIR, exist_ok=True)

    # --- 1. ROC-кривые ---
    fig, ax = plt.subplots(figsize=(9, 7))
    ax.plot(cm['fpr'], cm['tpr'],
            label=f'SiameseFaceNet (AUC={cm["auc"]:.4f})', lw=2, color='#2196F3')
    ax.plot(fm['fpr'], fm['tpr'],
            label=f'FaceNet (AUC={fm["auc"]:.4f})', lw=2, color='#4CAF50')
    ax.plot([0, 1], [0, 1], 'k--', alpha=.3)
    ax.set_xlabel('False Positive Rate (FAR)')
    ax.set_ylabel('True Positive Rate (1 - FRR)')
    ax.set_title('ROC-curves: model comparison')
    ax.legend(loc='lower right', fontsize=11)
    ax.grid(True, alpha=.3)
    _save(fig, 'roc_curves.png')

    # --- 2. FAR / FRR ---
    fig, (a1, a2) = plt.subplots(1, 2, figsize=(16, 6))
    for ax, m, title in [(a1, cm, 'SiameseFaceNet'), (a2, fm, 'FaceNet')]:
        ax.plot(m['thresholds'], m['fars'], label='FAR', color='red', lw=2)
        ax.plot(m['thresholds'], m['frrs'], label='FRR', color='blue', lw=2)
        ax.axvline(m['eer_threshold'], color='gray', ls='--', alpha=.5,
                   label=f'EER={m["eer"]:.4f}')
        ax.set_xlabel('Threshold')
        ax.set_ylabel('Error Rate')
        ax.set_title(f'{title}: FAR / FRR')
        ax.legend()
        ax.grid(True, alpha=.3)
    _save(fig, 'far_frr.png')

    # --- 3. Распределения расстояний ---
    same = labels == 1
    diff = labels == 0
    fig, (a1, a2) = plt.subplots(1, 2, figsize=(16, 6))
    for ax, d, m, title in [(a1, cd, cm, 'SiameseFaceNet'), (a2, fd, fm, 'FaceNet')]:
        ax.hist(d[same], bins=50, alpha=.7, label='Same person', color='#2196F3')
        ax.hist(d[diff], bins=50, alpha=.7, label='Different persons', color='#F44336')
        ax.axvline(m['threshold'], color='k', ls='--',
                   label=f'Threshold={m["threshold"]:.3f}')
        ax.set_xlabel('Euclidean distance')
        ax.set_ylabel('Number of pairs')
        ax.set_title(f'{title}: distance distributions')
        ax.legend()
        ax.grid(True, alpha=.3)
    _save(fig, 'distance_distributions.png')

    # --- 4. Кривые обучения ---
    if train_losses is not None:
        fig, ax = plt.subplots(figsize=(10, 6))
        ep = range(1, len(train_losses) + 1)
        ax.plot(ep, train_losses, label='Train Loss', lw=2, color='#2196F3')
        ax.plot(ep, val_losses, label='Val Loss', lw=2, color='#F44336')
        ax.set_xlabel('Epoch')
        ax.set_ylabel('Triplet Loss')
        ax.set_title('SiameseFaceNet: training curves')
        ax.legend()
        ax.grid(True, alpha=.3)
        _save(fig, 'training_curves.png')

    # --- 5. Accuracy vs Threshold ---
    fig, ax = plt.subplots(figsize=(10, 6))
    ax.plot(cm['thresholds'], cm['accs'],
            label=f'SiameseFaceNet (max={cm["accuracy"]:.4f})', lw=2, color='#2196F3')
    ax.plot(fm['thresholds'], fm['accs'],
            label=f'FaceNet (max={fm["accuracy"]:.4f})', lw=2, color='#4CAF50')
    ax.set_xlabel('Threshold')
    ax.set_ylabel('Accuracy')
    ax.set_title('Accuracy vs threshold')
    ax.legend(fontsize=11)
    ax.grid(True, alpha=.3)
    _save(fig, 'accuracy_vs_threshold.png')

    # --- 6. Сводная столбчатая диаграмма ---
    fig, ax = plt.subplots(figsize=(10, 6))
    metrics_names = ['Accuracy', 'AUC', '1 - EER']
    custom_vals = [cm['accuracy'], cm['auc'], 1 - cm['eer']]
    facenet_vals = [fm['accuracy'], fm['auc'], 1 - fm['eer']]
    x = np.arange(len(metrics_names))
    w = 0.35
    ax.bar(x - w/2, custom_vals, w, label='SiameseFaceNet', color='#2196F3')
    ax.bar(x + w/2, facenet_vals, w, label='FaceNet', color='#4CAF50')
    ax.set_ylabel('Value')
    ax.set_title('Key metrics comparison')
    ax.set_xticks(x)
    ax.set_xticklabels(metrics_names)
    ax.set_ylim(0, 1.1)
    ax.legend()
    ax.grid(True, alpha=.3, axis='y')
    for i, (cv, fv) in enumerate(zip(custom_vals, facenet_vals)):
        ax.text(i - w/2, cv + 0.02, f'{cv:.3f}', ha='center', fontsize=10)
        ax.text(i + w/2, fv + 0.02, f'{fv:.3f}', ha='center', fontsize=10)
    _save(fig, 'metrics_comparison.png')


def _save(fig, name):
    path = os.path.join(OUTPUT_DIR, name)
    fig.tight_layout()
    fig.savefig(path, dpi=150)
    plt.close(fig)
    print(f'  -> {path}')


# ======================== Таблица ========================

def print_table(cm, fm, ct, ft, cp, fp_, cs, fs):
    """Печатает итоговую таблицу сравнения."""
    hr = '=' * 72
    print(f'\n{hr}')
    print('  COMPARATIVE TABLE: FACE RECOGNITION MODELS')
    print(hr)
    h = f'{"Metric":<32} {"SiameseFaceNet":>18} {"FaceNet":>18}'
    print(h)
    print('-' * 72)
    rows = [
        ('Accuracy',              f'{cm["accuracy"]:.4f}',   f'{fm["accuracy"]:.4f}'),
        ('AUC (ROC)',             f'{cm["auc"]:.4f}',        f'{fm["auc"]:.4f}'),
        ('EER',                   f'{cm["eer"]:.4f}',        f'{fm["eer"]:.4f}'),
        ('FAR (best threshold)',  f'{cm["far"]:.4f}',        f'{fm["far"]:.4f}'),
        ('FRR (best threshold)',  f'{cm["frr"]:.4f}',        f'{fm["frr"]:.4f}'),
        ('Optimal threshold',     f'{cm["threshold"]:.4f}',  f'{fm["threshold"]:.4f}'),
        ('Inference time (sec)',  f'{ct:.3f}',               f'{ft:.3f}'),
        ('Parameters',            f'{cp:,}',                 f'{fp_:,}'),
        ('Model size (MB)',       f'{cs:.1f}',               f'{fs:.1f}'),
        ('Embedding dim',         '128',                     '512'),
        ('Backbone',              'CNN (5 blocks)',           'InceptionResNet'),
        ('Loss function',         'Triplet Loss',            'Triplet Loss'),
        ('Training data',         'LFW (~1.7K)',             'VGGFace2 (3.3M)'),
    ]
    for name, v1, v2 in rows:
        print(f'  {name:<30} {v1:>18} {v2:>18}')
    print(hr)


# ======================== main ========================

def main():
    print('=' * 60)
    print('  Comparative analysis: face recognition models')
    print('=' * 60)

    device = torch.device(
        'cuda' if torch.cuda.is_available() else
        'mps' if torch.backends.mps.is_available() else 'cpu'
    )
    print(f'Device: {device}\n')

    # ---------- LFW Pairs ----------
    print('Loading LFW Pairs (test set)...')
    lfw = fetch_lfw_pairs(subset='test', resize=0.5)
    pairs = lfw.pairs        # (N, 2, H, W)
    labels = lfw.target      # 1 = same person, 0 = different
    img1 = pairs[:, 0]
    img2 = pairs[:, 1]
    print(f'  Pairs: {len(labels)}  (same: {labels.sum():.0f}, '
          f'different: {(1-labels).sum():.0f})\n')

    # ---------- Custom model ----------
    model_path = 'custom_face_model.pth'
    if not os.path.exists(model_path):
        print(f'ERROR: {model_path} not found.')
        print('Train the model first:  python3 train_custom_model.py')
        return

    print('--- SiameseFaceNet ---')
    custom = load_custom_model(model_path, device)
    c_params = sum(p.numel() for p in custom.parameters())
    c_size = os.path.getsize(model_path) / 1024**2
    print(f'  Parameters: {c_params:,}   Size: {c_size:.1f} MB')

    ce1, t1 = embeddings_custom(custom, img1, device)
    ce2, t2 = embeddings_custom(custom, img2, device)
    c_time = t1 + t2
    c_dist = np.linalg.norm(ce1 - ce2, axis=1)
    print(f'  Inference: {c_time:.3f} sec\n')

    # ---------- FaceNet ----------
    print('--- FaceNet (InceptionResnetV1, VGGFace2) ---')
    facenet = load_facenet(device)
    f_params = sum(p.numel() for p in facenet.parameters())
    tmp = '/tmp/_facenet_size.pth'
    torch.save(facenet.state_dict(), tmp)
    f_size = os.path.getsize(tmp) / 1024**2
    os.remove(tmp)
    print(f'  Parameters: {f_params:,}   Size: {f_size:.1f} MB')

    fe1, t1 = embeddings_facenet(facenet, img1, device)
    fe2, t2 = embeddings_facenet(facenet, img2, device)
    f_time = t1 + t2
    f_dist = np.linalg.norm(fe1 - fe2, axis=1)
    print(f'  Inference: {f_time:.3f} sec\n')

    # ---------- Metrics ----------
    print('Computing metrics...')
    cm = compute_metrics(c_dist, labels)
    fm = compute_metrics(f_dist, labels)

    print_table(cm, fm, c_time, f_time, c_params, f_params, c_size, f_size)

    # ---------- Plots ----------
    print('\nGenerating plots...')
    tl, vl = None, None
    if os.path.exists('training_history.npz'):
        h = np.load('training_history.npz')
        tl, vl = h['train_losses'], h['val_losses']

    plot_all(cm, fm, c_dist, f_dist, labels, tl, vl)

    print(f'\nDone! Plots saved to {OUTPUT_DIR}/')


if __name__ == '__main__':
    main()
