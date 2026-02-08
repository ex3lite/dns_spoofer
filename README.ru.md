![DNS Spoofer](header.png)

**DNS relay с выборочным спуфингом + прозрачный TCP прокси** для перенаправления трафика сервисов (OpenAI, ChatGPT, Google Gemini, Cursor, Microsoft Copilot, GitHub Copilot) через ваш собственный сервер. Один бинарник: DNS отвечает вашим IP для выбранных доменов, затем проксирует сырой TCP (HTTP/HTTPS) к реальным бэкендам.

**Назначение:** Этот инструмент предназначен для обхода региональных блокировок и гео-ограничений путём маршрутизации трафика через ваш собственный сервер, расположенный в регионе без ограничений. Он позволяет получить доступ к AI сервисам, которые могут быть заблокированы или ограничены в определённых странах.

**English version:** [README.md](README.md)

**Автор:** [Kakadu Secure Technologies](https://github.com/Matrena-VPN)

---

## Что делает

1. **DNS сервер (UDP :53)**  
   - Для настроенных суффиксов доменов → возвращает IP вашего сервера (спуф).  
   - Для HTTPS/SVCB записей (тип 65/64) на спуфнутых доменах → возвращает NODATA, чтобы предотвратить QUIC/HTTP3 подсказки и ECH ключи.
   - Для всего остального → перенаправляет на upstream DNS (8.8.8.8, 1.1.1.1 с failover).

2. **TCP прокси (:80, :443)**  
   - Принимает соединения на ваш IP.  
   - Читает SNI (TLS) или `Host` (HTTP), резолвит хост через upstream DNS (чтобы избежать циклов), затем туннелирует сырые байты к реальному серверу. Без расшифровки TLS.

3. **UDP sink (:443)**  
   - Слушает UDP 443 и отбрасывает все пакеты (без ответа).
   - Заставляет QUIC/HTTP3 соединения падать, заставляя клиентов откатываться на TCP.

Результат: клиенты, использующие ваш сервер как DNS, получают домены AI сервисов, указывающие на вас; ваш прокси перенаправляет этот трафик к реальным эндпоинтам через TCP.

---

## Поддерживаемые домены (по умолчанию)

| Сервис | Суффиксы |
|---------|----------|
| **OpenAI / ChatGPT** | `.openai.com`, `.chatgpt.com`, `.oaistatic.com`, `.oaiusercontent.com` |
| **Google Gemini** | `.gemini.google.com`, `.aistudio.google.com`, `.ai.google.dev`, `.generativelanguage.googleapis.com`, `.makersuite.google.com` |
| **Cursor IDE** | `.cursor.sh`, `.cursor.com`, `.cursorapi.com`, `.cursor-cdn.com` |
| **Microsoft Copilot** | `.copilot.microsoft.com`, `.bing.com`, `.bingapis.com`, `.edgeservices.bing.com`, `.edgecopilot.microsoft.com` |
| **GitHub Copilot** | `.githubcopilot.com`, `.individual.githubcopilot.com`, `.api.individual.githubcopilot.com`, `.business.githubcopilot.com`, `.enterprise.githubcopilot.com`, `.copilot-proxy.githubusercontent.com`, `.origin-tracker.githubusercontent.com`, `.copilot-telemetry.githubusercontent.com` |

---

## Обработка QUIC / HTTP3

Современные браузеры (Chrome, Edge, Firefox) и некоторые сервисы используют **QUIC (HTTP/3)** через UDP порт 443. Это обходит традиционные TCP прокси. DnsSpoofer обрабатывает это многоуровневым подходом:

### Как мы заставляем откат на TCP

1. **На уровне DNS:** Блокируем HTTPS/SVCB записи (тип 65/64) для спуфнутых доменов. Эти записи рекламируют поддержку HTTP/3 и ECH ключи. Без них клиенты не знают, что QUIC доступен.

2. **На сетевом уровне:** UDP sink на порту 443 читает и отбрасывает все UDP пакеты. QUIC handshake падает, заставляя клиентов откатываться на TCP.

3. **Результат:** Весь трафик идёт через наш TCP прокси, где мы можем читать SNI и правильно маршрутизировать.

### Потенциальные проблемы

| Проблема | Описание | Решение |
|-------|-------------|----------|
| **DNS over HTTPS (DoH)** | Браузеры могут использовать зашифрованный DNS (8.8.8.8, 1.1.1.1), обходя ваш DNS сервер | Отключите DoH в настройках браузера или установите системный DNS на ваш сервер |
| **Alt-Svc заголовок** | После первого TCP визита сервер может рекламировать QUIC через HTTP заголовок | UDP sink гарантирует, что попытки QUIC всё равно падают |
| **ECH (Encrypted Client Hello)** | Скрывает реальный SNI от прокси | Мы блокируем HTTPS RR в DNS, поэтому клиенты не получают ECH ключи для наших доменов |
| **Кэшированный QUIC** | Браузер может помнить, что QUIC работал раньше | UDP sink заставляет падать; браузер откатывается на TCP |

### Отключение QUIC в браузерах (опционально)

Если вы хотите полностью отключить QUIC на стороне клиента:

- **Chrome/Edge:** `chrome://flags` → поиск "QUIC" → установить "Experimental QUIC protocol" в **Disabled**
- **Firefox:** `about:config` → `network.http.http3.enable` → **false**

---

## Требования

- Go 1.21+
- Linux: порты 53, 80, 443 (или используйте `CAP_NET_BIND_SERVICE` / запуск от root для <1024)

---

## Конфигурация

**Конфигурация по умолчанию:** IP по умолчанию для спуфа - `95.164.123.192`. Чтобы использовать другой IP, используйте один из способов:

1. **Переменная окружения**:
   ```bash
   export DNS_SPOOFER_IP=YOUR_SERVER_IP
   ```

2. **Флаг командной строки**:
   ```bash
   ./dnsspoofer -spoof-ip=YOUR_SERVER_IP
   ```

3. **Файл сервиса systemd** (см. секцию Systemd ниже)

IP адрес будет возвращаться DNS для всех спуфнутых доменов. Это должен быть публичный IP вашего сервера, где запущен DnsSpoofer.

---

## Сборка

```bash
# Локально
go build -o dnsspoofer .

# Linux (например, Ubuntu сервер)
GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o dnsspoofer-linux-amd64 .

# Или используйте Makefile
make build        # локально
make build-linux  # linux/amd64
make build-ubuntu # linux/amd64 (алиас)

# Или используйте скрипт сборки
./scripts/build-ubuntu.sh
```

---

## Использование

**IP по умолчанию:** IP по умолчанию для спуфа - `95.164.123.192`. Чтобы использовать другой IP, используйте:
- Переменную окружения: `DNS_SPOOFER_IP=YOUR_SERVER_IP`
- Флаг командной строки: `-spoof-ip=YOUR_SERVER_IP`

```bash
# Используя переменную окружения
export DNS_SPOOFER_IP=YOUR_SERVER_IP
sudo ./dnsspoofer

# Используя флаг командной строки
sudo ./dnsspoofer -spoof-ip=YOUR_SERVER_IP

# Кастомные порты
./dnsspoofer \
  -spoof-ip=YOUR_SERVER_IP \
  -dns-port=:53 \
  -http-port=:80 \
  -https-port=:443

# Кастомный список доменов (суффиксы через запятую)
./dnsspoofer -spoof-ip=YOUR_SERVER_IP -spoof-suffixes=".openai.com,.chatgpt.com,.cursor.sh"

# Все флаги
./dnsspoofer -h
```

| Флаг | По умолчанию | Описание |
|------|---------|-------------|
| `-spoof-ip` | `$DNS_SPOOFER_IP` или `95.164.123.192` | IP, возвращаемый для спуфнутых доменов (по умолчанию: 95.164.123.192, можно переопределить через переменную `DNS_SPOOFER_IP` или флаг) |
| `-dns-port` | `:53` | Адрес прослушивания DNS |
| `-http-port` | `:80` | Адрес прослушивания HTTP прокси |
| `-https-port` | `:443` | Адрес прослушивания HTTPS прокси (TCP) |
| `-udp-sink-port` | `:443` | Адрес прослушивания UDP sink (отбрасывает QUIC/HTTP3 трафик) |
| `-spoof-suffixes` | (см. выше) | Суффиксы доменов для спуфа через запятую |
| `-upstream-dns` | `8.8.8.8:53,1.1.1.1:53` | Upstream DNS для не-спуфнутых + failover |
| `-resolver-dns` | `8.8.8.8:53` | DNS, используемый прокси для резолва бэкендов (избегает циклов) |

---

## Systemd (Linux)

Скопируйте бинарник и unit файл:

```bash
sudo cp dnsspoofer-linux-amd64 /usr/local/bin/dnsspoofer
sudo chmod +x /usr/local/bin/dnsspoofer
sudo cp dnsspoofer.service /etc/systemd/system/
```

Файл сервиса использует IP по умолчанию `95.164.123.192`. Чтобы использовать другой IP, отредактируйте `/etc/systemd/system/dnsspoofer.service` и раскомментируйте:

```ini
Environment=DNS_SPOOFER_IP=YOUR_SERVER_IP
```

Замените `YOUR_SERVER_IP` на реальный IP адрес вашего сервера. Альтернативно, можно использовать файл окружения:

1. Создайте `/etc/dnsspoofer.conf`:
   ```ini
   DNS_SPOOFER_IP=YOUR_SERVER_IP
   ```

2. Раскомментируйте строку `EnvironmentFile` в файле сервиса:
   ```ini
   EnvironmentFile=/etc/dnsspoofer.conf
   ```

Затем перезагрузите и перезапустите:

```bash
sudo systemctl daemon-reload
sudo systemctl enable dnsspoofer
sudo systemctl start dnsspoofer
sudo systemctl status dnsspoofer
```

Логи: `journalctl -u dnsspoofer -f`

---

## Развёртывание

### Первичная установка (bootstrap)

Для свежего сервера, где DnsSpoofer никогда не устанавливался:

```bash
./scripts/bootstrap-server.sh [SERVER]
# Или с флагом force для переустановки
./scripts/bootstrap-server.sh --force [SERVER]
```

Скрипт bootstrap будет:
- Проверять наличие systemd
- Собирать Linux бинарник
- Устанавливать бинарник и systemd unit
- Останавливать и отключать systemd-resolved (освобождает порт 53)
- Включать и запускать сервис

### Обновление существующей установки

Для серверов, где DnsSpoofer уже установлен:

```bash
./scripts/deploy.sh [SERVER]
# Или используйте Makefile
make deploy
```

Скрипт deploy будет:
- Собирать Linux бинарник
- Загружать и устанавливать бинарник и unit файл
- Перезапускать сервис

Оба скрипта поддерживают:
- Аргумент командной строки: `./scripts/deploy.sh root@192.168.1.1`
- Переменные окружения: `DNS_SPOOFER_SERVER=192.168.1.1 ./scripts/deploy.sh`
- Локальный конфиг файл: `scripts/deploy.local` (в gitignore, положите учётные данные туда)

См. заголовки скриптов для полных деталей использования.

---

## Инструменты отладки

Проект включает утилиты отладки в `scripts/debug/` для диагностики и мониторинга:

### Получение debug логов с сервера

Загружает детальные debug логи с сервера, когда установлена переменная окружения `DEBUG_LOG_PATH`.

**Настройка на сервере:**
1. Отредактируйте `/etc/systemd/system/dnsspoofer.service` и добавьте в секцию `[Service]`:
   ```ini
   Environment=DEBUG_LOG_PATH=/tmp/dnsspoofer_debug.log
   ```
2. Перезапустите сервис: `sudo systemctl restart dnsspoofer`

**Использование:**
```bash
./scripts/debug/fetch-server-debug.sh [файл_вывода]
# По умолчанию: .cursor/debug_server.log
```

Скрипт подключается к серверу по SSH и загружает файл debug лога. Требует конфигурацию `scripts/deploy.local` или доступ по SSH ключу.

### Просмотр логов ошибок

Фильтрует и показывает ошибки, проблемы с соединениями и закрытия туннелей из логов сервера.

**Использование:**
```bash
./scripts/debug/logs-errors.sh [строк]
# По умолчанию: последние 500 строк
```

Показывает отфильтрованный вывод для:
- Ошибок
- Таймаутов SNI peek
- Ошибок подключения к бэкенду
- Ошибок резолва
- Закрытий туннелей
- Таймаутов I/O

### Проверка DNS резолвера

Тестирует поведение DNS резолвера при повторных запросах для проверки ротации IP.

**Использование:**
```bash
go run scripts/debug/check_resolver.go
```

Проверяет, какие IP адреса возвращаются при множественных DNS запросах одного и того же хоста. Полезно для отладки балансировки нагрузки и выбора IP.

**Примечание:** Все инструменты отладки требуют доступа к серверу через конфигурацию `scripts/deploy.local` или SSH ключи.

---

## Как это работает

- **DNS:** [miekg/dns](https://github.com/miekg/dns) для UDP сервера и upstream `Exchange()`. Сопоставление суффиксов без учёта регистра; A записи спуфятся, AAAA возвращает пусто (принудительный IPv4), HTTPS/SVCB возвращают NODATA (блокируют QUIC подсказки).
- **SNI:** Подглядывание TLS ClientHello через `crypto/tls` + фейковый read-only `net.Conn` и `GetConfigForClient`; байты воспроизводятся к бэкенду с `io.TeeReader` / `io.MultiReader`.
- **Прокси:** Резолвит хост бэкенда с выделенным резолвером, указывающим на `-resolver-dns`, чтобы хост никогда не резолвился через ваш собственный DNS (без циклов). Затем сырой `io.Copy` клиент ↔ бэкенд.
- **UDP Sink:** Простой `net.ListenUDP`, который читает и отбрасывает все пакеты. Заставляет QUIC падать, вызывая откат на TCP.

---

**[Kakadu Secure Technologies](https://github.com/Matrena-VPN)**
