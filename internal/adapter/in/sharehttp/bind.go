package sharehttp

import (
	"errors"
	"fmt"
	"net"
)

const (
	basePort     = 47800
	maxPortProbe = 10 // try basePort..basePort+9
)

var errNoPrivateInterface = errors.New("sharehttp: no private network interface available")

// privateIPv4Addrs returns the up, non-loopback IPv4 addresses on private
// (RFC1918) interfaces -- the only addresses this server ever binds, so it
// can never be reached from outside the local network.
func privateIPv4Addrs() ([]string, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, fmt.Errorf("sharehttp: list interfaces: %w", err)
	}

	var addrs []string
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		ifaceAddrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, a := range ifaceAddrs {
			ipNet, ok := a.(*net.IPNet)
			if !ok {
				continue
			}
			ip4 := ipNet.IP.To4()
			if ip4 == nil || !ip4.IsPrivate() {
				continue
			}
			addrs = append(addrs, ip4.String())
		}
	}
	if len(addrs) == 0 {
		return nil, errNoPrivateInterface
	}
	return addrs, nil
}

// bindAll tries binding one plain TCP listener per address at the same
// port, starting at basePort and probing upward on conflict -- mDNS
// advertises a single port, so every listener must agree. The whole set is
// closed and retried together on any single bind failure.
func bindAll(addrs []string) ([]net.Listener, int, error) {
	for port := basePort; port < basePort+maxPortProbe; port++ {
		listeners, err := bindPort(addrs, port)
		if err == nil {
			return listeners, port, nil
		}
		if !isAddrInUse(err) {
			return nil, 0, err
		}
	}
	return nil, 0, fmt.Errorf("sharehttp: no free port in range %d-%d", basePort, basePort+maxPortProbe-1)
}

func bindPort(addrs []string, port int) ([]net.Listener, error) {
	listeners := make([]net.Listener, 0, len(addrs))
	for _, addr := range addrs {
		l, err := net.Listen("tcp", fmt.Sprintf("%s:%d", addr, port))
		if err != nil {
			closeAll(listeners)
			return nil, err
		}
		listeners = append(listeners, l)
	}
	return listeners, nil
}

func closeAll(listeners []net.Listener) {
	for _, l := range listeners {
		l.Close()
	}
}

func isAddrInUse(err error) bool {
	var opErr *net.OpError
	return errors.As(err, &opErr) && opErr.Op == "listen"
}
