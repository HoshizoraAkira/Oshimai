package chaos

import (
	"fmt"
	"net"
	"strings"
)

// ParsedFilter holds validated network classifier parameters.
type ParsedFilter struct {
	Interface string
	IPNet     *net.IPNet
	Port      int
	Protocol  string
}

// ParseFilter validates and converts FilterConfig into structured network classifier parameters.
func ParseFilter(cfg FilterConfig) (*ParsedFilter, error) {
	if strings.TrimSpace(cfg.Interface) == "" {
		return nil, fmt.Errorf("interface name cannot be empty")
	}

	pf := &ParsedFilter{
		Interface: cfg.Interface,
		Port:      cfg.TargetPort,
		Protocol:  strings.ToLower(strings.TrimSpace(cfg.Protocol)),
	}

	if cfg.TargetIP != "" {
		target := strings.TrimSpace(cfg.TargetIP)
		if !strings.Contains(target, "/") {
			// Single IP: append /32 for IPv4 or /128 for IPv6
			ip := net.ParseIP(target)
			if ip == nil {
				return nil, fmt.Errorf("invalid IP address: %s", target)
			}
			if ip.To4() != nil {
				target += "/32"
			} else {
				target += "/128"
			}
		}

		_, ipNet, err := net.ParseCIDR(target)
		if err != nil {
			return nil, fmt.Errorf("invalid CIDR %s: %w", target, err)
		}
		pf.IPNet = ipNet
	}

	if cfg.TargetPort < 0 || cfg.TargetPort > 65535 {
		return nil, fmt.Errorf("invalid target port %d (must be 0-65535)", cfg.TargetPort)
	}

	return pf, nil
}

// Matches checks if a simulated packet with dstIP, dstPort, and protocol matches the filter.
func (pf *ParsedFilter) Matches(dstIP net.IP, dstPort int, protocol string) bool {
	if pf.IPNet != nil {
		if dstIP == nil || !pf.IPNet.Contains(dstIP) {
			return false
		}
	}

	if pf.Port > 0 {
		if dstPort != pf.Port {
			return false
		}
	}

	if pf.Protocol != "" {
		if !strings.EqualFold(pf.Protocol, protocol) {
			return false
		}
	}

	return true
}

// BuildU32FilterArgs generates u32 filter classifier command arguments for Linux TC.
// Example: match ip dst 192.168.1.50/32 match ip dport 8080 0xffff flowid 1:2
func BuildU32FilterArgs(cfg FilterConfig, parentHandle string, flowID string) ([]string, error) {
	pf, err := ParseFilter(cfg)
	if err != nil {
		return nil, err
	}

	args := []string{
		"filter", "add", "dev", pf.Interface,
		"protocol", "ip",
		"parent", parentHandle,
		"prio", "1",
		"u32",
	}

	if pf.IPNet != nil {
		args = append(args, "match", "ip", "dst", pf.IPNet.String())
	}

	if pf.Port > 0 {
		args = append(args, "match", "ip", "dport", fmt.Sprintf("%d", pf.Port), "0xffff")
	}

	args = append(args, "flowid", flowID)
	return args, nil
}
