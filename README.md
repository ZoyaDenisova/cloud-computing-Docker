# Практическая работа по Kafka

Тематика: **посты и комментарии в соцсети**.

Система на микросервисной архитектуре с контейнеризацией:
- `api-service`
- `data-service`
- `db` (PostgreSQL)
- `kafka`

Все компоненты запускаются в Docker-сети через `docker compose`.

## Компоненты

### API Service
HTTP API для доступа извне Docker-сети:
- `POST /data` — добавить порцию данных (отправка в Kafka)
- `GET /search?q=...` — поиск через обращение к `data-service`
- `GET /reports?type=...` — отчёты через обращение к `data-service`

### Data Service
- Читает сообщения из Kafka и сохраняет в PostgreSQL
- HTTP API:
  - `GET /search?q=...` — поиск по БД
  - `GET /reports?type=...` — отчёты по БД

### Database
Схема включает 2 связанные таблицы:
- `posts`
- `comments` (`comments.post_id` -> `posts.id`)

### Kafka
Брокер сообщений для передачи данных от `api-service` в `data-service`.

## Формат входных данных (`POST /data`)

Создание поста:
```json
{
  "type": "post",
  "post": {
    "author": "Ivan",
    "title": "Kafka basics",
    "body": "My first post"
  }
}
```

Создание комментария:
```json
{
  "type": "comment",
  "comment": {
    "post_id": 1,
    "author": "Olga",
    "text": "Nice post"
  }
}
```

## Отчёты (`GET /reports?type=...`)
Реализовано 3 разных отчёта:
- `top_posts_by_comments` — топ-10 постов по числу комментариев
- `posts_by_day` — количество постов по дням
- `comments_by_day` — количество комментариев по дням

## Обработка ошибок
- `400 Bad Request`:
  - неверный `type` (должен быть `post` или `comment`)
  - отсутствуют обязательные поля
  - невалидный JSON
- `404 Not Found`:
  - при добавлении комментария, если `post_id` не существует
- `502 Bad Gateway`:
  - недоступен `data-service` при валидации `post_id`
  - недоступен Kafka при отправке данных

## Запуск
1. Подготовить `.env`:
```powershell
Copy-Item .env.example .env
```

2. Запустить:
```powershell
docker compose up --build -d
```

3. API Service доступен на:
- `http://localhost:8080`

## Примеры запросов

Создать пост (`POST /data`):
```powershell
Invoke-RestMethod -Method Post -Uri "http://localhost:8080/data" -ContentType "application/json" -Body '{"type":"post","post":{"author":"Ivan","title":"Kafka","body":"Hello"}}'
```

Создать комментарий (`POST /data`):
```powershell
Invoke-RestMethod -Method Post -Uri "http://localhost:8080/data" -ContentType "application/json" -Body '{"type":"comment","comment":{"post_id":1,"author":"Olga","text":"Nice post"}}'
```

Поиск по строке (`GET /search?q=...`):
```powershell
Invoke-RestMethod -Uri "http://localhost:8080/search?q=Kafka"
```

Поиск без фильтра (`GET /search`):
```powershell
Invoke-RestMethod -Uri "http://localhost:8080/search"
```

Отчёт: топ постов по комментариям (`GET /reports?type=top_posts_by_comments`):
```powershell
Invoke-RestMethod -Uri "http://localhost:8080/reports?type=top_posts_by_comments"
```

Отчёт: количество постов по дням (`GET /reports?type=posts_by_day`):
```powershell
Invoke-RestMethod -Uri "http://localhost:8080/reports?type=posts_by_day"
```

Отчёт: количество комментариев по дням (`GET /reports?type=comments_by_day`):
```powershell
Invoke-RestMethod -Uri "http://localhost:8080/reports?type=comments_by_day"
```

## Остановка
```powershell
docker compose down
```

С удалением volume БД:
```powershell
docker compose down -v
```
