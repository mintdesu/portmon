package iptables

import (
	"bufio"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var (
	bytesPattern   = regexp.MustCompile(`^\s*\d+\s+(\d+)\s+`)
	commentPattern = regexp.MustCompile(`/\*\s*portmon:([0-9]+(?:-[0-9]+)?)\s*\*/`)
	rangePattern   = regexp.MustCompile(`\b[ds]ports\s+([0-9:,-]+)`)
	singlePattern  = regexp.MustCompile(`\b[ds]pt:(\d+)`)
)

func ParseCounters(output string) (map[string]uint64, error) {
	counters := make(map[string]uint64)
	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()
		bytesMatch := bytesPattern.FindStringSubmatch(line)
		if bytesMatch == nil {
			continue
		}

		label := counterLabel(line)
		if label == "" {
			continue
		}

		bytes, err := strconv.ParseUint(bytesMatch[1], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("parse bytes from %q: %w", line, err)
		}
		counters[label] += bytes
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return counters, nil
}

func counterLabel(line string) string {
	if match := commentPattern.FindStringSubmatch(line); match != nil {
		return match[1]
	}
	if match := rangePattern.FindStringSubmatch(line); match != nil {
		return strings.ReplaceAll(match[1], ":", "-")
	}
	if match := singlePattern.FindStringSubmatch(line); match != nil {
		return match[1]
	}
	return ""
}
