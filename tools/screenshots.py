import json
import sys
from pathlib import Path

from playwright.sync_api import sync_playwright

BASE = "http://127.0.0.1:8301"
ROOT = Path(__file__).resolve().parent.parent
SHOTS = ROOT / "docs" / "screenshots"
OUT = ROOT / "tools" / "out" / "layout"

PAGES = [
    ("dashboard", "/"),
    ("products", "/products"),
    ("orders", "/orders"),
    ("order", "/orders/1"),
    ("codes", "/codes"),
    ("labels", "/labels"),
    ("print", "/print"),
    ("circulation", "/circulation"),
    ("journal", "/journal"),
    ("onec", "/onec"),
    ("settings", "/settings"),
]

SCREENSHOTS = [
    ("02-dashboard", "/", 1440),
    ("03-products", "/products", 1440),
    ("04-orders", "/orders", 1440),
    ("05-codes", "/codes", 1440),
    ("06-labels", "/labels", 1440),
    ("07-print", "/print?order_id=2", 1440),
    ("08-circulation", "/circulation?order_id=1", 1440),
    ("09-journal", "/journal", 1440),
    ("10-onec", "/onec", 1440),
    ("11-mobile-dashboard", "/", 390),
]

OVERFLOW_JS = """
() => {
  const doc = document.documentElement;
  const vw = doc.clientWidth;
  const bad = [];
  for (const el of document.querySelectorAll('body *')) {
    const r = el.getBoundingClientRect();
    if (!r.width || !r.height) continue;
    const st = getComputedStyle(el);
    if (st.visibility === 'hidden' || st.display === 'none') continue;
    let p = el.parentElement, clipped = false;
    while (p && p !== document.body) {
      const ps = getComputedStyle(p);
      if (['auto', 'scroll', 'hidden', 'clip'].includes(ps.overflowX)) { clipped = true; break; }
      p = p.parentElement;
    }
    if (!clipped && r.right > vw + 1) bad.push((el.className || el.tagName) + ' ' + Math.round(r.right));
  }
  const texts = [...document.querySelectorAll('button, a, h1, h2, h3, label, .badge, input, select')]
    .filter(e => { const r = e.getBoundingClientRect(); return r.width && r.height && getComputedStyle(e).visibility !== 'hidden'; });
  const overlaps = [];
  for (let i = 0; i < texts.length; i++) {
    for (let j = i + 1; j < texts.length; j++) {
      const a = texts[i], b = texts[j];
      if (a.contains(b) || b.contains(a)) continue;
      const r1 = a.getBoundingClientRect(), r2 = b.getBoundingClientRect();
      const w = Math.min(r1.right, r2.right) - Math.max(r1.left, r2.left);
      const h = Math.min(r1.bottom, r2.bottom) - Math.max(r1.top, r2.top);
      if (w > 4 && h > 4) overlaps.push(`${a.tagName}:${(a.textContent||'').trim().slice(0,25)} / ${b.tagName}:${(b.textContent||'').trim().slice(0,25)}`);
    }
  }
  return { scroll: doc.scrollWidth > vw + 1, scrollWidth: doc.scrollWidth, vw, bad: bad.slice(0, 8), overlaps: overlaps.slice(0, 8) };
}
"""


def login(page):
    page.goto(BASE + "/login")
    page.fill("input[type=email]", "admin@kendir.test")
    page.fill("input[type=password]", "portal2026")
    page.click("button[type=submit]")
    page.wait_for_url(lambda url: "/login" not in url, timeout=15000)
    page.wait_for_load_state("networkidle")


def main():
    mode = sys.argv[1] if len(sys.argv) > 1 else "all"
    SHOTS.mkdir(parents=True, exist_ok=True)
    OUT.mkdir(parents=True, exist_ok=True)
    report = {}
    with sync_playwright() as p:
        browser = p.chromium.launch()
        if mode in ("all", "shots"):
            ctx = browser.new_context(viewport={"width": 1440, "height": 900}, locale="ru-RU", timezone_id="Asia/Almaty")
            page = ctx.new_page()
            page.goto(BASE + "/login")
            page.wait_for_load_state("networkidle")
            page.screenshot(path=str(SHOTS / "01-login.png"))
            login(page)
            for name, path, width in SCREENSHOTS:
                page.set_viewport_size({"width": width, "height": 900 if width > 400 else 844})
                page.goto(BASE + path)
                page.wait_for_load_state("networkidle")
                page.wait_for_timeout(900)
                page.screenshot(path=str(SHOTS / f"{name}.png"), full_page=width < 400)
            page.set_viewport_size({"width": 1440, "height": 900})
            page.goto("http://127.0.0.1:8303/emu/")
            page.wait_for_load_state("networkidle")
            page.screenshot(path=str(SHOTS / "12-emulator.png"))
            ctx.close()
        if mode in ("all", "layout"):
            for width in (1440, 1024, 768, 390):
                ctx = browser.new_context(viewport={"width": width, "height": 900}, locale="ru-RU", timezone_id="Asia/Almaty")
                page = ctx.new_page()
                login(page)
                for name, path in PAGES:
                    page.goto(BASE + path)
                    page.wait_for_load_state("networkidle")
                    page.wait_for_timeout(500)
                    result = page.evaluate(OVERFLOW_JS)
                    page.screenshot(path=str(OUT / f"{name}-{width}.png"), full_page=True)
                    if result["scroll"] or result["bad"] or result["overlaps"]:
                        report[f"{name}@{width}"] = result
                page.goto(BASE + "/login")
                ctx.close()
            page = browser.new_page(viewport={"width": 390, "height": 844})
            page.goto("http://127.0.0.1:8303/emu/")
            page.wait_for_load_state("networkidle")
            result = page.evaluate(OVERFLOW_JS)
            if result["scroll"]:
                report["emulator@390"] = result
        browser.close()
    print(json.dumps(report, ensure_ascii=False, indent=1) if report else "layout OK")


main()
