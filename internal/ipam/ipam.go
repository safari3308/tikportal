package ipam

import (
	"errors"
	"fmt"
	"net/netip"
	"strings"
)

// ErrSubnetExhausted is returned when all usable host IPs in the subnet are allocated.
var ErrSubnetExhausted = errors.New("no available IP addresses remaining in subnet")

// SubnetInfo holds validated IPv4 subnet metadata.
type SubnetInfo struct {
	Prefix    netip.Prefix
	GatewayIP netip.Addr
	PrefixLen int
	Capacity  int
}

// ValidateSubnet validates an IPv4 CIDR string (e.g. "10.10.0.0/24") and gateway IP (e.g. "10.10.0.1").
func ValidateSubnet(subnetCIDR, gatewayIPStr string) (*SubnetInfo, error) {
	prefix, err := netip.ParsePrefix(strings.TrimSpace(subnetCIDR))
	if err != nil {
		return nil, fmt.Errorf("invalid subnet CIDR %q: %w", subnetCIDR, err)
	}
	if !prefix.Addr().Is4() {
		return nil, errors.New("only IPv4 subnets are supported")
	}
	masked := prefix.Masked()
	bits := masked.Bits()
	if bits > 30 {
		return nil, errors.New("subnet prefix length must be /30 or larger (e.g. /24)")
	}

	var gw netip.Addr
	if strings.TrimSpace(gatewayIPStr) != "" {
		gw, err = netip.ParseAddr(strings.TrimSpace(gatewayIPStr))
		if err != nil || !gw.Is4() {
			return nil, fmt.Errorf("invalid gateway IP %q", gatewayIPStr)
		}
		if !masked.Contains(gw) {
			return nil, fmt.Errorf("gateway IP %s is outside subnet %s", gw, masked)
		}
	} else {
		// Default gateway IP is the first usable host (.1)
		gw = masked.Addr().Next()
	}

	totalAddresses := 1 << (32 - bits)
	// Subtract network address, broadcast address, and gateway IP
	capacity := totalAddresses - 3
	if capacity < 1 {
		capacity = 1
	}

	return &SubnetInfo{
		Prefix:    masked,
		GatewayIP: gw,
		PrefixLen: bits,
		Capacity:  capacity,
	}, nil
}

// AllocateNextIP finds the lowest available host IP in subnetCIDR that is not
// the network address, gatewayIP, broadcast address, or in usedIPs.
// When a peer is deleted and removed from usedIPs, its IP is automatically reused.
func AllocateNextIP(subnetCIDR, gatewayIPStr string, usedIPs []string) (string, error) {
	info, err := ValidateSubnet(subnetCIDR, gatewayIPStr)
	if err != nil {
		return "", err
	}

	usedSet := make(map[netip.Addr]struct{}, len(usedIPs))
	for _, raw := range usedIPs {
		trimmed := strings.TrimSpace(raw)
		// Strip optional /32 suffix if present
		if idx := strings.IndexByte(trimmed, '/'); idx != -1 {
			trimmed = trimmed[:idx]
		}
		if addr, err := netip.ParseAddr(trimmed); err == nil {
			usedSet[addr] = struct{}{}
		}
	}

	networkAddr := info.Prefix.Addr()
	broadcastAddr := ipv4Broadcast(info.Prefix)

	// Iterate from first host after network address up to broadcastAddr
	for ip := networkAddr.Next(); info.Prefix.Contains(ip) && ip != broadcastAddr; ip = ip.Next() {
		if ip == info.GatewayIP {
			continue
		}
		if _, taken := usedSet[ip]; !taken {
			return ip.String(), nil
		}
	}

	return "", ErrSubnetExhausted
}

func ipv4Broadcast(p netip.Prefix) netip.Addr {
	addr4 := p.Addr().As4()
	ipUint := uint32(addr4[0])<<24 | uint32(addr4[1])<<16 | uint32(addr4[2])<<8 | uint32(addr4[3])
	hostBits := 32 - p.Bits()
	mask := uint32((1 << hostBits) - 1)
	bcast := ipUint | mask
	return netip.AddrFrom4([4]byte{
		byte(bcast >> 24),
		byte(bcast >> 16),
		byte(bcast >> 8),
		byte(bcast),
	})
}
