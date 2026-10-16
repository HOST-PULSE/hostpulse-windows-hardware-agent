package services

import (
	"fmt"
	"strings"
	"syscall"
	"unsafe"
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
	VendorSpecific [512]uint8 // Возвращаем фиксированный размер массива байт
}

const (
	GENERIC_READ               = 0x80000000
	GENERIC_WRITE              = 0x40000000
	FILE_SHARE_READ           = 0x00000001
	FILE_SHARE_WRITE          = 0x00000002
	OPEN_EXISTING             = 3
	IOCTL_SMART_RCV_DRIVE_DATA = 0x0007c088
)

type ATA_REGISTERS struct {
	Features     byte
	SectorCount  byte
	SectorNumber byte
	CylLow       byte
	CylHigh      byte
	DriveHead    byte
	Command      byte
	Reserved     byte
}

type SENDCMDINPARAMS struct {
	BufferSize  uint32
	AtaRegs     ATA_REGISTERS
	DriveNumber byte
	Reserved    byte
	DwReserved  uint32
}

func GetAdvancedDiskStatus() []AdvancedDiskStatus {
	fmt.Println(" [💾 DISK SERVICE] Сбор расширенной статистики дисков...")

	var results []AdvancedDiskStatus

	// 1. СОВРЕМЕННЫЙ МЕТОД (Windows 10/11, Server 2016+)
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

			// КАСКАД СБОРА SMART
			realSectors, temp, workHours := GetSmartMetrics(targetID)
			if realSectors == 0 && temp == 0 && workHours == 0 {
				realSectors, temp, workHours = GetSmartNative(targetID)
			}

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

	// 2. БЕЗОПАСНЫЙ ОТКАТ (Для старых Windows Server или специфичных виртуалок)
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

		// КАСКАД СБОРА SMART ДЛЯ ОТКАТА
		realSectors, temp, workHours := GetSmartMetrics(disk.DeviceID)
		if realSectors == 0 && temp == 0 && workHours == 0 {
			realSectors, temp, workHours = GetSmartNative(disk.DeviceID)
		}

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

	err := wmi.QueryNamespace("SELECT InstanceName, VendorSpecific FROM MSStorageDriver_ATAPISmartData", &smartData, `root\WMI`)
	if err != nil {
		return 0, 0, 0
	}

	cleanTarget := strings.ToLower(strings.ReplaceAll(deviceID, `\\.\`, ""))

	for _, data := range smartData {
		cleanInstance := strings.ToLower(data.InstanceName)

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

		for i := 2; i < 362; i += 12 {
			attrID := data.VendorSpecific[i]
			if attrID == 0 {
				break
			}

			v5 := uint32(data.VendorSpecific[i+5])
			v6 := uint32(data.VendorSpecific[i+6])
			v7 := uint32(data.VendorSpecific[i+7])
			v8 := uint32(data.VendorSpecific[i+8])
			rawValue := v5 | (v6 << 8) | (v7 << 16) | (v8 << 24)

			switch attrID {
			case 5:
				reallocated = rawValue
			case 9:
				hours = rawValue
			case 194:
				temperature = data.VendorSpecific[i+5]
			}
		}
		return reallocated, temperature, hours
	}
	return 0, 0, 0
}

func GetSmartNative(deviceID string) (reallocated uint32, temperature uint8, hours uint32) {
	cleanID := deviceID
	if !strings.HasPrefix(cleanID, `\\.\`) {
		cleanID = `\\.\` + cleanID
	}

	pathPtr, err := syscall.UTF16PtrFromString(cleanID)
	if err != nil {
		return 0, 0, 0
	}

	handle, err := syscall.CreateFile(
		pathPtr,
		GENERIC_READ|GENERIC_WRITE,
		FILE_SHARE_READ|FILE_SHARE_WRITE,
		nil,
		OPEN_EXISTING,
		0,
		0,
	)
	if err != nil {
		return 0, 0, 0
	}
	defer syscall.CloseHandle(handle)

	var scip SENDCMDINPARAMS
	scip.BufferSize = 512
	scip.AtaRegs.Features = 0xD0
	scip.AtaRegs.SectorCount = 1
	scip.AtaRegs.SectorNumber = 1
	scip.AtaRegs.CylLow = 0x4F
	scip.AtaRegs.CylHigh = 0xC2
	scip.AtaRegs.Command = 0xB0

	driveNum := byte(0)
	for i := len(cleanID) - 1; i >= 0; i-- {
		if cleanID[i] >= '0' && cleanID[i] <= '9' {
			driveNum = cleanID[i] - '0'
			break
		}
	}
	scip.DriveNumber = driveNum

	outBuffer := make([]byte, 16+512)
	var bytesReturned uint32

	err = syscall.DeviceIoControl(
		handle,
		IOCTL_SMART_RCV_DRIVE_DATA,
		(*byte)(unsafe.Pointer(&scip)),
		uint32(unsafe.Sizeof(scip)),
		&outBuffer,
		uint32(len(outBuffer)),
		&bytesReturned,
		nil,
	)
	if err != nil {
		return 0, 0, 0
	}

	smartAttributes := outBuffer[16:]

	for i := 2; i < 362; i += 12 {
		if i+11 >= len(smartAttributes) {
			break
		}

		attrID := smartAttributes[i]
		if attrID == 0 {
			continue
		}

		v5 := uint32(smartAttributes[i+5])
		v6 := uint32(smartAttributes[i+6])
		v7 := uint32(smartAttributes[i+7])
		v8 := uint32(smartAttributes[i+8])
		rawValue := v5 | (v6 << 8) | (v7 << 16) | (v8 << 24)

		switch attrID {
		case 5:
			reallocated = rawValue
		case 9:
			hours = rawValue
		case 194:
			temperature = smartAttributes[i+5]
		}
	}

	return reallocated, temperature, hours
}
