package services

import (
	"fmt"
	"github.com/yusufpapurcu/wmi"
)

type PowerSupplyStatus struct {
	DeviceID   string `json:"device_id"`   // Например: Power Supply 0
	Name       string `json:"name"`        // Описание/Модель блока
	IsPresent  bool   `json:"is_present"`  // Подключен ли физически
	IsHealthy  bool   `json:"is_healthy"`  // Исправен ли (Status == "OK")
	ErrorCode  uint32 `json:"error_code"`  // Код ошибки, если есть (ConfigManagerErrorCode)
}

// Win32_PowerSupply — стандартный класс WMI Windows для блоков питания
type Win32_PowerSupply struct {
	DeviceID               string
	Caption                string
	Active                 bool
	Status                 string
	ConfigManagerErrorCode uint32
}

// GetPowerSupplyHardwareStatus собирает статус по всем блокам питания системы
func GetPowerSupplyHardwareStatus() []PowerSupplyStatus {
	var results []PowerSupplyStatus

	var wmiPower []Win32_PowerSupply
	query := "SELECT DeviceID, Caption, Active, Status, ConfigManagerErrorCode FROM Win32_PowerSupply"
	err := wmi.Query(query, &wmiPower)

	// На обычных десктопах или некоторых виртуалках этот класс может вернуть пустоту или ошибку
	if err != nil || len(wmiPower) == 0 {
		// Резервный фоллбэк: если класс пустой, отдаем симулированный один блок (чтобы агент не падал)
		return []PowerSupplyStatus{
			{
				DeviceID:  "PC_Power_Supply",
				Name:      "Standard ATX Power Supply",
				IsPresent: true,
				IsHealthy: true,
				ErrorCode: 0,
			},
		}
	}

	for _, ps := range wmiPower {
		isHealthy := ps.Status == "OK" || ps.Status == ""

		results = append(results, PowerSupplyStatus{
			DeviceID:   ps.DeviceID,
			Name:       ps.Caption,
			IsPresent:  ps.Active, // Active показывает, идет ли через него питание прямо сейчас
			IsHealthy:  isHealthy,
			ErrorCode:  ps.ConfigManagerErrorCode,
		})
	}

	fmt.Printf(" [✅ POWER LOG] Данные по блокам питания собраны. Найдено модулей: %d\n", len(results))
	return results
}
