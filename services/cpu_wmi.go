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

// Глобальная переменная для хранения байт драйвера процессора
var ohmLibraryBinary []byte

// SetOhmLibraryBinary принимает массив байт зашитой DLL из main.go
func SetOhmLibraryBinary(b []byte) {
	ohmLibraryBinary = b
}

type CpuTempStatus struct {
	ZoneName string  `json:"zone_name"`
	Celsius  float64 `json:"celsius"`
}

// extractOhmLib принудительно обновляет и извлекает DLL во временный каталог системы
func extractOhmLib() (string, error) {
	tmpDir := os.TempDir()
	libPath := filepath.Join(tmpDir, "hostpulse_ohm_core.dll")

	if len(ohmLibraryBinary) == 0 {
		return "", fmt.Errorf("массив ohmLibraryBinary пуст! Проверьте //go:embed в main.go")
	}

	// Если файл уже существует от старого запуска, принудительно перезаписываем его на случай обновлений
	if _, err := os.Stat(libPath); err == nil {
		_ = os.Remove(libPath)
	}

	err := os.WriteFile(libPath, ohmLibraryBinary, 0755)
	if err != nil {
		return "", err
	}
	return libPath, nil
}

// GetCpuTemperatures опрашивает шину датчиков через OpenHardwareMonitorLib.dll
func GetCpuTemperatures() []CpuTempStatus {
	var results []CpuTempStatus

	libPath, err := extractOhmLib()
	if err != nil {
		fmt.Printf(" [❌ CPU ERROR] Не удалось распаковать DLL-драйвер: %v\n", err)
		return getFallbackEstimate()
	}

	// Скрипт PowerShell: загружает сборку .NET, запускает драйвер WinRing0 и преобразует массив в компактный JSON
	psScript := fmt.Sprintf(`
		[System.Reflection.Assembly]::LoadFile('%s') | Out-Null
		$computer = New-Object OpenHardwareMonitor.Hardware.Computer
		$computer.CPUEnabled = $true
		$computer.Open()
		
		$temps = @()
		foreach ($hardware in $computer.Hardware) {
			$hardware.Update()
			foreach ($sensor in $hardware.Sensors) {
				if ($sensor.SensorType -eq 'Temperature') {
					$temps += [pscustomobject]@{
						zone_name = $sensor.Name
						celsius = [math]::Round($sensor.Value, 1)
					}
				}
			}
		}
		$computer.Close()
		
		if ($temps.Count -gt 0) {
			ConvertTo-Json -InputObject @($temps) -Compress
		}
	`, filepath.ToSlash(libPath))

	// Запуск фонового бесшумного процесса (без создания окон GUI и скрытых зависаний в NSSM)
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", psScript)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()
	if err != nil && stdout.Len() == 0 {
		fmt.Printf(" [ℹ️ CPU LOG] Драйвер шины недоступен или заблокирован OS: %v. Stderr: %s\n", err, stderr.String())
		return getFallbackEstimate()
	}

	rawJson := stdout.Bytes()
	if len(rawJson) == 0 {
		return getFallbackEstimate()
	}

	// Очищаем лог от возможных предупреждений PowerShell (оставляем строго структуру JSON)
	re := regexp.MustCompile(`\[\s*\{.*\}\s*\]`)
	cleanJson := re.FindString(string(rawJson))
	if cleanJson == "" {
		cleanJson = string(rawJson)
	}

	if err := json.Unmarshal([]byte(cleanJson), &results); err != nil {
		var singleResult CpuTempStatus
		if json.Unmarshal([]byte(cleanJson), &singleResult) == nil {
			results = append(results, singleResult)
		} else {
			fmt.Printf(" [❌ CPU ERROR] Ошибка десериализации JSON датчиков: %v\n", err)
			return getFallbackEstimate()
		}
	}

	if len(results) > 0 {
		fmt.Printf(" [✅ CPU LOG] Датчики успешно считаны через OpenHardwareMonitorLib. Зон: %d\n", len(results))
		return results
	}

	return getFallbackEstimate()
}

// Безопасный откат, если драйвер ядра заблокирован политиками безопасности или ОС виртуализирована
func getFallbackEstimate() []CpuTempStatus {
	return []CpuTempStatus{
		{
			ZoneName: "OS_Hardware_Counters_Unavailable",
			Celsius:  45.0,
		},
	}
}
