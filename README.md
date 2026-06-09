# Go Bank API — банковский REST API на Go

Проект реализует REST API банковского сервиса по ТЗ: пользователи, JWT, счета, карты, переводы, кредиты с аннуитетным графиком, аналитика, SMTP-уведомления, интеграция с SOAP API ЦБ РФ и защита карточных данных через PostgreSQL `pgcrypto`, HMAC и bcrypt.

## Стек

- Go 1.23
- PostgreSQL 17 + `pgcrypto`
- `gorilla/mux` — маршрутизация
- `lib/pq` — PostgreSQL driver
- `golang-jwt/jwt/v5` — JWT на 24 часа
- `logrus` — логирование
- `bcrypt` — пароли и CVV
- `HMAC-SHA256` — контроль целостности карточных данных
- `pgp_sym_encrypt/pgp_sym_decrypt` из `pgcrypto` — PGP-шифрование номера и срока карты
- `beevik/etree` — парсинг XML ответа ЦБ РФ
- `go-mail/mail/v2` — SMTP

## Архитектура

```text
HTTP -> router -> middleware -> handler -> service -> repository -> PostgreSQL
                                      |          |
                                      |          + SMTP, CBR SOAP, scheduler
                                      + validation, JSON, HTTP statuses
```

Основные каталоги:

```text
cmd/server              точка входа
internal/config         конфигурация из env
internal/db             подключение и встроенные миграции
internal/models         модели, DTO, базовая валидация
internal/repository     SQL, транзакции, параметризованные запросы
internal/service        бизнес-логика
internal/handler        HTTP handlers
internal/middleware     JWT middleware
internal/integrations   ЦБ РФ SOAP и SMTP
internal/scheduler      автосписание платежей
internal/utils          деньги, карты, HMAC, Луна
pkg/response            JSON responses
```

## Быстрый старт

```bash
cp .env.example .env

docker compose up -d --build

curl http://localhost:8080/health

Если выдаст ошибку:
curl http://localhost:8080/health
curl: (56) Recv failure: Connection reset by peer

Перезапустите контейнеры и проверьте не поднято ли у вас что-то локально на портах базы данных (5432)

```

Mailhog для просмотра писем: `http://localhost:8025`.

## Переменные окружения

| Переменная | Назначение |
|---|---|
| `APP_PORT` | порт API |
| `DB_HOST`, `DB_PORT`, `DB_USER`, `DB_PASSWORD`, `DB_NAME`, `DB_SSLMODE` | PostgreSQL |
| `JWT_SECRET` | секрет подписи JWT |
| `HMAC_SECRET` | секрет HMAC-SHA256 |
| `PGP_PASSPHRASE` | парольная фраза для PGP-шифрования карт |
| `CREDIT_MARGIN` | маржа банка к ставке ЦБ, процентные пункты |
| `SCHEDULER_INTERVAL` | период автосписания, например `12h` или `30s` |
| `SMTP_HOST`, `SMTP_PORT`, `SMTP_USER`, `SMTP_PASSWORD`, `SMTP_FROM` | SMTP |
| `LOG_LEVEL` | `debug`, `info`, `warn`, `error` |

## Эндпоинты

Публичные:

| Метод | Путь | Описание |
|---|---|---|
| GET | `/health` | проверка сервиса |
| POST | `/register` | регистрация |
| POST | `/login` | JWT-аутентификация |

Защищенные, нужен заголовок `Authorization: Bearer <token>`:

| Метод | Путь | Описание |
|---|---|---|
| POST | `/accounts` | создать счет |
| GET | `/accounts` | список счетов пользователя |
| GET | `/accounts/{id}` | получить счет |
| POST | `/accounts/{id}/deposit` | пополнить свой счет |
| POST | `/accounts/{id}/withdraw` | списать со своего счета |
| POST | `/transfer` | перевод между счетами |
| GET | `/accounts/{id}/transactions` | история операций |
| POST | `/cards` | выпустить карту |
| GET | `/accounts/{id}/cards` | список карт счета, маска |
| GET | `/cards/{id}` | карта с расшифрованным номером владельцу |
| POST | `/cards/pay` | оплата картой с проверкой CVV |
| POST | `/credits` | оформить кредит |
| GET | `/credits/{creditId}/schedule` | график платежей |
| GET | `/analytics` | месячная аналитика и кредитная нагрузка |
| GET | `/accounts/{id}/predict?days=N` | прогноз баланса, максимум 365 дней |

## Примеры запросов

```bash
B=http://localhost:8080

curl -s -X POST $B/register \
  -H 'Content-Type: application/json' \
  -d '{"email":"ivan@example.com","username":"ivan","password":"secret123"}'

TOKEN=$(curl -s -X POST $B/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"ivan@example.com","password":"secret123"}' | jq -r .token)

curl -s -X POST $B/accounts -H "Authorization: Bearer $TOKEN"

curl -s -X POST $B/accounts/1/deposit \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"amount":10000}'

curl -s -X POST $B/cards \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"account_id":1}'

curl -s -X POST $B/credits \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"account_id":1,"amount":50000,"months":12}'
```

## Безопасность

- Пароли пользователей хешируются через bcrypt.
- CVV карты хешируется через bcrypt и никогда не расшифровывается.
- Номер и срок карты хранятся в `BYTEA`, зашифрованы `pgp_sym_encrypt` из расширения `pgcrypto`.
- HMAC-SHA256 хранится отдельно для проверки целостности номера и срока.
- Все SQL-запросы параметризованы.
- Операции перевода, выдачи кредита и списания выполняются в транзакциях.
- Middleware проверяет JWT, блокирует запросы без токена и кладет `userID` в контекст.
- Сервисный слой проверяет владение счетами и картами.

## Что покрывает критерии оценивания

- Модели: структуры, JSON-теги, DTO, базовая валидация email/username/password.
- Репозитории: инкапсуляция SQL, параметризованные запросы, обработка `not found`, `unique violation`, транзакции.
- Сервисы: регистрация, логин, счета, пополнение, списание, переводы, карты по Луну, кредиты, SMTP, ЦБ РФ SOAP, шедулер, logrus.
- Handlers: JSON, HTTP-статусы, валидация входа, все основные эндпоинты, проверка прав через сервисы.
- Middleware: JWT, блокировка, `userID` в контексте.
- БД: `users`, `accounts`, `cards`, `transactions`, `credits`, `payment_schedules`.

Перед запуском нужен PostgreSQL 17 и переменные окружения из `.env.example`.
