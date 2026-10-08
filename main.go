package main

import (
	"fmt"
	"os"

	// Замените на ваш актуальный модуль из go.mod
	"windows-hardware-agent/services"
)

func main() {
	fmt.Println("=== Агент мониторинга железа HostPulse под Windows запущен ===")

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
	services.SendStaticHardwareInfo(metricsURL, agentToken)
	// Запускаем каждый воркер в своей отдельной горутине
	go services.StartCpuPoller(metricsURL, agentToken)
	go services.StartDiskPoller(metricsURL, agentToken)
	go services.StartFansPoller(metricsURL, agentToken)

	// Блокируем главный поток, чтобы агент работал бесконечно
	select {}
}

