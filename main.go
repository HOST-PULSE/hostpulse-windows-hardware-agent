package main

import (
	_ "embed"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"windows-hardware-agent/services"
)

////go:embed smartctl.exe
//var smartctlBytes []byte

// Кастомный писатель, который сбрасывает кэш Windows при каждой строчке лога
type syncWriter struct {
	file *os.File
}

func (sw syncWriter) Write(p []byte) (n int, err error) {
	n, err = sw.file.Write(p)
	if err == nil {
		sw.file.Sync() // Жесткий пробой буфера ОС на диск
	}
	return n, err
}

func initLogger() {
	targetDir := "C:\\Program Files\\HostPulse"
	if _, err := os.Stat(targetDir); os.IsNotExist(err) {
		targetDir = "."
	}
	logFilePath := filepath.Join(targetDir, "agent.log")

	logFile, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		fmt.Printf(" [❌ ERROR] Не удалось открыть лог: %v\n", err)
		return
	}

	// Оборачиваем файл в синхронный писатель
	synchronizedTarget := syncWriter{file: logFile}

	// Связываем MultiWriter
	mw := io.MultiWriter(os.Stdout, synchronizedTarget)
	log.SetOutput(mw)
	log.SetFlags(log.LstdFlags | log.Lshortfile)
}

func main() {
	initLogger()

	log.Println("\n=======================================================")
	log.Println("=== Агент мониторинга железа HostPulse под Windows запущен ===")
	log.Println("=======================================================")

	services.SetSmartctlBinary(smartctlBytes)

	metricsURL := os.Getenv("HOSTPULSE_HARDWARE_URL")
	if metricsURL == "" { metricsURL = "https://zedform.kz" }

	agentToken := os.Getenv("HOSTPULSE_TOKEN")
	if agentToken == "" { agentToken = "default_hardware_token_123" }

	log.Printf(" [INFO] Базовый эндпоинт: %s\n", metricsURL)

	services.SendStaticHardwareInfo(metricsURL, agentToken)

	//go services.StartDiskPoller(metricsURL, agentToken)
	//go services.StartFansPoller(metricsURL, agentToken)

	select {}
}
