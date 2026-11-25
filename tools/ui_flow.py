from pathlib import Path

from playwright.sync_api import expect, sync_playwright

BASE = "http://127.0.0.1:8301"
OUT = Path(__file__).resolve().parent / "out" / "ui"


def main():
    OUT.mkdir(parents=True, exist_ok=True)
    with sync_playwright() as p:
        browser = p.chromium.launch()
        ctx = browser.new_context(viewport={"width": 1440, "height": 900}, locale="ru-RU", accept_downloads=True)
        page = ctx.new_page()
        errors = []
        page.on("pageerror", lambda e: errors.append(str(e)))
        page.on("console", lambda m: m.type == "error" and errors.append(m.text))

        page.goto(BASE + "/login")
        page.fill("input[type=email]", "admin@kendir.test")
        page.fill("input[type=password]", "portal2026")
        page.click("button[type=submit]")
        page.wait_for_url(BASE + "/")
        print("[ok] вход через форму")

        page.wait_for_load_state("networkidle")
        connect = page.get_by_role("button", name="Подключить ИС МПТ")
        if connect.count():
            connect.first.click()
        expect(page.get_by_text("ИС МПТ подключена").first).to_be_visible(timeout=20000)
        page.screenshot(path=str(OUT / "connected.png"))
        print("[ok] подключение к ИС МПТ из кабинета")

        page.goto(BASE + "/orders?create=1")
        page.wait_for_load_state("networkidle")
        page.screenshot(path=str(OUT / "order-form.png"))
        page.locator("#o-product").fill("боксеры")
        page.locator("[role=option]", has_text="Трусы мужские боксеры").first.click()
        page.locator("input[inputmode=numeric]").fill("4")
        page.locator("button[type=submit][form=order-form]").click()
        expect(page.get_by_text("отправлен в ИС МПТ").first).to_be_visible(timeout=15000)
        page.wait_for_timeout(1000)
        page.locator("a", has_text="З-0000").first.click()
        page.wait_for_url("**/orders/*", timeout=15000)
        expect(page.locator("h1 + .badge, .badge", has_text="Готов").first).to_be_visible(timeout=40000)
        page.wait_for_timeout(1000)
        order_id = page.url.rstrip("/").split("/")[-1]
        page.screenshot(path=str(OUT / "order-ready.png"))
        print(f"[ok] заказ кодов из кабинета готов (id {order_id})")

        page.goto(BASE + f"/print?order_id={order_id}")
        page.wait_for_load_state("networkidle")
        page.wait_for_timeout(1500)
        with page.expect_download(timeout=60000) as dl:
            page.locator("button[type=submit]").click()
        path = OUT / dl.value.suggested_filename
        dl.value.save_as(str(path))
        assert path.read_bytes().startswith(b"%PDF"), path
        page.screenshot(path=str(OUT / "printed.png"))
        print(f"[ok] PDF скачан из кабинета: {path.name}")

        page.goto(BASE + f"/circulation?order_id={order_id}")
        page.wait_for_load_state("networkidle")
        page.wait_for_timeout(800)
        page.get_by_role("button", name="Нанести и ввести в оборот").click()
        expect(page.get_by_role("button", name="Нанести и ввести в оборот")).to_be_enabled(timeout=90000)
        page.wait_for_timeout(1500)
        page.screenshot(path=str(OUT / "circulation-done.png"), full_page=True)
        page.goto(BASE + f"/orders/{order_id}")
        page.wait_for_load_state("networkidle")
        page.wait_for_timeout(800)
        page.screenshot(path=str(OUT / "order-final.png"))
        token = page.evaluate("localStorage.getItem('markirovka.token')")
        data = page.request.get(BASE + f"/api/code-orders/{order_id}", headers={"Authorization": f"Bearer {token}"}).json()
        assert data["data"]["codes"]["introduced"] == 4, data
        print("[ok] отчёт о нанесении и ввод в оборот из кабинета: 4 кода в обороте")
        browser.close()
        if errors:
            print("JS errors:", errors)


main()
