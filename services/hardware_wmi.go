package services

import (
	"log"
	"strings"
	"github.com/yusufpapurcu/wmi"
)

// MSFT_PhysicalDisk содержит расширенные свойства диска в современных Windows
type MSFT_PhysicalDisk struct {
	DeviceId          string
	FriendlyName      string
	MediaType         uint16   // 3 = HDD, 4 = SSD, 0 = Unspecified
	BusType           uint16   // 11 = SATA, 17 = NVMe т.д.
	OperationalStatus []uint16 // Массив статусов
	HealthStatus      uint16   // 0 = Healthy, 1 = Warning, 2 = Unhealthy
}

// Win32_DiskDrive используется для получения железных счетчиков ошибок
type Win32_DiskDrive struct {
	DeviceID               string
	Caption                string
	Status                 string
	InterfaceType          string
	SectorsPerTrack        uint32
	TotalSectors           uint64
	Capabilities           []uint16
	ConfigManagerErrorCode uint32
}

// AdvancedDiskStatus собирает абсолютно все доступные метрики диска для Django
type AdvancedDiskStatus struct {
	DeviceID       string `json:"device_id"`
	Model          string `json:"model"`
	Type           string `json:"type"`
	Interface      string `json:"interface"`
	HealthStatus   string `json:"health_status"`
	IsHealthy      bool   `json:"is_healthy"`
	HardwareErrors uint32 `json:"hardware_errors_code"`
}

func GetAdvancedDiskStatus() []AdvancedDiskStatus {
	log.Println("[💾 DISK SERVICE] Сбор расширенной статистики дисков...")

	var results []AdvancedDiskStatus

	// 1. Пробуем современный метод (Storage Namespace)
	var storageData []MSFT_PhysicalDisk
	queryStorage := "SELECT DeviceId, FriendlyName, MediaType, BusType, HealthStatus FROM MSFT_PhysicalDisk"
	err := wmi.QueryNamespace(queryStorage, &storageData, `root\Microsoft\Windows\Storage`)

	// ИСПРАВЛЕНО: Заходим внутрь ТОЛЬКО если диски РЕАЛЬНО найдены
	if err == nil && len(storageData) > 0 {
		log.Printf("[🔍 DISK SERVICE] Успешно найдено %d дисков через MSFT_PhysicalDisk", len(storageData))
		for _, disk := range storageData {
			mediaType := "Unknown"
			if disk.MediaType == 3 { mediaType = "HDD" } else if disk.MediaType == 4 { mediaType = "SSD" }

			busType := "Unknown"
			switch disk.BusType {
			case 11: busType = "SATA"
			case 17: busType = "NVMe"
			case 7:  busType = "USB"
			}

			health := "Unknown"
			switch disk.HealthStatus {
			case 0: health = "Healthy"
			case 1: health = "Warning"
			case 2: health = "Unhealthy"
			}

			results = append(results, AdvancedDiskStatus{
				DeviceID:       "PHYSICALDRIVE" + disk.DeviceId,
				Model:          disk.FriendlyName,
				Type:           mediaType,
				Interface:      busType,
				HealthStatus:   health,
				IsHealthy:      disk.HealthStatus == 0,
				HardwareErrors: 0,
			})
		}
		return results
	}

	// 2. БЕЗОПАСНЫЙ ОТКАТ: Переходим сюда, если первый метод упал ИЛИ вернул 0 дисков
	log.Println("[⚠️ WARNING] MSFT_PhysicalDisk вернул 0 дисков или недоступен. Переход на Win32_DiskDrive...")
	var driveData []Win32_DiskDrive
	queryDrive := "SELECT DeviceID, Caption, Status, InterfaceType, ConfigManagerErrorCode FROM Win32_DiskDrive"

	err = wmi.Query(queryDrive, &driveData)
	if err != nil {
		log.Printf("[❌ CRITICAL ERROR] Все методы опроса дисков провалились: %v", err)
		return []AdvancedDiskStatus{}
	}

	log.Printf("[🔍 DISK SERVICE] Найдено %d дисков через Win32_DiskDrive", len(driveData))
	for _, disk := range driveData {
		id := strings.TrimPrefix(disk.DeviceID, `\\.\`)

		// Обработка статуса здоровья (для старых ОС "OK" или пустота означает успех)
		isHealthy := disk.Status == "OK" || disk.Status == ""
		healthStatus := "Healthy"
		if !isHealthy { healthStatus = "Unhealthy" }

		// Пытаемся определить интерфейс подключения
		interfaceType := "Unknown"
		if disk.InterfaceType != "" {
			interfaceType = disk.InterfaceType
		}

		results = append(results, AdvancedDiskStatus{
			DeviceID:       id,
			Model:          disk.Caption,
			Type:           "HDD", // Для старого класса ставим HDD по умолчанию, либо Unknown
			Interface:      interfaceType,
			HealthStatus:   healthStatus,
			IsHealthy:      isHealthy,
			HardwareErrors: disk.ConfigManagerErrorCode,
		})
	}

	return results
}
