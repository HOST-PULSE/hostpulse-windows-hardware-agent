package services

import (
	"log"

	"github.com/yusufpapurcu/wmi"
)

// MSFT_StorageReliabilityStatus — WMI класс для проверки здоровья дисков
type MSFT_StorageReliabilityStatus struct {
	DeviceId       string
	PredictFailure bool
}

// DiskStatus представляет структуру диска для отправки на бэкенд HostPulse
type DiskStatus struct {
	DeviceID string `json:"device_id"`
	IsHealthy bool   `json:"is_healthy"` // true = всё ок, false = SMART бьет тревогу
}

// GetSmartStatus собирает состояние SMART по всем дискам
func GetSmartStatus() []DiskStatus {
	var dst []MSFT_StorageReliabilityStatus
	query := "SELECT DeviceId, PredictFailure FROM MSFT_StorageReliabilityStatus"

	// Запрос к пространству хранения Windows
	err := wmi.QueryNamespace(query, &dst, `root\Microsoft\Windows\Storage`)
	if err != nil {
		log.Printf(" [❌ ERROR] Не удалось получить SMART через WMI: %v", err)
		return nil
	}

	var results []DiskStatus
	for _, disk := range dst {
		results = append(results, DiskStatus{
			DeviceID:  disk.DeviceId,
			IsHealthy: !disk.PredictFailure, // Если PredictFailure true, значит диск умирает (IsHealthy = false)
		})
	}

	return results
}
