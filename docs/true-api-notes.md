# True API ИС МПТ РК: выжимка из документации

Источник: публичный PDF «(kz) True API. Версия 2.18» (12.01.2022, 161 стр.),
https://tanba.telecom.kz/content/dam/ocp/companies/Markirovka/documents/TrueAPI%202.18.pdf
(скачан 25.11.2026, HTTP 200). На странице https://tanba.telecom.kz/ru/business/documents
есть только True API 2.11, «Описание резилов True API», руководство пользователя СУЗ (.doc)
и инструкция по НК. True API 2.32 и API СУЗ (OMS) в публичном доступе не найдены.

## 1. Общее

- Базовый URL (стенд, хост): `https://elk.prod.markirovka.ismet.kz/api/v3/true-api/`.
  Все пути ниже относительно него (`<url стенда>`), в том числе `/edo-api/...`:
  в примерах документа `POST <url стенда>/edo-api/outgoing-documents/formal`.
- Формат: JSON (RFC 7159, UTF-8), CSV (RFC 4180), XML по XSD. Документ до 30 МБ.
- Accept: рекомендовано `*/*`, т.к. часть методов отдаёт `application/zip`.
  Если подходящего типа нет, 406 с пустым телом.
- Экранирование КМ: JSON по RFC 8259 (`"` -> `\"`), URL по RFC 3986 (`"` -> `%22`),
  CSV: кавычка удваивается.

## 2. Ошибки (общий формат)

- JSON: `{"error_message": "<текст>"}`. Примеры текстов:
  `"Отсутствует обязательный параметр: <имя>"`, `"Параметр <имя> требует значение типа <тип>"`,
  403 `"Отсутствует доступ к ресурсу"`, 404 `"<Описание параметра> не найден"`,
  404 `"Метод с указанным URL не найден"`, 413 `"Слишком большой запрос"`,
  500 `"Ошибка при выполнении запроса"`, 503 `"Удаленная система недоступна"`.
- 401 отдаётся в XML:
  ```xml
  <UnauthorizedException>
    <error>unauthorized</error>
    <error_description>Full authentication is required to access this resource</error_description>
  </UnauthorizedException>
  ```
  для устаревшего токена:
  ```xml
  <InvalidTokenException>
    <error>invalid_token</error>
    <error_description>Access token expired: <токен></error_description>
  </InvalidTokenException>
  ```
- Частые 403: `{"error_message": "Отсутствует действующий договор по ТГ"}`.
- Отдельные методы: /cises/info при 500 отдаёт `{"errorMessage": "...", "errorCode": "500"}`.

## 3. Авторизация

- `GET /auth/key` -> `{"uuid": "a63ff582-...", "data": "GNUFBAZBMPIUUMLXNMIOGSHTGFXZMT"}`
  (data в примерах: 30 заглавных латинских букв).
- Строку `data` кодируют в Base64 и подписывают в NCALayer методом
  `kz.gov.pki.knca.commonUtils.createCMSSignatureFromBase64` с флагом `true`
  (подпись с включением данных, attached CMS). Результат `responseObject` (base64 DER, `MIIS...`).
- `POST /auth/simpleSignIn`, `Content-Type: application/json`,
  тело `{"uuid": "<из /auth/key>", "data": "<CMS base64>"}` -> `{"token": "eyJhbGciOiJIUzI1NiIs..."}`
  (JWT). Поля ответа при ошибке: `code`, `error_message`, `description`.
  400 при отсутствии uuid/data; 401 `{"error_message": "Ошибка авторизации или компания не зарегистрирована в ИС МПТ"}`.
- Срок жизни токена: 10 часов.
- Заголовок: `Authorization: Bearer <ТОКЕН>` (в одном примере биллинга `Bearer:<ТОКЕН>`).
- В токене есть свойство `pid` (упоминается в ошибках биллинга).

## 4. Методы КМ

### POST /cises/my
Заказ на **список уже существующих КМ участника** (не более 10 000), не эмиссия.
Тело: `gtin` (String, обязательно; для UNIT `like 'gtin%'`), `packageType` (String, нет),
`producedDate` (`yyyy-MM-ddTHH:mm:ss.SSS'Z'`, нет). Ответ: `{"uuid": "..."}`.
400 `"GTIN должен быть указан в параметрах запроса"`, 400 `"JSON parse error: ..."`.

### GET /cises/orders/{uuid}/status
Ответ `{"status": "..."}`. Значения: `NEW` (Зарегистрирован), `PRODUCTION`, `VALIDATION_FAILED`,
`IN PROGRESS` (так в документе, с пробелом), `SUCCESS`, `ERROR`, `RESULT_EXPIRED`.
404 `{"error_message": "Заказ с идентификатором <uuid> не найден"}`.

### GET /cises/orders/{uuid}/result
Отдаёт **ZIP** (`application/zip`, имя `<uuid>.zip`, до ~1 МБ, хранится час, можно качать повторно).
Внутри JSON-массив объектов: `requestedCis`, `cis` (обяз.), `code`, `status` (обяз.), `gtin` (обяз.),
`producedDate`, `packageType` (обяз.), `child`, `parent`. Пример элемента:
`{"cis": "(01)00000046210654(21)4u4qrBQ", "gtin": "00000046210654", "producedDate": "2019-12-04T00:46:40Z", "packageType": "LEVEL1", "status": "APPLIED"}`.

### POST /cises/info
Тело: JSON-массив КИ (КМ без криптохвоста), до 1000:
`["000000462106549pJu6lt", "(01)00000046210654(21)4u4qrBQ"]`.
Ответ: массив `[{"cisInfo": {...}, "errorCode": "...", "errorMessage": "..."}]`, поля cisInfo:
`requestedCis, cis, gtin, productGroupId, productGroup, productName, brand, producedDate
(дата ввода в оборот), packageType, ownerBin, ownerName, status, child, parent, producerBin,
producerName, exporterName`. Чужие КМ не показываются (пустой массив).
400 `"В запросе не указан ни один КМ"`, `"Required request body is missing"`; 404 если ни один не найден.

### Прочие (не эмулируются)
`POST /cis/cises-units-count` (`cis[]` -> `cisesCounts`, `gtinsCounts`), `POST|GET /cises/aggregated/list`,
`POST /cises/list?values=...` (поля как в cisInfo), `GET /cises/listV2?gtin=&cisStatus=&limit=`, `GET /cises/history`.

## 5. Товары

- `GET /product/gtin?pg=<ТГ>&limit=<до 10000, по умолчанию 10>&page=<с 0>` ->
  `{"gtins": ["04630034070029", ...], "total": 2}` (в таблице поле опечатано как `gtings`).
  400 `"Could not determine product group by name 'shoe'"`.
- `GET /products/info?cis=<КИ>` -> `{"id": 92, "name": "...", "gtin": "04870003870025",
  "packageType": "trade-unit", "innerUnitCount": 1, "specificAttributes": {}}`.
  packageType: `trade-unit`, `inner-pack`, `box`, `pallet`. 400 `"Неверный формат КМ"`, 404 `"Продукт не найден"`.
- `POST /product/route/gtin`, `GET /products/listV2`: не эмулируются.

## 6. Документы

- Общие: `GET /edo-api/outgoing-documents`, `GET /edo-api/incoming-documents`,
  `GET outgoing-documents/{id}/base64`, `GET /documents/info/{documentId}`, `POST /documents/search`,
  события по Акту ПП (`/events/{decision}`, `/events/withdraw`).
- `GET /documents/info/{documentId}` (для уведомлений о вводе/выводе/ввозе ЕАЭС):
  `{"status": "CHECKED_OK", "type": "KZ_INTRODUCE_GOODS", "body": {"document": {"mainInfo": {...},
  "cisList": [{"value": "...", "name": "...", "status": "Возвращен в оборот"}]}, "fileId": "...",
  "version": "1.0", "sendingDateTime": "2020-10-17T09:30:47Z"}}`.
- `GET /documents/{document_id}/info` (прочие документы): поля `number, docDate, receivedAt, type,
  status, externalId, senderBin, senderName, receiverBin, receiverName, invoiceNumber, invoiceDate,
  total, vat, downloadStatus, downloadDesc, body, content, input, pdfFile, errors (массив строк),
  docErrors, errorMessage ("code: 71 error: ..."), errorMessages, sender{name,address,bin,kpp}, receiver{...}`.
  404 `"Документ с указанным идентификатором не найден"`.
- `GET /doc/listV2`: фильтры `documentStatus`, `documentType`, `participantBin`, `pg`, `pageDir`, `limit`.
- Создание уведомлений (JSON-тело): `{"document": "<base64>", "signature": "<CMS base64 без включения данных>"}`
  (открепленная подпись):
  - `POST /documents/transit/introduce-goods/create`: повторный ввод в оборот (`KZ_INTRODUCE_GOODS`).
    Структура документа: `fileId, version, sendingDateTime, document.mainInfo{reason (1 излишки,
    2 до даты обязательной маркировки, 3 конфискат), participantInfo{participantName,
    participantIdentificationCode}, originalDocInfo{docName, docNumber, docDate dd.mm.yyyy}},
    productList[{iCode (КМ без криптоключа), count}]`. Успех: 200 с пустым телом, id = fileId.
    400 невалидный документ, 422 `Ошибка проверки подписи`.
  - `POST /documents/writeOff`: вывод из оборота (`MTTPS_90`), структура как выше, reason
    1 Брак, 2 Утрата, 3 Повреждение, 4 Уничтожение, 5 Собственные нужды, 6 Отбор образцов, 7 ..., 8 Возврат физлицу.
  - `POST /documents/transit/import/third_countries`: импорт.
- `POST /edo-api/outgoing-documents/formal`, `multipart/form-data`, поля `content` (base64 XML)
  и `signature`. Только Акт приема-передачи (`KZ_UPD`), уведомление о расхождениях, уведомление
  о ввозе ЕАЭС (`IMPORT_TRANSIT`, fileId `IMPORT_TRANSIT_<UUID>`). Ответ: `documentId`.
- `POST /edo-api/shipments`, `GET /edo-api/shipments/{id}/status`: экспорт.

## 7. Справочники

- ТГ: `2 shoes`, `3 tobacco`, `7 pharma`, `8 milk`, `11 alcohol`. **Лёгкой промышленности нет.**
- Типы упаковки: `UNIT, LEVEL1..LEVEL5, BUNDLE`.
- Статусы КМ: `0 EMITTED` Эмитирован, `1 APPLIED` Нанесён, `12 APPLIED_NOT_PAID`,
  `2 INTRODUCED` Введен в оборот, `6 INTRODUCED_RETURNED`, `4 WITHDRAWN` Выведен из оборота,
  `10 LOAN_RETIRED`, `11 REMARK_RETIRED`, `7 DISAGGREGATED`, `5 RESERVED_NOT_USED`, `9 EXPORTED`,
  `3 WRITTEN_OFF` Утилизирован. Статуса «аннулирован» нет.
- Типы документов: `RECEIPT, RECEIPT_RETURN, KZ_INTRODUCE_GOODS, MTTPS_90, IMPORT, IMPORT_TRANSIT,
  WRITE_OFF, AGGREGATION_DOCUMENT, KZ_UPD`.
- Статусы документов: `IN_PROGRESS` Проверяется, `CHECKED_OK` Оформлен, `CHECKED_NOT_OK` Ошибка
  при проверке, `PROCESSING_ERROR`, `UNDEFINED`.
- Способ выпуска в оборот: `FOREIGN` Ввезён в РК, `LOCAL` Производство РК.

## 8. Чего в документе 2.18 нет

- Эмиссии (заказа новых) КМ: это API СУЗ (OMS), документа нет в открытом доступе.
  `/cises/my` в True API возвращает только уже существующие КМ.
- Отчёта о нанесении (в СУЗ это отчёт о нанесении / utilisation).
- Ввода в оборот товаров собственного производства РК (есть только повторный ввод `KZ_INTRODUCE_GOODS`).
- ТГ «Лёгкая промышленность», её кода `pg` и атрибутов карточки (размер, цвет, состав).
- Формата КМ лёгкой промышленности (длина серийного номера, AI 91/92), правил аннулирования
  неиспользованных КМ.
- Формата построчных ошибок проверки документа (есть только `errors`, `errorMessage`, `errorMessages`).
- API регистрации карточки в НКТ.

Всё это в эмуляторе сделано правдоподобно и перечислено в `emulator/README.md`, раздел «Что предположено».
