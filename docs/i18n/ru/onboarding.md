# Гайд по онбордингу разработчика

> Русская версия · [English version](../../onboarding.md)

Для разработчика, который приходит в проект: Go знает, этот код видит впервые.
Прочитайте один раз сверху вниз; дальше будете возвращаться в основном за рецептами и граблями.

## Содержание

1. [Общая картина](#1-общая-картина)
2. [Настройка локального окружения](#2-настройка-локального-окружения)
3. [Настройка Keycloak](#3-настройка-keycloak)
4. [Экскурсия по коду: request-service](#4-экскурсия-по-коду-request-service)
5. [Паттерны, которым нужно следовать](#5-паттерны-которым-нужно-следовать)
6. [Конвенции HTTP-кодов](#6-конвенции-http-кодов)
7. [Рецепты](#7-рецепты)
8. [Грабли](#8-грабли)
9. [Процесс и workflow](#9-процесс-и-workflow)

---

## 1. Общая картина

Система автоматизирует компанию по оценке имущества. Бизнес-флоу:

1. **Клиент** подаёт заявку на оценку (квартира / дом / земля / коммерческая / авто)
2. **Оценщик** принимает её и назначает **инспектора**
3. Инспектор выезжает на объект, загружает фото и данные
4. Оценщик формирует отчёт; клиент его скачивает

Принципы архитектуры («почему» — в [ADR](../../adr/i18n/ru/README.md)):

- **Микросервисы на Go** — chi, pgx, golang-migrate; никаких фреймворков сверх этого
- **База на сервис** — каждый сервис владеет своей PostgreSQL; JOIN между базами запрещены
- **События только через Kafka** — бизнес-сервисы никогда не вызывают друг друга синхронно
- **Keycloak** — OAuth2/OIDC; сервисы проверяют JWT через JWKS-эндпоинт Keycloak, роли берутся из claims токена
- **Четыре React SPA** (client / appraiser / inspector / admin) за API Gateway

Что есть против целевой картины ([C4-диаграммы](../../architecture/i18n/ru/README.md)):

| Компонент | Статус |
|-----------|--------|
| `request-service` | ✅ рабочий MVP — **эталон для всего остального** |
| Инфраструктурный compose (PostgreSQL, Redis, Keycloak) | ✅ |
| Kafka + события | ❌ пока нет (следующая веха) |
| API Gateway | ❌ |
| inspect-service, notification-service | ❌ |
| review-service | ❌ **заблокирован** — клиент не формализовал формулы оценки; не начинать |
| Фронтенды (4 SPA) | ❌ |

Главное доменное правило — жизненный цикл заявки строго линеен и контролируется в коде:

```
new → in_progress → inspection_scheduled → inspection_completed → appraisal → report_sent → closed
```

По одному шагу, без пропусков, без движения назад. Всегда.

## 2. Настройка локального окружения

Требования: Go 1.22+, Docker, [migrate CLI](https://github.com/golang-migrate/migrate), [swag CLI](https://github.com/swaggo/swag).

```bash
# 1. Инфраструктура
docker compose -f infra/docker-compose.yml up -d
# PostgreSQL → localhost:5433 (user/pass: appraisal/appraisal)
# Redis      → localhost:6380
# Keycloak   → localhost:8180 (admin/admin)

# 2. Бутстрап Keycloak — раздел 3, обязателен один раз на свежий volume

# 3. Миграции
cd services/request-service
make migrate-up

# 4. Запуск
make run     # swag generate + go run, слушает :8080
```

Проверка: `curl localhost:8080/health` → `{"status":"ok"}`, Swagger UI на http://localhost:8080/swagger/index.html.

## 3. Настройка Keycloak

**Compose поднимает Keycloak с пустой базой** — realm `appraisal`, роли и пользователей нужно создать один раз (дальше они живут в volume `postgres_data`). Copy-paste бутстрап через `kcadm` внутри контейнера:

```bash
KC="docker exec appraisal-keycloak /opt/keycloak/bin/kcadm.sh"

# логин
$KC config credentials --server http://localhost:8080 --realm master --user admin --password admin

# realm + роли
$KC create realms -s realm=appraisal -s enabled=true
for r in client appraiser inspector admin; do $KC create roles -r appraisal -s name=$r; done

# публичный клиент с password grant (для локальных токенов / Postman / фронтендов)
$KC create clients -r appraisal -s clientId=appraisal-frontend \
  -s publicClient=true -s directAccessGrantsEnabled=true -s enabled=true

# тестовые пользователи (email/firstName/lastName обязательны, иначе запрос токена
# упадёт с "Account is not fully set up")
for u in test-client test-appraiser; do
  $KC create users -r appraisal -s username=$u -s enabled=true \
    -s email=$u@example.com -s firstName=Test -s lastName=User -s emailVerified=true
  $KC set-password -r appraisal --username $u --new-password test123
done
$KC add-roles -r appraisal --uusername test-client --rolename client
$KC add-roles -r appraisal --uusername test-appraiser --rolename appraiser
```

Получить токен (живёт 5 минут):

```bash
TOKEN=$(curl -s -X POST http://localhost:8180/realms/appraisal/protocol/openid-connect/token \
  -d "grant_type=password&client_id=appraisal-frontend&username=test-client&password=test123" \
  | jq -r .access_token)

curl -s -X POST localhost:8080/requests \
  -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
  -d '{"email":"test@example.com","phone_number":"+79161234567","object_type":"apartment","address":"Lenina 1"}'
```

## 4. Экскурсия по коду: request-service

Каждый будущий сервис копирует эту структуру — разберитесь один раз.

```
services/request-service/
  cmd/server/main.go       ← точка входа: конфиг, pgx pool, JWKS, DI, http.Server
  config/config.go         ← конфиг из ENV, только os.Getenv (DATABASE_URL обязателен)
  internal/
    domain/request.go      ← сущность Request, enum'ы Status и ObjectType
    repository/
      repository.go        ← интерфейс RequestRepository + sentinel-ошибки (ErrNotFound, ErrConflict)
      postgres.go          ← pgx-реализация, сырой SQL
    service/
      service.go           ← интерфейс RequestService, DTO CreateInput/UpdateInput
      request.go           ← бизнес-логика: state machine, optimistic locking, доменные ошибки
      request_test.go      ← unit-тесты с mock-репозиторием (testify/mock)
    handler/
      route.go             ← chi-роутер: цепочка middleware + RBAC по роутам
      request.go           ← HTTP-хендлеры: parse → validate → service → map error → respond
      dto.go               ← DTO запросов с тегами `validate:`
      validate.go          ← экземпляр go-playground/validator
      health.go, response.go
    middleware/auth.go     ← JWT через Keycloak JWKS (MicahParks/keyfunc), роли из realm_access.roles
    httputil/response.go   ← общие RespondJSON / RespondError
  migrations/              ← SQL для golang-migrate, сквозная нумерация, всегда пары up+down
  docs/                    ← генерируется swag; В GITIGNORE — пересоздавать через `make generate`
```

**Путь запроса через слои** на примере `PATCH /requests/{id}/status`:

1. `middleware.Auth` проверяет JWT по JWKS Keycloak, кладёт ID пользователя и роли в контекст
2. `middleware.RequireRoles("appraiser")` охраняет роут (иначе 403)
3. Хендлер парсит UUID, декодирует тело (лимит 1 МБ), гоняет validator-теги → 400 при ошибке
4. Сервис читает заявку, сверяется с картой `allowedTransitions` → `ErrInvalidStatusTransition`, если шаг недопустим
5. Репозиторий выполняет **compare-and-set** UPDATE: `... SET status=$new WHERE id=$id AND status=$old`. Ноль затронутых строк + строка существует ⇒ нас опередили ⇒ `ErrConflict`
6. Хендлер маппит доменную ошибку в HTTP: 404 / 409 / 422 / 500 — **статус-коды выбираются только в handler-слое**

**Optimistic locking** (колонки `version` нет):
- `ChangeStatus` — CAS по текущему значению `status`
- `Update` (PATCH полей) — CAS по `updated_at`; намеренно **не** пишет `status`, поэтому устаревший PATCH полей никогда не откатит конкурентную смену статуса
- Оба случая наружу — `ErrConflict` → HTTP 409; клиент перечитывает и повторяет

**Снапшот RBAC** (из `route.go` — источник истины):

| Эндпоинт | client | appraiser | admin | inspector |
|----------|--------|-----------|-------|-----------|
| POST /requests | ✅ | — | — | — |
| GET /requests (список) | только свои | все + пагинация | все | — |
| GET /requests/{id} | только свою | ✅ | ✅ | — |
| PATCH /requests/{id} | — | ✅ | ✅ | — |
| PATCH /requests/{id}/status | — | ✅ | — | — |

Admin намеренно не может создавать заявки и двигать жизненный цикл; inspector получит доступ вместе с inspect-service. Если это удивляет — это продуктовое решение, а не недосмотр.

## 5. Паттерны, которым нужно следовать

Источники: [CLAUDE.md](../../../CLAUDE.md) (жёсткие правила) и `.claude/rules/go-services.md`:

- **Никаких фреймворков** сверх chi, pgx, playground/validator. Конфиг только через `os.Getenv` — без viper/cobra.
- **Доменные ошибки** — типизированные sentinel'ы, маппятся в HTTP-коды **только в handler-слое**. В новых сервисах кладите их в `internal/domain/errors.go` (в request-service они в `service/request.go` — legacy, не копируйте).
- **Миграции**: `00000N_description.up.sql` + `.down.sql`, сквозная нумерация, всегда обе стороны. **Никогда не менять применённую миграцию** — только новые.
- **События**: каждое событие Kafka — отдельный тип в `domain/events.go`; публикация **после** коммита транзакции; консьюмеры идемпотентны (at-least-once).
- **Никаких синхронных вызовов между бизнес-сервисами**, никаких JOIN между базами.
- **Swagger-аннотации** на каждом публичном эндпоинте (`make generate` перед коммитом правок хендлеров).
- **Unit-тесты** бизнес-логики — в `service/` с mock-репозиторием; переходы state machine — табличные тесты, обязательно.

## 6. Конвенции HTTP-кодов

| Код | Что означает здесь | Где рождается |
|-----|--------------------|---------------|
| 400 | Битый JSON, провал валидации, кривой UUID в пути | handler |
| 401 | Нет/невалидный/протухший JWT | middleware |
| 403 | Роль не допущена, или клиент лезет в чужую заявку | middleware / handler |
| 404 | Заявка не найдена (`ErrNotFound`) | service → handler |
| 409 | Конфликт optimistic locking (`ErrConflict`) — повторить | service → handler |
| 422 | Недопустимый переход state machine (`ErrInvalidStatusTransition`) | service → handler |
| 500 | Всё неожиданное; наружу общий текст, детали только в логах | handler |

## 7. Рецепты

**Добавить эндпоинт:** DTO с validate-тегами в `dto.go` → метод хендлера со Swagger-аннотациями → роут + `RequireRoles` в `route.go` → метод в интерфейсе сервиса + реализация → репозиторий при необходимости → unit-тесты сервисной логики → `make generate`.

**Добавить миграцию:** следующий номер по порядку, оба файла `.up.sql` и `.down.sql`, только аддитивно. Применить `make migrate-up`, проверить откат `make migrate-down`.

**Добавить новый сервис:** скопировать структуру request-service (см. выше), путь модуля `github.com/Meidorislav/appraisal-crm/services/<name>-service`. Доменные ошибки — в `domain/errors.go`. Базу добавить в `infra/postgres/init/01-create-databases.sql` (сработает только на свежем volume), конфиг — через ENV.

**Прогнать всё перед пушем:**
```bash
cd services/request-service
make generate && go build ./... && go vet ./... && go test ./...
```

## 8. Грабли

- **`go build ./...` падает на свежем клоне** с `no required module provides package .../docs` — Swagger-пакет `docs/` в gitignore. Сначала `make generate` (или `make build`, он сделает это сам).
- **Keycloak после `docker compose up` пустой** — ни realm, ни пользователей. Лечится разделом 3; состояние живёт в volume `postgres_data`, пока не сделаете `docker compose down -v`.
- **Запрос токена падает с «Account is not fully set up»** — у пользователя Keycloak не заполнены email/имя/фамилия или висят required actions. Бутстрап из раздела 3 их проставляет.
- **Токены живут 5 минут** — внезапный 401 в Postman обычно значит «возьми новый токен», а не «сломалась авторизация».
- **Нестандартные порты**, чтобы не конфликтовать с локальными сервисами: PostgreSQL **5433**, Redis **6380**, Keycloak **8180**.
- **Логи пока в смешанном формате**: текстовый request-лог chi + JSON от slog. Известная проблема, не пугайтесь.
- **`DATABASE_URL` обязателен** — без него сервис сразу выходит. Остальные ENV имеют dev-дефолты (`config/config.go`).
- **409 на PATCH** — это не баг в вашем коде, это optimistic locking просит перечитать и повторить.

## 9. Процесс и workflow

- Задачи в Jira, проект **ACRM**. Берёте карточку — переводите в In Progress.
- Ветка от `dev`: `feature/<scope>` или `fix/<scope>`.
- Conventional commits с ключом Jira: `fix(requests): ... (ACRM-84)`.
- PR в `dev`. CI пока нет — ворота качества: команда перед пушем из раздела 7.
- Бизнес-доки: [BRD](../../brd/i18n/ru/README.md). Архитектура: [C4](../../architecture/i18n/ru/README.md), [ADR](../../adr/i18n/ru/README.md). Конвенции: [CLAUDE.md](../../../CLAUDE.md).
- review-service **заблокирован** клиентом (нет формализованных формул оценки) — не берите его в работу, что бы ни лежало в бэклоге.
