package main

import (

	"io"
	"log"
	"os"
	"path/filepath"
	// Замените на ваш актуальный модуль из go.mod
	//"windows-hardware-agent/services"
)
func initLogger() {
	// Жесткий, абсолютный путь к файлу лога.
	// Служба LocalSystem гарантированно запишет сюда данные без вызова os.Executable()
	logFilePath := "C:\\Program Files\\HostPulse\\agent.log"

	// Открываем или создаем файл agent.log в режиме добавления записи
	logFile, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		// Откат во временную папку ОС, если к Program Files нет доступа
		altPath := filepath.Join(os.TempDir(), "hostpulse_hardware_agent.log")
		logFile, _ = os.OpenFile(altPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	}

	// Настраиваем стандартный пакет log на запись одновременно в консоль и в файл
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
		log.Println(" [⚠️ WARNING] HOSTPULSE_TOKEN не задан. Используется дефолтный токен.\n")
		agentToken = "default_hardware_token_123"
	}

	log.Printf(" [INFO] Базовый эндпоинт: %s\n", metricsURL)
	//services.SendStaticHardwareInfo(metricsURL, agentToken)
	// Запускаем каждый воркер в своей отдельной горутине
	//go services.StartCpuPoller(metricsURL, agentToken)
	//go services.StartDiskPoller(metricsURL, agentToken)
	//go services.StartFansPoller(metricsURL, agentToken)

	// Блокируем главный поток, чтобы агент работал бесконечно
	select {}
}

