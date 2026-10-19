package services

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
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
	BadSectorsPending  uint32 `json:"bad_sectors_pending"` // Из предыдущего шага
	CrcErrors          uint32 `json:"crc_errors"`           // Ошибки шлейфа/SATA кабеля (ID 199)
	PercentageUsed     uint8  `json:"percentage_used"`     // Износ SSD в % (0-100+)
	SsdWrittenGb       uint64 `json:"ssd_written_gb"`       // Записано на SSD всего в Гигабайтах
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
	AtaSmartAttributes struct {
		Table []struct {
			Id   int    `json:"id"`
			Name string `json:"name"`
			Raw  struct {
				Value uint64 `json:"value"` // uint64 из прошлого шага
			} `json:"raw"`
		} `json:"table"`
	} `json:"ata_smart_attributes"`
	// ИСПРАВЛЕНО: Добавлены новые поля износа и записи для NVMe
	NvmeSmartHealthInformationLog struct {
		Temperature      uint32 `json:"temperature"`
		PowerOnHours     uint32 `json:"power_on_hours"`
		MediaErrors      uint32 `json:"media_errors"`
		PercentageUsed   uint8  `json:"percentage_used"`     // Сюда прилетит износ
		DataUnitsWritten uint64 `json:"data_units_written"`  // Сюда прилетит сырой счетчик терабайт
	} `json:"nvme_smart_health_information_log"`
}

// extractSmartctl извлекает зашитый smartctl.exe во временную папку ОС
func extractSmartctl() (string, error) {
	tmpDir := os.TempDir()
	execPath := filepath.Join(tmpDir, "hostpulse_smartctl.exe")

	fmt.Printf("   [DEBUG EXTRACT] Системная temp-директория: %s\r\n", tmpDir)
	fmt.Printf("   [DEBUG EXTRACT] Проверяем зашитый массив байт. Размер в памяти: %d байт\r\n", len(smartctlBinary))

	if len(smartctlBinary) == 0 {
		return "", fmt.Errorf("массив smartctlBinary пуст! Проверьте //go:embed")
	}

	// ИСПРАВЛЕНО: Принудительно удаляем старый файл, если он остался от прошлых запусков,
	// чтобы гарантировать запись свежего и исправленного бинарника
	if _, err := os.Stat(execPath); err == nil {
		fmt.Println("   [DEBUG EXTRACT] Найдена старая копия файла. Удаляем для принудительного обновления...")
		_ = os.Remove(execPath)
	}

	fmt.Printf("   [DEBUG EXTRACT] Записываем бинарник по пути: %s\r\n", execPath)
	err := os.WriteFile(execPath, smartctlBinary, 0755)
	if err != nil {
		fmt.Printf("   [DEBUG EXTRACT] ❌ Ошибка записи файла на диск: %v\r\n", err)
		return "", err
	}

	// Проверяем реальный размер записанного файла на диске
	if fi, err := os.Stat(execPath); err == nil {
		fmt.Printf("   [DEBUG EXTRACT] ✅ Файл успешно записан. Размер на диске: %d байт\r\n", fi.Size())
	}

	return execPath, nil
}

// GetSmartDataUniversal выполняет универсальный сбор SMART через извлеченный smartctl
// GetSmartDataUniversal выполняет универсальный сбор SMART через извлеченный smartctl с подробным логированием
func GetSmartDataUniversal(targetDeviceID string) (reallocated uint32, pending uint32, crcErrors uint32, pctUsed uint8, writtenGb uint64, temperature uint8, hours uint32) {
	smartctlPath, err := extractSmartctl()
	if err != nil {
		return 0, 0, 0, 0, 0, 0, 0
	}

	re := regexp.MustCompile(`\d+`)
	driveIndex := re.FindString(targetDeviceID)
	if driveIndex == "" {
		driveIndex = "0"
	}

	cmd := exec.Command(smartctlPath, "-a", "/dev/pd"+driveIndex, "--json")
	output, err := cmd.Output()
	if err != nil && len(output) == 0 {
		return 0, 0, 0, 0, 0, 0, 0
	}

	var data SmartctlOutput
	if err := json.Unmarshal(output, &data); err != nil {
		return 0, 0, 0, 0, 0, 0, 0
	}

	hours = data.PowerOnTime.Hours
	temperature = data.Temperature.Current

	// Проверяем, NVMe ли это устройство
	if data.NvmeSmartHealthInformationLog.PowerOnHours > 0 {
		hours = data.NvmeSmartHealthInformationLog.PowerOnHours
		reallocated = data.NvmeSmartHealthInformationLog.MediaErrors
		pctUsed = data.NvmeSmartHealthInformationLog.PercentageUsed

		if data.NvmeSmartHealthInformationLog.Temperature > 0 {
			temperature = uint8(data.NvmeSmartHealthInformationLog.Temperature)
		}

		// Формула перевода NVMe Data Units Written в Гигабайты:
		// (Счетчик * 512000 байт) / 1024^3
		// Чтобы избежать потери точности при делении больших чисел, сначала умножаем, потом делим
		if data.NvmeSmartHealthInformationLog.DataUnitsWritten > 0 {
			bytesWritten := data.NvmeSmartHealthInformationLog.DataUnitsWritten * 512000
			writtenGb = bytesWritten / (1024 * 1024 * 1024)
		}
	} else {
		// Для классических SATA дисков ищем параметры в таблице атрибутов
		for _, attr := range data.AtaSmartAttributes.Table {
			switch attr.Id {
			case 5:
				reallocated = uint32(attr.Raw.Value) // Стабильные бэды
			case 197:
				pending = uint32(attr.Raw.Value)     // Свежие бэды-кандидаты
			case 199:
				crcErrors = uint32(attr.Raw.Value)   // Ошибки SATA кабеля
			}
		}
	}

	return reallocated, pending, crcErrors, pctUsed, writtenGb, temperature, hours
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

			// Формируем чистый targetID для функции
			targetID := "PHYSICALDRIVE" + disk.DeviceId
			// Принимаем новые 5 параметров из функции
			realSectors, pendingSectors, cableErrors, wearPct, gbWritten, temp, workHours := GetSmartDataUniversal(targetID)

			results = append(results, AdvancedDiskStatus{
				DeviceID:           targetID,
				Model:              disk.FriendlyName,
				Type:               mediaType,
				Interface:          busType,
				HealthStatus:       health,
				IsHealthy:          disk.HealthStatus == 0,
				HardwareErrors:     0,
				ReallocatedSectors: realSectors,
				BadSectorsPending:  pendingSectors,
				CrcErrors:          cableErrors,
				PercentageUsed:     wearPct,
				SsdWrittenGb:       gbWritten,
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

	fmt.Printf(" [🔍 DISK SERVICE] Найдено %d дисков через Win32_DiskDrive\(\r\\)n", len(driveData))
	for _, disk := range driveData {
		id := strings.TrimPrefix(disk.DeviceID, `\\.\`)

		isHealthy := disk.Status == "OK" || disk.Status == ""
		healthStatus := "Healthy"
		if !isHealthy {
			healthStatus = "Unhealthy"
		}

		// ИСПРАВЛЕНО: Правильный вызов функции с 7 переменными для Win32_DiskDrive
		realSectors, pendingSectors, cableErrors, wearPct, gbWritten, temp, workHours := GetSmartDataUniversal(id)

		// ИСПРАВЛЕНО: Передаем родные поля структуры Win32_DiskDrive (Caption, InterfaceType)
		results = append(results, AdvancedDiskStatus{
			DeviceID:           id,
			Model:              disk.Caption, // Для старых ОС используем Caption вместо FriendlyName
			Type:               "HDD",        // Старое API всегда отдаёт HDD, smartctl исправит это на бэкенде
			Interface:          disk.InterfaceType,
			HealthStatus:       healthStatus,
			IsHealthy:          isHealthy,
			HardwareErrors:     disk.ConfigManagerErrorCode,
			ReallocatedSectors: realSectors,
			BadSectorsPending:  pendingSectors,
			CrcErrors:          cableErrors,
			PercentageUsed:     wearPct,
			SsdWrittenGb:       gbWritten,
			Temperature:        temp,
			PowerOnHours:       workHours,
		})
	}

	return results
}
