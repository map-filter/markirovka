import json
import sys
from pathlib import Path

import fitz
import zxingcpp
from PIL import Image

GS = "\x1d"


def decode_image(img):
    results = zxingcpp.read_barcodes(img, formats=zxingcpp.BarcodeFormat.DataMatrix, text_mode=zxingcpp.TextMode.Plain)
    out = []
    for r in results:
        raw = r.bytes.decode("latin-1")
        out.append({
            "text": raw,
            "symbology": r.symbology_identifier,
            "gs1": r.content_type == zxingcpp.ContentType.GS1 or r.symbology_identifier == "]d2",
            "gs_count": raw.count(GS),
        })
    return out


def decode_pdf(path):
    doc = fitz.open(path)
    pages = []
    for page in doc:
        pix = page.get_pixmap(dpi=300)
        img = Image.frombytes("RGB", (pix.width, pix.height), pix.samples)
        pages.append(decode_image(img))
    return pages


def main():
    target = Path(sys.argv[1])
    if target.suffix.lower() == ".pdf":
        pages = decode_pdf(target)
    else:
        pages = [decode_image(Image.open(target))]
    expected = None
    if len(sys.argv) > 2:
        expected = [line for line in Path(sys.argv[2]).read_text().split("\n") if line]
    report = {"file": str(target), "pages": len(pages), "decoded": sum(1 for p in pages if p), "results": pages}
    ok = all(p and p[0]["gs1"] and p[0]["gs_count"] >= 2 for p in pages)
    if expected is not None:
        got = [p[0]["text"] if p else None for p in pages]
        exp = [e.encode().decode("unicode_escape") for e in expected]
        report["matches_expected"] = got == exp[: len(got)]
        ok = ok and report["matches_expected"]
    report["ok"] = ok
    print(json.dumps(report, ensure_ascii=False, indent=1))
    sys.exit(0 if ok else 1)


main()
