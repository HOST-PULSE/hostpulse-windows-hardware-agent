package main

import (
	"bytes"
	_ "embed"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"windows-hardware-agent/services"
)

//go:embed smartctl.exe
var smartctlBytes []byte

//go:embed OpenHardwareMonitorLib.dll
var ohmLibraryBytes []byte

type winLineFixer struct {
	io.Writer
}

func initLogger() {
	// ИСПРАВЛЕНО: Проверяем переменную окружения для управления логированием
	logsEnable := os.Getenv("HOSTPULSE_LOGS_ENABLE")

	// Если принудительно передано "false", пишем ТОЛЬКО в стандартный вывод консоли (без файла)
	if logsEnable == "false" {
		log.SetOutput(os.Stdout)
		log.SetFlags(log.LstdFlags | log.Lshortfile)
		log.SetPrefix("")
		fmt.Println(" [ℹ️ SYSTEM] Агент запущен в тихом режиме. Запись файла логов отключена.")
		return
	}

	// Штатный режим записи в файл (если переменная true или не задана вовсе)
	exePath, err := os.Executable()
	if err != nil {
		exePath = "."
	}
	exeDir := filepath.Dir(exePath)
	logFilePath := filepath.Join(exeDir, "agent_hardware.log")

	logFile, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		fmt.Printf(" [❌ ERROR] Не удалось создать файл лога: %v\n", err)
		return
	}

	// Настраиваем дублирование: и в stdout консоли, и в файл agent_hardware.log
	mw := io.MultiWriter(os.Stdout, logFile)
	log.SetOutput(mw)

	// Перенаправляем системные выводы операционной системы в файл
	os.Stdout = logFile
	os.Stderr = logFile
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.SetPrefix("")
	log.SetOutput(&winLineFixer{logFile})
}

func (w *winLineFixer) Write(p []byte) (n int, err error) {
	if len(p) > 0 && p[len(p)-1] == '\n' && (len(p) == 1 || p[len(p)-2] != '\r') {
		fixed := bytes.ReplaceAll(p, []byte("\n"), []byte("\r\n"))
		_, err := w.Writer.Write(fixed)
		return len(p), err
	}
	return w.Writer.Write(p)
}

func main() {
	// Инициализация логгера с учетом флага HOSTPULSE_LOGS_ENABLE
	initLogger()

	services.SetSmartctlBinary(smartctlBytes)
	services.SetOhmLibraryBinary(ohmLibraryBytes)

	fmt.Println("\n=======================================================")
	fmt.Println("=== Агент мониторинга железа HostPulse под Windows запущен ===")
	fmt.Println("=======================================================")

	metricsURL := os.Getenv("HOSTPULSE_HARDWARE_URL")
	if metricsURL == "" {
		metricsURL = "https://zedform.kz"
	}

	agentToken := os.Getenv("HOSTPULSE_TOKEN")
	if agentToken == "" {
		fmt.Println(" [⚠️ WARNING] HOSTPULSE_TOKEN не задан. Используется дефолтный токен.")
		agentToken = "default_hardware_token_123"
	}

	fmt.Printf(" [INFO] Базовый эндпоинт: %s\n", metricsURL)

	// Разовый запуск паспорта инвентаризации
	services.SendStaticHardwareInfo(metricsURL, agentToken)

	// Запускаем воркеры в параллельных горутинах
	go services.StartCpuPoller(metricsURL, agentToken)
	go services.StartDiskPoller(metricsURL, agentToken)
	go services.StartMemoryHardwarePoller(metricsURL, agentToken)
	go services.StartPowerSupplyHardwarePoller(metricsURL, agentToken)
	go services.StartNetworkHardwarePoller(metricsURL, agentToken)

	// Блокируем поток для вечной работы
	select {}
}
