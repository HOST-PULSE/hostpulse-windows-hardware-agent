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
	Disks     []DiskStatus `json:"disks"`
}

type FanPayload struct {
	Token     string      `json:"token"`
	Timestamp int64       `json:"timestamp"`
	Fans      []FanStatus `json:"fans"`
}

type InfoPayload struct {
	Token     string       `json:"token"`
	Timestamp int64        `json:"timestamp"`
	Info      HardwareInfo `json:"info"`
}

// --- ВОРКЕР ДЛЯ CPU (Быстрый: 5 секунд) ---

func StartCpuPoller(baseURL string, token string) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	fullURL := baseURL + "/api/hardware/cpu"
	fmt.Printf(" [Воркер CPU] Запущен. Эндпоинт: %s\n", fullURL)

	// Первый запуск сразу
	sendCpuData(fullURL, token)

	for range ticker.C {
		sendCpuData(fullURL, token)
	}
}

func sendCpuData(url string, token string) {
	temps := GetCpuTemperatures()
	if temps == nil {
		return // Если ошибка чтения, не спамим бэкенд пустышками
	}

	payload := CpuPayload{
		Token:        token,
		Timestamp:    time.Now().Unix(),
		Temperatures: temps,
	}

	if jsonData, err := json.Marshal(payload); err == nil {
		sendRequest(url, jsonData, "CPU",token)
	}
}

// --- ВОРКЕР ДЛЯ ДИСКОВ (Медленный: 5 минут) ---

func StartDiskPoller(baseURL string, token string) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	fullURL := baseURL + "/api/hardware/disks"
	fmt.Printf(" [Воркер Disks] Запущен. Эндпоинт: %s\n", fullURL)

	// Первый запуск сразу
	sendDiskData(fullURL, token)

	for range ticker.C {
		sendDiskData(fullURL, token)
	}
}

func sendDiskData(url string, token string) {
	disks := GetSmartStatus()
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

func StartFansPoller(baseURL string, token string) {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	fullURL := baseURL + "/api/hardware/fans"
	fmt.Printf(" [Воркер Fans] Запущен. Эндпоинт: %s\n", fullURL)

	// Первый быстрый запуск
	sendFansData(fullURL, token)

	for range ticker.C {
		sendFansData(fullURL, token)
	}
}

func sendFansData(url string, token string) {
	fans := GetFansStatus()
	if fans == nil {
		return
	}

	payload := FanPayload{
		Token:     token,
		Timestamp: time.Now().Unix(),
		Fans:      fans,
	}

	if jsonData, err := json.Marshal(payload); err == nil {
		sendRequest(url, jsonData, "Fans",token) // использует общую функцию sendRequest из poller.go
	}
}

func SendStaticHardwareInfo(baseURL string, token string) {
	fmt.Println(" [i] Сбор паспортных данных системы (CPU, RAM, Motherboard)...")

	info := GetHardwareStaticInfo()
	fullURL := baseURL + "/api/hardware/info"

	payload := InfoPayload{
		Token:     token,
		Timestamp: time.Now().Unix(),
		Info:      info,
	}

	if jsonData, err := json.Marshal(payload); err == nil {
		// Использует уже готовую функцию sendRequest из poller.go
		sendRequest(fullURL, jsonData, "StaticInfo",token)
	}
}

// --- ОБЩАЯ СЕТЕВАЯ ФУНКЦИЯ ---

func sendRequest(url string, jsonData []byte, workerName string, token string) {
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		fmt.Printf(" [❌ %s ERROR] Ошибка создания запроса: %v\n", workerName, err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Agent-Type", "agent-hardware-windows")
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
