# Todo List (Go + PostgreSQL + Docker)

REST API для управления задачами. Приложение и PostgreSQL запускаются в отдельных контейнерах.

## Функциональность
- CRUD API:
  - `POST /tasks`
  - `GET /tasks`
  - `GET /tasks/{id}`
  - `PUT /tasks/{id}`
  - `DELETE /tasks/{id}`
- Поля задачи: `title`, `description`, `priority`, `due_date`, `completed`
- Docker Compose для локального запуска
- CI/CD pipeline (GitHub Actions): `build`, `lint`, `test`, `docker_build`, `docker_push`

## Структура проекта
```text
.
|-- .github/workflows/ci.yml
|-- .golangci.yml
|-- app
|   |-- Dockerfile
|   `-- src
|       |-- cmd/server/main.go
|       |-- internal
|       |   |-- config
|       |   |-- httpapi
|       |   |-- storage
|       |   `-- todo
|       |-- go.mod
|       `-- go.sum
|-- db
|   |-- Dockerfile
|   `-- init.sql
|-- docker-compose.yml
`-- README.md
```

## Требования
- Docker + Docker Compose
- Go 1.22+ (для локальных проверок без Docker)
- golangci-lint (для локального `lint`, опционально)

## Локальный запуск в Docker
1. Создайте `.env` на основе примера:
```bash
# Linux/macOS
cp .env.example .env
```
```powershell
# Windows PowerShell
Copy-Item .env.example .env
```

2. Поднимите сервисы:
```bash
docker compose up --build -d
```

3. Приложение доступно на `http://localhost:8080`.

## Остановка
```bash
docker compose down
```

Удалить вместе с volume БД:
```bash
docker compose down -v
```

## Локальные проверки (как в CI)
Все команды запускаются из `app/src`.

```bash
go mod download
go build ./...
golangci-lint run --config ../../.golangci.yml ./...
go test ./... -covermode=atomic -coverprofile=coverage.out
go tool cover -func coverage.out
```

Порог coverage: **50%**. Ниже порога pipeline падает.

## CI/CD (GitHub Actions)
Workflow: `.github/workflows/ci.yml`.

Запуск workflow:
- при `pull_request`
- при `push` в `main`/`master`
- при `push` тега

Jobs:
1. `build` — проверка сборки (`go build ./...`)
2. `lint` — запуск `golangci-lint`
3. `test` — тесты + расчет coverage + проверка порога 50%
4. `docker_build` — сборка Docker-образа с тегом `${branch_or_tag}-${short_sha}`
5. `docker_push` — push в Docker Hub (только для `main`/`master` и тегов)

### Artifacts и coverage
- `app/src/coverage.out`
- `app/src/coverage.txt`
- coverage-отчет публикуется как GitHub Artifact (`coverage-report`)

### GitHub Secrets для Docker Hub
Добавить в `Settings -> Secrets and variables -> Actions`:
- `DOCKERHUB_USERNAME` — логин Docker Hub
- `DOCKERHUB_TOKEN` — токен Docker Hub
- `DOCKERHUB_REPOSITORY` — репозиторий вида `username/todo-app`

Секреты не хардкодятся в репозитории.
