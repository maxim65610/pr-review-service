# PR Reviewer Assignment Service

Микросервис для автоматического назначения ревьюверов на Pull Request'ы.
Работает по HTTP API и соответствует спецификации `openapi.yaml`.


## Возможности

### Команды
- Создать команду с участниками
- Получить команду с пользователями

### Пользователи
- Изменить флаг активности `isActive`
- Получить список PR, где пользователь — ревьювер

### Pull Requests
- Создать PR (автоматическое назначение до 2 ревьюверов из команды автора)
- Merge PR (идемпотентно)
- Переназначить одного ревьювера

###  Доп. задание 
**Эндпоинт статистики назначений ревьюверов**  
`GET /stats/reviewerAssignments`  
Возвращает количество назначений каждого пользователя.


##  Запуск проекта

### 1. Через Docker

```bash
docker compose up --build
```

### 2.Локальный запуск (без Docker)
- Создать БД:
CREATE DATABASE prservice;

- (Опционально) указать DSN:
export DATABASE_DSN="postgres://postgres:postgres@localhost:5432/prservice?sslmode=disable"

- Запустить приложение:
make run

### Примеры API-запросов (cURL)

## Создать команду
curl -X POST http://localhost:8080/team/add \
-H "Content-Type: application/json" \
-d '{
"team_name":"backend",
"members":[
{"user_id":"u1","username":"Alice","is_active":true},
{"user_id":"u2","username":"Bob","is_active":true},
{"user_id":"u3","username":"Eve","is_active":true}
]
}'

## Создать PR
curl -X POST http://localhost:8080/pullRequest/create \
-H "Content-Type: application/json" \
-d '{"pull_request_id":"pr-1","pull_request_name":"Add feature","author_id":"u1"}'

## Merge PR
curl -X POST http://localhost:8080/pullRequest/merge \
-H "Content-Type: application/json" \
-d '{"pull_request_id":"pr-1"}'

## Переназначить ревьювера
curl -X POST http://localhost:8080/pullRequest/reassign \
-H "Content-Type: application/json" \
-d '{"pull_request_id":"pr-1","old_user_id":"u2"}'

##  Получить PR где пользователь — ревьювер
curl "http://localhost:8080/users/getReview?user_id=u2"


# ДОПЫ:

## 1. Статистика ревьюверов

Сервис предоставляет endpoint для получения количества назначений на ревью по каждому пользователю.

### Запрос:

```
GET /stats/reviewerAssignments
```

### Пример ответа:

```json
{
  "stats": [
    {"user_id": "u3", "assignments": 5},
    {"user_id": "u2", "assignments": 2},
    {"user_id": "u1", "assignments": 1}
  ]
}
```

## 2. Добавлен линтер, файл .golangchi.yml

