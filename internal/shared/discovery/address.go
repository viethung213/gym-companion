package discovery

import (
	"fmt"
	"net"
	"os"
)

// ResolveSelfAddress tự động tìm địa chỉ IP thực tế của máy/container và trả về URL http://<IP>:<PORT>.
func ResolveSelfAddress(port string) string {
	if port == "" {
		port = os.Getenv("APP_PORT")
		if port == "" {
			port = "8080"
		}
	}

	ip := GetLocalIP()
	return fmt.Sprintf("http://%s:%s", ip, port)
}

// GetLocalIP tự động dò tìm địa chỉ IP mạng nội bộ của chính service.
// 1. Dùng kết nối UDP tạm thời (không truyền tải dữ liệu) để kernel OS tự động chọn
//    IP của card mạng chính đang hoạt động (Wi-Fi, Ethernet, Docker eth0).
// 2. Nếu không có kết nối internet, quét các network interface để lấy IPv4 hợp lệ
//    (bỏ qua 127.0.0.1 và dải link-local 169.254.x.x).
// 3. Fallback cuối cùng là 127.0.0.1 nếu máy ngắt toàn bộ mạng.
func GetLocalIP() string {
	// Cách 1: Socket UDP routing table lookup
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err == nil {
		defer conn.Close()
		if localAddr, ok := conn.LocalAddr().(*net.UDPAddr); ok && localAddr.IP != nil {
			ipStr := localAddr.IP.String()
			if ipStr != "" && ipStr != "127.0.0.1" {
				return ipStr
			}
		}
	}

	// Cách 2: Quét tất cả card mạng (Network Interfaces)
	addrs, err := net.InterfaceAddrs()
	if err == nil {
		for _, addr := range addrs {
			if ipNet, ok := addr.(*net.IPNet); ok {
				ip := ipNet.IP.To4()
				if ip != nil && !ip.IsLoopback() && !ip.IsLinkLocalUnicast() {
					return ip.String()
				}
			}
		}
	}

	// Cách 3: Fallback cục bộ
	return "127.0.0.1"
}
