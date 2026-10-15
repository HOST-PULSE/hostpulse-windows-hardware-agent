package services

import (
	"fmt"
	"math"

	"github.com/yusufpapurcu/wmi"
)
type MSAcpi_ThermalZoneTemperature struct {
	InstanceName       string
	CurrentTemperature uint32
}

// Высокоточный класс CIMV2 для Windows Server 2012 R2 / 2016 / 2019 (Не требует софта)
type Win32_PerfFormattedData_Counters_ThermalZoneInformation struct {
	Name                     string
	Temperature              uint32 // Базовое поле (в Кельвинах)
	HighPrecisionTemperature uint32 // Высокоточное поле ядра (в десятых долях Кельвина)
}

type CpuTempStatus struct {
	ZoneName string  `json:"zone_name"`
	Celsius  float64 `json:"celsius"`
}

// GetCpuTemperatures вытаскивает РЕАЛЬНЫЕ градусы встроенными силами Windows и пишет логи
func GetCpuTemperatures() []CpuTempStatus {

	// --- МЕТОД 1: Высокоточный серверный счетчик ядра (Для Windows Server 2012 R2) ---
	var dstServer []Win32_PerfFormattedData_Counters_ThermalZoneInformation
	queryServer := "SELECT Name, Temperature, HighPrecisionTemperature FROM Win32_PerfFormattedData_Counters_ThermalZoneInformation"

	// Запрашиваем из root\cimv2 (работает везде, встроен в ОС)
	errServer := wmi.Query(queryServer, &dstServer)
	if errServer != nil {
		// Логируем ошибку серверного счетчика, чтобы понимать, почему он не сработал
		fmt.Printf(" [ℹ️ CPU LOG] Серверный счетчик CIMV2 вернул ошибку: %v. Пробуем резервный метод...\n", errServer)
	} else if len(dstServer) > 0 {
		var results []CpuTempStatus
		for _, zone := range dstServer {
			var celsius float64

			// 1. Проверяем высокоточный счетчик ядра (десятые доли Кельвина)
			if zone.HighPrecisionTemperature > 0 {
				kelvin := float64(zone.HighPrecisionTemperature) / 10.0
				celsius = kelvin - 273.15
			} else if zone.Temperature > 0 {
				// 2. Если высокоточный пуст, берем стандартный (в чистых Кельвинах)
				celsius = float64(zone.Temperature) - 273.15
			} else {
				continue // Пропускаем неинициализированные зоны
			}

			// Фильтруем заведомо ложные значения (иногда BIOS отдает системный мусор)
			if celsius > -40 && celsius < 130 {
				results = append(results, CpuTempStatus{
					ZoneName: zone.Name,
					Celsius:  math.Round(celsius*10) / 10,
				})
			}
		}

		if len(results) > 0 {
			fmt.Printf(" [✅ CPU LOG] Успешно собраны данные через серверный счетчик CIMV2. Датчиков: %d\n", len(results))
			return results // Возвращаем реальные физические градусы сервера!
		}
	}

	// --- МЕТОД 2: Классический ACPI (Резервный вариант для домашних ПК пользователей) ---
	var dstAcpi []MSAcpi_ThermalZoneTemperature
	queryAcpi := "SELECT InstanceName, CurrentTemperature FROM MSAcpi_ThermalZoneTemperature"

	errAcpi := wmi.QueryNamespace(queryAcpi, &dstAcpi, `root\wmi`)
	if errAcpi != nil {
		// Логируем ошибку ACPI (на Windows Server 2012 R2 мы увидим здесь "Не поддерживается")
		fmt.Printf(" [ℹ️ CPU LOG] Классический ACPI root\\wmi вернул ошибку: %v\n", errAcpi)
	} else if len(dstAcpi) > 0 {
		var results []CpuTempStatus
		for _, zone := range dstAcpi {
			celsius := (float64(zone.CurrentTemperature) / 10.0) - 273.15
			if celsius > -40 && celsius < 130 {
				results = append(results, CpuTempStatus{
					ZoneName: zone.InstanceName,
					Celsius:  math.Round(celsius*10) / 10,
				})
			}
		}
		fmt.Printf(" [✅ CPU LOG] Успешно собраны данные через ACPI root\\wmi. Датчиков: %d\n", len(results))
		return results
	}

	// --- МЕТОД 3: Крайний случай (Если на сервере физически заблокированы или вырезаны термодатчики в BIOS) ---
	// Логируем факт полной блокировки подсистемы датчиков
	fmt.Println(" [⚠️ CPU LOG] Внимание: все встроенные подсистемы датчиков температуры Windows вернули пустой результат.")

	// Отправляем маркер -1, но пишем понятную зону, чтобы бэкенд вывел красивое уведомление в ЛК
	return []CpuTempStatus{
		{
			ZoneName: "Hardware_Counters_Not_Available_In_OS",
			Celsius:  -1.0,
		},
	}
}