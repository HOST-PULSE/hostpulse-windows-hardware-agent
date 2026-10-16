package main

import (
	_ "embed" // Обязательный импорт для работы механизма вшивания файлов
	"fmt"
	"time"
	"log"
	"os"
	"path/filepath"
	"windows-hardware-agent/services"
)

//go:embed smartctl.exe
var smartctlBytes []byte

func WriteToLog(message string) {
	targetDir := "C:\\Program Files\\HostPulse"
	if _, err := os.Stat(targetDir); os.IsNotExist(err) {
		targetDir = "."
	}
	logFilePath := filepath.Join(targetDir, "agent.log")

	f, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return
	}
	defer f.Close()

	timestamp := time.Now().Format("2006-01-02 15:04:05")
	f.WriteString(fmt.Sprintf("[%s] %s\n", timestamp, message))
	f.Sync() // Принудительный жесткий сброс на диск
}

func main() {
	// Инициализируем логирование
	

	// Передаем байты утилиты из корня в пакет services перед стартом опроса дисков
	services.SetSmartctlBinary(smartctlBytes)

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
	// go services.StartCpuPoller(metricsURL, agentToken)
	go services.StartDiskPoller(metricsURL, agentToken)
	go services.StartFansPoller(metricsURL, agentToken)

	// Блокируем главный поток, чтобы агент работал бесконечно в качестве службы
	select {}
}
