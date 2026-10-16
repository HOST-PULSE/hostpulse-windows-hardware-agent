package services

import (
	
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/yusufpapurcu/wmi"
)

var smartctlBinary []byte

// SetSmartctlBinary принимает массив байт зашитого smartctl.exe из main
func SetSmartctlBinary(b []byte) {
	smartctlBinary = b
}

// MSFT_PhysicalDisk используется для базового обнаружения дисков на современных ОС
type MSFT_PhysicalDisk struct {
	DeviceId     string
	FriendlyName string
	MediaType    uint16
	BusType      uint16
	HealthStatus uint16
}

// Win32_DiskDrive используется как откат обнаружения на старых ОС (например, Server 2012)
type Win32_DiskDrive struct {
	DeviceID               string
	Caption                string
	Status                 string
	InterfaceType          string
	ConfigManagerErrorCode uint32
}

// AdvancedDiskStatus структура, отправляемая на Django-бэкенд HostPulse
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

// SmartctlOutput структура для десериализации JSON вывода утилиты smartctl
type SmartctlOutput struct {
	SmartStatus struct {
		Passed bool `json:"passed"`
	} `json:"smart_status"`
	PowerOnTime struct {
		Hours uint32 `json:"hours"`
	} `json:"power_on_time"`
	Temperature struct {
		Current uint8 `json:"current"`
	} `json:"temperature"`
	// Таблица для ATA/SATA накопителей
	AtaSmartAttributes struct {
		Table []struct {
			Id   int    `json:"id"`
			Name string `json:"name"`
			Raw  struct {
				Value uint32 `json:"value"`
			} `json:"raw"`
		} `json:"table"`
	} `json:"ata_smart_attributes"`
	// Лог для NVMe SSD накопителей
	NvmeSmartHealthInformationLog struct {
		Temperature  uint32 `json:"temperature"`
		PowerOnHours uint32 `json:"power_on_hours"`
		MediaErrors  uint32 `json:"media_errors"` // Основной показатель аппаратных сбоев ячеек
	} `json:"nvme_smart_health_information_log"`
}

// extractSmartctl извлекает зашитый smartctl.exe во временную папку ОС
func extractSmartctl() (string, error) {
	tmpDir := os.TempDir()
	execPath := filepath.Join(tmpDir, "hostpulse_smartctl.exe")

	if _, err := os.Stat(execPath); os.IsNotExist(err) {
		err = os.WriteFile(execPath, smartctlBinary, 0755)
		if err != nil {
			return "", err
		}
	}
	return execPath, nil
}

// GetSmartDataUniversal выполняет универсальный сбор SMART через извлеченный smartctl
func GetSmartDataUniversal(targetDeviceID string) (reallocated uint32, temperature uint8, hours uint32) {
	smartctlPath, err := extractSmartctl()
	if err != nil {
		return 0, 0, 0
	}

	// Извлекаем индекс диска (последний символ строки, например "PHYSICALDRIVE0" -> "0")
	driveIndex := "0"
	if len(targetDeviceID) > 0 {
		driveIndex = string(targetDeviceID[len(targetDeviceID)-1])
	}

	// ИСПРАВЛЕНО: Флаг изменен на корректный строго двойной дефис "--json"
	cmd := exec.Command(smartctlPath, "-a", "/dev/pd"+driveIndex, "--json")
	output, _ := cmd.Output()

	var data SmartctlOutput
	if err := json.Unmarshal(output, &data); err != nil {
		return 0, 0, 0
	}

	// Читаем базовые параметры из корня (актуально для SATA)
	hours = data.PowerOnTime.Hours
	temperature = data.Temperature.Current

	// ИСПРАВЛЕНО: Добавлен разбор специфичной ветки NVMe, чтобы избежать нулей на SSD
	if data.NvmeSmartHealthInformationLog.PowerOnHours > 0 {
		hours = data.NvmeSmartHealthInformationLog.PowerOnHours
		reallocated = data.NvmeSmartHealthInformationLog.MediaErrors
	} else {
		// Для классических SATA HDD/SSD ищем атрибут Reallocated Sectors (ID 5)
		for _, attr := range data.AtaSmartAttributes.Table {
			if attr.Id == 5 {
				reallocated = attr.Raw.Value
				break
			}
		}
	}

	return reallocated, temperature, hours
}

// GetAdvancedDiskStatus собирает итоговый слайс данных по дискам в системе
func GetAdvancedDiskStatus() []AdvancedDiskStatus {
	fmt.Println(" [💾 DISK SERVICE] Сбор расширенной статистики дисков...")
	var results []AdvancedDiskStatus

	// 1. Попытка через современное пространство имён Storage
	var storageData []MSFT_PhysicalDisk
	queryStorage := "SELECT DeviceId, FriendlyName, MediaType, BusType, HealthStatus FROM MSFT_PhysicalDisk"
	err := wmi.QueryNamespace(queryStorage, &storageData, `root\Microsoft\Windows\Storage`)

	if err == nil && len(storageData) > 0 {
		fmt.Printf(" [🔍 DISK SERVICE] Найдено %d дисков через MSFT_PhysicalDisk\r\n", len(storageData))
		for _, disk := range storageData {
			mediaType := "Unknown"
			if disk.MediaType == 3 {
				mediaType = "HDD"
			} else if disk.MediaType == 4 {
				mediaType = "SSD"
			}

			busType := "Unknown"
			switch disk.BusType {
			case 11:
				busType = "SATA"
			case 17:
				busType = "NVMe"
			case 7:
				busType = "USB"
			}

			health := "Unknown"
			switch disk.HealthStatus {
			case 0:
				health = "Healthy"
			case 1:
				health = "Warning"
			case 2:
				health = "Unhealthy"
			}

			targetID := "PHYSICALDRIVE" + disk.DeviceId
			realSectors, temp, workHours := GetSmartDataUniversal(targetID)

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

	// 2. ОТКАТ: Если первая ветка пуста, переходим на классический Win32_DiskDrive
	fmt.Println(" [⚠️ WARNING] MSFT_PhysicalDisk вернул 0 дисков. Переход на Win32_DiskDrive...")
	var driveData []Win32_DiskDrive
	queryDrive := "SELECT DeviceID, Caption, Status, InterfaceType, ConfigManagerErrorCode FROM Win32_DiskDrive"

	err = wmi.Query(queryDrive, &driveData)
	if err != nil {
		return []AdvancedDiskStatus{}
	}

	fmt.Printf(" [🔍 DISK SERVICE] Найдено %d дисков через Win32_DiskDrive\r\n", len(driveData))
	for _, disk := range driveData {
		id := strings.TrimPrefix(disk.DeviceID, `\\.\`)

		isHealthy := disk.Status == "OK" || disk.Status == ""
		healthStatus := "Healthy"
		if !isHealthy {
			healthStatus = "Unhealthy"
		}

		realSectors, temp, workHours := GetSmartDataUniversal(id)

		results = append(results, AdvancedDiskStatus{
			DeviceID:           id,
			Model:              disk.Caption,
			Type:               "HDD", // По умолчанию для старого API
			Interface:          disk.InterfaceType,
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
