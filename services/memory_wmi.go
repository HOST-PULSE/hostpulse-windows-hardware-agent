package services

import (
	"fmt"
	"github.com/yusufpapurcu/wmi"
)

type MemorySlotStatus struct {
	SlotLabel          string `json:"slot_label"`           // Например: DIMM_A1
	Manufacturer       string `json:"manufacturer"`         // Например: Samsung, Kingston
	CapacityGB         uint32 `json:"capacity_gb"`          // Объем планки в ГБ
	SpeedMhz           uint32 `json:"speed_mhz"`            // Частота работы
	CorrectableErrors   uint64 `json:"correctable_errors"`   // Исправленные ошибки ECC
	UncorrectableErrors uint64 `json:"uncorrectable_errors"` // Критические ошибки ECC
}

// Win32_MemoryDevice содержит счетчики ошибок ECC
type Win32_MemoryDevice struct {
	DeviceID               string
	CorrectableErrorCount   *uint64 // Используем указатель, так как на обычных ПК поля может не быть
	UncorrectableErrorCount *uint64
}

// Win32_PhysicalMemory содержит паспортные данные планок
type Win32_PhysicalMemory struct {
	DeviceID     string
	BankLabel    string
	Manufacturer string
	Capacity     uint64
	Speed        uint32
}

// GetMemoryHardwareStatus собирает детальное состояние по каждой планке памяти
func GetMemoryHardwareStatus() []MemorySlotStatus {
	var results []MemorySlotStatus

	// 1. Читаем счетчики ошибок
	var wmiDevices []Win32_MemoryDevice
	queryDevices := "SELECT DeviceID, CorrectableErrorCount, UncorrectableErrorCount FROM Win32_MemoryDevice"
	errDevices := wmi.Query(queryDevices, &wmiDevices)

	// Создаем карту ошибок для быстрого сопоставления по DeviceID
	errorMap := make(map[string]Win32_MemoryDevice)
	if errDevices == nil {
		for _, dev := range wmiDevices {
			errorMap[dev.DeviceID] = dev
		}
	}

	// 2. Читаем паспортные данные планок
	var wmiPhysical []Win32_PhysicalMemory
	queryPhysical := "SELECT DeviceID, BankLabel, Manufacturer, Capacity, Speed FROM Win32_PhysicalMemory"
	errPhysical := wmi.Query(queryPhysical, &wmiPhysical)

	if errPhysical != nil || len(wmiPhysical) == 0 {
		fmt.Printf(" [❌ RAM ERROR] Не удалось получить данные Win32_PhysicalMemory: %v\n", errPhysical)
		return nil
	}

	for _, ram := range wmiPhysical {
		var corrErr, uncorrErr uint64

		// Сопоставляем планку со счетчиком её ошибок по DeviceID
		if dev, exists := errorMap[ram.DeviceID]; exists {
			if dev.CorrectableErrorCount != nil {
				corrErr = *dev.CorrectableErrorCount
			}
			if dev.UncorrectableErrorCount != nil {
				uncorrErr = *dev.UncorrectableErrorCount
			}
		}

		// Переводим байты емкости в Гигабайты
		gb := uint32(ram.Capacity / (1024 * 1024 * 1024))

		results = append(results, MemorySlotStatus{
			SlotLabel:           ram.BankLabel,
			Manufacturer:        ram.Manufacturer,
			CapacityGB:          gb,
			SpeedMhz:            ram.Speed,
			CorrectableErrors:   corrErr,
			UncorrectableErrors: uncorrErr,
		})
	}

	if len(results) > 0 {
		fmt.Printf(" [✅ RAM LOG] Данные по планкам памяти успешно собраны. Найдено модулей: %d\n", len(results))
	}
	return results
}
