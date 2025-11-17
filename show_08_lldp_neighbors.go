package cisco

import (
	"fmt"
	"log"
	"strings"
)

// LldpNeighbor defines the structure for a single LLDP neighbor entry.
type LldpNeighbor struct {
	Interface         string
	Neighbor          string
	NeighborInterface string
	HoldTime          string
	Capability        string
}

func Show_lldp_neighbors(switch_hostname string) ([]LldpNeighbor, error) {
	outputString, err := RunCommand(switch_hostname, "show lldp neighbors")
	if err != nil {
		return nil, err
	}

	lldp_neighbors_data, err := parseLldpNeighbors(outputString)
	if err != nil {
		log.Printf("%s ::Show LLDP Neighbors :: Error during parsing: %v", switch_hostname, err)
	}

	for i := range lldp_neighbors_data {
		lldp_neighbors_data[i].Interface = normalizeInterfaceName(lldp_neighbors_data[i].Interface)
		lldp_neighbors_data[i].NeighborInterface = normalizeInterfaceName(lldp_neighbors_data[i].NeighborInterface)
	}

	// Check the length of the slice, not the map.
	if len(lldp_neighbors_data) == 0 {
		log.Printf("Show LLDP Neighbors :: Warning: Parsing completed for %s, but no interfaces were found.", switch_hostname)
		return nil, nil
	}

	return lldp_neighbors_data, nil

}

// parseLldpNeighbors processes the raw CLI output from "show lldp neighbors".
func parseLldpNeighbors(rawOutput string) ([]LldpNeighbor, error) {
	var neighbors []LldpNeighbor
	lines := strings.Split(rawOutput, "\n")

	headerLine := ""
	dataStartIndex := -1

	// Find the header line
	for i, line := range lines {
		if strings.HasPrefix(line, "Device ID") {
			headerLine = line
			dataStartIndex = i + 1
			break
		}
	}

	if dataStartIndex == -1 {
		log.Println("LLDP neighbors header not found, returning empty list.")
		return neighbors, nil
	}

	// Determine start indices of each column
	deviceIDIndex := strings.Index(headerLine, "Device ID")
	localIntfIndex := strings.Index(headerLine, "Local Intf")
	holdtmeIndex := strings.Index(headerLine, "Hold-time")
	capabilityIndex := strings.Index(headerLine, "Capability")
	portIDIndex := strings.Index(headerLine, "Port ID")

	if deviceIDIndex == -1 || localIntfIndex == -1 || holdtmeIndex == -1 || capabilityIndex == -1 || portIDIndex == -1 {
		return nil, fmt.Errorf("could not parse LLDP neighbors header columns")
	}

	for i := dataStartIndex; i < len(lines); i++ {
		line := strings.TrimRight(lines[i], "\r")
		if strings.TrimSpace(line) == "" || strings.Contains(line, "Total entries displayed") {
			continue
		}
		if len(line) < portIDIndex {
			continue
		}

		neighbor := LldpNeighbor{
			Interface:         strings.TrimSpace(line[localIntfIndex:holdtmeIndex]),
			Neighbor:          strings.TrimSpace(line[deviceIDIndex:localIntfIndex]),
			NeighborInterface: strings.TrimSpace(line[portIDIndex:]),
			HoldTime:          strings.TrimSpace(line[holdtmeIndex:capabilityIndex]),
			Capability:        strings.TrimSpace(line[capabilityIndex:portIDIndex]),
		}

		neighbors = append(neighbors, neighbor)
	}

	return neighbors, nil
}
