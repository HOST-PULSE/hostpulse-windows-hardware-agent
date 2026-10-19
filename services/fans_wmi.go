package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
)

type FanStatus struct {
	FanName string  `json:"fan_name"`
	RPM     float64 `json:"rpm"`
}

// GetFansStatus опрашивает датчики оборотов кулеров через извлеченную ранее DLL
func GetFansStatus() []FanStatus {

	tmpDir := os.TempDir()
	libPath := filepath.Join(tmpDir, "hostpulse_ohm_core.dll")

	if _, err := os.Stat(libPath); os.IsNotExist(err) {
		fmt.Println(" [❌ FANS ERROR] Драйвер DLL отсутствует во временной папке.")
		return []FanStatus{}
	}

	// Скрипт PowerShell: активируем опрос материнской платы и ищем тип 'Fan'
	psScript := fmt.Sprintf(`
		[System.Reflection.Assembly]::LoadFile('%s') | Out-Null
		$computer = New-Object OpenHardwareMonitor.Hardware.Computer
		$computer.MainboardEnabled = $true
		$computer.FanControllerEnabled = $true
		$computer.Open()
		
		$fans = @()
		foreach ($hardware in $computer.Hardware) {
			$hardware.Update()
			foreach ($sub in $hardware.SubHardware) { $sub.Update() } # Проверяем чипы Super I/O
			
			# Ищем датчики на основном железе и суб-железе
			$allHardware = @($hardware) + $hardware.SubHardware
			foreach ($hw in $allHardware) {
				foreach ($sensor in $hw.Sensors) {
					if ($sensor.SensorType -eq 'Fan') {
						$fans += [pscustomobject]@{
							zone_name = $sensor.Name
							celsius = [math]::Round($sensor.Value, 0) # Маппим RPM в celsius для универсальности Go-парсера
						}
					}
				}
			}
		}
		$computer.Close()
		
		if ($fans.Count -gt 0) {
			ConvertTo-Json -InputObject @($fans) -Compress
		}
	`, filepath.ToSlash(libPath))

	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", psScript)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil && stdout.Len() == 0 {
		fmt.Printf(" [ℹ️ FANS LOG] Датчики кулеров недоступны: %v\n", err)
		return []FanStatus{}
	}

	rawJson := stdout.Bytes()
	if len(rawJson) == 0 {
		return []FanStatus{}
	}

	fmt.Printf("   [DEBUG FANS RAW JSON]: %s\n", string(rawJson))
	
	// Вычищаем JSON-массив
	re := regexp.MustCompile(`\[\s*\{.*\}\s*\]`)
	cleanJson := re.FindString(string(rawJson))
	if cleanJson == "" {
		cleanJson = string(rawJson)
	}

	// Чтобы не дублировать код парсинга, десериализуем в структуру CpuTempStatus,
	// а затем красиво перегоняем в итоговый FanStatus слайс
	var rawData []CpuTempStatus
	if err := json.Unmarshal([]byte(cleanJson), &rawData); err != nil {
		var singleResult CpuTempStatus
		if json.Unmarshal([]byte(cleanJson), &singleResult) == nil {
			rawData = append(rawData, singleResult)
		}
	}

	// Формируем чистые структуры кулеров для отправки
	var finalFans []FanStatus
	for _, item := range rawData {
		finalFans = append(finalFans, FanStatus{
			FanName: item.ZoneName,
			RPM:     item.Celsius,
		})
	}

	if len(finalFans) > 0 {
		fmt.Printf(" [✅ FANS LOG] Обороты кулеров успешно считаны. Найдено вентиляторов: %d\n", len(finalFans))
	}
	return finalFans
}
