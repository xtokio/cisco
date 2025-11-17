package cisco

import (
	"fmt"
	"log"
	"regexp"
	"strings"
)

type InterfaceConfig struct {
	Interface   string
	ConfigLines []string
}

// Show_running_config executes the command, parses the interface configs, and saves them to the DB.
func Show_running_config(switch_hostname string) ([]InterfaceConfig, error) {
	// 1. Run the command
	outputString, err := RunCommand(switch_hostname, "show running-config")
	if err != nil {
		return nil, err
	}

	// 2. Parse the output
	interfaceConfigs, err := parseInterfaceConfig(outputString)
	if err != nil {
		log.Printf("%s :: Show Running-Config :: Error during parsing: %v", switch_hostname, err)
		return nil, err
	}

	if len(interfaceConfigs) == 0 {
		log.Printf("Show Running-Config :: Warning: Parsing completed for %s, but no interfaces were found.", switch_hostname)
		return nil, nil
	}

	for i := range interfaceConfigs {
		interfaceConfigs[i].Interface = normalizeInterfaceName(interfaceConfigs[i].Interface)
	}

	return interfaceConfigs, nil
}

// --- PARSING FUNCTION ---

// parseInterfaceConfig processes the raw CLI output from "show running-config"
// to extract the configuration block for each interface.
func parseInterfaceConfig(rawOutput string) ([]InterfaceConfig, error) {
	var configs []InterfaceConfig
	lines := strings.Split(rawOutput, "\n")

	// Regex to match the start of an interface block: "interface <name>"
	// It captures the interface name group (e.g., FastEthernet0/1, Vlan1, Port-channel1)
	interfaceStartRegex := regexp.MustCompile(`^interface\s+(\S+)$`)

	var currentConfig *InterfaceConfig = nil

	for _, line := range lines {
		line = strings.TrimSpace(line) // Remove leading/trailing whitespace

		if line == "" || strings.HasPrefix(line, "!") || strings.HasPrefix(line, "version") || strings.HasPrefix(line, "hostname") {
			// Skip empty lines, '!', and global configuration lines for simplicity.
			// A more robust parser might track indentation to properly skip non-interface blocks.
			continue
		}

		// Check for the start of a new interface block
		if matches := interfaceStartRegex.FindStringSubmatch(line); len(matches) > 1 {
			// 1. If we were already in an interface block, save the previous one.
			if currentConfig != nil {
				configs = append(configs, *currentConfig)
			}

			// 2. Start a new interface block
			interfaceName := matches[1]
			currentConfig = &InterfaceConfig{
				Interface: interfaceName,
				// Include the 'interface <name>' line itself as the first config line
				ConfigLines: []string{line},
			}
		} else if currentConfig != nil {
			// If we are currently inside an interface block, and the line is NOT a new 'interface' command,
			// it must be a sub-command (e.g., 'switchport access vlan 10').
			// The original 'show running-config' output uses indentation for sub-commands,
			// but TrimSpace(line) handles that.
			currentConfig.ConfigLines = append(currentConfig.ConfigLines, line)
		}
		// If currentConfig is nil, we are in the global config block, so we ignore the line (due to the initial 'continue' checks).
	}

	// Append the *last* collected configuration block, if one exists.
	if currentConfig != nil {
		configs = append(configs, *currentConfig)
	}

	if len(configs) == 0 {
		return nil, fmt.Errorf("no interface configurations found")
	}

	return configs, nil
}
