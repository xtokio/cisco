package cisco

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

func Show_interfaces_status(switch_hostname string) ([]InterfaceStatus, error) {
	outputString, err := RunCommand(switch_hostname, "show interface status")
	if err != nil {
		return nil, err
	}

	// 3. Parse the output and convert to JSON
	interfaceStatusList, err := parseInterfaceStatus(outputString)
	if err != nil {
		log.Printf("%s :: Show Interface Status ::Error during parsing: %v", switch_hostname, err)
		return nil, err
	}

	// Check the length of the slice, not the map.
	if len(interfaceStatusList) == 0 {
		log.Printf("Show Interface Status :: Warning: Parsing completed for %s, but no interfaces were found.", switch_hostname)
		return nil, nil
	}

	return interfaceStatusList, nil
}

// parseInterfaceStatus processes the raw CLI output and converts it into a list of InterfaceStatus structs.
// It locates the 'Status' field first, which correctly handles variable-length
// Description and Type fields.
func parseInterfaceStatus(rawOutput string) ([]InterfaceStatus, error) {
	var interfaces []InterfaceStatus
	lines := strings.Split(rawOutput, "\n")

	dataStartIndex := -1
	for i := 1; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if strings.Contains(line, "Port") && strings.Contains(line, "Vlan") {
			dataStartIndex = i + 1
			break
		}
	}

	if dataStartIndex == -1 || dataStartIndex >= len(lines) {
		return nil, fmt.Errorf("could not find interface status header in output")
	}

	for i := dataStartIndex; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])

		if line == "" || strings.HasPrefix(line, "----") || strings.HasPrefix(line, "Name") {
			continue // Skip blank lines, separators, or secondary headers
		}

		fields := strings.Fields(line)

		// A line must have at least 6 fields:
		// Port, Status, Vlan, Duplex, Speed, Type (Type can be multi-word)
		if len(fields) < 6 {
			// log.Printf("Show interface status :: Skipping line with insufficient field count (%d) :: %s", len(fields), line)
			continue
		}

		status := InterfaceStatus{}
		status.Interface = fields[0]

		// Find the Status field. It's the first field after the Interface
		// that is a known status keyword. We must leave at least 4 fields
		// after it (Vlan, Duplex, Speed, Type).
		statusIndex := -1

		// We search from index 1 (after Port) up to len(fields) - 5
		// (to leave room for Status, Vlan, Duplex, Speed, and at least one word for Type)
		maxSearchIndex := len(fields) - 5
		for j := 1; j <= maxSearchIndex; j++ {
			s := fields[j]
			// Add all known status types here
			if s == "connected" || s == "notconnect" || s == "disabled" || s == "err-disabled" || s == "suspended" || s == "monitoring" {
				statusIndex = j
				break
			}
		}

		// If we didn't find a status, this line is malformed.
		if statusIndex == -1 {
			// log.Printf("Show interface status :: Skipping line: could not determine Status field :: %s", line)
			continue
		}

		// Now, assign all fields based on the correctly found statusIndex

		// Description is everything between Interface (fields[0]) and Status (fields[statusIndex])
		status.Description = strings.Join(fields[1:statusIndex], " ")

		status.Status = fields[statusIndex]
		status.VlanID = fields[statusIndex+1]
		status.Duplex = fields[statusIndex+2]
		status.Speed = fields[statusIndex+3]

		// Type is everything that remains
		status.Type = strings.Join(fields[statusIndex+4:], " ")

		interfaces = append(interfaces, status)
	}

	return interfaces, nil
}
