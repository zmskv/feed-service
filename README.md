# feed-service

Посты и иерархические комментарии через GraphQL (Query/Mutation/Subscription) — аналог комментариев на Хабре/Reddit. 

Go, gqlgen, gin, sqlx+pgx (Postgres) или in-memory — хранилище выбирается параметром при запуске.

**Посты**
- список постов
- пост и комментарии под ним
- автор поста может запретить комментирование

**Комментарии**
- иерархия без ограничения вложенности
- лимит длины текста (2000 символов)
- пагинация списка комментариев

## Быстрый старт (Docker)

```bash
docker-compose up --build
```

Поднимет Postgres, накатит миграции, запустит приложение на `:8080` со `STORAGE=postgres`.

- Playground: http://localhost:8080/playground
- Эндпоинт: `POST/GET http://localhost:8080/query`
- Healthcheck: http://localhost:8080/ping

## Быстрый старт (локально)

```bash
cp example.env .env
go run ./cmd/feed-service --storage=memory
```

Или через [Taskfile](https://taskfile.dev) (`task run`, `task build`, `task test`). Для `--storage=postgres` нужна поднятая БД: `docker-compose up postgres migrate`.

## Конфигурация

Флаги или переменные окружения (флаг приоритетнее), см. [example.env](example.env).

| Флаг | Env | По умолчанию | Описание |
|---|---|---|---|
| `--storage` | `STORAGE` | `memory` | `memory` \| `postgres` |
| `--addr` | `ADDR` | `:8080` | адрес HTTP |
| `--dsn` | `DSN` | — | полная строка подключения; если задана — переопределяет `--pg-*` |
| `--pg-host` / `--pg-port` | `PGHOST` / `PGPORT` | `localhost` / `5432` | адрес Postgres |
| `--pg-user` / `--pg-password` | `PGUSER` / `PGPASSWORD` | `feed` / `feed` | учётные данные |
| `--pg-database` | `PGDATABASE` | `feed` | имя базы |
| `--pg-sslmode` | `PGSSLMODE` | `disable` | sslmode |

## GraphQL API

Схема: [schema.graphqls](internal/presentation/graphql/schema.graphqls).

```graphql
query {
  posts(first: 10) {
    edges { node { id title commentsDisabled } cursor }
    pageInfo { hasNextPage endCursor }
  }
}

query {
  post(id: "...") {
    title
    comments(first: 10) {
      edges { node { body replies(first: 10) { edges { node { body } } } } }
    }
  }
}

mutation { createPost(input: { authorId: "...", title: "hi", body: "..." }) { id } }
mutation { createComment(input: { postId: "...", parentId: null, authorId: "...", body: "..." }) { id } }
mutation { disableComments(input: { postId: "...", requesterId: "..." }) { commentsDisabled } }

# доставка новых комментариев без повторного запроса (websocket)
subscription { commentAdded(postId: "...") { id body } }
```

## Архитектура

Зависимости идут только внутрь: `presentation → application → domain`, `infrastructure` реализует то, что просит `application`.

- **domain** — правила (пустой заголовок, комментарий длиннее 2000 символов, комментарий на закрытом посте), без внешних зависимостей.
- **application** — сценарии использования (создать пост/комментарий, запретить комментарии) + интерфейсы к хранилищу.
- **infrastructure** — `repository/memory`, `repository/postgres`, `pubsub` (рассылка новых комментариев подписчикам).
- **presentation/graphql** — схема, кодогенерация (`generated/`) + резолверы, dataloader, обработка ошибок, HTTP-роуты.
- **pagination** — курсор постраничности, общий для всех слоёв.
- **di** — di контейнер, нужный для упаковки и удобства добавления новых зависимостей.

```
cmd/feed-service/
  main.go

internal/
  domain/
    post/
      post.go
      errors.go
    comment/
      comment.go
      errors.go
  application/
    post/
      service.go
      errors.go
    comment/
      service.go
      errors.go
  infrastructure/
    repository/
      memory/
        post.go
        comment.go
        cursor.go
        interfaces.go
      postgres/
        post.go
        comment.go
        interfaces.go
    pubsub/
      broadcaster.go
  presentation/
    graphql/
      schema.graphqls
      generated/
      resolver.go
      schema.resolvers.go
      mapper.go
      connection.go
      dataloader.go
      errors.go
      router.go
  pagination/
    cursor.go
  config/
    config.go
  di/
    di.go

migrations/
  0001_init.up.sql
  0001_init.down.sql

logger/
  logger.go
```

## Хранение и пагинация

Таблицы: [migrations/0001_init.up.sql](migrations/0001_init.up.sql).

```sql
posts(id, author_id, title, body, comments_disabled, created_at)
comments(id, post_id, parent_id → comments.id ON DELETE CASCADE, author_id, body VARCHAR(2000), created_at)

CREATE INDEX idx_posts_created_at_id ON posts (created_at DESC, id DESC);
CREATE INDEX idx_comments_post_parent_created ON comments (post_id, parent_id, created_at, id);
```

Комментарий хранит только `parent_id` — ссылку на родителя (пусто — верхний уровень). Дерево нигде не хранится как дерево, оно возникает при выводе по этим ссылкам, поэтому вложенность ничем не ограничена.

Списки отдаются страницами через курсор, не `OFFSET`: вместо "страница 501" — "всё после вот этой записи" (время создания + id, закодированные в одну строку — [cursor.go](internal/pagination/cursor.go)). Быстро на любой странице, не только на первых, и не путается, если между запросами что-то добавили. 

## Проблема N+1

Наивный вывод комментариев для списка из N постов — N отдельных запросов. Решение — [dataloader.go](internal/presentation/graphql/dataloader.go): пока GraphQL параллельно резолвит комментарии для всех постов, id откладываются в очередь, и когда все набрались — уходит один запрос сразу за всех.

Обычный `LIMIT` тут не подходит — он ограничивает результат целиком, а не по каждому посту отдельно. Вместо него — нумерация строк отдельно внутри каждой группы:

```sql
WITH ranked AS (
  SELECT *, ROW_NUMBER() OVER (PARTITION BY post_id ORDER BY created_at, id) AS rn
  FROM comments WHERE post_id = ANY($1) AND parent_id IS NULL
)
SELECT * FROM ranked WHERE rn <= $2
```

Для ответов на комментарии — то же самое, но по `parent_id`, отдельным уровнем. [dataloader_test.go](internal/presentation/graphql/dataloader_test.go) проверяет: 3 поста и 50 постов дают одинаковое число запросов (один).

## Подписки

`pubsub/broadcaster.go` — рассылка внутри процесса (по сути fan-out): у каждого поста свой список подписчиков, новый комментарий уходит только им. GraphQL-подписка — постоянное websocket-соединение вместо запроса-ответа; апгрейд до него браузер делает GET-запросом, поэтому `/query` в `router.go` слушает и `GET`, и `POST`.

## Обработка ошибок

Внутренний код возвращает обычные Go-ошибки, ничего не зная про GraphQL. [errors.go](internal/presentation/graphql/errors.go) — здесь они переводятся в код для клиента:

```go
{domainPost.ErrCommentsDisabled, "COMMENTS_DISABLED"},
{domainComment.ErrBodyTooLong,   "BODY_TOO_LONG"},
{apppost.ErrForbidden,           "FORBIDDEN"},
```

Пост по несуществующему id — не ошибка, а пустой ответ (`null`).

## Одновременная работа

- In-memory хранилище — блокировка, читают все сразу, пишут по одному.
- Postgres — пул из 25 готовых соединений вместо нового на каждый запрос.
- Остановка сервиса ждёт до 10с, пока текущие запросы не доработают (graceful shutdown) ([main.go](cmd/feed-service/main.go)).

## Тестирование

```bash
task test             
task test:coverage   

docker-compose up -d postgres migrate
TEST_DATABASE_URL="postgres://feed:feed@localhost:5432/feed?sslmode=disable" go test ./...
```
