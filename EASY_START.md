# EASY_START — быстрый старт за 5 минут

## 1. Установка

```
go install github.com/rus-lan/bogoslavAnalytics/apps/cmd/bogoslav-cli@latest
go install github.com/rus-lan/bogoslavAnalytics/apps/cmd/bogoslav-mcp@latest
go install github.com/rus-lan/bogoslavAnalytics/apps/cmd/bogoslav-skills@latest
```

Бинарники попадут в `$GOBIN` (или `~/go/bin`) — эта директория должна быть в `PATH`.
Нужен Go не ниже 1.25.0 (требование зависимости `modelcontextprotocol/go-sdk`).
Путь модуля регистрозависимый — скопируйте, не набирайте вручную.

## 2. Настройка

```
export GITLAB_URL=https://gitlab.вашсервер
export GITLAB_TOKEN=<токен, scope read_user или api>
```

## 3. Поиск

```
bogoslav-cli find-mrs \
  --user ivanov --from 2026-01-01 --to 2026-06-30 \
  --more-than 10 --group my-group/subgroup \
  --format yaml
# -> artifacts/mr_list_<hash>.yaml
```

Какая стратегия выбрана (events/bruteforce) и результат smoke-теста — в stderr.

## 4. Комментарии

```
bogoslav-cli get-comments \
  --user ivanov --from 2026-01-01 --to 2026-06-30 \
  --from-artifact artifacts/mr_list_<hash>.yaml
# -> artifacts/comment_list_<hash>.yaml
```

## 5. Разметка — три шага

```
bogoslav-cli get-classify-batch \
  --from-artifact artifacts/comment_list_<hash>.yaml \
  --model glm-5.2 --out batch.yaml

# ваш агент/модель размечает batch.yaml -> labels.json
# формат: [{"note_id": 123, "label": "bug"}, ...]

bogoslav-cli save-labels \
  --from-artifact artifacts/comment_list_<hash>.yaml \
  --labels labels.json --tool opencode --model glm-5.2
# -> artifacts/labeled_comments_<hash>.yaml
```

Невалидная разметка не пишет файл и сразу перечисляет все нарушения.

## 6. Анализ

```
bogoslav-cli filter-comments \
  --from-artifact artifacts/labeled_comments_<hash>.yaml \
  --label bug --label security
# -> artifacts/filtered_comments_<hash>.yaml

bogoslav-cli get-stats --from-artifact artifacts/filtered_comments_<hash>.yaml
```

## 7. Через агента вместо CLI

```
bogoslav-skills install --target opencode   # или claude | kilo | cline | cursor
```

Те же шесть операций, что и в CLI, доступны как инструменты MCP над тем же кодом.
Живых целей пять; `aider` MCP не поддерживает и получает только `CONVENTIONS.md`; Roo Code не поддерживается вовсе.

## 8. Грабли

| Что | Как на самом деле |
|---|---|
| `--more-than 10` | СТРОГО больше: у MR ровно с 10 комментариями результат не попадёт в выдачу |
| `--project` | путь или число в `find-mrs`, но ТОЛЬКО число в `get-comments` — один и тот же флаг, разные типы |
| `--user ivanov` | резолв имени — 1 доп. вызов API; результат кешируется на 24ч и в `find-mrs`, и в `get-comments` (`--refresh` сбрасывает). `--user 42` — 0 доп. вызовов |
| `--format text` / `html` | только на запись: не читаются обратно, не годятся для `--from-artifact`, никогда не дают cache hit |
| `get-stats`, `get-classify-batch` | поддерживают только `json`/`yaml` |
| Повторный запуск | отдаётся из кеша; `--refresh` заставляет идти в GitLab |
| Диапазон дат старше ~3 лет | автоматически переключается на bruteforce — примерно в 10 раз больше вызовов API |
| `--format` при cache hit | молча игнорируется: возвращается кешированный файл в том формате, в котором он был записан изначально (об этом пишется в stderr) |

Полное описание всех команд, флагов и внутреннего устройства — в [README.md](./README.md).
