# Appraisal CRM

> Русская версия · [English version](../../README.md)

CRM для компании по оценке имущества (квартиры, дома, земля, автомобили, коммерческая недвижимость).
Оцифровывает полный цикл: клиент подаёт заявку → инспектор выезжает на объект → оценщик проводит оценку → клиент получает отчёт.

Коммерческий проект для реального клиента. Код идёт в продакшен.

> **Новичок в проекте?** Начните с [гайда по онбордингу](../../docs/i18n/ru/onboarding.md).

## Карта репозитория

| Путь | Что там |
|------|---------|
| `services/request-service/` | Первый бизнес-сервис (Go): CRUD заявок + state machine жизненного цикла. **Эталонная реализация для всех будущих сервисов.** |
| `infra/docker-compose.yml` | Локальная инфраструктура: PostgreSQL 17, Redis 7, Keycloak 26 |
| `docs/brd/` | Бизнес-требования (en/ru) |
| `docs/architecture/` | C4-диаграммы в Structurizr DSL (en/ru) |
| `docs/adr/` | Architecture Decision Records (en/ru) |
| `docs/onboarding.md` | Гайд по онбордингу разработчика — начинать отсюда |
| `CLAUDE.md` | Конвенции проекта и жёсткие правила (прочитайте, даже если не пользуетесь Claude) |

## Архитектура в двух словах

Микросервисы на Go, по одной базе PostgreSQL на сервис, взаимодействие только асинхронное через события Kafka (никаких прямых HTTP-вызовов между сервисами), Keycloak для OAuth2/OIDC, четыре React SPA по ролям за API Gateway.

**Реализовано сейчас:** инфраструктурный compose + `request-service` (CRUD, state machine, JWT/RBAC, Swagger, unit-тесты).
**Ещё нет:** API Gateway, inspect-service, review-service (заблокирован — клиент не формализовал формулы оценки), notification-service, фронтенды, интеграция с Kafka.

Целевая картина — в [C4-диаграммах](../../docs/architecture/i18n/ru/README.md), обоснование стека — в [ADR](../../docs/adr/i18n/ru/README.md).

## Быстрый старт

```bash
# 1. Инфраструктура (PostgreSQL :5433, Redis :6380, Keycloak :8180)
docker compose -f infra/docker-compose.yml up -d

# 2. Бутстрап Keycloak — compose поднимает Keycloak ПУСТЫМ, нужна разовая настройка:
#    realm `appraisal`, роли, публичный клиент, тестовые пользователи.
#    См. docs/onboarding.md § «Настройка Keycloak» (5 минут, команды copy-paste).

# 3. Миграции + запуск сервиса
cd services/request-service
make migrate-up
make run          # генерирует Swagger-доки, стартует на :8080
```

Дальше — Swagger UI на http://localhost:8080/swagger/index.html.
Получение токена и вызовы API: см. [README request-service](../../services/request-service/README.md).

## Процесс разработки

- Ветка от `dev`: `feature/<scope>` или `fix/<scope>` (примеры — в истории веток)
- Conventional commits: `feat(requests): ...`, `fix(server): ...`; в заголовке указывайте ключ задачи Jira (`ACRM-...`)
- Задачи ведутся в Jira, проект **ACRM**
- PR в `dev`; CI пока нет — перед пушем запускайте `go build ./... && go vet ./... && go test ./...`
- Перед первым PR прочитайте раздел **Hard rules** в [CLAUDE.md](../../CLAUDE.md) — они не обсуждаются (никаких синхронных вызовов между сервисами, никаких JOIN между базами, никогда не менять применённые миграции и т.д.)

## Жизненный цикл заявки (главное доменное правило)

```
new → in_progress → inspection_scheduled → inspection_completed → appraisal → report_sent → closed
```

Строго линейный, по одному шагу, без движения назад. Контролируется в service-слое; нарушение — HTTP 422.
