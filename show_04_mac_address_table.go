package cisco

import (
	"fmt"
	"log"
	"regexp"
	"strings"
)

// MacAddressEntry defines the structure for a single entry in the MAC address table.
type MacAddressEntry struct {
	Interface  string
	MacAddress string
	VlanID     string
	Type       string // e.g., DYNAMIC, STATIC, SECURE
}

// Show_mac_address_table constructs the command, runs it, and processes the output.
func Show_mac_address_table(switch_hostname string) ([]MacAddressEntry, error) {
	outputString, err := RunCommand(switch_hostname, "show mac address-table")
	if err != nil {
		return nil, err
	}

	// 2. Parse the output
	mac_table_data, err := parseMacAddressTable(outputString)
	if err != nil {
		log.Printf("Error during parsing 'show mac address-table' output for %s: %v", switch_hostname, err)
		return nil, fmt.Errorf("error during parsing 'show mac address-table' output for %s: %v", switch_hostname, err)
	}

	if len(mac_table_data) == 0 {
		log.Printf("Show MAC Address Table :: Warning: Parsing completed for %s, but no MAC entries were found.", switch_hostname)
		return nil, nil
	}

	return mac_table_data, nil
}

// parseMacAddressTable takes the raw output and extracts MacAddressEntry structs.
func parseMacAddressTable(rawOutput string) ([]MacAddressEntry, error) {
	var macEntries []MacAddressEntry
	reEntry := regexp.MustCompile(`^\s*\*?\s*(\d+)\s+([\w\.]+)\s+([\w]+)(?:\s+[\w\-])*\s+(\S+)`)

	lines := strings.Split(rawOutput, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Skip header, separator lines, and summary lines
		if len(line) == 0 ||
			strings.Contains(line, "Mac Address Table") ||
			strings.Contains(line, "Vlan") ||
			strings.Contains(line, "----") ||
			strings.Contains(line, "Total Mac Addresses") ||
			strings.Contains(line, "CPU") { // Often the 'CPU' entries are less relevant for port checks
			continue
		}

		if matches := reEntry.FindStringSubmatch(line); len(matches) == 5 {
			entry := MacAddressEntry{
				// Clean up the VLAN ID in case the '*' was captured with it
				VlanID:     strings.TrimSpace(matches[1]),
				MacAddress: matches[2],
				Type:       matches[3],
				Interface:  matches[4],
			}
			macEntries = append(macEntries, entry)
		}
	}

	return macEntries, nil
}
