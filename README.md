# Debt Bot

Telegram-бот для учёта совместных расходов, сделок и взаиморасчётов.
Backend хранит данные в PostgreSQL; Telegram frontend обращается к нему по gRPC.
Telegram Bot API использует HTTP согласно внешнему контракту Telegram.

## Версии и проверка

- Go **1.26.7** в обоих модулях, CI и Docker builder.
- Telegram SDK: `github.com/mymmrac/telego` **v1.12.1**.
- Для контейнерной сборки нужны Docker и Docker Compose.
- `make install` устанавливает закреплённые версии buf и Go protobuf plugins
  в `GOBIN` (по умолчанию `$(go env GOPATH)/bin`); каталог должен быть в `PATH`.

```sh
make install
make proto
make format      # применить gofmt
make check       # backend + frontend: тесты, форматирование, go vet
make proto-lint
make build       # собрать контейнеры без запуска
```

Тесты frontend используют локальный HTTP fake Telegram API и синтетические
данные. Проверяются отправка и редактирование, callback-навигация,
недоступные сообщения, ForwardOrigin, команды и отмена long polling.
Боевой токен и работающая БД для тестов не нужны.

## Запуск

Compose ожидает локальный `.env` с `POSTGRES_USER`, `POSTGRES_PASSWORD`,
`POSTGRES_DB` и `TELEGRAM_BOT_TOKEN`. Секреты не добавляются в Git.
Нужна существующая Docker-сеть `monitoring-net`; OTLP отправляется на `jaeger:4318`.

```sh
make up          # сборка и запуск
make logs
make down        # остановка без удаления тома PostgreSQL
```

Backend применяет миграции при запуске. Frontend использует long polling
(60 секунд); `SIGINT`/`SIGTERM` отменяет HTTP-запросы и обработку обновлений.
Обновления обрабатываются последовательно, состояние диалогов хранится в памяти.
Для запуска frontend вне Docker после генерации proto:

```sh
cd src/frontend/telegram
# TELEGRAM_BOT_TOKEN и BACKEND_ADDR должны быть заданы в окружении.
go run ./cmd/bot
```

`BACKEND_ADDR` по умолчанию — `backend:50051`; `OTLP_ENDPOINT` опционален.
Остановка локального frontend — Ctrl+C.

CI проверяет PR и push в `main`. После успешных проверок push в `main`
запускает SSH-деплой в `VM_PROJECT_PATH` с `git pull` и `make up`.
