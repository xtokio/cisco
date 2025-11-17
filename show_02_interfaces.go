package cisco

import (
	"fmt"
	"log"
	"regexp"
	"strings"
)

// InterfaceDetails defines the structure for the detailed information of a single interface.
type InterfaceDetails struct {
	Interface      string
	Description    string
	Hardware       string
	MacAddress     string
	IPAddress      string
	LinkStatus     string
	ProtocolStatus string
	Duplex         string
	Speed          string
	MediaType      string
	Mtu            string
	Bandwidth      string
	Delay          string
	Reliability    string
	TxLoad         string
	RxLoad         string
	Encapsulation  string
	LastInput      string
	LastOutput     string
	OutputHang     string
	QueueStrategy  string
	InputRateBps   string
	OutputRateBps  string
	PacketsInput   string
	PacketsOutput  string
	Runts          string
	Giants         string
	Throttles      string
	BytesInput     string
	BytesOutput    string
	InputErrors    string
	OutputErrors   string
	CrcErrors      string
	Collisions     string
}

// Show_interfaces connects to a switch, gets interface data, and returns it as a map.
func Show_interfaces(switch_hostname string) ([]InterfaceDetails, error) {
	outputString, err := RunCommand(switch_hostname, "show interface")
	if err != nil {
		return nil, err
	}

	show_interface_data, err := parseInterfaces(outputString)
	if err != nil {
		log.Printf("Error during parsing 'show interfaces' output for %s: %v", switch_hostname, err)
		return nil, fmt.Errorf("error during parsing 'show interfaces' output for %s: %v", switch_hostname, err)
	}

	// Check the length of the slice, not the map.
	if len(show_interface_data) == 0 {
		log.Printf("Show Interfaces ::Warning: Parsing completed for %s, but no interfaces were found.", switch_hostname)
		return nil, nil
	}

	// We no longer need the map, as it discards the order.
	// We will normalize the names directly in the slice.
	for i := range show_interface_data {
		show_interface_data[i].Interface = normalizeInterfaceName(show_interface_data[i].Interface)
	}

	// Return the data
	return show_interface_data, nil
}

// findString helper remains the same.
func findString(re *regexp.Regexp, s string) string {
	matches := re.FindStringSubmatch(s)
	if len(matches) > 1 {
		return strings.TrimSpace(matches[1])
	}
	return ""
}

// parseInterfaces is updated with a highly specific reInterfaceStart regex.
func parseInterfaces(rawOutput string) ([]InterfaceDetails, error) {
	var interfaces []InterfaceDetails
	var currentBlock []string

	// THIS IS THE CRITICAL CHANGE:
	// We now require the first word to contain at least one digit.
	// This matches "GigabitEthernet1/0/13" and "Ethernet101/1/23"
	// but will NOT match "admin state is up...".
	reInterfaceStart := regexp.MustCompile(`^(\S+\d+\S*)\s+is\s+.*`)

	// --- Cleaning Logic ---
	var cleanLines []string
	parsingActive := false
	rePrompt := regexp.MustCompile(`^\S+[>#]\s*$`)

	lines := strings.Split(rawOutput, "\n")
	for _, line := range lines {
		line = strings.TrimRight(line, "\r")
		if !parsingActive && strings.Contains(line, "show interface") {
			parsingActive = true
			continue
		}
		if parsingActive && rePrompt.MatchString(line) {
			parsingActive = false
		}
		if parsingActive {
			cleanLines = append(cleanLines, line)
		}
	}

	for _, line := range cleanLines {
		if reInterfaceStart.MatchString(line) {
			if len(currentBlock) > 0 {
				iface := parseSingleInterface(strings.Join(currentBlock, "\n"))
				if iface.Interface != "" {
					interfaces = append(interfaces, iface)
				}
			}
			currentBlock = []string{line}
		} else if len(currentBlock) > 0 {
			// This line is part of the previous block
			currentBlock = append(currentBlock, line)
		}
	}

	// Process the last block
	if len(currentBlock) > 0 {
		iface := parseSingleInterface(strings.Join(currentBlock, "\n"))
		if iface.Interface != "" {
			interfaces = append(interfaces, iface)
		}
	}

	return interfaces, nil
}

// parseSingleInterface is updated to handle both IOS and Nexus-style output.
func parseSingleInterface(block string) InterfaceDetails {
	iface := InterfaceDetails{}

	// --- Regex definitions updated for flexibility ---

	// Status: Made "line protocol" optional to handle both "is up, line protocol is up" (IOS)
	// and just "is up" (Nexus)
	reStatus := regexp.MustCompile(`^(\S+)\s+is\s+(administratively down|down|up|err-disabled|deleted)(?:,\s+line\s+protocol\s+is\s+(down|up|down \(disabled\)))?`)

	// Hardware: Allows "Hardware is" (IOS) or "Hardware:" (Nexus)
	reHardware := regexp.MustCompile(`Hardware(?::| is) ([^,]+), address is ([\w\.]+)`)

	reDescription := regexp.MustCompile(`Description:\s*(.*)`)
	reAddress := regexp.MustCompile(`Internet address is ([\d\.]+\/\d+)`)

	// Mtu/Bw/Dly: Made "/sec" and trailing comma optional
	reMtuBwDly := regexp.MustCompile(`MTU (\d+) bytes, BW (\d+) Kbit(?:/sec)?, DLY (\d+) usec(?:,)?`)

	// Duplex/Speed/Media: Made "media type" optional (present in IOS, absent in Nexus)
	reDuplexSpeedMedia := regexp.MustCompile(`\s*(\S+-duplex),\s*([^,]+)(?:,\s*media type is (.*))?`)

	// Encapsulation: Made trailing comma optional
	reEncapsulation := regexp.MustCompile(`\s*Encapsulation ([^,]+),?`)

	reReliabilityLoad := regexp.MustCompile(`reliability\s+(\d+\/\d+),\s+txload\s+(\d+\/\d+),\s+rxload\s+(\d+\/\d+)`)

	// Rates: Looks for "5 minute" (IOS) or "30 seconds" (Nexus) which both use "bits/sec"
	reRates := regexp.MustCompile(`(?s)(?:5 minute|30 seconds) input rate (\d+) bits/sec,.*(?:5 minute|30 seconds) output rate (\d+) bits/sec`)

	// Counters: Allows "packets input" (IOS) or "input packets" (Nexus) and an optional comma
	reInputCounters := regexp.MustCompile(`(\d+)\s+(?:packets\s+input|input\s+packets)(?:,)?\s+(\d+)\s+bytes`)

	// --- Split Input/CRC Errors for Nexus ---
	reInputErrors := regexp.MustCompile(`(\d+)\s+input\s+errors,\s+(\d+)\s+CRC`) // IOS
	reInputErrorsNexus := regexp.MustCompile(`(\d+)\s+input\s+error(?:s)?`)      // Nexus Input Errors
	reCrcErrorsNexus := regexp.MustCompile(`(\d+)\s+CRC`)                        // Nexus CRC (found elsewhere in block)

	// Counters: Allows "packets output" (IOS) or "output packets" (Nexus) and an optional comma
	reOutputCounters := regexp.MustCompile(`(\d+)\s+(?:packets\s+output|output\s+packets)(?:,)?\s+(\d+)\s+bytes`)

	// Output Errors: Allows optional comma and "collision" or "collisions"
	reOutputErrors := regexp.MustCompile(`(\d+)\s+output\s+errors(?:,)?\s+(\d+)\s+collision(?:s)?`)

	reLastIO := regexp.MustCompile(`\s*Last input\s+(.*?),` + `\s+output\s+(.*?),` + `\s+output hang\s+(.*)`)
	reQueueStrategy := regexp.MustCompile(`Queueing strategy:\s*(.*)`)

	// --- Split Runts/Giants/Throttles for Nexus ---
	reRuntsGiantsThrottles := regexp.MustCompile(`\s*(\d+)\s+runts,\s+(\d+)\s+giants,\s+(\d+)\s+throttles`) // IOS
	reRuntsGiantsNexus := regexp.MustCompile(`\s*(\d+)\s+runts\s+(\d+)\s+giants`)                           // Nexus (no throttles here, and no commas)

	// --- Logic to assign values ---

	if matches := reStatus.FindStringSubmatch(block); len(matches) > 2 {
		iface.Interface = matches[1]
		iface.LinkStatus = matches[2]
		// Check if the optional 3rd capture group (protocol status) was captured
		if len(matches) > 3 && matches[3] != "" {
			iface.ProtocolStatus = matches[3] // IOS: "up" or "down"
		} else {
			// If not (e.g., Nexus output), set protocol status to be the same as link status
			iface.ProtocolStatus = matches[2]
		}
	} else {
		// This log should no longer be hit by "admin state"
		log.Printf("Failed to parse block with reStatus regex. Block content:\n---\n%s\n---", block)
		return InterfaceDetails{}
	}

	if matches := reHardware.FindStringSubmatch(block); len(matches) > 2 {
		iface.Hardware = strings.TrimSpace(matches[1])
		iface.MacAddress = strings.TrimSpace(matches[2])
	}

	iface.Description = strings.TrimSpace(findString(reDescription, block))
	iface.IPAddress = findString(reAddress, block)

	if matches := reMtuBwDly.FindStringSubmatch(block); len(matches) > 3 {
		iface.Mtu = matches[1]
		iface.Bandwidth = matches[2]
		iface.Delay = matches[3]
	}

	if matches := reDuplexSpeedMedia.FindStringSubmatch(block); len(matches) > 2 {
		iface.Duplex = strings.TrimSpace(matches[1])
		iface.Speed = strings.TrimSpace(matches[2])
		// Check if optional "media type" (group 3) was captured
		if len(matches) > 3 && matches[3] != "" {
			iface.MediaType = strings.TrimSpace(matches[3])
		}
	}

	iface.Encapsulation = findString(reEncapsulation, block)

	if matches := reReliabilityLoad.FindStringSubmatch(block); len(matches) > 3 {
		iface.Reliability = matches[1]
		iface.TxLoad = matches[2]
		iface.RxLoad = matches[3]
	}

	if matches := reRates.FindStringSubmatch(block); len(matches) > 2 {
		iface.InputRateBps = matches[1]
		iface.OutputRateBps = matches[2]
	}

	if matches := reInputCounters.FindStringSubmatch(block); len(matches) > 2 {
		iface.PacketsInput = matches[1]
		iface.BytesInput = matches[2]
	}

	// Use conditional logic for errors, as formats differ significantly
	if matches := reInputErrors.FindStringSubmatch(block); len(matches) > 2 {
		// IOS style
		iface.InputErrors = matches[1]
		iface.CrcErrors = matches[2]
	} else {
		// Try Nexus style (errors and CRC are on different lines)
		iface.InputErrors = findString(reInputErrorsNexus, block)
		iface.CrcErrors = findString(reCrcErrorsNexus, block) // findString will get the CRC value
	}

	if matches := reOutputCounters.FindStringSubmatch(block); len(matches) > 2 {
		iface.PacketsOutput = matches[1]
		iface.BytesOutput = matches[2]
	}

	if matches := reOutputErrors.FindStringSubmatch(block); len(matches) > 2 {
		iface.OutputErrors = matches[1]
		iface.Collisions = matches[2]
	}

	if matches := reLastIO.FindStringSubmatch(block); len(matches) > 3 {
		iface.LastInput = strings.TrimSpace(matches[1])
		iface.LastOutput = strings.TrimSpace(matches[2])
		iface.OutputHang = strings.TrimSpace(matches[3])
	}

	iface.QueueStrategy = findString(reQueueStrategy, block)

	// Use conditional logic for runts/giants, as formats differ
	if matches := reRuntsGiantsThrottles.FindStringSubmatch(block); len(matches) > 3 {
		// IOS style
		iface.Runts = matches[1]
		iface.Giants = matches[2]
		iface.Throttles = matches[3]
	} else if matches := reRuntsGiantsNexus.FindStringSubmatch(block); len(matches) > 2 {
		// Try Nexus style (no throttles on this line)
		iface.Runts = matches[1]
		iface.Giants = matches[2]
		// Throttles will remain empty, which is correct
	}

	return iface
}
