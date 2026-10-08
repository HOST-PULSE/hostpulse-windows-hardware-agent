package services

import (
	"fmt"

	"github.com/yusufpapurcu/wmi"
)

// Win32_Fan — стандартный класс Windows для вентиляторов охлаждения
type Win32_Fan struct {
	DeviceID    string
	Name        string
	DesiredSpeed uint64 // Ожидаемая скорость вращения (если поддерживается)
	Status      string // Статус работы (например, "OK")
}

// FanStatus представляет структуру кулера для бэкенда HostPulse
type FanStatus struct {
	FanID  string `json:"fan_id"`
	Name   string `json:"name"`
	Status string `json:"status"`
}

// GetFansStatus опрашивает состояние вентиляторов охлаждения
func GetFansStatus() []FanStatus {
	var dst []Win32_Fan
	query := "SELECT DeviceID, Name, DesiredSpeed, Status FROM Win32_Fan"

	// Используем стандартное пространство root\cimv2 (админские права всё равно желательны)
	err := wmi.QueryNamespace(query, &dst, `root\cimv2`)
	if err != nil {
		fmt.Printf(" [❌ ERROR] Не удалось получить статус вентиляторов: %v\n", err)
		return nil
	}

	var results []FanStatus
	for _, fan := range dst {
		// Если Windows не может прочесть имя, даем дефолтное на основе ID
		name := fan.Name
		if name == "" {
			name = "System Fan " + fan.DeviceID
		}

		results = append(results, FanStatus{
			FanID:  fan.DeviceID,
			Name:   name,
			Status: fan.Status, // Обычно возвращает "OK", "Degraded" или "Unknown"
		})
	}

	return results
}
