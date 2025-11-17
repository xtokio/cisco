package cisco

import (
	"fmt"
	"log"
	"regexp"
	"strings"
)

// VlanInfo defines the structure for a single VLAN entry.
type VlanInfo struct {
	VLANID   string
	VLANName string
	Status   string
	Ports    []string
}

func Show_vlan(switch_hostname string) ([]VlanInfo, error) {
	outputString, err := RunCommand(switch_hostname, "show vlan")
	if err != nil {
		return nil, err
	}

	// --- PARSE OUTPUT ---
	vlan_data, err := parseVlanInfo(outputString)
	if err != nil {
		log.Printf("%s :: Show Vlans :: Error during parsing: %v", switch_hostname, err)
		return nil, err
	}

	// Check the length of the slice, not the map.
	if len(vlan_data) == 0 {
		log.Printf("Show VLAN :: Warning: Parsing completed for %s, but no interfaces were found.", switch_hostname)
		return nil, nil
	}

	return vlan_data, nil
}

// parseVlanInfo processes the raw CLI output from "show vlan" and converts it into a list of VlanInfo structs.
// This corrected version knows when to stop parsing and properly handles empty port lists.
func parseVlanInfo(rawOutput string) ([]VlanInfo, error) {
	var vlans []VlanInfo
	lines := strings.Split(rawOutput, "\n")

	// Regex to identify a line that starts a new VLAN entry (begins with a number).
	isNewVlanLine := regexp.MustCompile(`^\d`)

	dataStartIndex := -1
	// Find the start of the data, which is 2 lines after the header "VLAN Name..."
	for i, line := range lines {
		if strings.HasPrefix(line, "VLAN Name") {
			dataStartIndex = i + 2 // Skip header and separator line "----..."
			break
		}
	}

	if dataStartIndex == -1 {
		return nil, fmt.Errorf("could not find VLAN header in output")
	}

	for i := dataStartIndex; i < len(lines); i++ {
		line := strings.TrimRight(lines[i], "\r")

		// *** FIX #1: Stop parsing before the second, unrelated table begins. ***
		if strings.HasPrefix(line, "VLAN Type") {
			break
		}

		if line == "" {
			continue
		}

		if isNewVlanLine.MatchString(line) {
			// This is a new VLAN entry
			fields := strings.Fields(line)
			if len(fields) < 3 {
				continue // Malformed line
			}

			// *** FIX #2: Initialize Ports as an empty slice to avoid 'null' in JSON. ***
			vlan := VlanInfo{
				VLANID:   fields[0],
				VLANName: fields[1],
				Status:   fields[2],
				Ports:    make([]string, 0),
			}

			// If there are ports listed on this line, parse them
			if len(fields) > 3 {
				portStr := strings.Join(fields[3:], "")
				ports := strings.Split(portStr, ",")
				vlan.Ports = append(vlan.Ports, ports...)
			}
			vlans = append(vlans, vlan)
		} else if len(vlans) > 0 {
			// This is a continuation of the previous VLAN's port list
			lastVlan := &vlans[len(vlans)-1]
			portStr := strings.TrimSpace(line)
			ports := strings.Split(portStr, ",")
			lastVlan.Ports = append(lastVlan.Ports, ports...)
		}
	}

	// Clean up empty strings from port lists
	for i := range vlans {
		var cleanPorts []string
		for _, port := range vlans[i].Ports {
			if trimmedPort := strings.TrimSpace(port); trimmedPort != "" {
				cleanPorts = append(cleanPorts, trimmedPort)
			}
		}
		vlans[i].Ports = cleanPorts
	}

	return vlans, nil
}
