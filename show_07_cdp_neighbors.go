package cisco

import (
	"fmt"
	"log"
	"strings"
)

// CdpNeighbor defines the structure for a single CDP neighbor entry.
type CdpNeighbor struct {
	Neighbor          string
	Interface         string
	HoldTime          string
	Capability        string
	Platform          string
	NeighborInterface string
}

func Show_cdp_neighbors(switch_hostname string) ([]CdpNeighbor, error) {
	outputString, err := RunCommand(switch_hostname, "show cdp neighbors")
	if err != nil {
		return nil, err
	}

	cdp_neighbors_data, err := parseCdpNeighbors(outputString)
	if err != nil {
		log.Printf("%s ::Show CDP Neighbors :: Error during parsing: %v", switch_hostname, err)
	}

	for i := range cdp_neighbors_data {
		cdp_neighbors_data[i].Interface = normalizeInterfaceName(cdp_neighbors_data[i].Interface)
		cdp_neighbors_data[i].NeighborInterface = normalizeInterfaceName(cdp_neighbors_data[i].NeighborInterface)
	}

	// Check the length of the slice, not the map.
	if len(cdp_neighbors_data) == 0 {
		log.Printf("Warning: Parsing completed for %s, but no cdp_neighbors were found.", switch_hostname)
		return nil, nil
	}

	return cdp_neighbors_data, nil
}

// parseCdpNeighbors processes the raw CLI output from "show cdp neighbors" and converts it into a list of CdpNeighbor structs.
// This parser is robust because it finds column positions from the header and handles entries that span multiple lines,
// parseCdpNeighbors processes the raw CLI output from "show cdp neighbors" and converts it into a list of CdpNeighbor structs.
func parseCdpNeighbors(rawOutput string) ([]CdpNeighbor, error) {
	var neighbors []CdpNeighbor
	lines := strings.Split(rawOutput, "\n")

	headerLine := ""
	headerIndex := -1

	// 1. Find the header line ("Device ID" / "Device-ID" and "Port ID")
	for i, line := range lines {
		// Use a generic search for "Device" and "Port ID"
		if strings.Contains(line, "Device") && strings.Contains(line, "Port ID") {
			headerLine = line
			headerIndex = i
			break
		}
	}

	if headerIndex == -1 {
		log.Println("CDP neighbors header not found, returning empty list.")
		return neighbors, nil
	}

	// Start parsing from the line IMMEDIATELY following the header
	dataStartIndex := headerIndex + 1

	// 2. Determine the start index of each column from the header.
	localIntfIndex := strings.Index(headerLine, "Local Intrfce")

	// Check for both Hldtme (Nexus) and Holdtme (IOS)
	holdtmeIndex := strings.Index(headerLine, "Hldtme")
	if holdtmeIndex == -1 {
		holdtmeIndex = strings.Index(headerLine, "Holdtme") // Fallback for full IOS spelling
	}

	capabilityIndex := strings.Index(headerLine, "Capability")
	platformIndex := strings.Index(headerLine, "Platform")
	portIDIndex := strings.Index(headerLine, "Port ID")

	if localIntfIndex == -1 || holdtmeIndex == -1 || capabilityIndex == -1 || platformIndex == -1 || portIDIndex == -1 {
		return nil, fmt.Errorf("could not parse CDP neighbors header columns correctly (check alignment)")
	}

	var lastDeviceID string

	for i := dataStartIndex; i < len(lines); i++ {
		line := lines[i]
		trimmedLine := strings.TrimSpace(line)

		// Explicitly skip the dash line and other non-data lines.
		if trimmedLine == "" || strings.Contains(trimmedLine, "Total cdp entries") || strings.Contains(trimmedLine, "Device-ID") || strings.Contains(trimmedLine, "---") {
			continue
		}

		// Check if the current line is long enough for the Platform column.
		isLongEnoughForDetail := len(line) > platformIndex

		var deviceIDFromColumn string
		if isLongEnoughForDetail {
			// Check if the Device ID column (up to the Local Intrfce index) is blank.
			deviceIDFromColumn = strings.TrimSpace(line[0:localIntfIndex])
		}

		// It's a detail line if the Device ID column is blank AND the line contains data from the interface columns onwards.
		isDetailLine := deviceIDFromColumn == "" && isLongEnoughForDetail && strings.TrimSpace(line[localIntfIndex:]) != ""

		if isDetailLine {
			// This is the detail line

			if lastDeviceID == "" {
				continue
			}

			// *** FIX: Apply a -1 offset to the boundary between Capability and Platform ***
			neighbor := CdpNeighbor{
				Neighbor:  lastDeviceID,
				Interface: strings.TrimSpace(line[localIntfIndex:holdtmeIndex]),
				// Shift end of slice left by 1
				Capability: strings.TrimSpace(line[capabilityIndex : platformIndex-1]),
				// Shift start of slice left by 1
				Platform:          strings.TrimSpace(line[platformIndex-1 : portIDIndex-1]),
				NeighborInterface: strings.TrimSpace(line[portIDIndex-1:]),
			}
			neighbors = append(neighbors, neighbor)
			lastDeviceID = ""

		} else if trimmedLine != "" {
			// This is a standalone Device ID line.
			lastDeviceID = trimmedLine
		}
	}

	return neighbors, nil
}
