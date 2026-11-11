package parse

import (
	"fmt"
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
func Show_interfaces(outputString string) ([]InterfaceDetails, error) {
	show_interface_data, err := parseInterfaces(outputString)
	if err != nil {
		return nil, fmt.Errorf("error during parsing 'show interfaces' output: %v", err)
	}

	// Check the length of the slice, not the map.
	if len(show_interface_data) == 0 {
		return nil, fmt.Errorf("Show Interfaces ::Warning: Parsing completed but no interfaces were found.: %v", err)
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

	lines := strings.SplitSeq(rawOutput, "\n")
	for line := range lines {
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

func parseSingleInterface(block string) InterfaceDetails {
	iface := InterfaceDetails{
		Duplex: "auto",
		Speed:  "auto",
	}

	// --- Regex definitions ---

	// UPDATED Description Regex:
	// This matches anything that DOES NOT start with a backslash ([^\\\r\n]).
	// If the line is "Description: \", it will fail the first part and match the empty alternative (|$).
	reDescription := regexp.MustCompile(`Description:\s*([^\\\r\n].*|$)`)

	reStatus := regexp.MustCompile(`^(\S+)\s+is\s+([^,]+)(?:,\s+line\s+protocol\s+is\s+([^\r\n,]+))?`)
	reHardware := regexp.MustCompile(`Hardware(?::| is) ([^,]+), address is ([\w\.]+)`)
	reAddress := regexp.MustCompile(`Internet address is ([\d\.]+\/\d+)`)
	reMtuBwDly := regexp.MustCompile(`MTU (\d+) bytes, BW (\d+) Kbit(?:/sec)?, DLY (\d+) usec(?:,)?`)
	reDuplexSpeedMedia := regexp.MustCompile(`\s*(\S+-duplex),\s*([^,]+)(?:,\s*media type is (.*))?`)
	reEncapsulation := regexp.MustCompile(`\s*Encapsulation ([^,]+),?`)
	reReliabilityLoad := regexp.MustCompile(`reliability\s+(\d+\/\d+),\s+txload\s+(\d+\/\d+),\s+rxload\s+(\d+\/\d+)`)
	reRates := regexp.MustCompile(`(?s)(?:5 minute|30 seconds) input rate (\d+) bits/sec,.*(?:5 minute|30 seconds) output rate (\d+) bits/sec`)
	reInputCounters := regexp.MustCompile(`(\d+)\s+(?:packets\s+input|input\s+packets)(?:,)?\s+(\d+)\s+bytes`)
	reOutputCounters := regexp.MustCompile(`(\d+)\s+(?:packets\s+output|output\s+packets)(?:,)?\s+(\d+)\s+bytes`)
	reInputErrors := regexp.MustCompile(`(\d+)\s+input\s+errors,\s+(\d+)\s+CRC`)
	reInputErrorsNexus := regexp.MustCompile(`(\d+)\s+input\s+error(?:s)?`)
	reCrcErrorsNexus := regexp.MustCompile(`(\d+)\s+CRC`)
	reOutputErrors := regexp.MustCompile(`(\d+)\s+output\s+errors(?:,)?\s+(\d+)\s+collision(?:s)?`)
	reLastIO := regexp.MustCompile(`\s*Last input\s+(.*?),` + `\s+output\s+(.*?),` + `\s+output hang\s+(.*)`)
	reQueueStrategy := regexp.MustCompile(`Queueing strategy:\s*(.*)`)
	reRuntsGiantsThrottles := regexp.MustCompile(`\s*(\d+)\s+runts,\s+(\d+)\s+giants,\s+(\d+)\s+throttles`)
	reRuntsGiantsNexus := regexp.MustCompile(`\s*(\d+)\s+runts\s+(\d+)\s+giants`)

	// --- Logic ---

	if matches := reStatus.FindStringSubmatch(block); len(matches) > 2 {
		iface.Interface = strings.TrimSpace(matches[1])
		iface.LinkStatus = strings.TrimSpace(matches[2])
		if len(matches) > 3 && matches[3] != "" {
			iface.ProtocolStatus = strings.TrimSpace(matches[3])
		} else {
			iface.ProtocolStatus = iface.LinkStatus
		}
	} else {
		return InterfaceDetails{}
	}

	if matches := reHardware.FindStringSubmatch(block); len(matches) > 2 {
		iface.Hardware = strings.TrimSpace(matches[1])
		iface.MacAddress = strings.TrimSpace(matches[2])
	}

	// Capture and sanitize Description
	desc := strings.TrimSpace(findString(reDescription, block))
	// Extra Safety: if the description is literally just a backslash, clear it.
	if desc == "\\" {
		iface.Description = ""
	} else {
		// Replace any backslashes with forward slashes to prevent SQL escaping issues entirely
		iface.Description = strings.ReplaceAll(desc, "\\", "/")
	}

	iface.IPAddress = strings.TrimSpace(findString(reAddress, block))

	if matches := reMtuBwDly.FindStringSubmatch(block); len(matches) > 3 {
		iface.Mtu = strings.TrimSpace(matches[1])
		iface.Bandwidth = strings.TrimSpace(matches[2])
		iface.Delay = strings.TrimSpace(matches[3])
	}

	if matches := reDuplexSpeedMedia.FindStringSubmatch(block); len(matches) > 2 {
		iface.Duplex = strings.TrimSpace(matches[1])
		iface.Speed = strings.TrimSpace(matches[2])
		if len(matches) > 3 && matches[3] != "" {
			iface.MediaType = strings.TrimSpace(matches[3])
		}
	}

	iface.Encapsulation = strings.TrimSpace(findString(reEncapsulation, block))

	if matches := reReliabilityLoad.FindStringSubmatch(block); len(matches) > 3 {
		iface.Reliability = strings.TrimSpace(matches[1])
		iface.TxLoad = strings.TrimSpace(matches[2])
		iface.RxLoad = strings.TrimSpace(matches[3])
	}

	if matches := reRates.FindStringSubmatch(block); len(matches) > 2 {
		iface.InputRateBps = strings.TrimSpace(matches[1])
		iface.OutputRateBps = strings.TrimSpace(matches[2])
	}

	if matches := reInputCounters.FindStringSubmatch(block); len(matches) > 2 {
		iface.PacketsInput = strings.TrimSpace(matches[1])
		iface.BytesInput = strings.TrimSpace(matches[2])
	}

	if matches := reInputErrors.FindStringSubmatch(block); len(matches) > 2 {
		iface.InputErrors = strings.TrimSpace(matches[1])
		iface.CrcErrors = strings.TrimSpace(matches[2])
	} else {
		iface.InputErrors = strings.TrimSpace(findString(reInputErrorsNexus, block))
		iface.CrcErrors = strings.TrimSpace(findString(reCrcErrorsNexus, block))
	}

	if matches := reOutputCounters.FindStringSubmatch(block); len(matches) > 2 {
		iface.PacketsOutput = strings.TrimSpace(matches[1])
		iface.BytesOutput = strings.TrimSpace(matches[2])
	}

	if matches := reOutputErrors.FindStringSubmatch(block); len(matches) > 2 {
		iface.OutputErrors = strings.TrimSpace(matches[1])
		iface.Collisions = strings.TrimSpace(matches[2])
	}

	if matches := reLastIO.FindStringSubmatch(block); len(matches) > 3 {
		iface.LastInput = strings.TrimSpace(matches[1])
		iface.LastOutput = strings.TrimSpace(matches[2])
		iface.OutputHang = strings.TrimSpace(matches[3])
	}

	iface.QueueStrategy = strings.TrimSpace(findString(reQueueStrategy, block))

	if matches := reRuntsGiantsThrottles.FindStringSubmatch(block); len(matches) > 3 {
		iface.Runts = strings.TrimSpace(matches[1])
		iface.Giants = strings.TrimSpace(matches[2])
		iface.Throttles = strings.TrimSpace(matches[3])
	} else if matches := reRuntsGiantsNexus.FindStringSubmatch(block); len(matches) > 2 {
		iface.Runts = strings.TrimSpace(matches[1])
		iface.Giants = strings.TrimSpace(matches[2])
	}

	return iface
}
