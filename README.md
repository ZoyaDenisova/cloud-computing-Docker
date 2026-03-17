# Todo List (Go + PostgreSQL + Docker)

Практическая работа: веб-приложение и база данных работают в отдельных Docker-контейнерах.

## Что реализовано
- CRUD API для задач через HTTP:
  - `POST /tasks` - добавить задачу
  - `GET /tasks` - получить список задач
  - `GET /tasks/{id}` - получить задачу по ID
  - `PUT /tasks/{id}` - изменить задачу
  - `DELETE /tasks/{id}` - удалить задачу
- Поля задач: приоритет, срок выполнения, статус выполнения
- Отдельные контейнеры для `app` и `db`
- Docker-сеть (`todo_network`)
- У БД нет `port-forwarding` наружу
- Для БД настроен `volume`
- Пароли передаются только через переменные окружения

## Структура проекта
```
├── app
│   ├── Dockerfile
│   └── src
│       ├── cmd
│       │   └── server
│       │       └── main.go
│       ├── internal
│       │   ├── config
│       │   │   └── config.go
│       │   ├── httpapi
│       │   │   ├── handler.go
│       │   │   └── middleware.go
│       │   ├── storage
│       │   │   └── postgres.go
│       │   └── todo
│       │       ├── errors.go
│       │       ├── model.go
│       │       ├── postgres_repository.go
│       │       ├── repository.go
│       │       └── service.go
│       ├── go.mod
│       └── go.sum
├── db
│   ├── Dockerfile
│   └── init.sql
├── .env.example
├── .gitignore
├── docker-compose.yml
└── README.md
```

## Запуск
1. Создать `.env` на основе примера:
```bash
# Linux/macOS
cp .env.example .env
```
```bash
# Windows (PowerShell)
copy .env.example .env
```

2. Запустить проект:
```bash
docker compose up --build -d
```

## Примеры запросов
### Создать задачу
```bash
curl -X POST http://localhost:8080/tasks \
  -H "Content-Type: application/json" \
  -d '{
    "title":"Сделать лабораторную",
    "description":"Docker + Go",
    "priority":3,
    "due_date":"2026-03-20",
    "completed":false
  }'
```

### Получить все задачи
```bash
curl http://localhost:8080/tasks
```

### Получить задачу по ID
```bash
curl http://localhost:8080/tasks/1
```

### Обновить задачу
```bash
curl -X PUT http://localhost:8080/tasks/1 \
  -H "Content-Type: application/json" \
  -d '{
    "title":"Сделать лабораторную (обновлено)",
    "description":"Проверить чеклист",
    "priority":4,
    "due_date":"2026-03-22",
    "completed":true
  }'
```

### Удалить задачу
```bash
curl -X DELETE http://localhost:8080/tasks/1
```

## Поля задачи
- `title` (обязательно)
- `description` (строка)
- `priority` (1..5)
- `due_date` (формат `YYYY-MM-DD`, можно `null`)
- `completed` (`true/false`)

## Остановка
```bash
docker compose down
```

Чтобы удалить и volume БД:
```bash
docker compose down -v
```
