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

	// ПАСПОРТНЫЕ ДАННЫЕ ДИСКА
	SerialNumber    string `json:"serial_number"`     // Серийный номер диска
	FirmwareVersion string `json:"firmware_version"`   // Версия заводской прошивки

	HealthStatus       string `json:"health_status"`
	IsHealthy          bool   `json:"is_healthy"`
	HardwareErrors     uint32 `json:"hardware_errors_code"`
	ReallocatedSectors uint32 `json:"reallocated_sectors"`
	BadSectorsPending  uint32 `json:"bad_sectors_pending"`
	CrcErrors          uint32 `json:"crc_errors"`
	PercentageUsed     uint8  `json:"percentage_used"`
	SsdWrittenGb       uint64 `json:"ssd_written_gb"`
	Temperature        uint8  `json:"temperature"`
	PowerOnHours       uint32 `json:"power_on_hours"`
}

// SmartctlOutput структура для десериализации JSON вывода утилиты smartctl
type SmartctlOutput struct {
	// ТОЧЕЧНАЯ ВСТАВКА: Корневые поля паспорта из JSON утилиты smartctl
	SerialNumber    string `json:"serial_number"`
	FirmwareVersion string `json:"firmware_version"`

	SmartStatus struct {
		Passed bool `json:"passed"`
	} `json:"smart_status"`
	PowerOnTime struct {
		Hours uint32 `json:"hours"`
	} `json:"power_on_time"`
	Temperature struct {
		Current uint8 `json:"current"`
	} `json:"temperature"`
	Device struct {
		Type string `json:"type"` // Сюда прилетит "ata", "nvme" или "scsi"
	} `json:"device"`
	AtaSmartAttributes struct {
		Table []struct {
			Id   int    `json:"id"`
			Name string `json:"name"`
			Raw  struct {
				Value uint64 `json:"value"`
			} `json:"raw"`
		} `json:"table"`
	} `json:"ata_smart_attributes"`
	NvmeSmartHealthInformationLog struct {
		Temperature      uint32 `json:"temperature"`
		PowerOnHours     uint32 `json:"power_on_hours"`
		MediaErrors      uint32 `json:"media_errors"`
		PercentageUsed   uint8  `json:"percentage_used"`
		DataUnitsWritten uint64 `json:"data_units_written"`
	} `json:"nvme_smart_health_information_log"`
}

// extractSmartctl извлекает зашитый smartctl.exe во временную папку ОС
func extractSmartctl() (string, error) {
	tmpDir := os.TempDir()
	execPath := filepath.Join(tmpDir, "hostpulse_smartctl.exe")

	if len(smartctlBinary) == 0 {
		return "", fmt.Errorf("массив smartctlBinary пуст! Проверьте //go:embed")
	}

	if _, err := os.Stat(execPath); err == nil {
		_ = os.Remove(execPath)
	}

	err := os.WriteFile(execPath, smartctlBinary, 0755)
	if err != nil {
		return "", err
	}

	return execPath, nil
}

// ИСПРАВЛЕНО: Теперь функция возвращает ещё две строки — серийник (sn) и прошивку (fw)
func GetSmartDataUniversal(targetDeviceID string) (reallocated uint32, pending uint32, crcErrors uint32, pctUsed uint8, writtenGb uint64, temperature uint8, hours uint32, iface string, sn string, fw string) {
	smartctlPath, err := extractSmartctl()
	if err != nil {
		return 0, 0, 0, 0, 0, 0, 0, "Unknown", "Unknown", "Unknown"
	}

	re := regexp.MustCompile(`\d+`)
	driveIndex := re.FindString(targetDeviceID)
	if driveIndex == "" {
		driveIndex = "0"
	}

	cmd := exec.Command(smartctlPath, "-a", "/dev/pd"+driveIndex, "--json")
	output, err := cmd.Output()
	if err != nil && len(output) == 0 {
		return 0, 0, 0, 0, 0, 0, 0, "Unknown", "Unknown", "Unknown"
	}

	var data SmartctlOutput
	if err := json.Unmarshal(output, &data); err != nil {
		return 0, 0, 0, 0, 0, 0, 0, "Unknown", "Unknown", "Unknown"
	}

	iface = "Unknown"
	switch strings.ToLower(data.Device.Type) {
	case "ata":
		iface = "SATA"
	case "nvme":
		iface = "NVMe"
	case "scsi":
		iface = "SATA"
	}

	hours = data.PowerOnTime.Hours
	temperature = data.Temperature.Current

	// Вытаскиваем заначенные серийный номер и прошивку
	sn = "Unknown"
	if data.SerialNumber != "" {
		sn = strings.TrimSpace(data.SerialNumber)
	}
	fw = "Unknown"
	if data.FirmwareVersion != "" {
		fw = strings.TrimSpace(data.FirmwareVersion)
	}

	// Проверяем, NVMe ли это устройство
	if data.NvmeSmartHealthInformationLog.PowerOnHours > 0 {
		hours = data.NvmeSmartHealthInformationLog.PowerOnHours
		reallocated = data.NvmeSmartHealthInformationLog.MediaErrors
		pctUsed = data.NvmeSmartHealthInformationLog.PercentageUsed

		if data.NvmeSmartHealthInformationLog.Temperature > 0 {
			temperature = uint8(data.NvmeSmartHealthInformationLog.Temperature)
		}

		if data.NvmeSmartHealthInformationLog.DataUnitsWritten > 0 {
			bytesWritten := data.NvmeSmartHealthInformationLog.DataUnitsWritten * 512000
			writtenGb = bytesWritten / (1024 * 1024 * 1024)
		}
	} else {
		for _, attr := range data.AtaSmartAttributes.Table {
			switch attr.Id {
			case 5:
				reallocated = uint32(attr.Raw.Value)
			case 197:
				pending = uint32(attr.Raw.Value)
			case 199:
				crcErrors = uint32(attr.Raw.Value)
			}
		}
	}

	return reallocated, pending, crcErrors, pctUsed, writtenGb, temperature, hours, iface, sn, fw
}

// GetAdvancedDiskStatus собирает итоговый слайс данных по дискам в системе
func GetAdvancedDiskStatus() []AdvancedDiskStatus {
	fmt.Println(" [💾 DISK SERVICE] Сбор расширенной статистики дисков...")
	var results []AdvancedDiskStatus

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

			// ИСПРАВЛЕНО: Ловим новые 2 строковых параметра (serialNumber, firmwareVersion) на выходе из функции
			realSectors, pendingSectors, cableErrors, wearPct, gbWritten, temp, workHours, smartIface, serialNumber, firmwareVersion := GetSmartDataUniversal(targetID)

			finalInterface := busType
			if finalInterface == "Unknown" && smartIface != "Unknown" {
				finalInterface = smartIface
			}

			results = append(results, AdvancedDiskStatus{
				DeviceID:           disk.DeviceId,
				Model:              disk.FriendlyName,
				Type:               mediaType,
				Interface:          finalInterface,
				SerialNumber:       serialNumber,     // Записываем серийник
				FirmwareVersion:    firmwareVersion,  // Записываем прошивку
				HealthStatus:       health,
				IsHealthy:          health == "Healthy",
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
	}
	return results
}
