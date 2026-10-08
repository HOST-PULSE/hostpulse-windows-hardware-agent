package services

import (
	"fmt"
	"math"

	"github.com/yusufpapurcu/wmi"
)

// Структуры для маппинга WMI ответов
type Win32_Processor struct {
	Name string // Модель процессора
}

type Win32_ComputerSystem struct {
	TotalPhysicalMemory uint64 // Память в байтах
}

type Win32_BaseBoard struct {
	Manufacturer string // Производитель платы (Asus, Gigabyte и т.д.)
	Product      string // Модель платы
}

// HardwareInfo представляет итоговый паспорт машины для HostPulse
type HardwareInfo struct {
	CpuModel     string `json:"cpu_model"`
	Motherboard  string `json:"motherboard"`
	TotalRamGb   uint32 `json:"total_ram_gb"`
}

// GetHardwareStaticInfo собирает паспортные данные системы (вызывается 1 раз при старте)
func GetHardwareStaticInfo() HardwareInfo {
	var info HardwareInfo

	// 1. Получаем модель процессора
	var processors []Win32_Processor
	if err := wmi.Query("SELECT Name FROM Win32_Processor", &processors); err == nil && len(processors) > 0 {
		info.CpuModel = processors[0].Name
	} else {
		info.CpuModel = "Unknown CPU"
	}

	// 2. Получаем объем оперативной памяти
	var sys []Win32_ComputerSystem
	if err := wmi.Query("SELECT TotalPhysicalMemory FROM Win32_ComputerSystem", &sys); err == nil && len(sys) > 0 {
		// Переводим байты в Гигабайты с округлением в большую сторону
		gb := float64(sys[0].TotalPhysicalMemory) / (1024 * 1024 * 1024)
		info.TotalRamGb = uint32(math.Ceil(gb))
	}

	// 3. Получаем данные материнской платы
	var board []Win32_BaseBoard
	if err := wmi.Query("SELECT Manufacturer, Product FROM Win32_BaseBoard", &board); err == nil && len(board) > 0 {
		info.Motherboard = fmt.Sprintf("%s %s", board[0].Manufacturer, board[0].Product)
	} else {
		info.Motherboard = "Unknown Motherboard"
	}

	return info
}
