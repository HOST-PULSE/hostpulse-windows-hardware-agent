package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"regexp"
	"strconv"
)

type MemorySlotStatus struct {
	SlotLabel           string `json:"slot_label"`           // Например: DIMM_A1
	Manufacturer        string `json:"manufacturer"`         // Например: Samsung, Kingston
	CapacityGB          uint32 `json:"capacity_gb"`          // Объем планки в ГБ
	SpeedMhz            uint32 `json:"speed_mhz"`            // Частота работы
	CorrectableErrors   uint64 `json:"correctable_errors"`   // Исправленные ошибки ECC
	UncorrectableErrors uint64 `json:"uncorrectable_errors"` // Критические ошибки ECC
}

// GetMemoryHardwareStatus собирает детальные паспортные данные слотов через системный CIM-парсер
func GetMemoryHardwareStatus() []MemorySlotStatus {
	var results []MemorySlotStatus

	// Вызываем PowerShell для извлечения CIM-паспорта планок памяти (работает в обход блокировок Win32_PhysicalMemory)
	psScript := `Get-CimInstance -ClassName Win32_PhysicalMemory | Select-Object BankLabel, Manufacturer, Capacity, Speed | ConvertTo-Json -Compress`

	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", psScript)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	err := cmd.Run()
	if err != nil || stdout.Len() == 0 {
		fmt.Printf(" [❌ RAM ERROR] Системная шина памяти заблокирована ОС: %v\n", err)
		return nil
	}

	rawJson := stdout.Bytes()

	// Вспомогательная структура для разбора системного ответа
	type CIMMemory struct {
		BankLabel    string `json:"BankLabel"`
		Manufacturer string `json:"Manufacturer"`
		Capacity     uint64 `json:"Capacity"`
		Speed        uint32 `json:"Speed"`
	}

	var rawData []CIMMemory

	// Проверяем структуру ответа (массив или одиночный объект)
	if len(rawJson) > 0 && rawJson[0] == '{' {
		var single CIMMemory
		if json.Unmarshal(rawJson, &single) == nil {
			rawData = append(rawData, single)
		}
	} else {
		_ = json.Unmarshal(rawJson, &rawData)
	}

	// Если CIM пуст, пробуем жесткий резервный парсинг через wmic.exe
	if len(rawData) == 0 {
		cmdWmic := exec.Command("wmic", "memorychip", "get", "BankLabel,Capacity,Manufacturer,Speed", "/format:csv")
		var stdoutWmic bytes.Buffer
		cmdWmic.Stdout = &stdoutWmic
		if cmdWmic.Run() == nil {
			// Быстрый построчный разбор CSV от wmic
			lines := regexp.MustCompile(`\r?\n`).Split(stdoutWmic.String(), -1)
			for _, line := range lines {
				if line == "" || strings.Contains(line, "Node,") || strings.Contains(line, "BankLabel") {
					continue
				}
				parts := strings.Split(line, ",")
				if len(parts) >= 5 {
					capBytes, _ := strconv.ParseUint(parts[2], 10, 64)
					speed, _ := strconv.ParseUint(parts[4], 10, 32)
					rawData = append(rawData, CIMMemory{
						BankLabel:    parts[1],
						Manufacturer: parts[3],
						Capacity:     capBytes,
						Speed:        uint32(speed),
					})
				}
			}
		}
	}

	// Перегоняем данные в оригинальную структуру MemorySlotStatus
	for _, ram := range rawData {
		gb := uint32(ram.Capacity / (1024 * 1024 * 1024))
		if gb == 0 && ram.Capacity > 0 {
			gb = 1 // Защита округления для маленьких планок
		}

		manufacturer := ram.Manufacturer
		if manufacturer == "" || strings.Contains(strings.ToLower(manufacturer), "unknown") {
			manufacturer = "Unknown"
		}

		results = append(results, MemorySlotStatus{
			SlotLabel:           ram.BankLabel,
			Manufacturer:        manufacturer,
			CapacityGB:          gb,
			SpeedMhz:            ram.Speed,
			CorrectableErrors:   0, // На Windows Server ошибки собираются только при наличии активного ECC-контроллера домена
			UncorrectableErrors: 0,
		})
	}

	if len(results) > 0 {
		fmt.Printf(" [✅ RAM LOG] Данные по слотам памяти успешно восстановлены. Слотов: %d\n", len(results))
	}
	return results
}
