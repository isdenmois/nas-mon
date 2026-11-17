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
| `MON_TEMP_INTERVAL` | `10s` | hwmon-температуры |
| `MON_DOCKER_INTERVAL` | `15s` | контейнеры: CPU, RAM, сеть |
| `MON_HOST_INTERVAL` | `15s` | CPU, RAM, load, сеть хоста |
| `MON_VOLUME_INTERVAL` | `60s` | занятость ФС в `/volumes` |
| `MON_SMART_INTERVAL` | `12h` | SMART и лог износа |
| `MON_AGGREGATE_INTERVAL` | `1m` | как часто закрытые минутки пишутся в БД |
| `MON_RETENTION_MINUTE` | `48h` | хранение минуток (`0` — вечно) |
| `MON_RETENTION_HOUR` | `2160h` (90 д) | хранение часовых (`0` — вечно) |
| `MON_TEMP_WARN_C` | `80` | порог красной подсветки, °C |
| `MON_TEMP_INCLUDE` | пусто | regex по ключу сенсора; **заменяет** выбор по умолчанию |
| `MON_TEMP_EXCLUDE` | пусто | regex-исключение, применяется всегда |
| `MON_DOCKER_EXCLUDE` | пусто | regex по имени контейнера |
| `MON_LOG_LEVEL` | `info` | `debug`, `info`, `warn`, `error` |
| `MON_DOCKER_SOCKET` | `/var/run/docker.sock` | путь к сокету Docker |
| `MON_VOLUMES_DIR` | `/volumes` | каталог с монтируемыми ФС |

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

## Что хранится

| Таблица | Содержимое | Срок |
|---|---|---|
| `points_1m` | среднее/min/max по минутным бакетам | 48 ч |
| `points_1h` | свёртка минуток | 90 дней |
| `points_1d` | свёртка часов | вечно |
| `wear_log` | износ SSD: `percentage_used`, TBW, часы, ошибки | вечно |

Свёртка идемпотентна (`UPSERT`), повторный запуск после падения не задваивает
данные. Последняя неполная минута при остановке отбрасывается.

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
| `GET /api/history?range=24h&series=temp:k10temp:Tctl` | агрегаты; `range`: `1h 6h 24h 48h 7d 30d 90d 1y all` |
| `GET /api/wear?limit=200` | последний SMART по каждому диску + лог износа (`limit` до 20000) |
| `GET /metrics` | Prometheus text exposition |
| `GET /healthz` | `{"ok":true}` |

`series` можно повторять. Доступные kind: `temp`, `host_cpu`, `host_mem`,
`host_load1/5/15`, `host_net_rx/tx`, `container_cpu`, `container_mem`,
`container_mem_pct`, `container_net_rx/tx`, `volume_used_pct`.

Пример scrape-конфига:

```yaml
scrape_configs:
  - job_name: mon
    static_configs:
      - targets: ["homelab:8090"]
```

## Разработка

Нужны Go 1.27+ и Node 24+.

```sh
make web     # npm ci + vite build -> internal/server/dist (вшивается в бинарь)
make test    # go test ./...
make vet
make build   # bin/mon
make docker  # docker compose up -d --build
make e2e     # Playwright: UI-сценарии на замоканном API
```

`go build` без фронтенда тоже работает: в `internal/server/dist/index.html`
лежит заглушка. Для локального запуска SMART нужен root, остальное — нет:

```sh
MON_DB_PATH=./data/mon.db MON_SMART_INTERVAL=0 go run ./cmd/mon
```

### E2E-тесты

Playwright поднимает Vite dev server и перехватывает все `/api/*` запросы,
поэтому Go-бэкенд, root и Docker не нужны. Время в браузере заморожено,
локаль и таймзона зафиксированы, данные — из `web/e2e/fixtures.ts`.

```sh
cd web
npx playwright install chromium   # один раз
npm run test:e2e                  # или make e2e из корня
npm run test:e2e:update           # перегенерировать эталоны графиков
```

Скриншоты чартов лежат в `web/e2e/__screenshots__/` и коммитятся. После
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
