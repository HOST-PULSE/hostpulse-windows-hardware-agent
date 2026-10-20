### HostPulse Windows Hardware Agent 📊

### Choose language / Выберите язык / Тілді таңдаңыз

* 🇷🇺 [Русский](#-русский)
* 🇬🇧 [English](#-english)
* 🇰🇿 [Қазақша](#-қазақша)

---

### 🇷🇺 Русский

Низкоуровневый аппаратный агент для глубокого мониторинга физического состояния серверов на базе Windows Server / Windows Desktop. Разработан на языке Go с использованием гибридных вызовов Win32 API, С# драйверов ядра и консольных утилит инвентаризации. Преддназначен для выявления скрытых аппаратных дефектов на серверах автоматизации (iiko, бэк-офисы) до того, как они приведут к критическому сбою бизнес-логики. Работает в фоне как полноценная системная служба.

### ✨ Особенности

* 🦾 **Глубокий аппаратный анализ**: агент не просто считывает загрузку системы, а опрашивает контроллеры дисков, ECC-памяти, датчики температур и линии питания.
* 📦 **Автономность и инкапсуляция**: все необходимые бинарные зависимости (`smartctl.exe`, `OpenHardwareMonitorLib.dll`) вшиты напрямую в тело Go-агента через директиву `//go:embed` и извлекаются в память процесса автоматически.
* 🛡 **Отказоустойчивость**: методы сбора имеют дублирующие ветки (WMI, CIM-парсеры, прямой реестр Windows), гарантируя стабильную работу агента как на голом железе (Bare Metal), так и в изолированных виртуальных машинах.
* 🔒 **Безопасный скрытый режим**: скомпилирован с флагами отсечения консольного окна (`-H=windowsgui`), работает полностью незаметно для конечного пользователя системы.

### 📊 Собираемые аппаратные метрики

Агент разделяет сбор данных на быстрые и медленные циклы для минимизации нагрузки на хост-машину:

1. **Мониторинг процессора (каждые 5 секунд)**:
    * Поядерный съем реальной температуры в градусах Цельсия (°C).
    * Автоматический расчет и кэширование пикового нагрева для триггеров защиты от перегрева.
2. **Паспорт и здоровье дисков (каждые 5 минут)**:
    * Извлечение SMART-атрибутов: переназначенные сектора (`Reallocated Sectors`), нестабильные сектора (`Bad Sectors Pending`), ошибки кабеля (`CRC Errors`).
    * Полная инвентаризация: серийный номер накопителя (`serial_number`) и версия заводской прошивки (`firmware_version`) для быстрой замены сбойного диска в стойке.
3. **Паспорт оперативной памяти (каждые 5 минут)**:
    * Покомпонентная структура слотов материнской платы (`DIMM 0`, `DIMM 1`).
    * Фиксация производителя чипов, частоты работы шины в МГц и аппаратных счетчиков ECC-ошибок.
4. **Резервирование электропитания (каждые 5 минут)**:
    * Опрос статуса активности блоков питания (`DeviceID`).
    * Мгновенный алерт при потере фазы, отключении кабеля или аварии одного из БП в резервированной цепи.
5. **Физическая сеть и линки (каждые 5 минут)**:
    * Контроль физической скорости адаптеров (`speed_mbps`) с отсечением виртуальных VPN-хвостов.
    * Фиксация деградации сети (например, если гигабитный порт из-за поврежденного кабеля зажался в аварийные 100 Mbps).
6. **Статический паспорт инвентаризации (1 раз при старте)**:
    * Модель CPU, объем RAM, модель материнской платы и точная сборка операционной системы через вызов ядра.

### 🛠 Быстрый запуск

Агент разворачивается в систему автоматически с помощью PowerShell скрипта инсталляции HostPulse. Для ручной отладки или запуска пропишите необходимые системные переменные.

**Требования к запуску**: строго от имени **Администратора** операционной системы (необходимо для загрузки низкоуровневых драйверов мониторинга чипсета в ядро Windows).

### ⚙️ Переменные окружения

* `HOSTPULSE_HARDWARE_URL` — Целевой API эндпоинт вашей CRM для приема аппаратных JSON-пакетов (например, `https://your-domain.com`).
* `HOSTPULSE_TOKEN` — Уникальный мастер-токен авторизации сервера, сгенерированный в панели управления CRM.

---

### 🇬🇧 English

A low-level hardware telemetry agent designed for deep physical health monitoring of Windows Server and Windows Desktop machines. Developed in Go, it leverages a hybrid stack of Win32 API calls, embedded C# kernel-level drivers, and native CIM inventory parsers. It is built to detect hidden hardware degradation on automation servers (such as iiko points) before it results in critical business downtime. Runs seamlessly as a background system service.

### ✨ Features

* 🦾 **Deep Hardware Inspection**: Beyond surface-level OS statistics, the agent queries storage controllers, ECC memory banks, thermal sensors, and power lines.
* 📦 **Self-Contained Architecture**: Core dependencies (`smartctl.exe`, `OpenHardwareMonitorLib.dll`) are natively compiled inside the Go binary via `//go:embed` and extracted into runtime memory automatically.
* 🛡 **Fault-Tolerant Fallbacks**: Telemetry routines use multi-layered extraction paths (WMI, CIM-queries, direct Windows Registry access), ensuring stable operation across both Bare Metal hardware and hypervisor environments.
* 🔒 **Stealth Background Execution**: Built with strict GUI-suppression flags (`-H=windowsgui`) to remain completely invisible to the end-user.

### 📊 Collected Hardware Metrics

Telemetry collection is split into high-frequency and low-frequency pollers to ensure near-zero overhead:

1. **CPU Telemetry (Every 5 seconds)**:
    * Per-core temperature reading in Celsius (°C).
    * Real-time calculation of peak thermal layout to trigger critical overheat alert sequences.
2. **Storage SMART & Inventory (Every 5 minutes)**:
    * Extraction of raw SMART attributes: `Reallocated Sectors`, `Bad Sectors Pending`, and interface `CRC Errors`.
    * Complete inventory profile: drive `serial_number` and factory `firmware_version` to pinpoint the exact failing physical drive in the server rack.
3. **RAM Layout & ECC Health (Every 5 minutes)**:
    * Motherboard memory bank slot mapping (`DIMM 0`, `DIMM 1`).
    * Logging of chip manufacturer, bus clock speed (MHz), and hardware ECC error correction counters.
4. **Power Supply Redundancy (Every 5 minutes)**:
    * Health and availability tracking of power units (`DeviceID`).
    * Real-time alerting upon cable disconnection, power line drops, or single PSU failures within a redundant circuit.
5. **Network Interface Link Speed (Every 5 minutes)**:
    * Hard-coded validation of physical link speed (`speed_mbps`) with clean extraction of virtual VPN/TAP adapters.
    * Trapping of link degradation states (e.g., a gigabit interface dropping to 100 Mbps due to a crimping fault or broken RJ-45 latch).
6. **Static Inventory Passport (Once at startup)**:
    * Core CPU model, total RAM, motherboard model, and Windows OS build specification.

### 🛠 Quick Start

The executable is deployed autonomously via the HostPulse PowerShell installation manager. For manual testing or debugging, ensure the binary is executed with elevated privileges.

**Execution requirement**: Must be executed strictly **As Administrator** to allow the agent to register and load ring-0 kernel drivers for chipset communication.

### ⚙️ Environment Variables

* `HOSTPULSE_HARDWARE_URL` — Target API endpoint of your CRM platform (e.g., `https://your-domain.com`).
* `HOSTPULSE_TOKEN` — Unique UUID authorization token linked to the server asset inside the CRM.

---

### 🇰🇿 Қазақша

Windows Server / Windows Desktop негізіндегі серверлердің физикалық жағдайын терең бақылауға арналған төмен деңгейлі аппараттық агент. Win32 API гибридті шақыруларын, С# ядро деңгейіндегі драйверлерін және жүйелік CIM инвентаризация парсерлерін қолдану арқылы Go тілінде жасалған. Автоматтандыру серверлеріндегі (iiko нүктелері, бэк-офистер) жабдықтың жасырын ақауларын олар бизнес-логиканың критикалық тоқтауына әкеп соқтырғанға дейін анықтауға арналған. Фондық режимде толыққанды жүйелік қызмет ретінде жұмыс істейді.

### ✨ Ерекшеліктері

* 🦾 **Терең аппараттық талдау**: агент тек жүйенің жүктелуін оқып қоймайды, сонымен қатар диск контроллерлерін, ECC жадын, температура датчиктерін және қуат беру желілерін тексереді.
* 📦 **Автономдылық және инкапсуляция**: барлық қажетті бинарлық тәуелділіктер (`smartctl.exe`, `OpenHardwareMonitorLib.dll`) `//go:embed` директивасы арқылы тікелей Go агентінің денесіне енгізілген және процесс жадына автоматты түрде шығарылады.
* 🛡 **Ақауларға төзімділік**: мәліметтерді жинау әдістерінің дублирленген тармақтары бар (WMI, CIM парсерлері, тікелей Windows тізілімі), бұл агенттің нақты жабдықта (Bare Metal) да, оқшауланған виртуалды машиналарда да тұрақты жұмыс істеуіне кепілдік береді.
* 🔒 **Қауіпсіз жасырын режим**: консольдік терезені жасыру жалаушаларымен (`-H=windowsgui`) компиляцияланған, жүйенің соңғы пайдаланушысына толықтай байқалмай жұмыс істейді.

### 📊 Жиналатын аппараттық метрикалар

Агент хост-машинаға түсетін жүктемені азайту үшін деректерді жинауды жылдам және баяу циклдарға бөледі:

1. **Процессор мониторингі (әр 5 секунд сайын)**:
    * Цельсий градусымен (°C) ядролардың нақты температурасын алу.
    * Қызып кетуден қорғау триггерлері үшін ең жоғары қызу деңгейін автоматты түрде есептеу және кэштеу.
2. **Дискілердің паспорты мен денсаулығы (әр 5 минут сайын)**:
    * SMART атрибуттарын алу: қайта тағайындалған секторлар (`Reallocated Sectors`), тұрақсыз секторлар (`Bad Sectors Pending`), кабель қателіктері (`CRC Errors`).
    * Толық инвентаризация: сервер сөресіндегі ақаулы дискті жылдам ауыстыру үшін жетекшінің сериялық нөмірі (`serial_number`) және зауыттық микробағдарлама нұсқасы (`firmware_version`).
3. **Жедел жадтың паспорты (әр 5 минут сайын)**:
    * Аналық плата слоттарының құрылымы (`DIMM 0`, `DIMM 1`).
    * Чип өндірушісін, шинаның жұмыс жиілігін (МГц) және аппараттық ECC қателіктер есептегіштерін тіркеу.
4. **Электр қуатын резервтеу (әр 5 минут сайын)**:
    * Қуат беру блоктарының белсенділік күйін тексеру (`DeviceID`).
    * Резервтелген тізбектегі БП-ның біреуі істен шыққанда, кабель ажыратылғанда немесе қуат жоғалғанда лезде ескерту беру.
5. **Физикалық желі және линктер (әр 5 минут сайын)**:
    * Виртуалды VPN адаптерлерін тазарта отырып, адаптерлердің физикалық жылдамдығын (`speed_mbps`) бақылау.
      • Желінің нашарлауын тіркеу (мысалы, гигабиттік порт зақымдалған кабель салдарынан авариялық 100 Mbps жылдамдыққа түсіп кетсе).
6. Инвентаризацияның статикалық паспорты (іске қосылғанда 1 рет):
   • Ядро шақыруы арқылы CPU моделі, RAM көлемі, аналық плата моделі және операциялық жүйенің нақты нұсқасы.

### 🛠 Жылдам іске қосу

Aгент жүйеге HostPulse PowerShell орнату скриптинің көмегімен автоматты түрде орнатылады. Қолмен баптау немесе іске қосу үшін қажетті жүйелік айнымалыларды көрсетіңіз.
Іске қосу талабы: тек қана операциялық жүйенің Әкімші (Administrator) атынан орындалуы тиіс (чипсетпен байланыс орнату үшін Windows ядросына төмен деңгейлі драйверлерді жүктеу қажет).

###  ⚙️ Айнымалылар

* `HOSTPULSE_HARDWARE_URL` — Аппараттық JSON пакеттерін қабылдауға арналған CRM-нің API эндпоинті (мысалы, `https://your-domain.com`).
* `HOSTPULSE_TOKEN` — CRM басқару панелінде жасалған сервердің бірегей авторизация токені.