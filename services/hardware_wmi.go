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

	// 1. Запрашиваем физические параметры из пространства Storage
	var storageData []MSFT_PhysicalDisk
	queryStorage := "SELECT DeviceId, FriendlyName, MediaType, BusType, OperationalStatus, HealthStatus FROM MSFT_PhysicalDisk"
	err := wmi.QueryNamespace(queryStorage, &storageData, `root\Microsoft\Windows\Storage`)
	if err != nil {
		log.Printf("[❌ ERROR] Ошибка запроса к MSFT_PhysicalDisk: %v", err)
		return nil
	}

	// 2. Запрашиваем системные счетчики ошибок из CIMV2
	var driveData []Win32_DiskDrive
	queryDrive := "SELECT DeviceID, InterfaceType, ConfigManagerErrorCode FROM Win32_DiskDrive"
	wmi.Query(queryDrive, &driveData) // не критично если упадет, подстрахует первый запрос

	// Создаем мапу для быстрого маппинга счетчиков ошибок по DeviceID
	errorMap := make(map[string]uint32)
	for _, d := range driveData {
		// Приводим к единому формату для сравнения (убираем префиксы вроде \\.\)
		id := strings.TrimPrefix(d.DeviceID, `\\.\`)
		errorMap[id] = d.ConfigManagerErrorCode
	}

	var results []AdvancedDiskStatus
	for _, disk := range storageData {
		// Определение типа медиа (SSD / HDD)
		mediaType := "Unknown"
		if disk.MediaType == 3 {
			mediaType = "HDD"
		} else if disk.MediaType == 4 {
			mediaType = "SSD"
		}

		// Определение интерфейса (SATA / NVMe и т.д.)
		busType := "Unknown"
		switch disk.BusType {
		case 11: busType = "SATA"
		case 17: busType = "NVMe"
		case 7:  busType = "USB"
		case 8:  busType = "RAID"
		}

		// Текстовый статус здоровья
		health := "Unknown"
		switch disk.HealthStatus {
		case 0: health = "Healthy"
		case 1: health = "Warning"
		case 2: health = "Unhealthy"
		}

		// Ищем код аппаратной ошибки из Диспетчера Устройств Windows для этого диска
		hwError, exists := errorMap["PHYSICALDRIVE"+disk.DeviceId]
		if !exists {
			hwError = 0
		}

		results = append(results, AdvancedDiskStatus{
			DeviceID:       "PHYSICALDRIVE" + disk.DeviceId,
			Model:          disk.FriendlyName,
			Type:           mediaType,
			Interface:      busType,
			HealthStatus:   health,
			IsHealthy:      disk.HealthStatus == 0,
			HardwareErrors: hwError,
		})
	}

	return results
}