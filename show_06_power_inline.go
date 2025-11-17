package cisco

import (
	"fmt"
	"log"
	"strings"
)

// PowerModuleInfo defines the structure for a power supply module.
type PowerModuleInfo struct {
	Module    string
	Available string
	Used      string
	Remaining string
}

// PowerInterfaceInfo defines the structure for a single PoE interface.
type PowerInterfaceInfo struct {
	Interface string
	Admin     string
	Oper      string
	Power     string // (Watts)
	Device    string
	Class     string
	Max       string // (Watts)
}

// Show_power_inline fetches and processes "show power inline" output.
func Show_power_inline(switch_hostname string) ([]PowerModuleInfo, []PowerInterfaceInfo, error) {
	outputString, err := RunCommand(switch_hostname, "show power inline")
	if err != nil {
		return nil, nil, err
	}

	// --- PARSE OUTPUT ---
	power_inline_modules_data, power_inline_interfaces_data, err := parsePowerInline(outputString)
	if err != nil {
		log.Printf("Show power inline :: Warning :: Parsing completed for %s: %v", switch_hostname, err)
		// We can continue if one part failed, but not if both are empty.
		return nil, nil, nil
	}

	return power_inline_modules_data, power_inline_interfaces_data, nil
}

// parsePowerInline processes the raw CLI output from "show power inline".
// It splits parsing into two sections and returns two different slices.
func parsePowerInline(rawOutput string) ([]PowerModuleInfo, []PowerInterfaceInfo, error) {
	var modules []PowerModuleInfo
	var interfaces []PowerInterfaceInfo
	lines := strings.Split(rawOutput, "\n")

	// Define states for our state machine parser
	type section int
	const (
		None section = iota
		Module
		Interface
	)
	currentSection := None

	for _, line := range lines {
		// Get fields *first* to handle all whitespace types.
		fields := strings.Fields(line)

		// --- 1. State Detection ---

		// If it's a blank line, reset the state.
		if len(fields) == 0 {
			currentSection = None
			continue
		}

		// Check the *first field* to determine the section.
		// This is the ONLY place we should change the state,
		// aside from a blank line.
		switch fields[0] {
		case "Module":
			currentSection = Module
			continue // Skip this header line
		case "Interface":
			currentSection = Interface
			continue // Skip this header line
		}

		// --- 2. State-Based Parsing ---
		//
		// --- FIX: NO 'else { currentSection = None }' BLOCKS ---
		// We just ignore lines that don't match our data format.
		// The state stays the same until a new header or blank line.

		switch currentSection {
		case Module:
			// A valid data line has 4 fields and does NOT start with "------"
			if len(fields) == 4 && !strings.HasPrefix(fields[0], "---") {
				mod := PowerModuleInfo{
					Module:    fields[0],
					Available: fields[1],
					Used:      fields[2],
					Remaining: fields[3],
				}
				modules = append(modules, mod)
			}
			// If it's the "(Watts)" line or "------" line, it fails the 'if'
			// and is simply ignored, without changing the state.

		case Interface:
			// A valid data line has >= 6 fields and the first field
			// must look like an interface name (e.g., contains '/')
			if len(fields) >= 6 && strings.Contains(fields[0], "/") {
				iface := PowerInterfaceInfo{
					Interface: fields[0],
					Admin:     fields[1],
					Oper:      fields[2],
					Power:     fields[3],
					// Join all fields between Power and Class
					Device: strings.Join(fields[4:len(fields)-2], " "),
					Class:  fields[len(fields)-2],
					Max:    fields[len(fields)-1],
				}
				interfaces = append(interfaces, iface)
			}
			// If it's the "(Watts)" line, "------", or "Totals:" line,
			// it fails the 'if' and is ignored, without changing the state.
		}
	} // end for loop

	if len(modules) == 0 && len(interfaces) == 0 {
		return nil, nil, fmt.Errorf("could not find module or interface data in output")
	}

	return modules, interfaces, nil
}
