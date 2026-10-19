package main

import (
	_ "embed"
	"fmt"
	"io"
	"log"
	"bytes"

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
	// Получаем путь к папке, где лежит запущенный бинарник
	exePath, err := os.Executable()
	if err != nil {
		exePath = "."
	}
	exeDir := filepath.Dir(exePath)
	logFilePath := filepath.Join(exeDir, "agent_hardware.log")

	// Открываем или создаем файл agent.log в режиме добавления записи
	logFile, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		fmt.Printf(" [❌ ERROR] Не удалось создать файл лога: %v\n", err)
		return
	}

	// Настраиваем вывод: и в файл, и в стандартную консоль одновременно (MultiWriter)
	mw := io.MultiWriter(os.Stdout, logFile)
	log.SetOutput(mw)

	// Перенаправляем стандартные системные выводы операционной системы (Опционально)
	os.Stdout = logFile
	os.Stderr = logFile
	log.SetOutput(logFile)
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.SetPrefix("")
	// Добавьте эту строку ниже:
	log.SetOutput(&winLineFixer{logFile})
}
func (w *winLineFixer) Write(p []byte) (n int, err error) {
	// Если строка заканчивается на \n, но перед ним нет \r, заменяем на \r\n
	if len(p) > 0 && p[len(p)-1] == '\n' && (len(p) == 1 || p[len(p)-2] != '\r') {
		fixed := bytes.ReplaceAll(p, []byte("\n"), []byte("\r\n"))
		_, err := w.Writer.Write(fixed)
		return len(p), err
	}
	return w.Writer.Write(p)
}
func main() {
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
	//services.SendStaticHardwareInfo(metricsURL, agentToken)
	// Запускаем каждый воркер в своей отдельной горутине
	go services.StartCpuPoller(metricsURL, agentToken)
	go services.StartDiskPoller(metricsURL, agentToken)
	go services.StartMemoryHardwarePoller(metricsURL, agentToken)

	// Блокируем главный поток, чтобы агент работал бесконечно
	select {}
}

