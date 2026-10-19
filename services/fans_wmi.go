package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"

	"github.com/yusufpapurcu/wmi"
)

type FanStatus struct {
	FanName string  `json:"fan_name"`
	RPM     float64 `json:"rpm"`
}

// Структура для чтения IPMI через WMI на физических серверах
type Win32_NumericSensor struct {
	Caption      string
	CurrentReading int64
	SensorType   uint16
}

// GetFansStatus автоматически выбирает метод опроса (Драйвер или IPMI)
func GetFansStatus() []FanStatus {
	var results []FanStatus

	// --- МЕТОД 1: Опрос через OpenHardwareMonitorLib.dll ---
	tmpDir := os.TempDir()
	libPath := filepath.Join(tmpDir, "hostpulse_ohm_core.dll")

	if _, err := os.Stat(libPath); err == nil {
		psScript := fmt.Sprintf(`
			[System.Reflection.Assembly]::LoadFile('%s') | Out-Null
			$computer = New-Object OpenHardwareMonitor.Hardware.Computer
			$computer.MainboardEnabled = $true
			$computer.FanControllerEnabled = $true
			$computer.Open()
			
			$fans = @()
			foreach ($hardware in $computer.Hardware) {
				$hardware.Update()
				foreach ($sub in $hardware.SubHardware) { $sub.Update() }
				
				$allHardware = @($hardware) + $hardware.SubHardware
				foreach ($hw in $allHardware) {
					foreach ($sensor in $hw.Sensors) {
						if ($sensor.SensorType -eq 'Fan') {
							$fans += [pscustomobject]@{
								zone_name = $sensor.Name
								celsius = [math]::Round($sensor.Value, 0)
							}
						}
					}
				}
			}
			$computer.Close()
			if ($fans.Count -gt 0) { ConvertTo-Json -InputObject @($fans) -Compress }
		`, filepath.ToSlash(libPath))

		cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", psScript)
		var stdout bytes.Buffer
		cmd.Stdout = &stdout
		_ = cmd.Run()

		if stdout.Len() > 0 {
			re := regexp.MustCompile(`\[\s*\{.*\}\s*\]`)
			cleanJson := re.FindString(stdout.String())
			if cleanJson == "" { cleanJson = stdout.String() }

			var rawData []CpuTempStatus
			if json.Unmarshal([]byte(cleanJson), &rawData) == nil || json.Unmarshal([]byte(cleanJson), &singleResultDummy) == nil {
				// (Если парсинг успешен, переносим данные)
				for _, item := range rawData {
					results = append(results, FanStatus{FanName: item.ZoneName, RPM: item.Celsius})
				}
				if len(results) > 0 {
					fmt.Printf(" [✅ FANS LOG] Датчики кулеров считаны через DLL. Найдено: %d\n", len(results))
					return results
				}
			}
		}
	}

	// --- МЕТОД 2: Серверный откат через Аппаратный IPMI (Для HP/Dell/Supermicro) ---
	fmt.Println(" [🔍 FANS SERVICE] Опрос через DLL не дал результатов. Пробуем серверный IPMI...")
	var ipmiData []Win32_NumericSensor
	// SensorType 11 означает "Датчик охлаждения/Вентилятор" по спецификации IPMI
	queryIpmi := "SELECT Caption, CurrentReading, SensorType FROM Win32_NumericSensor WHERE SensorType = 11"

	err := wmi.Query(queryIpmi, &ipmiData)
	if err == nil && len(ipmiData) > 0 {
		for _, sensor := range ipmiData {
			// Отсекаем пустые или отключенные вентиляторы
			if sensor.CurrentReading > 0 {
				results = append(results, FanStatus{
					FanName: sensor.Caption,
					RPM:     float64(sensor.CurrentReading),
				})
			}
		}
	}

	if len(results) > 0 {
		fmt.Printf(" [✅ FANS LOG] Обороты серверных кулеров успешно считаны через IPMI WMI. Найдено: %d\n", len(results))
		return results
	}

	// --- МЕТОД 3: Крайний фоллбэк, если вентиляторы полностью скрыты на уровне ядра ОС ---
	fmt.Println(" [⚠️ FANS LOG] Внимание: вентиляторы не найдены на этой конфигурации оборудования.")
	return nil
}

var singleResultDummy CpuTempStatus // Вспомогательная переменная для отката парсинга
