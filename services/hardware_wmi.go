package services

import (
	"log"
	"time"
	"github.com/yusufpapurcu/wmi"
)
// Win32_DiskDrive — стандартный WMI класс, доступный во всех версиях Windows
type Win32_DiskDrive struct {
	DeviceID    string
	Caption     string // Понятное название модели (например, "Samsung SSD 970 EVO")
	Status      string // Статус здоровья: "OK", "Degraded", "Pred Fail"
	Size        uint64 // Общий объем диска в байтах
}

// DiskStatus представляет структуру диска для отправки на бэкенд HostPulse
type DiskStatus struct {
	DeviceID  string `json:"device_id"`
	IsHealthy bool   `json:"is_healthy"` // true = всё ок, false = SMART бьет тревогу
}

// GetSmartStatus собирает состояние SMART и параметры по всем дискам
func GetSmartStatus() []DiskStatus {
	start := time.Now()
	log.Println("[💾 DISK SERVICE] Запуск опроса физических дисков через WMI...")

	var dst []Win32_DiskDrive
	// Запрашиваем DeviceID, модель, статус и размер из дефолтного root\CIMV2
	query := "SELECT DeviceID, Caption, Status, Size FROM Win32_DiskDrive"

	err := wmi.Query(query, &dst)
	if err != nil {
		log.Printf("[❌ ERROR] Ошибка выполнения WMI-запроса к Win32_DiskDrive: %v", err)
		return nil
	}

	totalDisks := len(dst)
	if totalDisks == 0 {
		log.Println("[⚠️ WARNING] WMI запрос вернул 0 физических дисков в системе.")
		return []DiskStatus{}
	}

	log.Printf("[🔍 DISK SERVICE] Найдено физических дисков в системе: %d", totalDisks)

	var results []DiskStatus
	for i, disk := range dst {
		// Проверяем статус здоровья диска
		isHealthy := disk.Status == "OK"
		sizeGb := disk.Size / (1024 * 1024 * 1024) // Переводим байты в гигабайты для лога

		log.Printf(
			"  └─ [#%d] Диск: %s | Модель: %s | Размер: %d GB | Статус WMI: %s | Здоров: %t",
			i+1, disk.DeviceID, disk.Caption, sizeGb, disk.Status, isHealthy,
		)

		if !isHealthy {
			log.Printf("[🚨 SMART CRITICAL] Внимание! Диск %s (%s) сообщает о проблемах со здоровьем!", disk.DeviceID, disk.Caption)
		}

		results = append(results, DiskStatus{
			DeviceID:  disk.DeviceID,
			IsHealthy: isHealthy,
		})
	}

	log.Printf("[✅ SUCCESS] Опрос дисков завершен за %v. Подготовлено к отправке: %d записей.", time.Since(start), len(results))
	return results
}