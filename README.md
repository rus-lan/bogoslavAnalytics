# bogoslavAnalytics

Аналитика ревью-активности в GitLab: находит merge requests, где конкретный пользователь оставил больше N комментариев, вытаскивает эти комментарии, размечает их по смыслу и агрегирует результат.

Три бинаря живут в `apps/cmd/`:

| Бинарь | Роль |
|---|---|
| `bogoslav-cli` | шесть команд конвейера, вызываются напрямую из терминала |
| `bogoslav-mcp` | те же шесть операций как MCP-тулы поверх stdio — для агентских инструментов (Claude Code, opencode, Kilo, Cline, Cursor) |
| `bogoslav-skills` | ставит skill и MCP-регистрацию `bogoslav-mcp` в целевой инструмент одной командой |

---

## 1. Что это и зачем

Основной вопрос, на который отвечает инструмент: **в каких merge requests пользователь X оставил больше N комментариев** — с фильтрами по дате, группе и репозиторию. Дальше по цепочке: вытащить сами комментарии, разметить их по логическому смыслу, отфильтровать по смысловым меткам, посчитать статистику.

Шесть шагов конвейера:

```
find-mrs  →  get-comments  →  get-classify-batch  →  save-labels  →  filter-comments  →  get-stats
```

Большинство шагов пишет статический файл (json/yaml/text/html), который одновременно служит **кешем** для следующего одинакового запроса — но не все: `get-classify-batch` сам по себе ничего не пишет под `--artifacts-dir` (только читает и, при попадании в кеш разметки, сообщает об этом — раздел 6), а `get-stats` пишет файл, только если передан `--artifacts-dir`, иначе лишь печатает сводку. Это не побочная возможность, а вся суть инструмента: GitLab API имеет лимиты запросов, а поиск по группе перебором может стоить тысячи запросов (раздел 3.7). Не сходить в API повторно ради тех же данных — значит не словить троттлинг или бан по IP/токену на боевом self-managed инстансе. Поэтому конвейер спроектирован так, чтобы к любому шагу можно было вернуться из уже посчитанного файла через `--from-artifact`, вообще не трогая сеть.

Семантическую разметку комментариев (шаг 3–4) не делает ни CLI, ни MCP-сервер — это явное архитектурное решение (раздел 6): размечает вызывающий агент (LLM), инструмент лишь выдаёт батч, таксономию и JSON-схему, а после проверяет результат.

Целевая версия сервера — **GitLab 18.11 self-managed** (также работает и с gitlab.com).

---

## 2. Установка и настройка

### Сборка

```bash
cd apps
make build
```

Три бинаря появятся в `apps/bin/`: `bogoslav-cli`, `bogoslav-mcp`, `bogoslav-skills`. Каталог `bin/` в `.gitignore`, так что после сборки его нужно держать локально или разложить бинари в `PATH` самостоятельно.

Прямая альтернатива без Makefile:

```bash
go build -o bin/bogoslav-cli ./cmd/bogoslav-cli
go build -o bin/bogoslav-mcp ./cmd/bogoslav-mcp
go build -o bin/bogoslav-skills ./cmd/bogoslav-skills
```

### Подключение к GitLab

Обе переменные читаются из окружения процесса — и `bogoslav-cli`, и `bogoslav-mcp`:

| Переменная | Обязательна | Дефолт |
|---|---|---|
| `GITLAB_URL` | нет | `https://gitlab.com` |
| `GITLAB_TOKEN` | **да** | — (без неё клиент не собирается вообще) |

Требуемый scope токена — **`read_user` или `api`**. Результаты фильтруются GitLab по тому, что видно этому токену: если токен не имеет доступа к проекту, инструмент просто не увидит его MR и комментарии, без отдельной ошибки.

Токен передаётся в заголовке `PRIVATE-TOKEN` (рекомендованный GitLab способ) — заголовок не настраивается ни флагом, ни переменной окружения в сегодняшних бинарях.

Без `GITLAB_TOKEN` любая команда, которой нужен GitLab, останавливается сразу:

```
Error: connect to GitLab: gitlab: new client from env: gitlab: GITLAB_TOKEN is not set
```

**Важное предупреждение про reverse-proxy.** `--group`/`--project` принимают либо numeric id, либо путь (`my-group/my-repo`, в том числе вложенные подгруппы). Путь идёт в запрос URL-кодированным (`/` → `%2F`), как того требует сам GitLab. Если перед GitLab стоит обратный прокси (например, Apache), который декодирует `%2F` обратно в `/` до того, как запрос дойдёт до GitLab, — путь превращается в невалидный URL, и GitLab отвечает `404`, хотя группа/проект существуют. Это задокументированная ловушка самого GitLab, не баг этого инструмента. Обходной путь — передавать **numeric id** вместо пути: числовые id этой проблеме не подвержены.

---

## 3. bogoslav-cli

```
bogoslav-cli [command]
```

| Команда | Что делает | Артефакт |
|---|---|---|
| `find-mrs` | ищет MR, где `--user` оставил строго больше `--more-than` комментариев | `mr_list` |
| `get-comments` | вытаскивает сами комментарии пользователя по набору MR | `comment_list` |
| `get-classify-batch` | выдаёт батч + таксономию + JSON-схему + промпт на разметку | не кешируемый вид (batch/schema/prompt) |
| `save-labels` | валидирует разметку и пишет размеченный артефакт | `labeled_comments` |
| `filter-comments` | сужает `labeled_comments` до набора меток | `filtered_comments` |
| `get-stats` | агрегирует любой из четырёх артефактов | сводка (без записи, если не задан `--artifacts-dir`) |

Общие для большинства команд флаги: `--artifacts-dir` (каталог для результата/кеша, по умолчанию `artifacts`), `--format` (`json`/`yaml`/`text`/`html`, по умолчанию `yaml`), `--out` (писать результат в этот файл **вместо** stdout, а не вдобавок к нему — при `--out` stdout остаётся пустым), `--refresh` (там, где есть кеш GitLab-запроса — принудительно игнорировать его). Ни один из них не унаследован от родительской команды автоматически — каждая подкоманда регистрирует свой набор, поэтому не у всех шести команд одинаковый список (см. таблицы ниже).

### 3.1. `find-mrs`

Ищет MR, где `--user` оставил **строго больше** `--more-than` комментариев в окне `[--from, --to]`. MR ровно с `--more-than` комментариями **не возвращается** — нужен `--more-than + 1` и больше. Кандидатов ищет одна из двух стратегий — `events` (быстрая основная) или `bruteforce` (медленный, но всегда корректный фолбэк) — выбирает их не пользователь, а автоселектор (раздел 3.7); выбор и результат smoke-теста печатаются в stderr.

Точечный режим: `--mr` вместе с `--project` возвращает ровно этот один MR без какого-либо поиска кандидатов — точный подсчёт всё равно идёт через `/discussions`.

| Флаг | Дефолт | Описание |
|---|---|---|
| `--user` | — (обязателен) | GitLab username или numeric id |
| `--from`, `--to` | — (обязательны) | `YYYY-MM-DD`, обе границы включительно |
| `--more-than` | `0` | N: строго больше N комментариев |
| `--group` | — | numeric id или путь, включая подгруппы |
| `--project` | — | numeric id или путь; вместе с `--mr` включает точечный режим |
| `--mr` | — | iid MR для точечного режима; требует `--project` |
| `--strict` | `false` | форсировать bruteforce, пропустив events и smoke-тест |
| `--artifacts-dir` | `artifacts` | каталог результата/кеша |
| `--cache-ttl` | `24h` | срок жизни кеша |
| `--refresh` | `false` | игнорировать кеш |
| `--format` | `yaml` | `json`/`yaml`/`text`/`html` |
| `--out` | — | записать результат в этот файл вместо stdout (не вдобавок) |

**Обязательны на самом деле только `--user`, `--from`, `--to`.** `--more-than` выглядит рядом с ними как ещё один обязательный флаг, но это не так: если его не передать, действует дефолт `0`, и команда отработает без единой ошибки. `--more-than 0` — не заглушка, а осмысленный запрос: «любой MR, где `--user` оставил хотя бы один комментарий». Если вы забыли флаг, ожидая увидеть только MR с несколькими комментариями, результат не предупредит об этом — он просто окажется куда шире, чем предполагалось.

Пример (точечный режим — конкретный MR):

```bash
bogoslav-cli find-mrs \
  --user alice --from 2026-01-01 --to 2026-06-30 --more-than 3 \
  --project my-group/repo --mr 77
```

Вывод (yaml, в stdout) начинается с диагностики стратегии на stderr:

```
mode: point (single merge request, no candidate search)
```

или, вне точечного режима:

```
strategy: events
smoke: passed
```

### 3.2. `get-comments`

Вытаскивает все комментарии `--user` внутри `[--from, --to]` по набору MR — по одному вызову `/discussions` на MR, самый дорогой по числу запросов шаг конвейера. Набор MR берётся ровно из одного источника: `--from-artifact` (обычно результат `find-mrs`) **или** явный список `--project` + один/несколько `--mr`. MR из разных проектов без общего `mr_list`-артефакта явным списком не задать — для этого нужен предварительный `find-mrs`.

| Флаг | Дефолт | Описание |
|---|---|---|
| `--user` | — (обязателен) | username или numeric id |
| `--from`, `--to` | — (обязательны) | `YYYY-MM-DD` |
| `--from-artifact` | — | путь к `mr_list`; взаимоисключим с `--project`/`--mr` |
| `--project` | `0` | **numeric id проекта** (не путь!) — см. предупреждение ниже |
| `--mr` | `[]` | iid MR, повторяемый флаг; требует `--project` |
| `--artifacts-dir` | `artifacts` | |
| `--cache-ttl` | `24h` | |
| `--refresh` | `false` | |
| `--format` | `yaml` | |
| `--out` | — | |

**Важно**: у `find-mrs` флаг `--project` принимает numeric id **или путь** (строка). У `get-comments` флаг с тем же именем `--project` — **только numeric id** (тип `int64`, а не строка). Это не опечатка и не баг — одно и то же имя флага имеет разный тип на разных командах, потому что `get-comments` строит явный список `(project_id, mr_iid)` напрямую, без резолва пути.

Пример (продолжение цепочки от `find-mrs`):

```bash
bogoslav-cli get-comments \
  --user alice --from 2026-01-01 --to 2026-06-30 \
  --from-artifact artifacts/mr_list_<hash>.yaml
```

Пример явного списка без `find-mrs`:

```bash
bogoslav-cli get-comments \
  --user alice --from 2026-01-01 --to 2026-06-30 \
  --project 5 --mr 9 --mr 12
```

### 3.3. `get-classify-batch`

Читает существующий `comment_list` и отдаёт вызывающему агенту всё нужное для разметки: сам батч комментариев, таксономию, JSON-схему результата и готовый текст промпта. **Никогда не размечает сам и не вызывает LLM** — это делает вызывающий агент, а результат уходит в `save-labels`. Подробности контракта — раздел 6.

Если батч с теми же комментариями, той же `--model` и той же версией таксономии уже имеет готовый `labeled_comments`, повторная выдача батча не происходит — команда сообщает про попадание в кеш и не тратит время агента.

| Флаг | Дефолт | Описание |
|---|---|---|
| `--from-artifact` | — (обязателен) | путь к `comment_list` |
| `--model` | — (обязателен) | идентификатор модели — часть ключа кеша разметки |
| `--taxonomy-file` | — | свой JSON-файл таксономии; по умолчанию встроенный v1 |
| `--artifacts-dir` | `artifacts` | каталог, где ищется уже готовый `labeled_comments` — **только для чтения**, эта команда сама в него ничего не пишет |
| `--format` | `yaml` | **только `json` или `yaml`** — `text`/`html` не поддерживаются, у батча нет такого вида рендера |
| `--out` | — | |

Флага `--refresh` у этой команды **нет** — обойти кеш разметки нельзя иначе, чем сменить `--model` или версию таксономии.

Пример:

```bash
bogoslav-cli get-classify-batch \
  --from-artifact artifacts/comment_list_<hash>.yaml \
  --model claude-opus-4.8 \
  --out batch.yaml
```

Фрагмент реального вывода (`prompt` и `schema` сокращены для примера, полностью — раздел 6):

```yaml
batch:
    - author: {id: 42, username: alice}
      body: "looks good, ship it"
      created_at: "2026-03-02T10:00:00Z"
      id: 100
      ...
prompt: |
    You are labeling GitLab merge request review comments by their logical, semantic meaning.
    Taxonomy version 1. Assign exactly one label to each comment below, chosen only from this set:
    - bug
    ...
schema:
    type: [null, array]
    items: {type: object, properties: {note_id: {type: integer}, label: {type: string}}, required: [note_id, label], additionalProperties: false}
taxonomy:
    version: 1
    labels: [bug, style, naming, architecture, performance, security, test, docs, question, nitpick, praise, other]
```

### 3.4. `save-labels`

Валидирует результат разметки — который приготовил вызывающий агент, никогда не сам `bogoslav-cli` — против того же батча и таксономии, и только при успехе пишет `labeled_comments` с обязательным блоком провенанса `classifier`. Разметка, не прошедшая проверку (метка вне таксономии, лишний/пропущенный/повторный `note_id`), **не пишет файл вообще**, а возвращает **все** найденные нарушения разом, не только первое.

`--labels` — JSON-файл (или `-` для stdin) с массивом `{"note_id": <int>, "label": "<метка>"}`, ровно по одной записи на каждый комментарий батча.

| Флаг | Дефолт | Описание |
|---|---|---|
| `--from-artifact` | — (обязателен) | тот же `comment_list`, для которого готовилась разметка |
| `--labels` | — (обязателен) | путь к JSON или `-` для stdin |
| `--tool` | — (обязателен) | имя инструмента разметки, для провенанса |
| `--model` | — (обязателен) | модель, для провенанса |
| `--classified-at` | текущее время | RFC 3339, для провенанса |
| `--taxonomy-file` | — | тот же файл, что был передан в `get-classify-batch` |
| `--artifacts-dir` | `artifacts` | |
| `--format` | `yaml` | `json`/`yaml`/`text`/`html` |
| `--out` | — | |

Пример:

```bash
bogoslav-cli save-labels \
  --from-artifact artifacts/comment_list_<hash>.yaml \
  --labels labels.json \
  --tool claude-code --model claude-opus-4.8
```

Проверено на реальном запуске: при передаче метки вне таксономии и пропущенном `note_id` команда падает с одной строкой, перечисляющей оба нарушения, и файл не создаётся:

```
Error: save-labels: save labels: classify: labeling result rejected (2 problem(s)): note 101: label "not-a-real-label" is not in the taxonomy; note 102: note_id from the batch is missing from the labeling result
```

### 3.5. `filter-comments`

Читает `labeled_comments` и оставляет только строки с меткой из `--label` (флаг повторяемый, минимум один обязателен), с опциональным сужением по датам (`--from`+`--to` — оба сразу или ни одного) и по `--group`/`--project`.

`--group`/`--project` резолвятся в numeric id проектов через сам GitLab (`GET /groups/:id/projects` для группы, `GET /projects/:id` — если проект задан путём) — для этого нужен `GITLAB_TOKEN`; если не передавать ни один из флагов, к GitLab обращения не будет вовсе.

| Флаг | Дефолт | Описание |
|---|---|---|
| `--from-artifact` | — (обязателен) | путь к `labeled_comments` |
| `--label` | — (минимум один обязателен) | метка для сохранения, повторяемый |
| `--from`, `--to` | — | доп. фильтр по дате, оба сразу или ни одного |
| `--group` | — | numeric id или путь |
| `--project` | — | numeric id или путь |
| `--artifacts-dir` | `artifacts` | |
| `--format` | `yaml` | |
| `--out` | — | |

У `filter-comments` **нет** `--cache-ttl`/`--refresh`: команда никогда не проверяет кеш перед запуском, всегда читает `--from-artifact` заново и пересчитывает результат.

Пример:

```bash
bogoslav-cli filter-comments \
  --from-artifact artifacts/labeled_comments_<hash>.yaml \
  --label bug --label security
```

### 3.6. `get-stats`

Читает `--from-artifact` — любой из четырёх видов артефактов — и агрегирует: общее число записей, разбивку по MR (для `comment_list`/`labeled_comments`/`filtered_comments`), по меткам (для `labeled_comments`/`filtered_comments`) и по дню (`created_at`). **Никогда не ходит в GitLab** — только считает по уже написанному файлу.

| Флаг | Дефолт | Описание |
|---|---|---|
| `--from-artifact` | — (обязателен) | путь к любому из четырёх артефактов |
| `--artifacts-dir` | — (не задан) | если задан — пишет `stats_<имя-артефакта>.<ext>` в этот каталог; если не задан, только печатает |
| `--format` | `yaml` | **только `json`/`yaml`** |
| `--out` | — | |

Пример:

```bash
bogoslav-cli get-stats --from-artifact artifacts/labeled_comments_<hash>.yaml
```

Реальный вывод для батча из трёх размеченных комментариев:

```yaml
by_date:
    "2026-03-02": 1
    "2026-03-03": 1
    "2026-03-04": 1
by_label:
    naming: 1
    performance: 1
    praise: 1
by_mr:
    - count: 3
      mr_iid: 9
      project_id: 5
source_kind: labeled_comments
total_items: 3
```

`--format text`/`html` отклоняются что с `--artifacts-dir`, что без него — но с разным текстом ошибки: без `--artifacts-dir` сообщение прямо называет причину («stats is not one of the four artifact kinds and has no text or html rendering»), а с `--artifacts-dir` — более общее и менее полезное `artifact: unsupported format`. Оба случая корректно отклоняются, просто в одном объяснение подробнее.

### 3.7. Стратегия поиска: events vs bruteforce

Автоселектор, не пользователь, выбирает стратегию для `find-mrs` (кроме точечного режима и `--strict`, который форсирует `bruteforce` напрямую). Выбор и результат smoke-теста печатаются в stderr и попадают в поле `query.strategy`/`query.smoke` артефакта.

`bruteforce` включается, если верно хотя бы одно:

1. начало диапазона (`--from`) старше 3 лет от текущей даты;
2. smoke-тест `DiscussionNote` (проверка, не теряет ли конкретный инстанс ответы в тредах в Events API) провалился или дал неоднозначный результат;
3. передан `--strict`.

При условии (1) или (3) smoke-тест вообще не запускается — автоселектор решает без него, и поле `smoke` в артефакте отсутствует. Проверено на реальном запуске (диапазон с `2020-01-01`):

```
strategy: bruteforce
smoke:
```

(пустая строка после `smoke:` в stderr — это ожидаемо; в самом yaml-артефакте поле `smoke` просто отсутствует, а не пишется пустым).

На GitLab 18.11 self-managed события старше ~3 лет могут быть вычищены фоновым воркером (`PruneOldEventsWorker`), который включён по умолчанию и выключается только администратором инстанса через консоль — обычным пользователям это недоступно. Поэтому старый диапазон закономерно уходит в `bruteforce`, а не в баг. Обратная сторона: `bruteforce` стоит на порядок больше запросов, чем `events`, — не полагайтесь на него для больших диапазонов без запаса по лимитам.

### 3.8. Особенности, которые стоит знать заранее

- **`--more-than N` — строго больше N.** MR ровно с N комментариями в результат не попадает.
- **`text` и `html` — только на запись.** Их нельзя прочитать обратно ни одной командой, они никогда не участвуют в `--from-artifact`, никогда не дают попадание в кеш. Попытка передать `.txt`/`.html` в `--from-artifact` завершается ошибкой:
  ```
  artifact: this format is write-only and cannot be read back
  ```
- **`--user` — username стоит один лишний запрос на каждый запуск.** Значение из одних цифр используется как есть, без похода в API. Строка резолвится через `GET /users?username=...` — и делает это **заново при каждом вызове команды**: результат резолюции нигде не кешируется между запусками. Если планируете много повторных запусков с одним и тем же пользователем — передавайте сразу numeric id.
- **`--group`/`--project` — путь или numeric id, кроме одного места.** Везде, где это `--group`/`--project` для `find-mrs` и `filter-comments`, годится и то, и другое. Но `--project` у `get-comments` — только numeric id (см. раздел 3.2).

---

## 4. bogoslav-mcp

`bogoslav-mcp` — тот же набор из шести операций, что и `bogoslav-cli`, но как MCP-тулы поверх **stdio**. Инструмент спавнится локально агентским тулом (Claude Code, opencode, Kilo, Cline, Cursor) — отдельно поднимать сервер и порт не нужно.

Имена тулов — `snake_case`, намеренно отличаются от `kebab-case`-команд `bogoslav-cli`:

| Тул | Соответствующая команда CLI |
|---|---|
| `find_mrs` | `find-mrs` |
| `get_comments` | `get-comments` |
| `get_classify_batch` | `get-classify-batch` |
| `save_labels` | `save-labels` |
| `filter_comments` | `filter-comments` |
| `get_stats` | `get-stats` |

**stdout — это сам протокол.** Диагностика и ошибки идут только в stderr (`slog`, текстовый формат); ничто в `bogoslav-mcp` не пишет в stdout напрямую, иначе это сломало бы MCP-поток.

**`GITLAB_TOKEN` нужен при старте всегда**, даже если конкретная сессия ни разу не спросит `find_mrs`/`get_comments`. Сервер собирает GitLab-клиент один раз при запуске, до регистрации тулов, — без токена процесс завершается сразу с ошибкой в stderr и кодом выхода 1, ещё до того, как агент успевает вызвать хоть один тул. Из шести тулов реально ходят в GitLab не все: `find_mrs` и `get_comments` — всегда; `filter_comments` — только если передан `--group`/`--project` (для резолва путей в numeric id); `get_classify_batch`, `save_labels` и `get_stats` в GitLab не ходят никогда. Но заводится сервер одинаково для всех шести — по конструкции, без токена он не запустится вообще.

### Две семьи конфигов

| Инструмент | Файл | Семья |
|---|---|---|
| Claude Code | `.mcp.json` | A: ключ `mcpServers` |
| Cursor | `.cursor/mcp.json` | A |
| Cline | `~/.cline/mcp.json` (глобальный, не проектный) | A |
| opencode | `opencode.json` | B: ключ `mcp`, `command` — массив |
| Kilo | `kilo.jsonc` | B (та же форма, что у opencode) |

Семья A (проверено реальным выводом `bogoslav-skills install`):

```json
{"mcpServers": {"bogoslav": {
  "command": "/path/to/bogoslav-mcp",
  "args": [],
  "env": {}
}}}
```

Семья B — ключ называется `mcp`, `command` — массив, а не строка, и ключ окружения — `environment`, не `env`:

```json
{"mcp": {"bogoslav": {
  "type": "local",
  "command": ["/path/to/bogoslav-mcp"],
  "enabled": true,
  "environment": {}
}}}
```

**`env`/`environment` пустой по умолчанию.** `bogoslav-mcp` читает `GITLAB_URL`/`GITLAB_TOKEN` из собственного окружения процесса, а не из полей конфига — значит либо агентский инструмент уже передаёт их дочернему процессу (обычно наследует переменные из своего собственного окружения), либо их нужно прописать вручную в этот же блок `env`/`environment` конфига.

Писать эти файлы руками не обязательно — `bogoslav-skills install` (раздел 5) генерирует и мёржит их за вас.

---

## 5. bogoslav-skills

Две задачи в одном бинаре:

1. **`generate`** — рендерит `SKILL.md` из живого дерева команд cobra `bogoslav-cli` (те же описания и флаги, что в `--help`), не даёт документации разъехаться с кодом.
2. **`install`** — для пяти живых MCP-целей тот же шаг `generate`, плюс мёрж MCP-регистрации `bogoslav-mcp` в конфиг конкретного инструмента. Для шестой цели, `aider`, всё иначе — см. ниже: `generate` не запускается вовсе, `SKILL.md` не пишется, MCP-конфига нет, пишется только `CONVENTIONS.md`.

### `generate`

```bash
bogoslav-skills generate --project-dir .
```

Пишет `SKILL.md` **в оба** каталога: `.claude/skills/bogoslav/SKILL.md` и `.agents/skills/bogoslav/SKILL.md`. MCP-конфиг не трогает вообще — для этого нужен `install`.

### `install`

```bash
bogoslav-skills install --target <tool>
bogoslav-skills install --all
```

| Флаг | Дефолт | Описание |
|---|---|---|
| `--target` | — | один из `aider claude cline cursor kilo opencode` |
| `--all` | `false` | поставить для всех целей разом |
| `--project-dir` | `.` | куда ставить |
| `--mcp-command` | автоопределение рядом с `bogoslav-skills`, иначе `bogoslav-mcp` из `PATH` | путь к бинарю `bogoslav-mcp`, который пропишется в конфиг |
| `--dry-run` | `false` | показать, что изменится, ничего не записывая |

Пять живых целей: **claude, opencode, kilo, cline, cursor**. Для каждой: `SKILL.md` в оба каталога (`.claude/skills/bogoslav/` и `.agents/skills/bogoslav/`) плюс регистрация MCP-сервера `bogoslav` в **собственном файле конфигурации той цели**. Существующий конфиг **мёржится, не перезаписывается**: чужие MCP-серверы и прочие ключи выживают, добавляется/обновляется только запись `bogoslav`. Для `kilo.jsonc` мёрж — JSONC-aware: проверено вживую — исходные комментарии пользователя и посторонний сервер (`other-server`) в файле остались нетронутыми, добавилась только запись `bogoslav`. Байты вокруг вставки не двигаются — это осознанная цена мёржа: сама вставленная запись **не** получает красивое форматирование, как в примере ниже. В реальности она приезжает на той же строке, что и последний существующий ключ (`,"bogoslav":{`), с чужим, неподходящим отступом. Пример ниже показан в отформатированном виде для читаемости, а не как побайтовый вывод:

```jsonc
{
  // this is my project's own kilo config, please keep this comment
  "mcp": {
    "other-server": { "type": "local", "command": ["/path/to/other"], "enabled": true, "environment": {"FOO": "bar"} },
    "bogoslav": { "type": "local", "command": ["/usr/local/bin/bogoslav-mcp"], "enabled": true, "environment": {} }
  }
}
```

Реальный вывод `install --all` (пути, `create`/`update`/`unchanged` и число повторов `note:` — как есть, побайтово):

```
create .claude/skills/bogoslav/SKILL.md
create .agents/skills/bogoslav/SKILL.md
create .mcp.json
note: the merged entry's env/environment block is empty; bogoslav-mcp reads GITLAB_URL and GITLAB_TOKEN from its own process environment, so whatever spawns it needs to already have them set (or add them to that block yourself)
unchanged .claude/skills/bogoslav/SKILL.md
unchanged .agents/skills/bogoslav/SKILL.md
create opencode.json
note: the merged entry's env/environment block is empty; bogoslav-mcp reads GITLAB_URL and GITLAB_TOKEN from its own process environment, so whatever spawns it needs to already have them set (or add them to that block yourself)
unchanged .claude/skills/bogoslav/SKILL.md
unchanged .agents/skills/bogoslav/SKILL.md
create kilo.jsonc
note: the merged entry's env/environment block is empty; bogoslav-mcp reads GITLAB_URL and GITLAB_TOKEN from its own process environment, so whatever spawns it needs to already have them set (or add them to that block yourself)
unchanged .claude/skills/bogoslav/SKILL.md
unchanged .agents/skills/bogoslav/SKILL.md
create /home/<user>/.cline/mcp.json
note: the merged entry's env/environment block is empty; bogoslav-mcp reads GITLAB_URL and GITLAB_TOKEN from its own process environment, so whatever spawns it needs to already have them set (or add them to that block yourself)
unchanged .claude/skills/bogoslav/SKILL.md
unchanged .agents/skills/bogoslav/SKILL.md
create .cursor/mcp.json
note: the merged entry's env/environment block is empty; bogoslav-mcp reads GITLAB_URL and GITLAB_TOKEN from its own process environment, so whatever spawns it needs to already have them set (or add them to that block yourself)
create CONVENTIONS.md
aider does not read CONVENTIONS.md on its own -- point aider at it with
either:
  aider --read CONVENTIONS.md
or add to .aider.conf.yml:
  read: [CONVENTIONS.md]
```

Каждая из пяти MCP-целей заново пишет `SKILL.md` — но только первая по счёту реально его создаёт (`create`), остальные четыре видят уже написанный на этом прогоне файл и печатают `unchanged` (отсюда восемь строк `unchanged` и пять повторов строки `note:`, по одной на каждую MCP-цель). Хвост про `aider --read` — не часть MCP-цикла, а отдельный, шестой проход `install --all` по aider.

Обратите внимание: Cline получает файл **вне** `--project-dir` — `~/.cline/mcp.json` документирован как глобальный, не проектный, и `bogoslav-skills` следует этому.

### Шестая цель — деградация, не полноценная поддержка

**aider** не поддерживает ни MCP (в апстриме нет ни одной MCP-настройки), ни Agent Skills вовсе. Поэтому `--target aider` — не замена одной лишь MCP-части `install`, а совсем другой путь: это единственная цель, для которой `install` не выполняет шаг `generate` — `SKILL.md` не пишется **ни в один** из двух каталогов, и никакого MCP-конфига тоже нет. Вместо них создаётся ровно один файл — `CONVENTIONS.md`, та же справка по командам `bogoslav-cli`, что для остальных целей уходит в `SKILL.md`, только в формате, который понимает aider. Подключается вручную:

```bash
aider --read CONVENTIONS.md
```

или добавлением в `.aider.conf.yml`:

```yaml
read: [CONVENTIONS.md]
```

### Roo Code — не поддерживается

**Roo Code не является целью установщика.** Проект архивирован владельцем (15.05.2026), это read-only репозиторий на финальной версии — устанавливать в него нет смысла, и `bogoslav-skills` не пытается.

---

## 6. Разметка по смыслу

Ключевой принцип: **инструмент никогда не вызывает LLM**. Разметку выполняет вызывающий агент — тот же, что запустил конвейер, — с любой моделью на свой выбор.

Схема обмена:

1. `get-classify-batch` (CLI) / `get_classify_batch` (MCP) читает `comment_list` и отдаёт: батч комментариев, версию таксономии, JSON-схему ожидаемого результата и готовый текст промпта.
2. Вызывающий агент размечает батч по этому промпту любой моделью, какую сочтёт нужной.
3. `save-labels` / `save_labels` валидирует результат против той же схемы и таксономии и **только при полном успехе** пишет `labeled_comments`. Невалидная разметка не создаёт файл вообще — ни в одном из четырёх форматов — а возвращает список всех нарушений разом (не первое попавшееся).

### Таксономия

Дефолтный набор меток, версия 1 (`taxonomy_version: 1`):

```
bug, style, naming, architecture, performance, security, test, docs, question, nitpick, praise, other
```

`other` — обязательный фолбэк: набор меток без него в принципе не принимается (ни встроенный, ни пользовательский, переданный через `--taxonomy-file`) — попытка загрузить такой набор отклоняется на этапе конструирования таксономии, до всякой разметки.

Таксономия — редактируемые данные, свой JSON-файл того же вида:

```json
{"version": 2, "labels": ["bug", "improvement", "other"]}
```

Изменение состава меток (добавление/удаление/переименование) обязано сопровождаться повышением `version` — версия попадает в провенанс `labeled_comments` и в ключ кеша разметки: другая версия таксономии для тех же комментариев — это уже другой запрос, кеш не сработает и `get-classify-batch` снова выдаст батч на разметку (проверено вживую: смена `--taxonomy-file` с версии 1 на версию 2 приводит к новой выдаче батча, а не к попаданию в старый `labeled_comments`).

### Провенанс

Каждый `labeled_comments` несёт обязательный блок:

```yaml
classifier:
  tool: claude-code
  model: claude-opus-4.8
  taxonomy_version: 1
  classified_at: "2026-07-15T16:40:00+03:00"
```

Это не формальность: разметка одних и тех же комментариев разными моделями — это данные с разной надёжностью, и без явной записи «кто размечал» их легко случайно смешать при последующей фильтрации/сравнении. Блок обязателен целиком — неполный или пустой провенанс (например, `taxonomy_version <= 0`) тоже приводит к отказу записи файла.

### Кеш разметки

`get-classify-batch` не тратит запрос агента заново, если батч с теми же комментариями (хеш по содержимому, не по имени файла), той же `--model` и той же версией таксономии уже размечен — сообщает про попадание в кеш вместо выдачи батча. Этот кеш не имеет TTL в обычном смысле (проверяется по содержимому, а не по возрасту) и не подвержен риску переименования путей группы/проекта (раздел 7) — он не хранит ни группу, ни проект вовсе, только хеш комментариев + модель + версию таксономии.

---

## 7. Артефакты и кеш

Четыре вида артефактов, по цепочке конвейера:

| Артефакт | `kind` | Пишется командой | Роль |
|---|---|---|---|
| 1 | `mr_list` | `find-mrs` | найденные MR со счётчиками комментариев |
| 2 | `comment_list` | `get-comments` | сами комментарии пользователя |
| 3 | `labeled_comments` | `save-labels` | комментарии + метки + провенанс |
| 4 | `filtered_comments` | `filter-comments` | подмножество по меткам |

Каждый — в одном из **четырёх** форматов (`--format`): `json`, `yaml`, `text`, `html`. Имя файла — `<kind>_<hash>.<ext>`, где `hash` — SHA-256 нормализованного запроса.

**`json`/`yaml` — читаемые и кешируемые**; именно они участвуют в `--from-artifact` и в проверке кеша. **`text`/`html` — только на запись**: человекочитаемая сводка и самодостаточная HTML-страница (инлайновый CSS, светлая/тёмная тема, безопасное экранирование пользовательского текста через `html/template`) соответственно — оба не раунд-трипятся обратно в структуру, поэтому не участвуют ни в кеше, ни в `--from-artifact`.

### TTL и `--refresh`

Кеш работает для двух GitLab-зависимых шагов — `find-mrs` и `get-comments`: перед походом в API система ищет уже существующий `json`/`yaml`-артефакт с тем же нормализованным запросом; если найден и не старше TTL — файл отдаётся как есть вместо повторного поиска кандидатов/подсчёта комментариев, о чём сообщается в stderr (`cache hit: <путь>`).

**Важная оговорка (см. также раздел 3.8): `--user` резолвится в numeric id ДО проверки кеша, а не после.** Поэтому «GitLab не трогается вообще» верно только при **numeric** `--user`. При `--user`-username каждый вызов, даже попавший в кеш, всё равно делает один запрос `GET /users?username=...` — резолюция нигде не кешируется между запусками. Проверено вживую с остановленным GitLab-сервером, на одном и том же уже закешированном запросе:

```
--user alice → Error: ...resolve user "alice": ...dial tcp: connection refused
--user 42    → cache hit: artifacts/mr_list_<hash>.yaml   (тот же результат, без единого обращения к сети)
```

Если раздел 1 обещает «не сходить в API повторно ради тех же данных», то с username-ом это верно лишь частично: самые дорогие вызовы (перебор кандидатов, `/discussions`) кеш действительно экономит, но резолв пользователя — нет. Полностью офлайн-повтор гарантирован только с numeric `--user`.

**`--format` на попадании в кеш молча игнорируется.** Кеш-хит отдаёт файл ровно в том формате, в котором он был записан при первом запросе — а не в формате, запрошенном текущим вызовом `--format`. Например, если первый запрос записал `.yaml`, а следующий с теми же флагами, но `--format text`, попадает в кеш — на выходе всё равно будет yaml-содержимое, никакого `.txt`-файла не появится и ошибки не будет.

- `--cache-ttl` — по умолчанию `24h`.
- `--refresh` — принудительно игнорирует кеш и всегда идёт в GitLab заново.

`save-labels` и `filter-comments` кеш GitLab-запроса не проверяют вовсе — им незачем, они не ходят в GitLab. `get-classify-batch` использует отдельный, не зависящий от TTL кеш разметки (раздел 6). `get-stats` вообще не имеет понятия кеша — только читает и агрегирует уже готовый файл.

### `--from-artifact`: сквозная цепочка без похода в API

```bash
bogoslav-cli find-mrs --user alice --from 2026-01-01 --to 2026-06-30 --more-than 3 \
  --project my-group/repo --mr 77
# -> artifacts/mr_list_<hash1>.yaml

bogoslav-cli get-comments --user alice --from 2026-01-01 --to 2026-06-30 \
  --from-artifact artifacts/mr_list_<hash1>.yaml
# -> artifacts/comment_list_<hash2>.yaml (единственный шаг, реально ходящий в API здесь)

bogoslav-cli get-classify-batch --from-artifact artifacts/comment_list_<hash2>.yaml \
  --model claude-opus-4.8 --out batch.yaml
# агент размечает batch.yaml -> labels.json

bogoslav-cli save-labels --from-artifact artifacts/comment_list_<hash2>.yaml \
  --labels labels.json --tool claude-code --model claude-opus-4.8
# -> artifacts/labeled_comments_<hash3>.yaml

bogoslav-cli filter-comments --from-artifact artifacts/labeled_comments_<hash3>.yaml \
  --label bug --label security
# -> artifacts/filtered_comments_<hash4>.yaml

bogoslav-cli get-stats --from-artifact artifacts/filtered_comments_<hash4>.yaml
# сводка в stdout, GitLab не трогается
```

Четыре последних шага (`get-classify-batch`, `save-labels` при валидном входе, `filter-comments`, `get-stats`) не делают ни одного HTTP-запроса к GitLab, если входной артефакт уже на диске — весь их «поход в сеть» уже был выполнен на шагах 1–2.

### Известное ограничение кеша `mr_list`/`comment_list`

Ключ кеша хеширует `--group`/`--project` как **путь-строку**, а не резолвленный numeric id. Если группу или проект переименуют и **новый** объект займёт **старый** путь, тот же ключ кеша какое-то время будет отвечать данными старого объекта — до истечения TTL (по умолчанию 24 часа). Это принятый, осознанный риск, а не забытый баг: цена его устранения (резолвить numeric id заранее и хешировать по нему) не оправдана для окна в 24 часа.

---

## 8. Контракты

`contracts/openapi.yaml` — OpenAPI 3.1, генерируется командой:

```bash
cd apps
make contracts
```

- `paths: {}` — намеренно пусто: у продукта нет REST API, доступ только через CLI и MCP-по-stdio.
- В `components/schemas` — схемы всех четырёх артефактов (`mr_list`, `comment_list`, `labeled_comments`, `filtered_comments`) и схемы входа/выхода каждого MCP-тула, сгенерированные рефлексией из тех же Go-типов, что использует `mcp.AddTool` при регистрации тулов — одна генерация, два потребителя, ручного дублирования схем нет.
- Схема выхода `get_classify_batch` **сознательно отсутствует**: её Go-тип содержит self-referential поле (встраивает `*jsonschema.Schema`), которое рефлексия построить не может. Это не пропуск по забывчивости — файл прямо документирует эту причину в шапке.
- Файл — сгенерированный: правка руками не имеет смысла, `make contracts` перезапишет её при следующей генерации, а тест в `apps/internal/contracts` проверяет отсутствие расхождения между закоммиченным файлом и тем, что выдаёт генератор.
