package services

import (
	"fmt"
	"math"

	"github.com/yusufpapurcu/wmi"
)
type MSAcpi_ThermalZoneTemperature struct {
	InstanceName       string
	CurrentTemperature uint32 // В десятых долях Кельвина
}

// Класс №2: Для серверных Windows (Server 2012 R2 / 2016 / 2019)
type Win32_PerfFormattedData_Counters_ThermalZoneInformation struct {
	Name        string
	Temperature uint32 // В Кельвинах
}

type CpuTempStatus struct {
	ZoneName string  `json:"zone_name"`
	Celsius  float64 `json:"celsius"`
}

// GetCpuTemperatures собирает исключительно ТЕМПЕРАТУРУ из доступных системных источников
func GetCpuTemperatures() []CpuTempStatus {
	// --- ПОПЫТКА 1: Классический ACPI WMI (для обычных ПК) ---
	var dstAcpi []MSAcpi_ThermalZoneTemperature
	queryAcpi := "SELECT InstanceName, CurrentTemperature FROM MSAcpi_ThermalZoneTemperature"

	errAcpi := wmi.QueryNamespace(queryAcpi, &dstAcpi, `root\wmi`)
	if errAcpi == nil && len(dstAcpi) > 0 {
		var results []CpuTempStatus
		for _, zone := range dstAcpi {
			// Перевод из десятых долей Кельвина в Цельсий
			celsius := (float64(zone.CurrentTemperature) / 10.0) - 273.15
			results = append(results, CpuTempStatus{
				ZoneName: zone.InstanceName,
				Celsius:  math.Round(celsius*10) / 10,
			})
		}
		return results
	}

	// --- ПОПЫТКА 2: Серверный счетчик (Специально для Windows Server 2012 R2) ---
	var dstServer []Win32_PerfFormattedData_Counters_ThermalZoneInformation
	queryServer := "SELECT Name, Temperature FROM Win32_PerfFormattedData_Counters_ThermalZoneInformation"

	// Запрашиваем из стандартного пространства root\cimv2 (работает на серверах)
	errServer := wmi.Query(queryServer, &dstServer)
	if errServer == nil && len(dstServer) > 0 {
		var results []CpuTempStatus
		for _, zone := range dstServer {
			// Этот счетчик возвращает температуру в чистых Кельвинах
			celsius := float64(zone.Temperature) - 273.15

			// Иногда старые материнские платы возвращают некорректный 0 в Кельвинах, фильтруем его
			if zone.Temperature > 0 {
				results = append(results, CpuTempStatus{
					ZoneName: zone.Name,
					Celsius:  math.Round(celsius*10) / 10,
				})
			}
		}

		// Если данные успешно собраны, возвращаем их
		if len(results) > 0 {
			return results
		}
	}

	// --- ПОПЫТКА 3: Крайний случай (Если ОС наглухо закрыла все датчики) ---
	// Чтобы воркер CPU не прерывал выполнение по return и слал пустые пакеты на бэкенд
	fmt.Println(" [⚠️ CPU] Аппаратные датчики температуры заблокированы политиками Windows Server.")
	return []CpuTempStatus{
		{
			ZoneName: "ACPI_Sensors_Disabled_By_OS",
			Celsius:  -1.0, // Маркер для панели, что датчики недоступны
		},
	}
}