package manager

import (
	"net"

	"github.com/Cloud-Foundations/Dominator/lib/net/vsock"
)

func (m *Manager) checkVsockets() error {
	if err := vsock.CheckVsockets(); err != nil {
		m.Logger.Debugf(0, "CheckVsockets(): %v\n", err)
		return nil
	}
	m.vsocketsEnabled = true
	m.Logger.Println("VSOCK enabled")
	return nil
}

func (m *Manager) getVmCID(ipAddr net.IP) (uint32, error) {
	if !m.vsocketsEnabled {
		return 0, nil
	}
	if ip4 := ipAddr.To4(); ip4 == nil {
		return 0, nil
	} else {
		return uint32(ip4[0])<<24 |
				uint32(ip4[1])<<16 |
				uint32(ip4[2])<<8 |
				uint32(ip4[3]),
			nil
	}
}
