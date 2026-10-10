package manifest

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

// ParseCPU parses a Kubernetes-style CPU quantity into cores.
// Accepted forms: "2", "1.5", "500m".
func ParseCPU(s string) (float64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("cpu quantity is empty")
	}
	if strings.HasSuffix(s, "m") {
		n, err := strconv.ParseFloat(strings.TrimSuffix(s, "m"), 64)
		if err != nil || n <= 0 {
			return 0, fmt.Errorf("invalid cpu quantity %q", s)
		}
		return n / 1000, nil
	}
	n, err := strconv.ParseFloat(s, 64)
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("invalid cpu quantity %q", s)
	}
	return n, nil
}

// ParseMemory parses a Kubernetes-style memory quantity into bytes.
func ParseMemory(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("memory quantity is empty")
	}
	i := 0
	for i < len(s) && (unicode.IsDigit(rune(s[i])) || s[i] == '.') {
		i++
	}
	if i == 0 {
		return 0, fmt.Errorf("invalid memory quantity %q", s)
	}
	num, err := strconv.ParseFloat(s[:i], 64)
	if err != nil || num <= 0 {
		return 0, fmt.Errorf("invalid memory quantity %q", s)
	}
	unit := s[i:]
	var mult float64
	switch unit {
	case "", "B":
		mult = 1
	case "Ki":
		mult = 1 << 10
	case "Mi":
		mult = 1 << 20
	case "Gi":
		mult = 1 << 30
	case "K":
		mult = 1000
	case "M":
		mult = 1000 * 1000
	case "G":
		mult = 1000 * 1000 * 1000
	default:
		return 0, fmt.Errorf("invalid memory unit %q in %q (use Ki, Mi, Gi)", unit, s)
	}
	return int64(num * mult), nil
}

// FormatMemory renders bytes as a Mi quantity Docker accepts.
func FormatMemory(bytes int64) string {
	mib := bytes / (1024 * 1024)
	if mib < 1 {
		mib = 1
	}
	return fmt.Sprintf("%dm", mib)
}
