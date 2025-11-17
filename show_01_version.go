package cisco

import (
	"fmt"
	"log"
	"reflect"
	"regexp"
	"strings"
)

// VersionInfo defines the structure for the parsed "show version" output.
// It's used as an intermediate struct within the parsing function.
type VersionInfo struct {
	Hardware      string
	Version       string
	Release       string
	SoftwareImage string
	SerialNumber  string
	Uptime        string
	Restarted     string
	ReloadReason  string
	Rommon        string
}

// Show_version connects to a switch, runs "show version", and returns the parsed data as a map.
func Show_version(switch_hostname string) (map[string]string, error) {
	outputString, err := RunCommand(switch_hostname, "show version")
	if err != nil {
		return nil, err
	}

	// --- PARSE OUTPUT ---
	show_version_data, err := parseVersionInfo(outputString)
	if err != nil {
		log.Printf("Error parsing 'show version' output for %s: %v", switch_hostname, err)
		return nil, fmt.Errorf("error parsing 'show version' output for %s: %v", switch_hostname, err)
	}

	return show_version_data, nil
}

// parseVersionInfo processes the raw CLI output from "show version".
// It returns a map of string keys to string values.
func parseVersionInfo(rawOutput string) (map[string]string, error) {
	var info VersionInfo
	result := make(map[string]string) // Initialize the map to be returned

	// Define regular expressions for each piece of data we want to capture.
	regexes := map[string]*regexp.Regexp{
		// Hardware: (IOS/IE1000) | (Nexus: Chassis name)
		"Hardware": regexp.MustCompile(`(?i)cisco ([\w-]+[a-z\d\-]+) .* processor|Board Type\s*:\s*(\S+)|Product\s*:\s*Cisco ([\w\s]+) Switch|cisco (Nexus\S+ [\w-]+ Chassis)|cisco ([\w-]+ Chassis)`),

		// Version: (IOS) | (IE1000) | (Nexus: system version)
		"Version": regexp.MustCompile(`(?i)Version ([^,]+),|NXOS:\s*version\s*(\S+).*|Active Image\s*:\s*.*?\nVersion\s*:\s*(\S+)|Software Version\s*:\s*(\S+)|system:\s*version\s*(\S+)`),

		// Release: (IOS only, not easily mapped for NX-OS/IE1000)
		"Release": regexp.MustCompile(`(?i)Version [^,]+, (RELEASE SOFTWARE .*)`),

		// SoftwareImage: (IOS) | (IE1000) | (Nexus: system image file)
		"SoftwareImage": regexp.MustCompile(`(?i)System image file is "([^"]+)"|NXOS image file is:\s*(\S+)|Active Image\s*:\s*([^\s(]+)|system image file is:\s*(\S+)`),

		// SerialNumber: (IOS: System/Processor ID) | (IE1000: MAC Address) | (Nexus: Processor Board ID)
		"SerialNumber": regexp.MustCompile(`(?i)(?:System serial number\s*:\s*(\S+)|Processor board ID\s*(\S+)|MAC Address\s*:\s*(\S+)|Processor Board ID\s*(\S+))`),

		// Uptime: (IOS) | (IE1000) | (Nexus: Kernel uptime)
		"Uptime": regexp.MustCompile(`(?i)uptime is (.+)|System Uptime\s*:\s*(\S+)|Kernel uptime is (.+)`),

		// Restarted: (IOS) | (IE1000) | (Nexus: Last reset reason) - Nexus uses Last Reset/Reason instead of 'Restarted At'
		// We'll capture the time-like string from the IOS/IE1000, or the Reason/System Version from Nexus.
		"Restarted": regexp.MustCompile(`(?i)System restarted at (.*)|Previous Restart\s*:\s*(.*)|Last reset\s*\n\s*Reason:\s*(\S+)`),

		// ReloadReason: (IOS) | (Nexus: Last reset reason)
		"ReloadReason": regexp.MustCompile(`(?i)(?:Last reload reason: (.*)|System returned to ROM by (.*)|Last reset\s*\n\s*Reason:\s*(.*))`),

		// Rommon: (IOS: ROM) | (IE1000: Bootloader) | (Nexus: BIOS)
		"Rommon": regexp.MustCompile(`(?i)ROM: (.*)|Bootloader\s*:\s*(\S+)|BIOS:\s*version\s*(\S+)`),
	}

	// Use reflection to dynamically match regexes to struct fields
	v := reflect.ValueOf(&info).Elem()
	t := v.Type()

	for _, line := range strings.Split(rawOutput, "\n") {
		cleanLine := strings.TrimSpace(line)
		for i := 0; i < v.NumField(); i++ {
			fieldName := t.Field(i).Name
			fieldValue := v.Field(i)

			if fieldValue.String() == "" { // Only parse if not already found
				if re, ok := regexes[fieldName]; ok {
					if matches := re.FindStringSubmatch(cleanLine); len(matches) > 1 {
						// Iterate over all subgroups to find the first non-empty match
						for j := 1; j < len(matches); j++ {
							match := strings.TrimSpace(matches[j])
							if match != "" {
								fieldValue.SetString(match)
								break // Found the value for this field, move to next field
							}
						}
					}
				}
			}
		}
	}

	// Check if we found at least some data
	if info.Version == "" || info.SerialNumber == "" {
		return nil, fmt.Errorf("could not parse essential version info from output")
	}

	// Convert the populated struct to a map
	for i := 0; i < v.NumField(); i++ {
		value := v.Field(i).String()
		if value != "" { // Only add keys for values that were found
			key := t.Field(i).Name
			result[key] = value
		}
	}

	return result, nil
}
