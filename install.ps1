install.ps1 param (
    [string]$URL = "https://hostpulse.link/", # Базовый URL бэкенда HostPulse
    [string]$TOKEN = "",
    [string]$PASSWORD = ""
)
# Принудительно включаем UTF-8 кодировку для отображения текста без знаков вопроса
[console]::InputEncoding = [System.Text.Encoding]::UTF8
[console]::OutputEncoding = [System.Text.Encoding]::UTF8
$ErrorActionPreference = "Stop"

# 1. Проверяем права Администратора
$isAdmin = ([Security.Principal.WindowsPrincipal][Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
if (!$isAdmin) {
     Write-Output "Error: Run as Administrator required!"
    exit
}

$TargetDir = "C:\Program Files\HostPulse"
$ServiceName = "HostPulseWindowsHardwareAgent"

Write-Output "[HostPulse] Starting Hardware Agent installation..."

# 2. Если старая служба уже существует — останавливаем и удаляем её
if (Get-Service -Name $ServiceName -ErrorAction SilentlyContinue) {
    Write-Output "Old version found. Reinstalling..."
    Stop-Service -Name $ServiceName -Force -ErrorAction SilentlyContinue
    Start-Sleep -Seconds 2

    # Безопасное удаление старой службы через sc.exe
    & sc.exe delete $ServiceName | Out-Null
}

# 3. Создаем рабочую директорию, если её нет
if (!(Test-Path $TargetDir)) {
    New-Item -ItemType Directory -Force -Path $TargetDir | Out-Null
}

# 4. Скачиваем свежий скомпилированный EXE-файл агента из релизов GitHub
$AgentDownloadUrl = "https://github.com/HOST-PULSE/hostpulse-windows-hardware-agent/releases/download/v1.0.1/windows-hardware-agent.exe"
$AgentPath = "$TargetDir\hostpulse_agent.exe"

Write-Output "Downloading agent binary..."

try {
    Invoke-WebRequest -Uri $AgentDownloadUrl -OutFile $AgentPath -UseBasicParsing
} catch {
    (New-Object System.Net.WebClient).DownloadFile($AgentDownloadUrl, $AgentPath)
}

# 5. Скачиваем NSSM по жесткому абсолютному пути
$NssmPath = "$TargetDir\nssm.exe"
if (!(Test-Path $NssmPath)) {
    Write-Output "Downloading NSSM component..."
    $NssmUrl = "https://raw.githubusercontent.com/HOST-PULSE/hostpulse-windows-hardware-agent/main/nssm.exe"
    try {
        Invoke-WebRequest -Uri $NssmUrl -OutFile $NssmPath -UseBasicParsing
    } catch {
        (New-Object System.Net.WebClient).DownloadFile($NssmUrl, $NssmPath)
    }
}

# Дополнительная проверка на физическое наличие файлов на диске перед установкой
if (!(Test-Path $NssmPath) -or !(Test-Path $AgentPath)) {
    Write-Output "Error: Critical components download failed!"
    exit 1
}

Write-Output "Registering Windows Service..."

# 6. Создаем службу через NSSM
& $NssmPath install $ServiceName $AgentPath | Out-Null
& $NssmPath set $ServiceName Description "HostPulse Windows Hardware Monitoring Agent (CPU Temp, Fans, SMART)" | Out-Null
& $NssmPath set $ServiceName AppDirectory $TargetDir | Out-Null
& $NssmPath set $ServiceName Start SERVICE_AUTO_START | Out-Null

# 7. Устанавливаем переменные окружения для службы
# Исправлено: Убрали "api/v1/metrics/", так как Go-код сам добавляет нужные эндпоинты
$EnvPayload = @(
    "HOSTPULSE_HARDWARE_URL=$URL",
    "HOSTPULSE_TOKEN=$TOKEN",
    "HOSTPULSE_SECRET=$PASSWORD"
) -join "`n"

& $NssmPath set $ServiceName AppEnvironmentExtra $EnvPayload | Out-Null

# 8. Запускаем службу
Start-Service -Name $ServiceName

Write-Output "[SUCCESS] HostPulse Windows Hardware Agent successfully installed and started!"
