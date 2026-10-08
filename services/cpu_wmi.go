package services

import (
	"fmt"
	"math"

	"github.com/yusufpapurcu/wmi"
)

// MSAcpi_ThermalZoneTemperature — WMI класс Windows для температурных зон
type MSAcpi_ThermalZoneTemperature struct {
	InstanceName       string
	CurrentTemperature uint32 // Значение в десятых долях Кельвина
}

// CpuTempStatus — структура для передачи данных о температуре процессора
type CpuTempStatus struct {
	ZoneName string  `json:"zone_name"`
	Celsius  float64 `json:"celsius"`
}

// GetCpuTemperatures опрашивает WMI и возвращает срез температур
func GetCpuTemperatures() []CpuTempStatus {
	var dst []MSAcpi_ThermalZoneTemperature
	query := "SELECT InstanceName, CurrentTemperature FROM MSAcpi_ThermalZoneTemperature"

	// Запрашиваем из пространства root\wmi (требуются права администратора)
	err := wmi.QueryNamespace(query, &dst, `root\wmi`)
	if err != nil {
		fmt.Printf(" [❌ ERROR] Не удалось получить температуру CPU: %v\n", err)
		return nil
	}

	var results []CpuTempStatus
	for _, zone := range dst {
		// Переводим Кельвины*10 в нормальные градусы Цельсия
		kelvin := float64(zone.CurrentTemperature) / 10.0
		celsius := kelvin - 273.15
		celsiusRounded := math.Round(celsius*10) / 10

		results = append(results, CpuTempStatus{
			ZoneName: zone.InstanceName,
			Celsius:  celsiusRounded,
		})
	}

	return results
}
