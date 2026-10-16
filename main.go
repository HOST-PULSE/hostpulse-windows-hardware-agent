package main

import (
	
	"io"
	"log"
	"os"
	"path/filepath"
	"windows-hardware-agent/services"
)

func initLogger() {
	targetDir := "C:\\Program Files\\HostPulse"
	if _, err := os.Stat(targetDir); os.IsNotExist(err) {
		targetDir = "."
	}
	logFilePath := filepath.Join(targetDir, "agent.log")

	// Открываем файл. Если его нет, Go попытается его создать.
	logFile, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		// Если Windows блокирует создание в Program Files, откатываемся в Temp
		altPath := filepath.Join(os.TempDir(), "hostpulse_agent.log")
		logFile, _ = os.OpenFile(altPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	}

	// Настраиваем запись одновременно в консоль и в файл
	mw := io.MultiWriter(os.Stdout, logFile)
	log.SetOutput(mw)
	log.SetFlags(log.LstdFlags | log.Lshortfile)
}

func main() {
	initLogger()

	log.Println("\n=======================================================")
	log.Println("=== Агент мониторинга железа HostPulse под Windows запущен ===")
	log.Println("=======================================================")

	metricsURL := os.Getenv("HOSTPULSE_HARDWARE_URL")
	if metricsURL == "" {
		metricsURL = "https://zedform.kz"
	}

	agentToken := os.Getenv("HOSTPULSE_TOKEN")
	if agentToken == "" {
		log.Println(" [⚠️ WARNING] HOSTPULSE_TOKEN не задан. Используется дефолтный токен.")
		agentToken = "default_hardware_token_123"
	}

	log.Printf(" [INFO] Базовый эндпоинт: %s\n", metricsURL)

	// Отправляем паспорт сервера
	services.SendStaticHardwareInfo(metricsURL, agentToken)

	// Запускаем воркеры сбора метрик в фоновых горутинах
	go services.StartDiskPoller(metricsURL, agentToken)
	go services.StartFansPoller(metricsURL, agentToken)

	// Блокируем главный поток для непрерывной работы службы
	select {}
}
