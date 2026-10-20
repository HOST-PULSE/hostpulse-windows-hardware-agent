package services

import (
	"bytes"
	"fmt"
	"math"
	"os/exec"
	"strings"

	"github.com/yusufpapurcu/wmi"
)

type Win32_ComputerSystem struct {
	TotalPhysicalMemory uint64 // Память в байтах
}

type Win32_BaseBoard struct {
	Manufacturer string // Производитель платы (Asus, Gigabyte и т.д.)
	Product      string // Модель платы
}

// HardwareInfo представляет итоговый паспорт машины для HostPulse
type HardwareInfo struct {
	CpuModel    string `json:"cpu_model"`
	Motherboard string `json:"motherboard"`
	TotalRamGb  uint32 `json:"ram_total_gb"` // Соответствует ram_total_gb в DTO
	OsType      string `json:"os_type"`      // ИСПРАВЛЕНО: Добавлено поле операционной системы
}

// GetHardwareStaticInfo собирает паспортные данные системы (вызывается 1 раз при старте)
func GetHardwareStaticInfo() HardwareInfo {
	var info HardwareInfo

	// 1. Получаем модель процессора через системный реестр Windows (100% обход блокировок WMI)
	cmdCpu := exec.Command("reg", "query", "HKLM\\HARDWARE\\DESCRIPTION\\System\\CentralProcessor\\0", "/v", "ProcessorNameString")
	var stdoutCpu bytes.Buffer
	cmdCpu.Stdout = &stdoutCpu
	if err := cmdCpu.Run(); err == nil {
		// Очищаем и вытаскиваем строку после REG_SZ
		parts := strings.Split(stdoutCpu.String(), "REG_SZ")
		if len(parts) >= 2 {
			info.CpuModel = strings.TrimSpace(parts[1])
			info.CpuModel = strings.ReplaceAll(info.CpuModel, "\(\r\)", "")
			info.CpuModel = strings.ReplaceAll(info.CpuModel, "\n", "")
		}
	}

	// Резервный фоллбэк для CPU, если реестр недоступен
	if info.CpuModel == "" {
		cmdWmic := exec.Command("wmic", "cpu", "get", "name")
		var stdoutWmic bytes.Buffer
		cmdWmic.Stdout = &stdoutWmic
		if err := cmdWmic.Run(); err == nil {
			lines := strings.Split(stdoutWmic.String(), "\n")
			if len(lines) >= 2 && strings.TrimSpace(lines[1]) != "" {
				info.CpuModel = strings.TrimSpace(lines[1])
			}
		}
	}

	if info.CpuModel == "" {
		info.CpuModel = "Unknown Windows CPU"
	}

	// 2. Получаем версию операционной системы (ИСПРАВЛЕНО: Прямой вызов ядра)
	cmdOs := exec.Command("cmd", "/c", "ver")
	var stdoutOs bytes.Buffer
	cmdOs.Stdout = &stdoutOs
	if err := cmdOs.Run(); err == nil {
		info.OsType = strings.TrimSpace(stdoutOs.String())
		info.OsType = strings.ReplaceAll(info.OsType, "\(\r\)", "")
		info.OsType = strings.ReplaceAll(info.OsType, "\n", "")
	} else {
		info.OsType = "Microsoft Windows Server"
	}

	// 3. Получаем объем оперативной памяти (WMI класса Win32_ComputerSystem)
	var sys []Win32_ComputerSystem
	if err := wmi.Query("SELECT TotalPhysicalMemory FROM Win32_ComputerSystem", &sys); err == nil && len(sys) > 0 {
		gb := float64(sys[0].TotalPhysicalMemory) / (1024 * 1024 * 1024)
		info.TotalRamGb = uint32(math.Ceil(gb))
	} else {
		info.TotalRamGb = 0
	}

	// 4. Получаем данные материнской платы
	var board []Win32_BaseBoard
	if err := wmi.Query("SELECT Manufacturer, Product FROM Win32_BaseBoard", &board); err == nil && len(board) > 0 {
		info.Motherboard = fmt.Sprintf("%s %s", strings.TrimSpace(board[0].Manufacturer), strings.TrimSpace(board[0].Product))
	} else {
		info.Motherboard = "Unknown Motherboard"
	}

	return info
}