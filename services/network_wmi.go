package services

import (
	"fmt"
	"github.com/yusufpapurcu/wmi"
)

type NetworkInterfaceStatus struct {
	InterfaceName string `json:"interface_name"` // Название адаптера (например: Intel(R) I211 Gigabit Network Connection)
	MACAddress    string `json:"mac_address"`    // MAC-адрес для точной идентификации карты
	SpeedMbps     uint64 `json:"speed_mbps"`     // Текущая скорость линка в Mbps (например: 1000, 100)
	IsConnected   bool   `json:"is_connected"`   // Подключен ли кабель физически
}

type Win32_NetworkAdapter struct {
	Name                 string
	MACAddress           *string // Используем указатель, так как у виртуальных или скрытых адаптеров может быть null
	Speed                *uint64 // Указатель защитит от падения, если линк лежит
	NetConnectionStatus  uint16  // 2 = Подключено, остальные коды = отключено/сбой
	PhysicalAdapter      bool    // Фильтруем только реальное физическое железо, отсекая виртуальные VPN-соеднения
}

// GetNetworkHardwareStatus собирает статус по всем активным физическим сетевым интерфейсам
func GetNetworkHardwareStatus() []NetworkInterfaceStatus {
	var results []NetworkInterfaceStatus

	var wmiNet []Win32_NetworkAdapter
	// Фильтруем по PhysicalAdapter = True и наличию MAC-адреса, чтобы убрать виртуальный мусор ОС
	query := "SELECT Name, MACAddress, Speed, NetConnectionStatus, PhysicalAdapter FROM Win32_NetworkAdapter WHERE PhysicalAdapter = True AND MACAddress IS NOT NULL"
	err := wmi.Query(query, &wmiNet)

	if err != nil || len(wmiNet) == 0 {
		fmt.Printf(" [❌ NET ERROR] Не удалось получить данные Win32_NetworkAdapter: %v\n", err)
		return nil
	}

	for _, adapter := range wmiNet {
		var speedMbps uint64
		if adapter.Speed != nil {
			// WMI отдает скорость в битах в секунду (bps), переводим в Мегабиты (деление на 1 000 000)
			speedMbps = *adapter.Speed / 1000000
		}

		mac := "Unknown"
		if adapter.MACAddress != nil {
			mac = *adapter.MACAddress
		}

		// По спецификации Microsoft: NetConnectionStatus == 2 означает Connected
		isConnected := adapter.NetConnectionStatus == 2

		results = append(results, NetworkInterfaceStatus{
			InterfaceName: adapter.Name,
			MACAddress:    mac,
			SpeedMbps:     speedMbps,
			IsConnected:   isConnected,
		})
	}

	if len(results) > 0 {
		fmt.Printf(" [✅ NET LOG] Данные по сетевым интерфейсам собраны. Найдено адаптеров: %d\n", len(results))
	}
	return results
}
