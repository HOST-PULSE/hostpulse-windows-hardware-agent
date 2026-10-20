package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// --- СТРУКТУРЫ ДЛЯ ОТПРАВКИ ---

type CpuPayload struct {
	Token        string          `json:"token"`
	Timestamp    int64           `json:"timestamp"`
	Temperatures []CpuTempStatus `json:"temperatures"`
}

type DiskPayload struct {
	Token     string       `json:"token"`
	Timestamp int64        `json:"timestamp"`
	Disks     []AdvancedDiskStatus `json:"disks"`
}

type MemoryHardwarePayload struct {
	Token     string             `json:"token"`
	Timestamp int64              `json:"timestamp"`
	Memory    []MemorySlotStatus `json:"memory"`
}

type PowerSupplyHardwarePayload struct {
	Token        string              `json:"token"`
	Timestamp    int64               `json:"timestamp"`
	PowerSupplies []PowerSupplyStatus `json:"power_supplies"`
}

type NetworkHardwarePayload struct {
	Token      string                   `json:"token"`
	Timestamp  int64                    `json:"timestamp"`
	Interfaces []NetworkInterfaceStatus `json:"interfaces"`
}
//type InfoPayload struct {
//	Token     string       `json:"token"`
//	Timestamp int64        `json:"timestamp"`
//	Info      HardwareInfo `json:"info"`
//}

// --- ВОРКЕР ДЛЯ CPU (Быстрый: 5 секунд) ---

func StartCpuPoller(baseURL string, token string) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	fullURL := baseURL + "api/v1/hardware/cpu/"
	fmt.Printf(" [Воркер CPU] Запущен. Эндпоинт: %s\n", fullURL)

	// Первый запуск сразу
	sendCpuData(fullURL, token)

	for range ticker.C {
		sendCpuData(fullURL, token)
	}
}

func sendCpuData(url string, token string) {
	// Теперь эта функция никогда не вернет nil на Windows Server 2012 R2
	temps := GetCpuTemperatures()

	payload := CpuPayload{
		Token:        token,
		Timestamp:    time.Now().Unix(),
		Temperatures: temps,
	}

	if jsonData, err := json.Marshal(payload); err == nil {
		sendRequest(url, jsonData, "CPU", token)
	}
}

// --- ВОРКЕР ДЛЯ ДИСКОВ (Медленный: 5 минут) ---

func StartDiskPoller(baseURL string, token string) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	fullURL := baseURL + "api/v1/hardware/disks/"
	fmt.Printf(" [Воркер Disks] Запущен. Эндпоинт: %s\n", fullURL)

	// Первый запуск сразу
	sendDiskData(fullURL, token)

	for range ticker.C {
		sendDiskData(fullURL, token)
	}
}

func sendDiskData(url string, token string) {
	fmt.Println(" [i] Диски")
	disks := GetAdvancedDiskStatus()
	if disks == nil {
		return
	}

	payload := DiskPayload{
		Token:     token,
		Timestamp: time.Now().Unix(),
		Disks:     disks,
	}

	if jsonData, err := json.Marshal(payload); err == nil {
		sendRequest(url, jsonData, "Disks",token)
	}
}

func StartMemoryHardwarePoller(baseURL string, token string) {
	ticker := time.NewTicker(5 * time.Minute) // Опрос раз в 5 минут
	defer ticker.Stop()

	fullURL := baseURL + "api/v1/hardware/memory/"
	fmt.Printf(" [Воркер RAM Железа] Запущен. Эндпоинт: %s\n", fullURL)

	// Первый запуск сразу
	sendMemoryHardwareData(fullURL, token)

	for range ticker.C {
		sendMemoryHardwareData(fullURL, token)
	}
}

func sendMemoryHardwareData(url string, token string) {
	ramSlots := GetMemoryHardwareStatus()
	if len(ramSlots) == 0 {
		return
	}

	payload := MemoryHardwarePayload{
		Token:     token,
		Timestamp: time.Now().Unix(),
		Memory:    ramSlots,
	}

	if jsonData, err := json.Marshal(payload); err == nil {
		sendRequest(url, jsonData, "RAM Hardware", token)
	}
}

func StartPowerSupplyHardwarePoller(baseURL string, token string) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	fullURL := baseURL + "api/v1/hardware/powersupply/"
	fmt.Printf(" [Воркер Питания] Запущен. Эндпоинт: %s\n", fullURL)

	// Первый запуск сразу
	sendPowerSupplyHardwareData(fullURL, token)

	for range ticker.C {
		sendPowerSupplyHardwareData(fullURL, token)
	}
}

func sendPowerSupplyHardwareData(url string, token string) {
	psUnits := GetPowerSupplyHardwareStatus()
	if len(psUnits) == 0 {
		return
	}

	payload := PowerSupplyHardwarePayload{
		Token:         token,
		Timestamp:     time.Now().Unix(),
		PowerSupplies: psUnits,
	}

	if jsonData, err := json.Marshal(payload); err == nil {
		sendRequest(url, jsonData, "Power Supply Hardware", token)
	}
}
func StartNetworkHardwarePoller(baseURL string, token string) {
	// Для отладки ставим 5 секунд, чтобы сразу поймать пакет в Django. В продакшене вернем 5 минут!
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	fullURL := baseURL + "api/v1/hardware/network/"
	fmt.Printf(" [Воркер Сети] Запущен. Эндпоинт: %s\n", fullURL)

	// Первый запуск сразу
	sendNetworkHardwareData(fullURL, token)

	for range ticker.C {
		sendNetworkHardwareData(fullURL, token)
	}
}

func sendNetworkHardwareData(url string, token string) {
	netInterfaces := GetNetworkHardwareStatus()
	if len(netInterfaces) == 0 {
		return
	}

	payload := NetworkHardwarePayload{
		Token:      token,
		Timestamp:  time.Now().Unix(),
		Interfaces: netInterfaces,
	}

	if jsonData, err := json.Marshal(payload); err == nil {
		sendRequest(url, jsonData, "Network Hardware", token)
	}
}
//func SendStaticHardwareInfo(baseURL string, token string) {
//	fmt.Println(" [i] Сбор паспортных данных системы (CPU, RAM, Motherboard)...")
//
//	info := GetHardwareStaticInfo()
//	fullURL := baseURL + "api/v1/hardware/info/"
//
//	payload := InfoPayload{
//		Token:     token,
//		Timestamp: time.Now().Unix(),
//		Info:      info,
//	}
//
//	if jsonData, err := json.Marshal(payload); err == nil {
//		// Использует уже готовую функцию sendRequest из poller.go
//		sendRequest(fullURL, jsonData, "StaticInfo",token)
//	}
//}

// --- ОБЩАЯ СЕТЕВАЯ ФУНКЦИЯ ---

func sendRequest(url string, jsonData []byte, workerName string, token string) {
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		fmt.Printf(" [❌ %s ERROR] Ошибка создания запроса: %v\n", workerName, err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Agent-Type", "agent_hardware_windows")
	req.Header.Set("X-Agent-Token", token)
	client := &http.Client{Timeout: 4 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf(" [❌ %s ERROR] Не удалось отправить данные: %v\n", workerName, err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Printf(" [⚠️ %s WARNING] Сервер вернул статус: %d\n", workerName, resp.StatusCode)
	}
}
