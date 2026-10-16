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

	// 1. СОВРЕМЕННЫЙ МЕТОД (Windows 10/11, Server 2016/2019/2022)
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

			// --- УНИВЕРСАЛЬНЫЙ КАСКАД СБОРА SMART ---
			realSectors, temp, workHours := GetSmartMetrics(targetID) // Сначала WMI
			if realSectors == 0 && temp == 0 && workHours == 0 {
				// Если WMI выдал нули (как на Intel S5500HC), бьем напрямую через WinAPI
				realSectors, temp, workHours = GetSmartNative(targetID)
			}
			// ----------------------------------------

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

	// 2. БЕЗОПАСНЫЙ ОТКАТ (Для старых Windows Server 2008/2012 или специфичных виртуалок)
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

		// --- УНИВЕРСАЛЬНЫЙ КАСКАД СБОРА SMART (ДЛЯ ОТКАТА) ---
		realSectors, temp, workHours := GetSmartMetrics(disk.DeviceID) // Сначала WMI
		if realSectors == 0 && temp == 0 && workHours == 0 {
			// Если WMI пустой, запрашиваем WinAPI дескриптор
			realSectors, temp, workHours = GetSmartNative(disk.DeviceID)
		}
		// -----------------------------------------------------

		results = append(results, AdvancedDiskStatus{
			DeviceID:           id,
			Model:              disk.Caption,
			Type:               "HDD",
			Interface:          interfaceType,
			HealthStatus:       healthStatus,
			IsHealthy:          isHealthy,
			HardwareErrors:     disk.ConfigManagerErrorCode,
			ReallocatedSectors: realSectors,
			Temperature:        temp,
			PowerOnHours:       workHours,
		})
	}

	return results
}


func GetSmartMetrics(deviceID string) (reallocated uint32, temperature uint8, hours uint32) {
	var smartData []MSStorageDriver_ATAPISmartData

	// Опрашиваем пространство root\WMI
	err := wmi.QueryNamespace("SELECT InstanceName, VendorSpecific FROM MSStorageDriver_ATAPISmartData", &smartData, `root\WMI`)
	if err != nil || len(smartData) == 0 {
		return 0, 0, 0
	}

	cleanTarget := strings.ToLower(strings.ReplaceAll(deviceID, `\\.\`, ""))

	for _, data := range smartData {
		cleanInstance := strings.ToLower(data.InstanceName)

		// Проверка на соответствие диска (по названию или номеру в конце)
		matched := strings.Contains(cleanInstance, cleanTarget)
		if !matched {
			idx := strings.LastIndex(cleanTarget, "drive")
			if idx != -1 && idx+5 < len(cleanTarget) {
				diskNum := cleanTarget[idx+5:]
				if strings.HasSuffix(cleanInstance, "_"+diskNum) || strings.Contains(cleanInstance, "instance"+diskNum) {
					matched = true
				}
			}
		}

		if !matched {
			continue
		}

		// По спецификации ATA SMART: данные атрибутов начинаются с индекса 2. Всего 30 атрибутов по 12 байт.
		for i := 2; i < 362; i += 12 {
			attrID := data.VendorSpecific[i]
			if attrID == 0 {
				break // Конец списка атрибутов
			}

			// Исправленный побайтовый сбор значения Raw Value (байты с i+5 по i+10)
			// Берем первые 4 байта из 6 доступных для формирования uint32
			v5 := uint32(data.VendorSpecific[i+5])
			v6 := uint32(data.VendorSpecific[i+6])
			v7 := uint32(data.VendorSpecific[i+7])
			v8 := uint32(data.VendorSpecific[i+8])

			rawValue := v5 | (v6 << 8) | (v7 << 16) | (v8 << 24)

			switch attrID {
			case 5: // Reallocated Sectors Count
				reallocated = rawValue
			case 9: // Power-On Hours
				hours = rawValue
			case 194, 12: // 194 = Температура, иногда дублируется в 12 у старых HDD
				temperature = data.VendorSpecific[i+5] // Первый байт сырого значения — текущая температура в градусах Цельсия
			}
		}
		return reallocated, temperature, hours
	}
	return 0, 0, 0
}

