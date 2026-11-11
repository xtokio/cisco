package parse

import (
	"fmt"
	"log"
	"strings"
)

// InterfaceStatus defines the structure for a single network interface entry.
type InterfaceStatus struct {
	Interface   string
	Description string
	Status      string
	VlanID      string
	Duplex      string
	Speed       string
	Type        string
}

func Show_interfaces_status(outputString string) ([]InterfaceStatus, error) {
	interfaceStatusList, err := parseInterfaceStatus(outputString)
	if err != nil {
		return nil, fmt.Errorf("Error: Show Interface Status ::Error during parsing: %v", err)
	}

	// Check the length of the slice, not the map.
	if len(interfaceStatusList) == 0 {
		log.Printf("Show Interface Status :: Warning: Parsing completed, but no interfaces were found.")
		return nil, nil
	}

	return interfaceStatusList, nil
}

// parseInterfaceStatus processes the raw CLI output and converts it into a list of InterfaceStatus structs.
// It includes safety checks to remove backslashes that break ClickHouse SQL queries.
func parseInterfaceStatus(rawOutput string) ([]InterfaceStatus, error) {
	var interfaces []InterfaceStatus
	lines := strings.Split(rawOutput, "\n")

	// dataStartIndex := -1
	// for i := 0; i < len(lines); i++ {
	// 	if strings.HasPrefix(strings.TrimSpace(lines[i]), "Port") && strings.Contains(lines[i], "Status") {
	// 		dataStartIndex = i + 1
	// 		break
	// 	}
	// }

	dataStartIndex := -1
	for i, line := range lines {
		// Using 'line' directly instead of 'lines[i]'
		if strings.HasPrefix(strings.TrimSpace(line), "Port") && strings.Contains(line, "Status") {
			dataStartIndex = i + 1
			break
		}
	}

	if dataStartIndex == -1 || dataStartIndex >= len(lines) {
		return nil, fmt.Errorf("could not find the header line in 'show interface status' output")
	}

	for _, line := range lines[dataStartIndex:] {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 6 {
			continue
		}

		status := InterfaceStatus{}
		status.Interface = strings.TrimSpace(fields[0])

		// Find the Status field.
		statusIndex := -1
		maxSearchIndex := len(fields) - 5
		for j := 1; j <= maxSearchIndex; j++ {
			s := fields[j]
			// Added 'notconnect' and others to ensure matching
			if s == "connected" || s == "notconnect" || s == "disabled" || s == "err-disabled" || s == "suspended" || s == "monitoring" || s == "notconnected" {
				statusIndex = j
				break
			}
		}

		if statusIndex == -1 {
			continue
		}

		// --- Description Validation ---
		rawDesc := strings.TrimSpace(strings.Join(fields[1:statusIndex], " "))
		if rawDesc == "\\" {
			status.Description = ""
		} else {
			// Replace backslash with forward slash to prevent SQL escape errors
			status.Description = strings.ReplaceAll(rawDesc, "\\", "/")
		}

		// --- Standard Fields ---
		status.Status = strings.TrimSpace(fields[statusIndex])
		status.VlanID = strings.TrimSpace(fields[statusIndex+1])
		status.Duplex = strings.TrimSpace(fields[statusIndex+2])
		status.Speed = strings.TrimSpace(fields[statusIndex+3])

		// --- Type Validation (Type can also contain spaces/special characters) ---
		rawType := strings.TrimSpace(strings.Join(fields[statusIndex+4:], " "))
		status.Type = strings.ReplaceAll(rawType, "\\", "/")

		interfaces = append(interfaces, status)
	}

	return interfaces, nil
}
