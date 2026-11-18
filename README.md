# mon

Монитор homelab-сервера: температуры CPU/NVMe, ресурсы Docker-контейнеров и
хоста, занятость файловых систем и лог износа SSD. Один статический Go-бинарь с
вшитым дашбордом (Vue 3 + Chart.js) и Prometheus-эндпоинтом.

* страница `/` показывает только текущее состояние, `/charts` — историю
  температур, CPU/RAM/load/сети хоста, CPU/сети контейнеров и TBW SSD;
* сырые отсчёты живут только в памяти;
* на диск попадают средние: минутки 48 ч → часы 90 дней → сутки навсегда;
* SMART-лог износа хранится вечно и пополняется только при значимом
  изменении счётчиков.

## Быстрый старт

```sh
docker compose up -d --build
```

Открой `http://<host>:8090` (текущее состояние) и
`http://<host>:8090/charts` (графики). Для Prometheus:
`http://<host>:8090/metrics`, healthcheck: `/healthz`.

При первом запуске появится каталог `./data` с `mon.db` (SQLite, WAL).
Удалить историю: остановить контейнер и удалить `./data/mon.db*`.

Контейнер работает в `network_mode: host`: иначе `/proc/net/dev` внутри
контейнера описывает его собственный netns (`eth0`/`lo`), и «сеть хоста»
показывает трафик контейнера. В host-режиме `MON_PORT` — это порт прямо на
хосте, маппинг портов не используется.

### Какие диски видны

Контейнер запускается с `privileged: true`: SMART читается через
`smartctl -j -a` по всем `/dev/nvme*n*` и `/dev/sd*`, найденным в
`/sys/block`, а hwmon — из `/sys/class/hwmon`. HDD с ненулевым
`rotation_rate` пропускаются, остаются SSD/NVMe.

### Какие файловые системы видны

Каждый подкаталог `/volumes` считается отдельной ФС. Добавь нужные хостовые
пути в `compose.yaml`:

```yaml
    volumes:
      - /:/volumes/root:ro
      - /mnt/media:/volumes/media:ro
```

Имя в UI берётся из `/proc/self/mountinfo` (например `/mnt/media`), поэтому
видно реальный хостовый путь, а не `overlay` контейнера.

## Настройка через ENV

Все интервалы — Go-длительности (`10s`, `15m`, `12h`) или целые секунды.
`0` выключает сборщик; история по нему перестаёт пополняться.

| Переменная | Дефолт | Описание |
|---|---|---|
| `MON_PORT` | `8090` | HTTP-порт на хосте (контейнер в `network_mode: host`) |
| `MON_DB_PATH` | `/data/mon.db` | файл SQLite |
| `MON_TEMP_INTERVAL` | `30s` | hwmon-температуры |
| `MON_DOCKER_INTERVAL` | `60s` | контейнеры: CPU, RAM, сеть |
| `MON_HOST_INTERVAL` | `30s` | CPU, RAM, load, сеть хоста |
| `MON_VOLUME_INTERVAL` | `5m` | занятость ФС в `/volumes` |
| `MON_SMART_INTERVAL` | `12h` | SMART и лог износа |
| `MON_AGGREGATE_INTERVAL` | `5m` | как часто закрытые минутки пишутся в БД |
| `MON_RETENTION_MINUTE` | `48h` | хранение минуток (`0` — вечно) |
| `MON_RETENTION_HOUR` | `2160h` (90 д) | хранение часовых (`0` — вечно) |
| `MON_RETENTION_DAY` | `43800h` (5 лет) | хранение суточных (`0` — вечно) |
| `MON_RETENTION_WEAR` | `17520h` (2 года) | хранение лога износа SSD (`0` — вечно) |
| `MON_TEMP_WARN_C` | `80` | порог красной подсветки, °C |
| `MON_TEMP_INCLUDE` | пусто | regex по ключу сенсора; **заменяет** выбор по умолчанию |
| `MON_TEMP_EXCLUDE` | пусто | regex-исключение, применяется всегда |
| `MON_DOCKER_EXCLUDE` | пусто | regex по имени контейнера |
| `MON_LOG_LEVEL` | `info` | `debug`, `info`, `warn`, `error` |
| `MON_PPROF_ADDR` | пусто | адрес профайлера, только loopback (например `127.0.0.1:6061`); пусто — выключен |
| `MON_DOCKER_SOCKET` | `/var/run/docker.sock` | путь к сокету Docker |
| `MON_VOLUMES_DIR` | `/volumes` | каталог с монтируемыми ФС |
| `GOMAXPROCS` | `2` | размер планировщика Go; без лимита CPU в cgroup Go берёт все ядра хоста |

В `compose.yaml` значения пробрасываются из окружения/`.env`:

```sh
cp .env.example .env   # поправить интервалы
docker compose up -d --build
```

или разово: `MON_TEMP_INTERVAL=5s docker compose up -d`.

### Выбор датчиков

Ключ датчика — `chip:label` (`k10temp:Tctl`) или `nvme:<disk>`
(`nvme:nvme1`). По умолчанию показываются только температура CPU-пакета
(`Tctl`, иначе `Tdie`) и композитный сенсор каждого NVMe. Чтобы добавить
остальные:

```sh
MON_TEMP_INCLUDE='k10temp:|nvme:'
```

Температура SATA-дисков намеренно показывается только в таблице SMART
(обновляется раз в `MON_SMART_INTERVAL`), а не в плитках: опрос `smartctl`
каждые 10 секунд дёргал бы диски.

### Профилирование

Если кажется, что `mon` ест CPU, включи профайлер — отдельный порт, только
loopback:

```sh
MON_PPROF_ADDR=127.0.0.1:6061 docker compose up -d
curl -s -o /tmp/cpu.pprof 'http://127.0.0.1:6061/debug/pprof/profile?seconds=30'
go tool pprof -top -nodecount=20 /tmp/cpu.pprof
```

Там же `/debug/pprof/heap`, `/debug/pprof/goroutine` и `/debug/pprof/trace`.
Адрес обязан быть loopback (`127.0.0.1`, `::1`, `localhost`), иначе старт
завершится ошибкой; на основном порту дашборда профайлера нет.

Как мерить стоимость по компонентам и какие цифры получились до/после —
[`doc/cpu.md`](doc/cpu.md). Быстрая проверка на живом бинарнике:
`just bench idle 300` (или `dashboard`, `scrape`).

## Что хранится

| Таблица | Содержимое | Срок |
|---|---|---|
| `points_minute` | среднее/min/max по минутным бакетам | 48 ч |
| `points_hour` | свёртка минуток | 90 дней |
| `points_day` | свёртка часов | 5 лет |
| `wear_log` | износ SSD: `percentage_used`, TBW, часы, ошибки | вечно |

Схема версионируется goose (`internal/store/migrations`): при старте
применяются недостающие миграции, существующий `mon.db` обновляется на месте.

Свёртка идемпотентна (`UPSERT`), повторный запуск после падения не задваивает
данные. Последняя неполная минута при остановке отбрасывается; закрытые
минутки, накопленные с прошлого тика, сбрасываются в БД при остановке.

SMART-строка пишется только при значимом изменении:

* первый снимок диска;
* смена модели или серийного номера;
* рост `media_errors` или `reallocated`;
* рост процента износа;
* рост TBW минимум на 0.1 ТБ (100 ГБ); откат счётчика тоже фиксируется.

Изменение температуры и power-on hours строку не создаёт, суточного
heartbeat-а нет. График TBW строится по этому логу, поэтому обновляется не
чаще SMART-опроса (12 ч по умолчанию) и только когда накопилось значимое
изменение.

## API

| Endpoint | Назначение |
|---|---|
| `GET /api/overview` | текущий снимок: температуры, хост, контейнеры, ФС, SMART, здоровье сборщиков |
| `GET /api/history?range=24h&id=12` | агрегаты; `range`: `1h 6h 24h 48h 7d 30d 90d 1y all`; `id` можно повторять |
| `GET /api/wear?limit=200&range=30d` | последний SMART по каждому диску + лог износа (`limit` до 20000, `range` необязателен) |
| `GET /metrics` | Prometheus text exposition |
| `GET /healthz` | `{"ok":true}` |

Идентификаторы серий выдаёт сервер: `/api/overview` возвращает массив `series`
(`{id, kind, name}`) для всего, что уже записано, и по этим `id` запрашивается
история. Строка `kind:name` по проводу больше не ходит, поэтому имена с
двоеточиями (`nvme:nvme0:Composite`) не ломают разбор. Доступные kind: `temp`,
`host_cpu`, `host_mem`, `host_load1/5/15`, `host_net_rx/tx`, `container_cpu`,
`container_mem`, `container_mem_pct`, `container_net_rx/tx`, `volume_used_pct`.

Пример scrape-конфига:

```yaml
scrape_configs:
  - job_name: mon
    static_configs:
      - targets: ["homelab:8090"]
```

## Разработка

Нужны Go 1.27+, Bun 1.4+ и [just](https://github.com/casey/just).

```sh
bun install  # зависимости фронтенда и e2e
just install-hooks  # установить git-хуки (один раз)
just web     # bun install + vite build -> internal/httpapi/dist (вшивается в бинарь)
just test    # go test ./...
just unit    # frontend unit tests (Vitest)
just vet
just build   # bin/mon
just docker  # docker compose up -d --build
just e2e     # Playwright: UI-сценарии на замоканном API
just lint    # Biome: линт и проверка форматирования
```

`go build` без фронтенда тоже работает: в `internal/httpapi/dist/index.html`
лежит заглушка. Для локального запуска SMART нужен root, остальное — нет:

```sh
MON_DB_PATH=./data/mon.db MON_SMART_INTERVAL=0 go run ./cmd/mon
```

### Структура фронтенда

Фронтенд разложен по Feature-Sliced Design; импорт идёт только вниз по слоям:

* `web/src/app` — точка входа, маршрутизация, общий лейаут и глобальные стили;
* `web/src/pages` — экраны `/` (`overview`) и `/charts`;
* `web/src/entities` — домены со своей моделью и UI: температуры, хост,
  контейнеры, SMART;
* `web/src/shared` — инфраструктура: API-клиент и контракты, форматирование,
  диапазоны, графики и переиспользуемые элементы UI.

У каждого слайса своя публичная точка входа `index.ts`, алиас `@/` указывает на
`web/src`. Стили — UnoCSS (`web/uno.config.ts`): утилиты в разметке используют
цвета из CSS-переменных `web/src/app/styles/global.css`, а общие классы
компонентов остаются в этом файле.

### Unit-тесты

Frontend-тесты используют Vitest и `@testing-library/vue`, лежат рядом с
исходниками (`web/src/**/*.test.ts`) и написаны по AAA-паттерну. Сценарии
описаны в `doc/test/*.test-case.md`.

```sh
just unit                 # vitest run
bun run test:unit:watch   # режим наблюдения
```

Go-тесты запускаются через `just test`.

### Линтинг фронтенда

Biome проверяет `web/`, `e2e/`, `playwright.config.ts` и `package.json`.

```sh
just lint        # biome check: линт + форматирование
bun run lint:fix # автоисправления
just format      # переформатировать файлы
```

В `.vue`-файлах отключены `noUnusedImports` и `noUnusedVariables`: Biome пока
не видит использования из шаблона, эти проверки покрывает `vue-tsc`.

### Git-хуки

Хуки настраиваются через [Lefthook](https://lefthook.dev/) (`lefthook.yml`).
После клонирования репозитория:

```sh
just install-hooks   # bunx lefthook install
```

`pre-commit` последовательно форматирует и линтит фронтенд, проверяет типы и
запускает Go- и frontend-тесты. `pre-push` прогоняет Playwright e2e. Пропустить
хуки можно через `LEFTHOOK=0 git commit ...` или `LEFTHOOK=0 git push`.

### E2E-тесты

Playwright поднимает Vite dev server и перехватывает все `/api/*` запросы,
поэтому Go-бэкенд, root и Docker не нужны. Время в браузере заморожено,
локаль и таймзона зафиксированы, данные — из `e2e/fixtures.ts`.

```sh
bunx playwright install chromium  # один раз
bun run test:e2e                  # или just e2e из корня
bun run test:e2e:update           # перегенерировать эталоны графиков
```

Скриншоты чартов лежат в `e2e/__screenshots__/` и коммитятся. После
осознанного изменения внешнего вида графиков обнови их через
`test:e2e:update` и проверь diff картинок.

## Ограничения

* Linux only: читаются `/proc`, `/sys/class/hwmon`, `statfs`.
* CPU контейнера считается как у `docker stats`: 100% = одно ядро, значения
  могут быть больше 100%.
* Сеть хоста суммирует физические интерфейсы; `lo`, `veth*`, `docker*`,
  `br-*`, `virbr*`, `tun/tap`, `wg*` и CNI-интерфейсы исключены, чтобы не
  считать один и тот же трафик дважды. Для корректных цифр контейнер должен
  работать с `network_mode: host` (см. `compose.yaml`); в bridge-режиме
  видны только интерфейсы контейнера.
* Для SATA TBW считается из атрибута `Total_LBAs_Written` как 512-байтные
  секторы; некоторые прошивки используют другие единицы.
* Один экземпляр на базу: SQLite не рассчитан на параллельных писателей.
* Диски ищутся по `/sys/block` при старте сбора: новый диск появится после
  перезапуска контейнера.
* При агрегации теряется последняя неполная минута — это осознанный размен,
  чтобы не публиковать смещённые средние.

## Безопасность

Контейнер привилегированный, а `/var/run/docker.sock` даёт управление Docker,
то есть фактически root на хосте. Не публикуй порт в интернет; ставь за
firewall/VPN или заверни в reverse proxy с авторизацией. Авторизации в самом
`mon` нет.
