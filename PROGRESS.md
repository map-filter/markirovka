# PROGRESS: прототип «Маркировка за день»

Старт: 25.11.2026, ночь, Нитро. Работа кусками: кусок -> тесты -> коммит.

## План

- [ ] 0. Каркас репо, PROGRESS.md, seed/seed.json (фирма и 18 товаров)
- [ ] 1. Эмулятор ИС МПТ True API (Go stdlib) + тесты + README с допущениями
- [ ] 2. Агент печати (Go) + тесты
- [ ] 3. Docker Compose, nginx-шлюз, Makefile (`make up`, `make test`)
- [ ] 4. Бэкенд Laravel: модели, миграции, сидер, API, адаптер True API, очереди, напоминания
- [ ] 5. Печать: ZPL и PDF с GS1 Data Matrix (FNC1, GS), задания агенту
- [ ] 6. Кабинет Vue 3 + TS
- [ ] 7. NCALayer + режим эмуляции подписи
- [ ] 8. 1С: REST-выгрузка CSV/JSON + исходники внешней обработки (.bsl + форма)
- [ ] 9. OpenAPI /docs/api
- [ ] 10. Тесты PHP, Go, vue-tsc в `make test`
- [ ] 11. Живая проверка: make up с нуля, полный путь, декодирование Data Matrix, вёрстка 1440/1024/768/390
- [ ] 12. README, скриншоты, остановка стека

## Принятые решения

- Каталоги: backend (Laravel), frontend (Vue), emulator (Go), agent (Go), onec (1С), docker (nginx, Dockerfile PHP), seed (общие начальные данные), tools (скрипты проверки), docs.
- Фирма: ТОО «Кендір Тігін», г. Павлодар, БИН 190440015237 (выдуманный). Домен входа `kendir.test` (зарезервированный TLD .test). Пароль `portal2026`.
- GTIN: префикс 487 (GS1 Казахстан) + выдуманный префикс компании 0712, контрольная цифра посчитана, GTIN-14 с ведущим 0. Один файл seed/seed.json читают и бэкенд (сидер), и эмулятор (карточки НКТ).
- Порты: 8301 шлюз nginx, 8302 Postgres, 8303 эмулятор. Остальное внутри сети compose.

## Контракты между сервисами

### Эмулированная подпись (режим SIGN_MODE=emulate)
CMS-заглушка = base64 от текста из строк, разделитель `\n`:
```
MKEMU-CMS-v1
<base64 подписываемых данных>
BIN:<БИН>
SIG:<hex sha256(подписываемые данные как байты + БИН)>
```
Эмулятор принимает её, если EMU_ACCEPT_EMULATED_SIGNATURE=true. Настоящий CMS (DER, начинается с 0x30) принимается при EMU_ACCEPT_ANY_CMS=true без криптопроверки, БИН берётся из заголовка/тела запроса.

### Агент печати <-> бэкенд (HTTPS long-polling)
- `GET  {SERVER}/api/agent/jobs/next?wait=20`, заголовок `Authorization: Bearer <AGENT_TOKEN>`: 200 `{"id":"..","format":"zpl"|"pdf"|"raw","copies":1,"printer":"","payload_base64":".."}` или 204.
- `POST {SERVER}/api/agent/jobs/{id}/ack` `{"status":"printed"|"failed","error":""}`
- `POST {SERVER}/api/agent/heartbeat` `{"version":"..","hostname":"..","printer":".."}`

## Что проверено руками

(пока ничего)

## Проблемы и обходы

(пока нет)
