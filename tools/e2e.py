import base64
import json
import os
import random
import sys
import time
import urllib.error
import urllib.request
from pathlib import Path

sys.path.insert(0, "/tools")
from verify_datamatrix import decode_pdf  # noqa: E402

BASE = os.environ.get("BASE_URL", "http://127.0.0.1:8301")
OUT = Path(os.environ.get("OUT_DIR", "/out"))
GS = "\x1d"
TOKEN = None


def call(method, path, body=None, token=None, raw=False, expect=(200, 201, 204)):
    headers = {"Accept": "application/json"}
    data = None
    if body is not None:
        data = json.dumps(body).encode()
        headers["Content-Type"] = "application/json"
    auth = token or TOKEN
    if auth:
        headers["Authorization"] = f"Bearer {auth}"
    req = urllib.request.Request(BASE + path, data=data, headers=headers, method=method)
    try:
        with urllib.request.urlopen(req, timeout=60) as resp:
            content = resp.read()
            status = resp.status
    except urllib.error.HTTPError as e:
        content = e.read()
        status = e.code
    if status not in expect:
        raise SystemExit(f"{method} {path}: HTTP {status}: {content[:400]!r}")
    if raw:
        return content
    return json.loads(content) if content else None


def step(text):
    print(f"[ok] {text}", flush=True)


def gtin14(body12):
    digits = "0" + body12
    total = sum(int(c) * (3 if i % 2 == 0 else 1) for i, c in enumerate(reversed(digits)))
    return digits + str((10 - total % 10) % 10)


def wait(fn, label, timeout=60):
    end = time.time() + timeout
    while time.time() < end:
        result = fn()
        if result:
            return result
        time.sleep(1.5)
    raise SystemExit(f"timeout: {label}")


def main():
    global TOKEN
    OUT.mkdir(parents=True, exist_ok=True)
    TOKEN = call("POST", "/api/auth/login", {"email": "admin@kendir.test", "password": "portal2026"})["token"]
    step("вход admin@kendir.test")

    config = call("GET", "/api/config")
    key = call("POST", "/api/ismpt/auth/key")
    if config["sign_mode"] != "emulate":
        raise SystemExit("нужен SIGN_MODE=emulate")
    sig = call("POST", "/api/sign/emulate", {"data_base64": base64.b64encode(key["data"].encode()).decode()})["signature"]
    session = call("POST", "/api/ismpt/auth/sign-in", {"uuid": key["uuid"], "signature": sig})
    assert session["connected"], session
    step(f"подключение к ИС МПТ, токен до {session['expires_at']}")

    gtin = gtin14("4870712" + f"{random.randint(20000, 89999):05d}")
    product = call("POST", "/api/products", {
        "gtin": gtin, "name": "Футболка проверочная, синий, р. S", "model": "E2E", "size": "S",
        "color": "синий", "composition": "хлопок 100%", "tnved": "6109100000", "category": "футболки",
    })["data"]
    step(f"создан товар {gtin}")
    product = call("POST", f"/api/products/{product['id']}/nkt-check")["data"]
    assert product["nkt_status"] == "not_found", product
    product = call("POST", f"/api/products/{product['id']}/nkt-register")["data"]
    assert product["nkt_status"] == "found", product
    step("карточка зарегистрирована в НКТ эмулятора")

    order = call("POST", "/api/code-orders", {"product_id": product["id"], "quantity": 6})["data"]
    order = wait(lambda: (o := call("GET", f"/api/code-orders/{order['id']}")["data"])["status"] == "ready" and o, "заказ кодов")
    assert order["codes"]["received"] == 6, order
    step(f"заказ {order['number']} готов, получено 6 кодов, аннулирование {order['expires_at']}")

    codes = call("GET", f"/api/codes?order_id={order['id']}&per_page=50")["data"]
    codes.sort(key=lambda c: c["id"])
    for c in codes:
        assert c["code"].count(GS) == 2, c["code"]
    templates = call("GET", "/api/label-templates")["data"]
    big = next(t for t in templates if t["size"] == "58x40")
    small = next(t for t in templates if t["size"] == "43x25")

    ids_pdf = [c["id"] for c in codes[:3]]
    job = call("POST", "/api/print-jobs", {"template_id": big["id"], "format": "pdf", "target": "download", "code_ids": ids_pdf})["data"]
    pdf = call("GET", job["download_url"], raw=True)
    (OUT / "e2e-58x40.pdf").write_bytes(pdf)
    pages = decode_pdf(OUT / "e2e-58x40.pdf")
    expected = {c["id"]: c["code"] for c in codes}
    for page, cid in zip(pages, ids_pdf):
        assert page, "Data Matrix не прочитан"
        assert page[0]["text"] == expected[cid], (page[0]["text"], expected[cid])
        assert page[0]["symbology"] == "]d2", page[0]["symbology"]
    step(f"PDF 58x40: {len(pages)} этикетки, Data Matrix читается, ]d2 (GS1), GS на месте")

    ids_small = [codes[3]["id"]]
    job = call("POST", "/api/print-jobs", {"template_id": small["id"], "format": "pdf", "target": "download", "code_ids": ids_small})["data"]
    (OUT / "e2e-43x25.pdf").write_bytes(call("GET", job["download_url"], raw=True))
    page = decode_pdf(OUT / "e2e-43x25.pdf")[0]
    assert page and page[0]["text"] == expected[ids_small[0]] and page[0]["symbology"] == "]d2"
    step("PDF 43x25: Data Matrix читается, GS на месте")

    ids_zpl = [c["id"] for c in codes[4:6]]
    job = call("POST", "/api/print-jobs", {"template_id": big["id"], "format": "zpl", "target": "agent", "code_ids": ids_zpl})["data"]
    job = wait(lambda: (j := call("GET", f"/api/print-jobs/{job['id']}")["data"])["status"] in ("printed", "failed") and j, "агент печати", 90)
    assert job["status"] == "printed", job
    zpl = call("GET", job["download_url"], raw=True).decode()
    (OUT / "e2e.zpl").write_text(zpl)
    for cid in ids_zpl:
        field = "~1" + expected[cid].replace(GS, "~1")
        assert field in zpl, field
    assert zpl.count("^XA") == 2 and "^BXN,5,200,0,0,1,~,1" in zpl
    step("ZPL через агента печати: напечатано в файл, FNC1 ~1 в начале и вместо GS")

    status = call("GET", f"/api/code-orders/{order['id']}")["data"]["codes"]
    assert status["printed"] == 6, status

    def document(kind):
        doc = call("POST", "/api/documents/prepare", {"type": kind, "order_id": order["id"]})["data"]
        body = base64.b64decode(doc["body_base64"])
        sig = call("POST", "/api/sign/emulate", {"data_base64": doc["body_base64"]})["signature"]
        call("POST", f"/api/documents/{doc['id']}/submit", {"signature": sig})
        doc = wait(lambda: (d := call("GET", f"/api/documents/{doc['id']}")["data"])["status"] in ("accepted", "rejected", "failed") and d, kind)
        assert doc["status"] == "accepted", doc
        return json.loads(body)

    document("application")
    step("отчёт о нанесении принят ИС МПТ")
    document("introduction")
    status = call("GET", f"/api/code-orders/{order['id']}")["data"]["codes"]
    assert status["introduced"] == 6, status
    step("ввод в оборот принят, 6 кодов в обороте")

    token = call("POST", "/api/settings/integration-token")["token"]
    data = call("GET", f"/api/integration/v1/codes?order={urllib.parse.quote(order['number'])}", token=token)
    assert len(data["codes"]) == 6 and all(c["status"] == "introduced" for c in data["codes"])
    assert all(base64.b64decode(c["code_base64"]).decode() == c["code"] for c in data["codes"])
    csv = call("GET", f"/api/integration/v1/codes?format=csv&order={urllib.parse.quote(order['number'])}", token=token, raw=True)
    (OUT / "e2e-1c.csv").write_bytes(csv)
    assert csv.decode("utf-8-sig").count("\n") == 7
    statuses = call("POST", "/api/integration/v1/codes/statuses", {"codes": [codes[0]["code_hr"]]}, token=token)
    assert statuses["codes"][0]["status_label"] == "В обороте"
    step("выгрузка для 1С: JSON и CSV, статусы по списку кодов")
    print("E2E OK")


if __name__ == "__main__":
    import urllib.parse  # noqa: F401
    main()
