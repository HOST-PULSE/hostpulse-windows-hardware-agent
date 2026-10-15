### HostPulse Hardware Windows Metrics Agent 📊

### Choose language / Выберите язык / Тілді таңдаңыз

* 🇷🇺 [Русский](#-русский)
* 🇬🇧 [English](#-english)
* 🇰🇿 [Қазақша](#-қазақша)

---

### 🇷🇺 Русский

Легковесный и производительный агент для глубокого мониторинга аппаратных ресурсов (Hardware) серверов и рабочих станций на базе Windows. Разработан на языке Go. Он собирает детальные метрики процессора, температурных датчиков, скоростей кулеров и S.M.A.R.T. статусов дисков, работая в фоне как полноценная служба Windows (Windows Service) в экосистеме HostPulse.link.

### ✨ Особенности

* ⚡ **Асинхронные воркеры**: сбор и отправка метрик распределены по разным временным интервалам (от 5 секунд до 5 минут) для снижения нагрузки на сеть и базу данных.
* 🛡 **Кроссплатформенная архитектура**: структура данных спроектирована с прицелом на будущую поддержку Linux и FreeBSD (унифицированные JSON-пакеты).
* 🔒 **Авторизация на уровне заголовков**: агент безопасно передает токен в HTTP-заголовках (`X-Agent-Token`) и идентифицирует ОС через `X-Agent-Type`.
* 📦 **Управление через NSSM**: для обеспечения стабильности и автоматического перезапуска при сбоях агент упаковывается в фоновую службу с помощью встроенной утилиты NSSM (Non-Sucking Service Manager).

### 📊 Собираемые метрики и интервалы

Агент распределяет сбор данных по четырем независимым потокам:

1. **Метрики CPU (Каждые 5 секунд)** — отправка на `/api/hardware/cpu`. Содержит массив температур по каждому ядру процессора (`temperatures`).
2. **Метрики вентиляторов (Каждые 15 секунд)** — отправка на `/api/hardware/fans`. Содержит показатели скорости вращения кулеров в RPM (`fans`).
3. **Состояние дисков (Каждые 5 минут)** — отправка на `/api/hardware/disks`. Содержит полную диагностику, температуры накопителей и S.M.A.R.T. статусы (`disks`).
4. **Паспорт системы (1 раз при старте)** — отправка на `/api/hardware/info`. Передает статическую информацию: модель процессора, суммарный объем ОЗУ, данные материнской платы (`info`).

### 🛠 Быстрый запуск

Рекомендуемый способ запуска — использование автоматического скрипта установки, который скачивает бинарник, скомпилированный через GitHub Actions, и регистрирует службу Windows.

Вам достаточно открыть командную строку (`cmd.exe`) или PowerShell **от имени Администратора** и выполнить одну команду (все переменные и UUID генерируются вашей панелью HostPulse):

```powershell
powershell -Command "Set-ExecutionPolicy Bypass -Scope Process -Force; [System.Net.ServicePointManager]::SecurityProtocol = [System.Net.SecurityProtocolType]::Tls12; $cmd = [scriptblock]::Create((New-Object System.Net.WebClient).DownloadString('https://githubusercontent.com')); & $cmd -URL 'https://hostpulse.link' -TOKEN 'your-server-uuid-token' -PASSWORD 'your-secure-password'"
```

**Важно:** Запуск строго от имени Администратора обязателен, так как скрипту необходимы права для создания системной службы в директории `C:\Program Files\HostPulse`.

### ⚙️ Переменные окружения

Переменные прописываются утилитой NSSM напрямую в контекст службы (AppEnvironmentExtra):
* `HOSTPULSE_BASE_URL` — Базовый URL панели (например, `https://hostpulse.link`), к которому агент автоматически добавляет эндпоинты `/api/hardware/...`.
* `HOSTPULSE_TOKEN` — Уникальный токен сервера для авторизации в заголовке `X-Agent-Token`.

---

### 🇬🇧 English

A lightweight and high-performance agent for deep hardware resource monitoring of Windows-based servers and workstations. Developed in Go, it collects detailed metrics from the CPU, temperature sensors, fan speeds, and disk S.M.A.R.T. statuses. It runs seamlessly in the background as a native Windows Service within the HostPulse.link ecosystem.

### ✨ Features

* ⚡ **Asynchronous Workers**: Telemetry collection and scheduling are split into independent time intervals (from 5 seconds to 5 minutes) to dramatically reduce network and DB overhead.
* 🛡 **Cross-Platform Architecture**: Data structures are pre-engineered with future Linux and FreeBSD support in mind (via unified flexible JSON payloads).
* 🔒 **Header-Based Authentication**: The agent securely transmits its authorization token inside HTTP headers (`X-Agent-Token`) and explicitly declares its OS via `X-Agent-Type`.
* 📦 **NSSM-Managed Service**: Packaged as a reliable background service via embedded NSSM (Non-Sucking Service Manager) to guarantee automatic restarts on failure.

### 📊 Collected Metrics & Intervals

The agent orchestrates data collection across four separate background routines:

1. **CPU Metrics (Every 5 seconds)** — Sent to `/api/hardware/cpu`. Contains an array of core-specific temperature metrics (`temperatures`).
2. **Fan Metrics (Every 15 seconds)** — Sent to `/api/hardware/fans`. Tracks cooling fan rotation speeds in RPM (`fans`).
3. **Disk Status (Every 5 minutes)** — Sent to `/api/hardware/disks`. Collects health diagnostics, drive temperatures, and S.M.A.R.T. records (`disks`).
4. **System Hardware Info (Once at startup)** — Sent to `/api/hardware/info`. Transmits baseline physical specs: CPU model, total RAM capacity, motherboard details (`info`).

### 🛠 Quick Start

The recommended installation method is using the automated setup script that pulls the executable binary pre-built via GitHub Actions and registers the Windows service.

Simply open Command Prompt (`cmd.exe`) or PowerShell **as Administrator** and execute a single command (all arguments are automatically generated by your HostPulse dashboard):

```powershell
powershell -Command "Set-ExecutionPolicy Bypass -Scope Process -Force; [System.Net.ServicePointManager]::SecurityProtocol = [System.Net.SecurityProtocolType]::Tls12; $cmd = [scriptblock]::Create((New-Object System.Net.WebClient).DownloadString('https://githubusercontent.com')); & $cmd -URL 'https://hostpulse.link' -TOKEN 'your-server-uuid-token' -PASSWORD 'your-secure-password'"
```

**Important:** Running explicitly as Administrator is strictly required since the script needs elevated privileges to create a system service inside `C:\Program Files\HostPulse`.

### ⚙️ Environment Variables

Variables are securely injected by NSSM directly into the service runtime context (`AppEnvironmentExtra`):
* `HOSTPULSE_BASE_URL` — The root panel URL (e.g., `https://hostpulse.link`), onto which the agent dynamically appends `/api/hardware/...` paths.
* `HOSTPULSE_TOKEN` — Unique server token used for authorization in the `X-Agent-Token` header.

---

### 🇰🇿 Қазақша

Windows негізіндегі серверлер мен жұмыс станцияларының аппараттық ресурстарын (Hardware) терең бақылауға арналған жеңіл және өнімділігі жоғары агент. Go тілінде жасалған. Ол процессордың, температуралық датчиктердің, кулерлер жылдамдығының және дисткилердің S.M.A.R.T. статустарының егжей-тегжейлі метрикаларын жинайды, HostPulse.link экожүйесінде толыққанды Windows қызметі (Windows Service) ретінде фондық режимде жұмыс істейді.

### ✨ Ерекшеліктері

* ⚡ **Асинхронды воркерлер**: желі мен дерекқорға түсетін жүктемені азайту үшін метрикаларды жинау және жіберу әртүрлі уақыт аралықтарына (5 секундтан 5 минутқа дейін) бөлінген.
* 🛡 **Кроссплатформалық архитектура**: деректер құрылымы болашақта Linux және FreeBSD жүйелерін қолдауды ескере отырып жобаланған (бірыңғай JSON-пакеттер).
* 🔒 **Тақырыптар деңгейіндегі авторизация**: агент токенді HTTP-тақырыптарында (`X-Agent-Token`) қауіпсіз түрде тасымалдайды және ОЖ-ны `X-Agent-Type` арқылы анықтайды.
* 📦 **NSSM арқылы басқару**: қателіктер кезінде автоматты түрде қайта іске қосылуды қамтамасыз ету үшін агент кірістірілген NSSM (Non-Sucking Service Manager) утилитасы көмегімен жүйелік қызметке оралады.

### 📊 Жиналатын метрикалар мен аралықтар

Агент деректерді жинауды төрт тәуелсіз ағынға бөледі:

1. **CPU метрикалары (Әр 5 секунд сайын)** — `/api/hardware/cpu` мекенжайына жіберіледі. Әрбір процессор ядросы бойынша температуралар жиынын қамтиды (`temperatures`).
2. **Вентилятор метрикалары (Әр 15 секунд сайын)** — `/api/hardware/fans` мекенжайына жіберіледі. Кулерлердің айналу жылдамдығын RPM мәнінде қамтиды (`fans`).
3. **Дисткилердің күйі (Әр 5 минут сайын)** — `/api/hardware/disks` мекенжайына жіберіледі. Толық диагностиканы, дисткилердің температурасын және S.M.A.R.T. статустарын қамтиды (`disks`).
4. **Жүйе паспорты (Іске қосылғанда 1 рет)** — `/api/hardware/info` мекенжайына жіберіледі. Статикалық ақпаратты тасымалдайды: процессор моделі, жедел жадтың жалпы көлемі, аналық плата деректері (`info`).

### 🛠 Жылдам іске қосу

Ұсынылатын іске қосу әдісі — GitHub Actions арқылы автоматты түрде жиналған бинарлық файлды жүктеп алатын және Windows қызметін тіркейтін автоматты орнату скриптін пайдалану.

Пәрмен жолын (`cmd.exe`) немесе PowerShell-ді **Әкімші атынан (Run as Administrator)** ашып, келесі бір пәрменді орындау жеткілікті (барлық айнымалылар HostPulse панелі арқылы автоматты түрде жасалады):

```powershell
powershell -Command "Set-ExecutionPolicy Bypass -Scope Process -Force; [System.Net.ServicePointManager]::SecurityProtocol = [System.Net.SecurityProtocolType]::Tls12; $cmd = [scriptblock]::Create((New-Object System.Net.WebClient).DownloadString('https://githubusercontent.com')); & $cmd -URL 'https://hostpulse.link' -TOKEN 'your-server-uuid-token' -PASSWORD 'your-secure-password'"
```

**Маңызды:** Іске қосу қатаң түрде Әкімші атынан орындалуы тиіс, өйткені скриптке `C:\Program Files\HostPulse` директориясында жүйелік қызметті құру үшін жоғары құқықтар қажет.

### ⚙️ Жүйелік айнымалылар

Айнымалылар NSSM утилитасымен тікелей қызмет контекстіне жазылады (AppEnvironmentExtra):
* `HOSTPULSE_BASE_URL` — Панельдің негізгі URL мекенжайы (мысалы, `https://hostpulse.link`), оған агент автоматты түрде `/api/hardware/...` эндпоинттерін қосады.
* `HOSTPULSE_TOKEN` — `X-Agent-Token` тақырыбында авторизациялауға арналған сервердің бірегей токені.
