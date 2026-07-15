<!-- workspace-meta
name: bogoslavAnalytics
version: 2.0.0
created: 2026-07-15
apps:
  - dir: ., type: go, agent: go-dev
methodologies: []
-->

# bogoslavAnalytics

Инструмент поиска merge request'ов, где заданный пользователь оставил больше N комментариев в GitLab. Состоит из CLI (`bogoslav-cli`), MCP-сервера (`bogoslav-mcp`) и генератора/установщика скиллов (`bogoslav-skills`), которые оборачивают одни и те же internal-пакеты.

## Источники истины

- Полное ТЗ — `TZ.md` (заморожено, не редактировать).
- Research-факты (GitLab API, MCP SDK, форматы агентских инструментов) — `.research/`. В `.gitignore`, не попадает в git.

## Сборка и тесты

Go-модуль лежит в корне репозитория. Все команды `go` и `make` выполняются из корня.

```bash
make build   # собирает bogoslav-cli, bogoslav-mcp, bogoslav-skills в bin/
make test    # go test ./...
make lint    # go vet ./...
make fmt     # gofmt -w .
```

Запуск одного теста:

```bash
go test ./internal/<pkg> -run '<TestName>' -v
```

## Жёсткие правила

- Имя `glab` не использовать нигде (бинарь, пакет, команда) — коллизия с официальным CLI GitLab.
- `domain/` не импортирует другие internal-пакеты.
- LLM-вызовов нет ни в одном пакете — семантическую разметку выполняет вызывающий агент, MCP только владеет контрактом.

## Central Config

Правила синхронизируются из `~/claude-config/`. `/project-pull` обновляет правила проекта из центрального репозитория, `/project-push` отправляет локальные улучшения обратно.
