# EASY_START — быстрый старт за 5 минут

## 1. Установка

Рекомендуемый способ — без Go-тулчейна вообще (инструмент рассчитан на unattended-запуск в CI/CD):

```bash
curl -fsSL https://raw.githubusercontent.com/rus-lan/bogoslavAnalytics/v0.2.0/install.sh | sh
```

Вывод:
```
installing bogoslav (tag v0.2.0) for linux/amd64

installed: bogoslav-cli bogoslav-mcp bogoslav-skills
into: /home/<user>/.local/bin
note: ... is not on your PATH; add it, e.g.: export PATH="...:$PATH"
```

Ставит три бинарника в `~/.local/bin` (переопределяется `BOGOSLAV_INSTALL_DIR`) — эта директория должна быть в `PATH`, инсталлятор сам напомнит, если её там нет. linux/darwin, amd64/arm64 (детект через `uname`) — Windows не поддерживается. Linux-сборки статические — работают и в `alpine`/`scratch`-образах CI.

Перед установкой скрипт сверяет SHA-256 каждого бинарника с `SHA256SUMS` того же релиза и отказывается ставить при несовпадении или отсутствии `sha256sum`/`shasum` (обходится через `BOGOSLAV_ALLOW_NO_CHECKSUM=1`). Честно: это проверяет сами бинарники, а не скрипт — прочитать его перед запуском можно так: `curl -fsSL <url> | less`.

Для CI — версию фиксировать, а не тянуть `latest`: так сборка остаётся воспроизводимой, новый релиз не может незаметно поменять то, что реально запускается.

```bash
BOGOSLAV_VERSION=v0.2.0 sh -c "$(curl -fsSL https://raw.githubusercontent.com/rus-lan/bogoslavAnalytics/v0.2.0/install.sh)"
```

Переменные окружения: `BOGOSLAV_VERSION` (версия), `BOGOSLAV_INSTALL_DIR` (куда ставить, по умолчанию `$HOME/.local/bin`), `BOGOSLAV_BINS` (поставить не все три бинарника, а подмножество), `BOGOSLAV_ALLOW_NO_CHECKSUM` (ставить без проверки контрольных сумм).

Альтернатива — если Go уже стоит (нужен не ниже 1.25.0, требование зависимости `modelcontextprotocol/go-sdk`):

```bash
go install github.com/rus-lan/bogoslavAnalytics/cmd/bogoslav-cli@latest
go install github.com/rus-lan/bogoslavAnalytics/cmd/bogoslav-mcp@latest
go install github.com/rus-lan/bogoslavAnalytics/cmd/bogoslav-skills@latest
```

Бинарники попадут в `$GOBIN` (или `~/go/bin`) — эта директория должна быть в `PATH`.
Путь модуля регистрозависимый — скопируйте, не набирайте вручную.

## 2. Настройка

```
export GITLAB_URL=https://gitlab.вашсервер
export GITLAB_TOKEN=<токен, scope read_api — НЕ api, у этого инструмента только чтение>
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

## 7. Через агента (MCP) вместо CLI

```
bogoslav-skills install --target opencode   # или claude | kilo | cline | cursor
```

Одна команда ставит агенту сразу оба: skill (`SKILL.md` — описание тех же шести операций) и регистрацию MCP-сервера `bogoslav-mcp`. Дальше с инструментом не нужно говорить командами — пишете обычным текстом, агент сам решает, какой MCP-тул вызвать. Живых целей пять; `aider` MCP не поддерживает и получает только `CONVENTIONS.md`; Roo Code не поддерживается вовсе.

Шесть тулов — те же шесть операций CLI, но `snake_case`: `find_mrs`, `get_comments`, `get_classify_batch`, `save_labels`, `filter_comments`, `get_stats`.

**Разметка — самое частое место путаницы.** `get_classify_batch` никогда не вызывает модель сам — сервер вообще не умеет звонить в LLM. Он только отдаёт агенту батч комментариев, таксономию, JSON-схему результата и готовый текст промпта. Кто размечает — решает вызывающий агент: либо своей же текущей моделью, либо — если нужна ИМЕННО другая модель — спавнит под это отдельного саб-агента (если среда это умеет). `save_labels` затем валидирует результат против той же схемы и только при полном успехе пишет `labeled_comments`.

Параметр `model` в `get_classify_batch`/`save_labels` — это **не выбор модели** (сервер её всё равно не вызывает), а **метка**: она попадает в ключ кеша разметки и в блок `classifier` артефакта, чтобы потом было видно, кто на самом деле размечал. Передать `model: "glm-5.2"`, а разметить чем-то другим — тихо испортить провенанс.

Пример разговора в opencode:

> «Найди MR'ы, где иванов оставил больше 10 комментариев за первое полугодие, вытащи комментарии и размечи их саб-агентом на glm-5.2»

1. Агент вызывает `find_mrs` → `mr_list`.
2. Агент вызывает `get_comments` → `comment_list`.
3. Агент вызывает `get_classify_batch` (передав `model: "glm-5.2"` как метку) → батч + таксономия + схема + промпт.
4. Агент спавнит саб-агента на модели glm-5.2 и отдаёт ему промпт и батч; саб-агент возвращает JSON-массив `{"note_id": ..., "label": ...}`.
5. Агент вызывает `save_labels` с этим результатом и тем же `model: "glm-5.2"` → на диске `artifacts/labeled_comments_<hash>.yaml` с блоком `classifier: {tool: opencode, model: glm-5.2, ...}`.

Разметка одних и тех же комментариев разными моделями не сравнима напрямую — поэтому провенанс обязателен и не может быть пустым (раздел 6 README.md).

Подробнее про сам MCP-сервер, конфиги агентских инструментов и разметку — разделы 4 и 6 [README.md](./README.md).

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
