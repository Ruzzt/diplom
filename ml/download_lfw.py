"""Скачивание LFW в обход неработающего сайта UMass.

Пробует несколько источников по очереди:
  1. HuggingFace Hub (без регистрации)
  2. Kaggle (нужен ~/.kaggle/kaggle.json)
  3. Прямые GitHub-мирроры (для pairs.txt)

После скачивания приводит структуру к виду, который ждут наши скрипты:

    ml/data/lfw_raw/
        Aaron_Eckhart/Aaron_Eckhart_0001.jpg
        ...
    ml/data/pairs.txt

Запуск:
    python ml/download_lfw.py
    python ml/download_lfw.py --source kaggle
"""

from __future__ import annotations

import argparse
import re
import shutil
import ssl
import sys
import urllib.request
from pathlib import Path

try:
    import certifi
    _SSL_CTX = ssl.create_default_context(cafile=certifi.where())
except ImportError:
    _SSL_CTX = ssl.create_default_context()


PAIRS_MIRRORS = [
    "https://raw.githubusercontent.com/davidsandberg/facenet/master/data/pairs.txt",
    "https://raw.githubusercontent.com/serengil/deepface/master/tests/dataset/pairs.txt",
    "http://vis-www.cs.umass.edu/lfw/pairs.txt",
]


def _http_get(url: str, dst: Path, timeout: int = 30) -> int:
    """Скачивает url в dst с правильным SSL-контекстом (certifi).

    Сначала пробует requests (если установлен — самый надёжный путь на macOS),
    потом urllib с явным SSL-контекстом из certifi.
    """
    try:
        import requests  # noqa: WPS433

        with requests.get(url, stream=True, timeout=timeout) as r:
            r.raise_for_status()
            with open(dst, "wb") as f:
                for chunk in r.iter_content(chunk_size=1 << 14):
                    if chunk:
                        f.write(chunk)
        return dst.stat().st_size
    except ImportError:
        pass

    req = urllib.request.Request(url, headers={"User-Agent": "Mozilla/5.0"})
    with urllib.request.urlopen(req, context=_SSL_CTX, timeout=timeout) as resp:
        with open(dst, "wb") as f:
            shutil.copyfileobj(resp, f)
    return dst.stat().st_size


def download_pairs(dst: Path) -> None:
    """pairs.txt маленький и есть на нескольких мирроров — берём первый рабочий."""
    if dst.exists() and dst.stat().st_size > 0:
        print(f"[pairs] уже скачан: {dst}")
        return
    dst.parent.mkdir(parents=True, exist_ok=True)
    for url in PAIRS_MIRRORS:
        try:
            print(f"[pairs] пробую {url}")
            size = _http_get(url, dst)
            if size > 1000:
                print(f"[pairs] сохранён: {dst} ({size} bytes)")
                return
        except Exception as e:
            print(f"  не получилось: {e}")
    raise SystemExit("Не удалось скачать pairs.txt ни с одного миррора.")


def download_via_huggingface(dst_root: Path) -> None:
    """Качаем LFW с HuggingFace. Не требует регистрации.

    Можно поменять repo_id, если этот окажется недоступен.
    Проверенные варианты:
      - logasja/lfw
      - nateraw/lfw
    """
    try:
        from huggingface_hub import snapshot_download
    except ImportError:
        raise SystemExit(
            "Нужна библиотека huggingface_hub. Установи:\n"
            "  pip install huggingface_hub\n"
            "и запусти заново."
        )

    candidates = [
        ("logasja/lfw", "dataset"),
        ("nateraw/lfw", "dataset"),
    ]

    cache_dir = dst_root.parent / ".hf_cache"
    for repo_id, repo_type in candidates:
        try:
            print(f"[hf] пробую {repo_id} ({repo_type})")
            local_dir = snapshot_download(
                repo_id=repo_id,
                repo_type=repo_type,
                cache_dir=str(cache_dir),
            )
            print(f"[hf] скачано в: {local_dir}")
            _organize_from_path(Path(local_dir), dst_root)
            return
        except Exception as e:
            print(f"  {repo_id} не сработал: {e}")

    raise SystemExit(
        "Ни один HF-датасет не открылся. Попробуй --source kaggle или скачай вручную."
    )


def download_via_kaggle(dst_root: Path) -> None:
    """Качаем с Kaggle. Нужен ~/.kaggle/kaggle.json (Account → Create New API Token)."""
    try:
        import kagglehub
    except ImportError:
        raise SystemExit(
            "Нужна библиотека kagglehub. Установи:\n"
            "  pip install kagglehub\n"
            "и заведи ~/.kaggle/kaggle.json (см. https://www.kaggle.com/docs/api)."
        )

    print("[kaggle] качаю jessicali9530/lfw-dataset ...")
    path = kagglehub.dataset_download("jessicali9530/lfw-dataset")
    print(f"[kaggle] скачано в: {path}")
    _organize_from_path(Path(path), dst_root)


_NAME_FROM_PATH = re.compile(r"^(.+)_\d{4}\.(?:jpe?g|png)$", re.IGNORECASE)


def _organize_from_parquet(parquet_files: list[Path], dst_root: Path) -> None:
    """HuggingFace-датасет хранит лица в parquet. Берём только те файлы,
    у которых есть колонка image{bytes, path} — это основной набор LFW.
    Файлы с другой схемой (pairs/, aug/) пропускаем."""
    try:
        import pyarrow.parquet as pq
    except ImportError:
        raise SystemExit("Нужен pyarrow: pip install pyarrow")

    dst_root.mkdir(parents=True, exist_ok=True)
    saved = 0
    persons: set[str] = set()
    processed_files = 0

    for pq_path in parquet_files:
        pf = pq.ParquetFile(pq_path)
        field_names = {f.name for f in pf.schema_arrow}
        if "image" not in field_names:
            print(f"[parquet] пропуск {pq_path.parent.name}/{pq_path.name} "
                  f"(нет колонки image, есть: {sorted(field_names)})")
            continue
        print(f"[parquet] читаю {pq_path.parent.name}/{pq_path.name}")
        processed_files += 1
        for batch in pf.iter_batches(batch_size=512, columns=["image"]):
            images = batch.column("image").to_pylist()
            for item in images:
                path = item.get("path") or ""
                data = item.get("bytes")
                if not data or not path:
                    continue
                m = _NAME_FROM_PATH.match(Path(path).name)
                if not m:
                    continue
                person = m.group(1)
                person_dir = dst_root / person
                person_dir.mkdir(exist_ok=True)
                out_file = person_dir / Path(path).name
                if not out_file.exists():
                    out_file.write_bytes(data)
                    saved += 1
                persons.add(person)

    if processed_files == 0:
        raise SystemExit(
            f"В parquet-файлах нет колонки 'image'. "
            f"Возможно, формат датасета изменился — проверь {parquet_files[0]}"
        )
    print(f"[parquet] готово: фото={saved}, людей={len(persons)} → {dst_root}")


def _organize_from_path(src: Path, dst_root: Path) -> None:
    """Сначала ищет parquet-файлы (HuggingFace формат), потом — папки
    Person/Person_0001.jpg (классический LFW)."""

    parquet_files = sorted(src.rglob("*.parquet"))
    if parquet_files:
        print(f"[organize] нашёл {len(parquet_files)} parquet-файлов")
        _organize_from_parquet(parquet_files, dst_root)
        return

    # запасной путь: папки с jpg
    candidates: list[Path] = []
    for p in src.rglob("*"):
        if not p.is_dir():
            continue
        subdirs = [d for d in p.iterdir() if d.is_dir()]
        if len(subdirs) < 5:
            continue
        has_jpg = any(
            any(c.suffix.lower() in {".jpg", ".jpeg", ".png"} for c in d.iterdir())
            for d in subdirs[:5]
        )
        if has_jpg:
            candidates.append(p)

    if not candidates:
        raise SystemExit(
            f"В {src} не нашёл ни parquet, ни папки Person/Person_0001.jpg. "
            "Проверь скачанные файлы вручную."
        )

    best = max(candidates, key=lambda p: sum(1 for _ in p.iterdir()))
    persons = [d for d in best.iterdir() if d.is_dir()]
    print(f"[organize] корень: {best}")
    print(f"[organize] людей: {len(persons)}, копирую в {dst_root}")

    dst_root.mkdir(parents=True, exist_ok=True)
    for person_dir in persons:
        target = dst_root / person_dir.name
        if target.exists():
            continue
        shutil.copytree(person_dir, target)

    total = sum(1 for _ in dst_root.rglob("*.jpg"))
    print(f"[organize] готово, фото: {total}, людей: {len(persons)}")


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--source", choices=["hf", "kaggle"], default="hf",
                        help="откуда качать датасет (pairs.txt всегда с GitHub)")
    parser.add_argument("--dst", type=Path, default=Path("data/lfw_raw"))
    parser.add_argument("--pairs", type=Path, default=Path("data/pairs.txt"))
    args = parser.parse_args()

    print(f"[download] источник: {args.source}")
    print(f"[download] куда: {args.dst}")

    download_pairs(args.pairs)

    if args.source == "hf":
        download_via_huggingface(args.dst)
    else:
        download_via_kaggle(args.dst)

    print("\n[OK] датасет готов. Теперь:")
    print("  python align.py --src data/lfw_raw --dst data/lfw_aligned")


if __name__ == "__main__":
    try:
        main()
    except KeyboardInterrupt:
        sys.exit(1)
