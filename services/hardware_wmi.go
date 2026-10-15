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
	MediaType         uint16 // 3 = HDD, 4 = SSD, 0 = Unspecified
	BusType           uint16 // 3 = ATAPI, 8 = RAID, 11 = SATA, 17 = NVMe т.д.
	OperationalStatus []uint16 // Массив статусов. 1 = OK, 3 = Degraded, 5 = Predictive Failure
	HealthStatus      uint16 // 0 = Healthy, 1 = Warning, 2 = Unhealthy
}

// Win32_DiskDrive используется для получения железных счетчиков ошибок
type Win32_DiskDrive struct {
	DeviceID      string
	InterfaceType string // SCSI, HDC, IDE
	SectorsPerTrack uint32
	TotalSectors    uint64
	// Встроенные счетчики ошибок Windows (растут при аппаратных проблемах)
	Capabilities      []uint16 // Возможности диска
	ConfigManagerErrorCode uint32 // Код ошибки диспетчера устройств (0 = ок)
}

// AdvancedDiskStatus собирает абсолютно все доступные метрики диска
type AdvancedDiskStatus struct {
	DeviceID       string `json:"device_id"`
	Model          string `json:"model"`
	Type           string `json:"type"`       // SSD, HDD, NVMe, Unknown
	Interface      string `json:"interface"`  // SATA, NVMe, SCSI, USB
	HealthStatus   string `json:"health_status"` // Healthy, Warning, Unhealthy
	IsHealthy      bool   `json:"is_healthy"`
	HardwareErrors uint32 `json:"hardware_errors_code"` // 0 если все отлично на уровне ОС
}

func GetAdvancedDiskStatus() []AdvancedDiskStatus {
	log.Println("[💾 DISK SERVICE] Сбор расширенной статистики дисков...")

	var results []AdvancedDiskStatus

	// 1. Пробуем современный метод (Storage Namespace)
	var storageData []MSFT_PhysicalDisk
	queryStorage := "SELECT DeviceId, FriendlyName, MediaType, BusType, HealthStatus FROM MSFT_PhysicalDisk"
	err := wmi.QueryNamespace(queryStorage, &storageData, `root\Microsoft\Windows\Storage`)

	// Если этот метод поддерживается ОС
	if err == nil && len(storageData) > 0 {
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
				DeviceID:     "PHYSICALDRIVE" + disk.DeviceId,
				Model:        disk.FriendlyName,
				Type:         mediaType,
				Interface:    busType,
				HealthStatus: health,
				IsHealthy:    disk.HealthStatus == 0,
			})
		}
		return results
	}

	// 2. БЕЗОПАСНЫЙ ОТКАТ: Если старая ОС, используем классический Win32_DiskDrive
	log.Println("[⚠️ WARNING] MSFT_PhysicalDisk недоступен. Откат на Win32_DiskDrive...")
	var driveData []Win32_DiskDrive
	queryDrive := "SELECT DeviceID, Caption, Status FROM Win32_DiskDrive"

	err = wmi.Query(queryDrive, &driveData)
	if err != nil {
		log.Printf("[❌ CRITICAL ERROR] Все методы опроса дисков провалились: %v", err)
		return []AdvancedDiskStatus{} // Возвращаем пустой массив вместо паники
	}

	for _, disk := range driveData {
		id := strings.TrimPrefix(disk.DeviceID, `\\.\`)
		isHealthy := disk.Status == "OK"
		healthStatus := "Healthy"
		if !isHealthy { healthStatus = "Unhealthy" }

		results = append(results, AdvancedDiskStatus{
			DeviceID:     id,
			Model:        disk.Caption,
			Type:         "Unknown", // Win32_DiskDrive не знает точный тип SSD/HDD
			Interface:    "Unknown",
			HealthStatus: healthStatus,
			IsHealthy:    isHealthy,
		})
	}

	return results
}
