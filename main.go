package main

import (
	_ "embed" // Обязательный импорт для работы механизма вшивания файлов
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"windows-hardware-agent/services"
)

//go:embed smartctl.exe
var smartctlBytes []byte

func initLogger() {
	// Получаем путь к папке, где лежит запущенный бинарник
	exePath, err := os.Executable()
	if err != nil {
		exePath = "."
	}
	exeDir := filepath.Dir(exePath)
	logFilePath := filepath.Join(exeDir, "agent.log")

	// Открываем или создаем файл agent.log в режиме добавления записи
	logFile, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		fmt.Printf(" [❌ ERROR] Не удалось создать файл лога: %v\n", err)
		return
	}

	// Настраиваем вывод: и в файл, и в стандартную консоль одновременно (MultiWriter)
	mw := io.MultiWriter(os.Stdout, logFile)
	log.SetOutput(mw)
}

func main() {
	// Инициализируем логирование
	initLogger()

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
