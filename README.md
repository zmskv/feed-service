# feed-service

Система постов и комментариев с API на GraphQL — посты, вложенные без ограничения глубины ответы на комментарии, постраничная подгрузка списков, уведомления о новых комментариях в реальном времени через подписки, без повторных запросов от клиента.

![Go](https://img.shields.io/badge/Go-1.25-00ADD8?logo=go&logoColor=white)
![GraphQL](https://img.shields.io/badge/GraphQL-gqlgen-E10098?logo=graphql&logoColor=white)
![Postgres](https://img.shields.io/badge/Postgres-16-4169E1?logo=postgresql&logoColor=white)
![Docker](https://img.shields.io/badge/Docker-compose-2496ED?logo=docker&logoColor=white)
![License](https://img.shields.io/badge/license-MIT-blue)

## Возможности

**Посты**
- список постов, постранично
- пост и комментарии под ним одним запросом (вложенность любая [GraphQL API](#graphql-api))
- автор поста может запретить комментирование — после этого `createComment` на этот пост возвращает ошибку

**Комментарии**
- иерархия без ограничения вложенности — ответ на ответ на ответ и так далее
- лимит длины текста 2000 символов, считается по символам (кириллица не в два раза строже латиницы)
- пагинация списка комментариев курсором, не номером страницы ([ниже](#хранение-и-пагинация))

## Стек

- Go — сам сервис
- GraphQL — [gqlgen](https://github.com/99designs/gqlgen), кодогенерация из схемы вместо ручного парсинга запросов
- HTTP — [gin](https://github.com/gin-gonic/gin), роутинг поверх стандартного `net/http`
- Postgres — [sqlx](https://github.com/jmoiron/sqlx) + [pgx](https://github.com/jackc/pgx) как драйвер; либо in-memory-хранилище вместо базы — выбор параметром при запуске, без пересборки
- Docker / docker-compose — образ приложения + Postgres + контейнер с миграциями

## Быстрый старт (Docker)

```bash
docker-compose up --build
```

Три контейнера: `postgres` (поднимается первым, ждём healthcheck), `migrate` (одноразовый — накатывает [migrations/](migrations) и завершается), `app` (стартует только после успешной миграции). Приложение слушает `:8080` со `STORAGE=postgres`.

| | |
|---|---|
| Playground | http://localhost:8080/playground |
| Эндпоинт | `POST/GET http://localhost:8080/query` (GET — для апгрейда до websocket, подписки) |
| Healthcheck | http://localhost:8080/ping |

### Быстрый старт (локально)

```bash
cp example.env .env
go run ./cmd/feed-service --storage=memory
```

Без Docker и без базы — с `--storage=memory` все данные живут в оперативной памяти процесса и пропадают при перезапуске, зато поднимается мгновенно.

Или через [Taskfile](https://taskfile.dev) (`task run`, `task build`, `task test`). Для `--storage=postgres` нужна поднятая БД: `docker-compose up postgres migrate` (поднимет только базу и накатит миграции, без запуска самого приложения в контейнере).

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

`--dsn`/`DSN` нужен редко — покрывает случаи, которые не выразить через отдельные `--pg-*` (нестандартные параметры подключения, unix-сокет и т.п.). В `docker-compose.yml` `.env` из корня проекта подхватывается автоматически (стандартное поведение Compose), DSN контейнера `app` собирается из тех же `PG*`-переменных, что и в `example.env`, но с хостом `postgres` (имя сервиса в сети контейнеров, не `localhost`).

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

Списки (`posts`, `comments`, `replies`) отдаются в виде `edges { node, cursor }` + `pageInfo { hasNextPage, endCursor }` — общепринятый в GraphQL формат для курсорной пагинации (Relay-style connections). `createPost`/`createComment`/`disableComments` принимают один `input`- объект, так как новые поля добавляются без изменения сигнатуры мутации.

## Архитектура

Зависимости идут только внутрь: `presentation → application → domain`, `infrastructure` реализует то, что просит `application`.

| Слой | Отвечает за |
|---|---|
| `domain` | правила (пустой заголовок, комментарий длиннее 2000 символов, комментарий на закрытом посте), без внешних зависимостей |
| `application` | сценарии использования (создать пост/комментарий, запретить комментарии) + интерфейсы к хранилищу |
| `infrastructure` | `repository/memory`, `repository/postgres`, `pubsub` (рассылка новых комментариев подписчикам) |
| `presentation/graphql` | схема, кодогенерация (`generated/`), резолверы, dataloader, обработка ошибок, HTTP-роуты |
| `pagination` | курсор постраничности, общий для всех слоёв |
| `di` | собирает зависимости и переключает хранилище по параметру запуска |

Интерфейсы к хранилищу объявлены не рядом с его реализацией, а там, где их используют. Например, комментариям для проверки поста нужен только метод "найти пост по id" — это отдельный, урезанный интерфейс в `application/comment`, хотя реализует его тот же код хранилища, что и более широкий интерфейс для самих постов в `application/post`. Так каждый потребитель видит только то, что ему реально нужно.

<details>
<summary>Дерево проекта</summary>

```
cmd/feed-service/
  main.go

internal/
  domain/
    post/{post,errors}.go
    comment/{comment,errors}.go
  application/
    post/{service,errors}.go
    comment/{service,errors}.go
  infrastructure/
    repository/
      memory/{post,comment,cursor,interfaces}.go
      postgres/{post,comment,interfaces}.go
    pubsub/broadcaster.go
  presentation/graphql/
    schema.graphqls
    generated/
    resolver.go
    schema.resolvers.go
    mapper.go
    connection.go
    dataloader.go
    errors.go
    router.go
  pagination/cursor.go
  config/config.go
  di/di.go

migrations/
  0001_init.up.sql
  0001_init.down.sql

logger/logger.go
```

</details>

## Хранение и пагинация

Таблицы: [migrations/0001_init.up.sql](migrations/0001_init.up.sql).

```sql
posts(id, author_id, title, body, comments_disabled, created_at)
comments(id, post_id, parent_id → comments.id ON DELETE CASCADE, author_id, body VARCHAR(2000), created_at)

CREATE INDEX idx_posts_created_at_id ON posts (created_at DESC, id DESC);
CREATE INDEX idx_comments_post_parent_created ON comments (post_id, parent_id, created_at, id);
```

Комментарий хранит только `parent_id` — ссылку на родителя (пусто — верхний уровень). Дерево нигде не хранится как дерево, оно возникает при выводе по этим ссылкам: чтобы получить ответы на конкретный комментарий, просто ищутся строки с `parent_id`, равным его id. Вложенность ничем не ограничена — глубина нигде не записана, ответ на ответ ничем не отличается от обычного комментария.

Списки отдаются страницами через курсор, а не через `OFFSET`. `OFFSET 5000` заставляет базу прочитать и отбросить 5000 строк перед тем, как отдать нужные — с ростом номера страницы становится всё медленнее. Курсор — это время создания записи плюс её id, закодированные в одну строку ([cursor.go](internal/pagination/cursor.go)); id нужен на случай, если у двух записей время совпало до микросекунды. По курсору база сразу находит нужное место через индекс и продолжает оттуда — что первая страница, что тысячная, одинаково быстро, и порядок не сбивается, если между запросами что то добавили или удалили.

## Проблема N+1

Наивный вывод комментариев для списка из N постов — N отдельных запросов, один на каждый пост. Решение — [dataloader.go](internal/presentation/graphql/dataloader.go): GraphQL резолвит комментарии для всех N постов параллельно (в отдельных горутинах), и вместо похода в базу каждый такой вызов откладывает id поста в общую очередь и ждёт. Когда все запросы, пришедшие почти одновременно, накопились — уходит один запрос сразу за всех, и результат раздаётся каждому ожидающему по отдельности.

Обычный `LIMIT` тут не подходит — он ограничивает результат целиком, а не по каждому посту отдельно. Вместо него — нумерация строк отдельно внутри каждой группы:

```sql
WITH ranked AS (
  SELECT *, ROW_NUMBER() OVER (PARTITION BY post_id ORDER BY created_at, id) AS rn
  FROM comments WHERE post_id = ANY($1) AND parent_id IS NULL
)
SELECT * FROM ranked WHERE rn <= $2
```

## Подписки

`pubsub/broadcaster.go` — рассылка внутри процесса: карта "id поста → список подписавшихся каналов" под мьютексом. При создании комментария `comment.Service.Create` после сохранения зовёт `Publish`, тот рассылает новый комментарий только каналам подписчиков этого конкретного поста, не всем подряд. Подписка на GraphQL-стороне — постоянное websocket-соединение вместо разового запроса-ответа: клиент один раз подключается и дальше просто получает сообщения по мере появления. Апгрейд до вебсокета браузер делает GET-запросом, поэтому `/query` в `router.go` слушает и `GET`, и `POST` на одном и том же пути.

Сейчас реализация представляет из себя fan-out, который рассылает уведомления о новом коментарии подписчикам этого поста.

## Обработка ошибок

Внутренний код (`domain`/`application`) возвращает обычные Go-ошибки (`errors.New`), ничего не зная про GraphQL — например, "комментарии на этом посте запрещены" или "текст длиннее 2000 символов". [errors.go](internal/presentation/graphql/errors.go) — единственное место, где они сверяются через `errors.Is` и переводятся в короткий код в `extensions.code` ответа:

```go
{domainPost.ErrCommentsDisabled, "COMMENTS_DISABLED"},
{domainComment.ErrBodyTooLong,   "BODY_TOO_LONG"},
{appPost.ErrForbidden,           "FORBIDDEN"},
```

Всё, что не распознано, уходит с общим текстом ошибки, без утечки внутренних деталей наружу. Пост по несуществующему id — не ошибка, а пустой ответ (`null`): схема объявляет `Post` без `!`, отсутствие — валидный результат, не сбой.

## Тестирование

- `domain` — проверка правил (пустой заголовок, слишком длинный текст) напрямую, без подстановок.
- `application` — сценарии использования на подставном хранилище (генерируется, `//go:generate` в `service.go`) — проверяют логику, не поднимая ни память, ни базу.
- `infrastructure/memory` — уже настоящее in-memory хранилище: постраничный вывод, сортировка, параллельный доступ (100 горутин).
- `infrastructure/postgres` — часть на подставном хранилище + тесты на реальной базе, которые сами себя пропускают без `TEST_DATABASE_URL`.
- `presentation/graphql` — тестовый сервер целиком: запросы, мутации и подписка идут по-настоящему через HTTP/GraphQL и websocket (`resolver_test.go`, `subscription_test.go`), без внешних сервисов.

```bash
task test              
task test:coverage      # + порог покрытия

docker-compose up -d postgres migrate
TEST_DATABASE_URL="postgres://feed:feed@localhost:5432/feed?sslmode=disable" go test ./...
```
