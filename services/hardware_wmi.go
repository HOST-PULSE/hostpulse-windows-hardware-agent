package services

import (
	"fmt"
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
	ConfigManagerErrorCode uint32
}

// AdvancedDiskStatus собирает абсолютно все доступные метрики диска для Django
type AdvancedDiskStatus struct {
	DeviceID           string `json:"device_id"`
	Model              string `json:"model"`
	Type               string `json:"type"`
	Interface          string `json:"interface"`
	HealthStatus       string `json:"health_status"`
	IsHealthy          bool   `json:"is_healthy"`
	HardwareErrors     uint32 `json:"hardware_errors_code"`
	ReallocatedSectors uint32 `json:"reallocated_sectors"`
	Temperature        uint8  `json:"temperature"`
	PowerOnHours       uint32 `json:"power_on_hours"`
}

// MSStorageDriver_ATAPISmartData содержит сырые данные SMART от драйвера
type MSStorageDriver_ATAPISmartData struct {
	InstanceName   string
	VendorSpecific [512]uint8 // Байты, где зашиты все атрибуты SMART
}

func GetAdvancedDiskStatus() []AdvancedDiskStatus {
	fmt.Println(" [💾 DISK SERVICE] Сбор расширенной статистики дисков...")

	var results []AdvancedDiskStatus

	// 1. Пробуем современный метод (Storage Namespace)
	var storageData []MSFT_PhysicalDisk
	queryStorage := "SELECT DeviceId, FriendlyName, MediaType, BusType, HealthStatus FROM MSFT_PhysicalDisk"
	err := wmi.QueryNamespace(queryStorage, &storageData, `root\Microsoft\Windows\Storage`)

	if err == nil && len(storageData) > 0 {
		fmt.Printf(" [🔍 DISK SERVICE] Успешно найдено %d дисков через MSFT_PhysicalDisk\n", len(storageData))
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

			targetID := "PHYSICALDRIVE" + disk.DeviceId
			realSectors, temp, workHours := GetSmartMetrics(targetID)

			results = append(results, AdvancedDiskStatus{
				DeviceID:           targetID,
				Model:              disk.FriendlyName,
				Type:               mediaType,
				Interface:          busType,
				HealthStatus:       health,
				IsHealthy:          disk.HealthStatus == 0,
				HardwareErrors:     0,
				ReallocatedSectors: realSectors,
				Temperature:        temp,
				PowerOnHours:       workHours,
			})
		}
		return results
	}

	// 2. БЕЗОПАСНЫЙ ОТКАТ: Переходим сюда, если первый метод вернул 0 дисков
	fmt.Println(" [⚠️ WARNING] MSFT_PhysicalDisk вернул 0 дисков или недоступен. Переход на Win32_DiskDrive...")
	var driveData []Win32_DiskDrive
	queryDrive := "SELECT DeviceID, Caption, Status, InterfaceType, ConfigManagerErrorCode FROM Win32_DiskDrive"

	err = wmi.Query(queryDrive, &driveData)
	if err != nil {
		fmt.Printf(" [❌ CRITICAL ERROR] Все методы опроса дисков провалились: %v\n", err)
		return []AdvancedDiskStatus{}
	}

	fmt.Printf(" [🔍 DISK SERVICE] Найдено %d дисков через Win32_DiskDrive\n", len(driveData))
	for _, disk := range driveData {
		id := strings.TrimPrefix(disk.DeviceID, `\\.\`)

		isHealthy := disk.Status == "OK" || disk.Status == ""
		healthStatus := "Healthy"
		if !isHealthy { healthStatus = "Unhealthy" }

		interfaceType := "Unknown"
		if disk.InterfaceType != "" {
			interfaceType = disk.InterfaceType
		}

		// ИСПРАВЛЕНО: Заменили disk.DeviceId на правильный disk.DeviceID
		realSectors, temp, workHours := GetSmartMetrics(disk.DeviceID)

		results = append(results, AdvancedDiskStatus{
			DeviceID:           id,
			Model:              disk.Caption,
			Type:               "HDD",
			Interface:          interfaceType,
			HealthStatus:       healthStatus,
			IsHealthy:          isHealthy,
			HardwareErrors:     disk.ConfigManagerErrorCode,
			ReallocatedSectors: realSectors, // ИСПРАВЛЕНО: Добавлены двоеточия
			Temperature:        temp,        // ИСПРАВЛЕНО: Добавлены двоеточия
			PowerOnHours:       workHours,   // ИСПРАВЛЕНО: Добавлены двоеточия
		})
	}

	return results
}

func GetSmartMetrics(deviceID string) (reallocated uint32, temperature uint8, hours uint32) {
	var smartData []MSStorageDriver_ATAPISmartData

	err := wmi.QueryNamespace("SELECT InstanceName, VendorSpecific FROM MSStorageDriver_ATAPISmartData", &smartData, `root\WMI`)
	if err != nil {
		return 0, 0, 0
	}

	// Выделяем из пути (например, "IDE\DiskST1000VX013...") ключевую часть для сопоставления
	cleanTarget := strings.ToLower(strings.ReplaceAll(deviceID, `\\.\`, ""))

	for _, data := range smartData {
		cleanInstance := strings.ToLower(data.InstanceName)

		// ИСПРАВЛЕНО: Удален несуществующий вызов VendorSpecificNameCheck
		if !strings.Contains(cleanInstance, cleanTarget) && !strings.Contains(cleanTarget, "physicaldrive") {
			// Дополнительная проверка на случай, если пришел короткий ID вроде "PHYSICALDRIVE0"
			// Драйвер SMART в InstanceName обычно пишет что-то вроде "SCSI\Disk&Ven_...&Prod_..._0"
			// Если привязка жестко не совпала, проверим окончание строки с номером диска
			idx := strings.LastIndex(cleanTarget, "drive")
			if idx != -1 && idx+5 < len(cleanTarget) {
				diskNum := cleanTarget[idx+5:]
				if !strings.HasSuffix(cleanInstance, "_"+diskNum) && !strings.Contains(cleanInstance, "instance"+diskNum) {
					continue
				}
			} else {
				continue
			}
		}

		// Данные атрибутов начинаются с байта под индексом 2. Каждый атрибут — 12 байт.
		for i := 2; i < 362; i += 12 {
			attrID := data.VendorSpecific[i]
			if attrID == 0 {
				break
			}

			rawBytes := data.VendorSpecific[i+5 : i+11]

			switch attrID {
			case 5: // Reallocated Sectors Count
				reallocated = uint32(rawBytes[0]) | uint32(rawBytes[1])<<8 | uint32(rawBytes[2])<<16 | uint32(rawBytes[3])<<24
			case 194: // Temperature
				temperature = rawBytes[0]
			case 9: // Power-On Hours
				hours = uint32(rawBytes[0]) | uint32(rawBytes[1])<<8 | uint32(rawBytes[2])<<16 | uint32(rawBytes[3])<<24
			}
		}
		return reallocated, temperature, hours
	}
	return 0, 0, 0
}
