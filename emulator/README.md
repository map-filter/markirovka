# Эмулятор ИС МПТ True API (Казахстан)

Сервис на Go (только стандартная библиотека) повторяет поведение True API ИС МПТ РК
(оператор Tañba, Казахтелеком) для лёгкой промышленности: авторизация по ЭЦП, карточки НКТ,
заказ и получение кодов маркировки, отчёт о нанесении, ввод в оборот, вывод из оборота,
сведения о кодах. Состояние хранится в памяти и сохраняется в JSON-файл.

Основа: публичный PDF True API 2.18, выжимка в `../docs/true-api-notes.md`.
Где документ конкретен (пути, методы, поля, форматы ответов и ошибок), эмулятор следует ему.
Чего в документе нет, сделано правдоподобно и перечислено в разделе «Что предположено».

## Запуск

```sh
docker build -t markirovka-emulator:dev .
docker run --rm -p 127.0.0.1:8303:8080 \
  -v "$PWD/../seed:/seed:ro" -v markirovka-emu-data:/data \
  markirovka-emulator:dev
```

- API: `http://127.0.0.1:8303/api/v3/true-api/...`
- Консоль: `http://127.0.0.1:8303/emu/`
- Проверка живости: `GET /healthz` -> `{"status":"ok"}`

Тесты: `docker run --rm -v "$PWD:/src" -w /src -e CGO_ENABLED=0 golang:1.23-alpine go test ./...`

## Переменные окружения

| Переменная | По умолчанию | Назначение |
|---|---|---|
| `EMU_ADDR` | `:8080` | адрес прослушивания |
| `EMU_SEED` | `/seed/seed.json` | начальные данные: участник и карточки НКТ |
| `EMU_STATE_FILE` | `/data/state.json` | файл состояния; читается при старте, пишется после каждого изменения |
| `EMU_ORDER_DELAY_SEC` | `5` | через сколько секунд заказ кодов становится `SUCCESS` |
| `EMU_DOC_DELAY_SEC` | `2` | через сколько секунд документ проверяется (`IN_PROGRESS` -> `CHECKED_OK`/`CHECKED_NOT_OK`) |
| `EMU_ANNUL_DAYS` | `30` | через сколько дней от эмиссии ненанесённый код аннулируется |
| `EMU_TOKEN_TTL_HOURS` | `10` | срок жизни токена (по документу 10 часов) |
| `EMU_ACCEPT_EMULATED_SIGNATURE` | `true` | принимать эмулированную подпись `MKEMU-CMS-v1` |
| `EMU_ACCEPT_ANY_CMS` | `false` | принимать настоящую CMS (DER) без криптопроверки |
| `EMU_ANY_PARTICIPANT` | `false` | пускать любой БИН из эмулированной подписи (участник создаётся без карточек) |

Формат `seed.json`:
```json
{"participant": {"bin": "190440015237", "name": "ТОО «Кендір Тігін»", "address": "..."},
 "products": [{"gtin": "04870712001017", "name": "...", "model": "FT-M-01", "size": "M", "color": "белый",
   "composition": "хлопок 100%", "tnved": "6109100000", "category": "футболки", "brand": "Кендір", "country": "Казахстан"}]}
```

## Общие правила

- Базовый путь как в проде: `/api/v3/true-api`. Ниже пути указаны относительно него, если не сказано иное.
- Токен: заголовок `Authorization: Bearer <token>` (принимается и `Bearer:<token>`).
- Ошибки: JSON `{"error_message": "<текст>"}` с HTTP-кодом 400/404/409/413/422/500.
- 401 отдаётся в XML, как в документе (`Content-Type: application/xml`):
  - нет токена:
    `<UnauthorizedException><error>unauthorized</error><error_description>Full authentication is required to access this resource</error_description></UnauthorizedException>`
  - токен истёк:
    `<InvalidTokenException><error>invalid_token</error><error_description>Access token expired: <token></error_description></InvalidTokenException>`
  - токен подделан или от другого экземпляра: `<InvalidTokenException>...Cannot convert access token to JSON...`
  
  Адаптер на 401 должен заново пройти `/auth/key` + `/auth/simpleSignIn`.
- Неизвестный путь под `/api/v3/true-api/`: 404 `{"error_message": "Метод с указанным URL не найден"}`.
- Даты в ответах: RFC 3339 UTC с миллисекундами, например `2026-11-25T18:32:37.047Z`.
- Товарная группа: `pg=lp`, `productGroupId: 1`.

## Подпись (контракт эмуляции)

Эмулированная CMS = base64 от текста из 4 строк, разделитель `\n`:
```
MKEMU-CMS-v1
<base64 подписываемых данных>
BIN:<БИН>
SIG:<hex sha256(подписываемые данные как байты + БИН в ASCII)>
```
Пример на Python:
```python
def emu_sign(data: bytes, bin_: str) -> str:
    text = "\n".join(["MKEMU-CMS-v1", base64.b64encode(data).decode(), "BIN:" + bin_,
                      "SIG:" + hashlib.sha256(data + bin_.encode()).hexdigest()])
    return base64.b64encode(text.encode()).decode()
```
PHP: `base64_encode(implode("\n", ['MKEMU-CMS-v1', base64_encode($data), "BIN:$bin", 'SIG:'.hash('sha256', $data.$bin)]))`.

- Для `/auth/simpleSignIn` подписываемые данные = строка `data` из `/auth/key` (байты строки, как
  NCALayer подписывает после `createCMSSignatureFromBase64`). БИН определяет участника: должен
  совпадать с БИН из seed (или любой при `EMU_ANY_PARTICIPANT=true`).
- Для документов подпись открепленная: подписываемые данные = байты документа после base64-декодирования
  поля `document`/`content`. Эмулятор сверяет, что вложенный base64 совпадает с документом,
  `SIG` верен, а БИН подписи равен БИН из токена. Иначе 422 `"Ошибка проверки подписи: ..."`.
- Настоящая CMS (base64 DER, первый байт `0x30`) принимается только при `EMU_ACCEPT_ANY_CMS=true`,
  без криптопроверки; участником считается БИН из seed.

## Методы True API

### GET /auth/key
Ответ 200:
```json
{"uuid": "bcaf39f0-6884-43fc-9822-af5ad3fb01b9", "data": "TSLVKEKXQCMTAMGBTOKKKXEYHFREXZ"}
```
`uuid` одноразовый, действует 10 минут.

### POST /auth/simpleSignIn
Запрос: `{"uuid": "<из /auth/key>", "data": "<CMS base64>"}`.
Ответ 200: `{"token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJwaWQiOi..."}` (JWT HS256, в payload
`pid`, `bin`, `iat`, `exp`, `jti`; срок 10 часов).
Ошибки: 400 `{"error_message": "Отсутствует обязательный параметр: uuid"}` (или `data`);
401 `{"code": "401", "error_message": "Ошибка авторизации или компания не зарегистрирована в ИС МПТ", "description": "<причина>"}`.

### GET /product/gtin?pg=lp&limit=10&page=0
Ответ: `{"gtins": ["04870712001017", "04870712001024"], "total": 18}`. `limit` 1..10000 (по умолчанию 10),
`page` с 0. Ошибки: 400 `"Отсутствует обязательный параметр: pg"`, 400 `"Could not determine product group by name 'shoe'"`.

### GET /products/info?cis=<КИ или GTIN>
`cis` принимает КИ в любом виде (`(01)...(21)...`, `01...21...`, полный код с GS) или голый GTIN-14;
также работает `?gtin=<GTIN>`. Ответ:
```json
{"id": 1, "name": "Футболка мужская базовая, белый, р. M", "gtin": "04870712001017",
 "packageType": "trade-unit", "innerUnitCount": 1, "productGroupId": 1, "productGroup": "lp",
 "ownerBin": "190440015237",
 "specificAttributes": {"model": "FT-M-01", "size": "M", "color": "белый", "composition": "хлопок 100%",
   "tnved": "6109100000", "category": "футболки", "brand": "Кендір", "country": "Казахстан"}}
```
Ошибки: 400 `"Неверный формат КМ"`, 404 `"Продукт не найден"`.

### POST /cises/my
Два режима.
1. **Эмиссия (расширение эмулятора)**: `{"gtin": "04870712001017", "quantity": 100}`, `quantity` 1..10000,
   `packageType` необязателен (только `UNIT`). Создаются новые коды в статусе `EMITTED`.
2. **Список существующих КМ (как в документе)**: `{"gtin": "04870712001017"}` без `quantity`.
   Результат: все коды участника с этим GTIN (до 10000).

Ответ 200: `{"uuid": "203b3d7c-a97e-4257-afc8-9926167d927a"}`.
Ошибки: 400 `"GTIN должен быть указан в параметрах запроса"`, 400 `"Неверный формат GTIN: ..."`,
400 `"Параметр quantity требует значение от 1 до 10000"`, 400 `"JSON parse error: ..."`,
404 `"Товар с GTIN <gtin> не найден в НКТ участника"`.

### GET /cises/orders/{uuid}/status
Ответ: `{"status": "IN_PROGRESS"}`, через `EMU_ORDER_DELAY_SEC` секунд `{"status": "SUCCESS"}`.
404 `{"error_message": "Заказ с идентификатором <uuid> не найден"}`.

### GET /cises/orders/{uuid}/result
По умолчанию (как в документе) `application/zip`, файл `<uuid>.zip`, внутри один файл `<uuid>.json`
с JSON-массивом. Если в `Accept` только `application/json` (без `*/*` и zip), тот же массив
отдаётся сразу как JSON. До готовности: 409 `"Заказ с идентификатором <uuid> еще не выполнен"`.
Элемент массива:
```json
{"cis": "(01)04870712001017(21)_5nMTSx6mNQRT",
 "code": "010487071200101721_5nMTSx6mNQRT",
 "printCode": "010487071200101721_5nMTSx6mNQRT\u001d919N1N\u001d92XaM87V/IiqK1LF0uQiueoBNgbPqFLZ3fRUrh4Jp98F4=",
 "humanCode": "(01)04870712001017(21)_5nMTSx6mNQRT(91)9N1N(92)XaM87V/IiqK1LF0uQiueoBNgbPqFLZ3fRUrh4Jp98F4=",
 "gtin": "04870712001017", "packageType": "UNIT", "status": "EMITTED",
 "emissionDate": "2026-11-25T18:32:37.047Z"}
```
- `cis`, `gtin`, `packageType`, `status`, `producedDate` (только после ввода в оборот) из документа;
- `code` = КИ без скобок (31 символ), `printCode` = полная строка для Data Matrix с символом
  GS (0x1D, в JSON `\u001d`) перед AI 91 и 92, `humanCode` = человекочитаемая форма, `emissionDate`:
  расширения эмулятора. Печатать нужно `printCode` (FNC1 в начале добавляет принтер/библиотека).

### POST /cises/info
Запрос: JSON-массив кодов в любом виде, до 1000: `["(01)04870712001017(21)_5nMTSx6mNQRT", "0104870712001017..."]`.
Ответ:
```json
[{"cisInfo": {"requestedCis": "(01)04870712001017(21)_5nMTSx6mNQRT", "cis": "(01)04870712001017(21)_5nMTSx6mNQRT",
   "gtin": "04870712001017", "productGroupId": 1, "productGroup": "lp",
   "productName": "Футболка мужская базовая, белый, р. M", "brand": "Кендір",
   "producedDate": "2026-11-25T18:32:45.120Z", "packageType": "UNIT",
   "ownerBin": "190440015237", "ownerName": "ТОО «Кендір Тігін»", "status": "INTRODUCED",
   "producerBin": "190440015237", "producerName": "ТОО «Кендір Тігін»",
   "emissionDate": "2026-11-25T18:32:37.047Z", "applicationDate": "2026-11-25T18:32:41.300Z"}},
 {"cisInfo": {"requestedCis": "0104870712001017..."}, "errorCode": "404", "errorMessage": "КМ не найден"},
 {"cisInfo": {"requestedCis": "..."}, "errorCode": "400", "errorMessage": "потерян символ GS"}]
```
Для аннулированного кода `status: "WRITTEN_OFF"` и `statusExt: "ANNULLED"`. Есть также `withdrawalDate`.
Чужие коды молча не выводятся (как в документе). Ошибки: 400 `"В запросе не указан ни один КМ"`,
400 `"Required request body is missing"`, 404 `"КМ не найдены"` (если ни один код не найден).

### POST /documents/create (расширение эмулятора)
Запрос:
```json
{"type": "KZ_APPLICATION_REPORT", "document": "<base64 JSON документа>", "signature": "<открепленная CMS base64>"}
```
`type`: `KZ_APPLICATION_REPORT` (отчёт о нанесении), `KZ_LP_INTRODUCE_GOODS` (ввод в оборот, производство РК),
`MTTPS_90` (вывод из оборота). Ответ 200: `{"documentId": "<id>"}`.

Структура документа (JSON до base64, по образцу уведомлений из документа):
```json
{"fileId": "6b2c1f0e-...", "version": "1.0", "sendingDateTime": "2026-11-25T18:00:00Z",
 "document": {
   "mainInfo": {"participantInfo": {"participantName": "ТОО «Кендір Тігін»", "participantIdentificationCode": "190440015237"},
                "originalDocInfo": {"docName": "...", "docNumber": "...", "docDate": "25.11.2026"},
                "reason": "1"},
   "productList": [{"iCode": "<код>"}, {"iCode": "<код>"}]}}
```
- `fileId` необязателен; если задан, он и есть `documentId`. Повтор того же `fileId` с тем же содержимым
  возвращает тот же `documentId` (идемпотентно), с другим содержимым 409.
- `participantIdentificationCode` необязателен, но если указан, должен совпадать с БИН токена.
- `reason` обязателен только для `MTTPS_90` (1..8, как в документе); `originalDocInfo` не проверяется.
- `iCode` принимает: полный код с GS (`printCode`), человекочитаемый с (91)(92), КИ со скобками или без.
  Если передан код с криптохвостом, 91/92 сверяются с эмитированными.

Ошибки при создании: 400 `"Отсутствует обязательный параметр: type|document|signature|document.productList"`,
400 `"Неизвестный тип документа: X"`, 400 `"Невалидный формат документа: ..."`,
422 `"Ошибка проверки подписи: ..."`, 409 повтор `fileId`.

### POST /documents/writeOff
Как в документе: `{"document": "<base64>", "signature": "<CMS>"}`, тип всегда `MTTPS_90`.
Ответ `{"documentId": "..."}`.

### POST /edo-api/outgoing-documents/formal
Также доступен как `/api/v3/true-api/edo-api/outgoing-documents/formal` и как `/edo-api/outgoing-documents/formal`
от корня. `multipart/form-data` или `application/x-www-form-urlencoded`, поля `content` (base64 JSON
документа, структура как выше), `signature`, `type` (расширение). Ответ `{"documentId": "..."}`.
XML-документы (Акт ПП, ЕАЭС) не поддерживаются.

### GET /documents/{documentId}/info и GET /documents/info/{documentId}
Ответ:
```json
{"documentId": "cd23ef91-...", "number": "00000001", "docDate": "2026-11-25T18:32:41.300Z",
 "receivedAt": "2026-11-25T18:32:41.300Z", "processedAt": "2026-11-25T18:32:43.500Z",
 "type": "KZ_APPLICATION_REPORT", "status": "CHECKED_NOT_OK", "externalId": "cd23ef91-...",
 "senderBin": "190440015237", "senderName": "ТОО «Кендір Тігін»", "receiverBin": "", "receiverName": "",
 "downloadStatus": "CHECKED_NOT_OK", "body": {"...": "исходный JSON документа"}, "content": "<base64>", "input": false,
 "errorMessage": "Документ не прошёл проверку: ошибок по КМ 1 из 3",
 "errors": ["(01)04870712001017(21)_5nMTSx6mNQRT: Недопустимый статус КМ INTRODUCED, ожидается EMITTED"],
 "cisErrors": [{"cis": "(01)04870712001017(21)_5nMTSx6mNQRT", "errorCode": "CIS_WRONG_STATUS",
                "errorMessage": "Недопустимый статус КМ INTRODUCED, ожидается EMITTED"}]}
```
`status`: `IN_PROGRESS` -> через `EMU_DOC_DELAY_SEC` `CHECKED_OK` или `CHECKED_NOT_OK`.
При `CHECKED_OK` поля `errors` и `cisErrors` пустые массивы, `errorMessage` отсутствует.
404 `"Документ с указанным идентификатором не найден"`.

Коды ошибок по КМ (`cisErrors[].errorCode`):

| Код | Когда |
|---|---|
| `GS_LOST` | в полном коде потерян символ GS (`errorMessage: "потерян символ GS"`) |
| `CIS_FORMAT` | код не разбирается (GTIN, контрольная цифра, серийный номер) |
| `CIS_DUPLICATE` | код повторяется в документе |
| `CIS_NOT_FOUND` | код не эмитирован |
| `CIS_NOT_OWNED` | код принадлежит другому участнику |
| `CRYPTO_MISMATCH` | 91/92 не совпадают с эмитированными |
| `CIS_ANNULLED` | код аннулирован (не нанесён за `EMU_ANNUL_DAYS` дней) |
| `CIS_WRONG_STATUS` | статус кода не подходит для перехода |

Документ атомарный: при любой ошибке ни один код не меняет статус.

### GET /doc/listV2?limit=10&documentStatus=CHECKED_OK&documentType=MTTPS_90
Ответ `{"results": [<как /documents/{id}/info без body и content>], "total": N}`, новые первыми.

## Статусы

Коды (справочник «Статусы КМ» из документа):

| Состояние в эмуляторе | `status` | Переход |
|---|---|---|
| эмитирован | `EMITTED` | при заказе `quantity` |
| нанесён | `APPLIED` | `KZ_APPLICATION_REPORT`, только из `EMITTED` |
| введён в оборот | `INTRODUCED` | `KZ_LP_INTRODUCE_GOODS`, только из `APPLIED` |
| выведен из оборота | `WITHDRAWN` | `MTTPS_90`, только из `INTRODUCED` |
| аннулирован | `WRITTEN_OFF` + `statusExt: "ANNULLED"` | автоматически: `EMITTED` старше `EMU_ANNUL_DAYS` по времени эмулятора |

Заказы: `IN_PROGRESS`, `SUCCESS`. Документы: `IN_PROGRESS`, `CHECKED_OK`, `CHECKED_NOT_OK`.

## Формат кода (пакет `internal/gs1`)

Строка элементов GS1 Data Matrix:
`01` + GTIN-14 + `21` + серийный номер (13 символов) + GS + `91` + ключ (4 символа) + GS + `92` + криптоподпись (44 символа).

- Серийный номер генерируется из набора GS1 AI 82 без `"`, `'`, `\`, `^`, `~` (безопасно для ZPL и CSV)
  и без `(`, `)` (чтобы человекочитаемая форма разбиралась однозначно). Разбор принимает весь набор AI 82.
- Ключ 91: `[0-9A-Za-z]{4}`, криптоподпись 92: base64 (`A-Za-z0-9+/=`), 44 символа. Оба детерминированно
  выводятся из КИ по секрету экземпляра (HMAC), поэтому их можно проверить.
- Разбор: префиксы `]d2`, `]C1` и ведущий GS отбрасываются; поддерживается человекочитаемая форма
  `(01)...(21)...(91)...(92)...`. Если после 13 символов серийного номера сразу идёт `91` без GS,
  или в значении 91 сразу после 4 символов идёт `92`, возвращается ошибка «потерян символ GS».

## Служебные методы эмулятора (без токена)

| Метод и путь | Тело | Ответ |
|---|---|---|
| `GET /emu/` | | HTML-консоль, обновляется каждые 5 с |
| `GET /emu/api/state` | | `{"state": {now, emuNow, offsetSec, annulDays, participants, products, orders, codeCounts, totalCodes, documents}, "log": [...]}` |
| `POST /emu/api/nkt/cards` | `{"gtin", "name", "model", "size", "color", "composition", "tnved", "category", "brand", "country", "participantBin"?}` | 201, тело как `/products/info`; 400 неверный GTIN/контрольная цифра/нет `name`; 409 GTIN уже есть |
| `POST /emu/api/time` | `{"addDays": 30}` или `{"reset": true}` (или форма `days=30` / `action=reset`) | `{"ok": true, "emuNow": "...", "offsetSec": 2592000}` |
| `POST /emu/reset` (или `/emu/api/reset`) | | `{"ok": true, ...}`; восстанавливает состояние из seed, сдвиг времени 0 |
| `GET /healthz` | | `{"status": "ok"}` |

Если в `Accept` есть `text/html` (формы консоли), вместо JSON будет редирект 303 на `/emu/`.

## Время эмулятора

Время эмулятора = реальное время + сдвиг (кнопки +1, +7, +30 дней в консоли). От него считаются даты эмиссии,
нанесения, ввода и аннулирование. Аннулирование необратимо: сброс сдвига не возвращает коды.
Задержки заказов и документов и срок жизни токена считаются по реальному времени, поэтому сдвиг
не разлогинивает бэкенд.

## Сброс

Кнопка «Сбросить эмулятор» в консоли или `curl -X POST http://127.0.0.1:8303/emu/reset`.
Удаляются коды, заказы, документы и добавленные карточки, сдвиг времени обнуляется. Секрет подписи
токенов сохраняется, поэтому выданные токены продолжают работать. Полный сброс: удалить `EMU_STATE_FILE`
(или volume `/data`) и перезапустить.

## Что предположено

1. **Эмиссия кодов.** В True API 2.18 `POST /cises/my` только заказывает список уже существующих КМ;
   эмиссия идёт через API СУЗ (OMS), его описания в открытом доступе нет. Эмулятор добавил к `/cises/my`
   поле `quantity`: с ним заказ эмитирует новые коды. Без него поведение как в документе.
2. **Статусы заказа.** Используются только `IN_PROGRESS` (в документе написано «IN PROGRESS» с пробелом)
   и `SUCCESS`. `RESULT_EXPIRED` не наступает: результат можно скачивать сколько угодно.
3. **Результат заказа.** До готовности 409. Внутри ZIP файл `<uuid>.json`. Поля `code` (КИ),
   `printCode`, `humanCode`, `emissionDate` добавлены, т.к. в документе нет кода с криптохвостом для печати.
4. **Отчёт о нанесении и ввод в оборот.** В документе нет ни отчёта о нанесении (он в СУЗ), ни ввода
   в оборот собственного производства (есть только повторный ввод `KZ_INTRODUCE_GOODS`). Придуманы типы
   `KZ_APPLICATION_REPORT` и `KZ_LP_INTRODUCE_GOODS` и общий метод `POST /documents/create` с полями
   `document` и `signature` как у `/documents/transit/introduce-goods/create`, плюс `type`.
   Ответ `{"documentId"}` взят из метода `formal` (у `introduce-goods/create` в документе пустое тело).
5. **Тело документа** в JSON (документ разрешает JSON), структура `fileId/version/sendingDateTime/document.mainInfo/productList[].iCode`
   из уведомления о вводе. XML и CSV не поддерживаются.
6. **Вывод из оборота** `MTTPS_90` через `POST /documents/writeOff` разрешён только из `INTRODUCED`.
7. **Построчные ошибки.** В документе только `errors` (массив строк) и `errorMessage`; эмулятор заполняет
   `errors` строками `"<cis>: <текст>"` и добавляет `cisErrors` с кодами из таблицы выше.
8. **Аннулирование.** Статуса «аннулирован» в справочнике нет; используется `WRITTEN_OFF` (Утилизирован)
   с признаком `statusExt: "ANNULLED"`. Срок 30 дней от эмиссии, только для `EMITTED`.
9. **Товарная группа.** Лёгкой промышленности в справочнике 2.18 нет; взято `pg=lp`, `productGroupId: 1`.
10. **Формат кода** лёгкой промышленности (13 символов серийного номера, 91 из 4 символов, 92 из 44 символов)
    в документе не описан, взят по аналогии с практикой GS1 Data Matrix.
11. **`/products/info`** по документу принимает только `cis`; эмулятор также принимает голый GTIN и `?gtin=`.
    Атрибуты НКТ лежат в `specificAttributes`; `productGroupId`, `productGroup`, `ownerBin` добавлены.
12. **`/cises/info`**: ненайденные и неразборчивые коды возвращаются элементами с `errorCode`/`errorMessage`
    (в документе эти поля есть, но не описано, когда), чужие коды скрываются. Добавлены `statusExt`,
    `emissionDate`, `applicationDate`, `withdrawalDate`.
13. **Токен**: JWT HS256 с `pid` = `bin` = БИН. `uuid` из `/auth/key` одноразовый и живёт 10 минут.
    Ответ 401 у `simpleSignIn` дополнен полем `description` с причиной (поле есть в документе).
14. **Ошибка «товар не в НКТ»** при заказе: 404 `"Товар с GTIN ... не найден в НКТ участника"`.
15. **Регистрация карточки НКТ** (`POST /emu/api/nkt/cards`) в True API отсутствует, это служебный метод эмулятора.
16. **Договор по ТГ** (403 `"Отсутствует действующий договор по ТГ"`) не эмулируется: договор считается подписанным.
17. Остальные методы документа (агрегация, `/cises/list`, `/cises/listV2`, история, Акт ПП, ЕАЭС, импорт,
    экспорт, биллинг, чеки) не реализованы и отвечают 404 `"Метод с указанным URL не найден"`.
