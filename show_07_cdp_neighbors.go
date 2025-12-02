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

	// 1. Find the header line
	for i, line := range lines {
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
	holdtmeIndex := strings.Index(headerLine, "Hldtme")
	if holdtmeIndex == -1 {
		holdtmeIndex = strings.Index(headerLine, "Holdtme")
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

		// Explicitly skip non-data lines.
		if trimmedLine == "" || strings.Contains(trimmedLine, "Total cdp entries") || strings.Contains(trimmedLine, "Device-ID") || strings.Contains(trimmedLine, "---") {
			continue
		}

		// LOGIC: Distinguish between 3 types of lines:
		// A. Detail Line: Starts with whitespace (Device ID column is empty).
		// B. Single-Line Entry: Starts with text AND is long enough to contain a Platform.
		// C. Device ID Only: Starts with text BUT is too short to be a full entry.

		// Check 1: Is it a Detail Line? (Indented)
		// We check if the line is long enough to have interface data, but the Device ID area is empty.
		isDetailLine := false
		if len(line) > localIntfIndex {
			deviceIDArea := strings.TrimSpace(line[0:localIntfIndex])
			if deviceIDArea == "" && strings.TrimSpace(line[localIntfIndex:]) != "" {
				isDetailLine = true
			}
		}

		if isDetailLine {
			// *** TYPE A: DETAIL LINE (Second line of a split entry) ***
			if lastDeviceID == "" {
				log.Printf("Warning: Found detail line without preceding Device ID: %s", line)
				continue
			}

			// Bounds check for safety
			if len(line) < portIDIndex {
				// Try to salvage what we can, or skip if critical data is missing
				if len(line) < platformIndex {
					log.Printf("Warning: Detail line too short to parse: %s", line)
					continue
				}
			}

			// For the detail line, we extract assuming the standard column headers apply
			neighbor := CdpNeighbor{
				Neighbor:          lastDeviceID,
				Interface:         strings.TrimSpace(line[localIntfIndex:holdtmeIndex]),
				HoldTime:          strings.TrimSpace(line[holdtmeIndex:capabilityIndex]),
				Capability:        strings.TrimSpace(line[capabilityIndex : platformIndex]),
				Platform:          strings.TrimSpace(line[platformIndex : portIDIndex]),
				NeighborInterface: strings.TrimSpace(line[portIDIndex:]),
			}
			neighbors = append(neighbors, neighbor)
			lastDeviceID = ""

		} else {
			// It starts with text. It is either Type B (Single Line) or Type C (Device ID Only).

			// *** CRITICAL FIX: Use platformIndex as the threshold ***
			// If a line is shorter than the start of the Platform column, it CANNOT be a full entry.
			// This correctly identifies "debb015-a.hub.nd.edu" as a "Device ID Only" line,
			// even if it spills into the Local Intrfce column area.

			if len(line) >= platformIndex {
				// *** TYPE B: SINGLE-LINE ENTRY ***

				// Extract the Device ID safely
				deviceID := ""
				if len(line) > localIntfIndex {
					deviceID = strings.TrimSpace(line[0:localIntfIndex])
				} else {
					deviceID = trimmedLine
				}

				neighbor := CdpNeighbor{
					Neighbor:          deviceID,
					Interface:         strings.TrimSpace(line[localIntfIndex:holdtmeIndex]),
					HoldTime:          strings.TrimSpace(line[holdtmeIndex:capabilityIndex]),
					Capability:        strings.TrimSpace(line[capabilityIndex : platformIndex]),
					Platform:          strings.TrimSpace(line[platformIndex : portIDIndex]),
					NeighborInterface: strings.TrimSpace(line[portIDIndex:]),
				}
				neighbors = append(neighbors, neighbor)
				lastDeviceID = ""

			} else {
				// *** TYPE C: DEVICE ID ONLY LINE ***
				// The line is too short to be a full record. It is just a Device ID.
				lastDeviceID = trimmedLine
			}
		}
	}

	return neighbors, nil
}
