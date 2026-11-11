package database

import (
	"fmt"
	"strings"

	"cisco/parse"
	"charm.land/log/v2"
)

// Show_mac_address_table constructs the command, runs it, and processes the output.
func database_show_mac_address_table(mac_table_data []parse.MacAddressEntry, interface_status []parse.InterfaceStatus, switch_id string, switch_hostname string) {
	if len(mac_table_data) == 0 {
		log.Warnf("Show MAC Address Table :: Warning: Parsing completed for %s, but no MAC entries were found.", switch_hostname)
		return
	}

	// 1. Build a lookup set of all trunk/routed interfaces for this switch
		trunkPorts := make(map[string]bool)
		for _, status := range interface_status {
			vlanLower := strings.ToLower(strings.TrimSpace(status.VlanID))
			if vlanLower == "trunk" || vlanLower == "routed" {
				// Store raw interface name (e.g., "Gi1/0/48")
				trunkPorts[status.Interface] = true

				// Store normalized canonical variations to handle name mismatch (e.g. "GigabitEthernet1/0/48" vs "Gi1/0/48")
				canonical := normalizeInterfaceName(status.Interface)
				trunkPorts[canonical] = true
			}
		}

	// Execute_query(fmt.Sprintf("DELETE FROM mac_address_table WHERE toDate(created_at) = today() AND switch_id = '%s'", switch_id))

	// ClickHouse is most efficient when receiving many rows in one large string.
	sqlStr := "INSERT INTO mac_address_table (switch_id, interface, mac_address, vlan_id, type) VALUES "
	var valueRows []string
	var row_count int

	for _, details := range mac_table_data {

		iface := strings.TrimSpace(details.Interface)

		// 2. Skip CPU, Router, and any interface identified as a trunk or routed port
		if iface == "" || iface == "CPU" || iface == "Router" {
			continue
		}
		if trunkPorts[iface] || trunkPorts[normalizeInterfaceName(iface)] {
			continue
		}

		row_count += 1

		// Escape single quotes in strings for safety
		safeIface := strings.ReplaceAll(details.Interface, "'", "''")
		safeMac := strings.ReplaceAll(details.MacAddress, "'", "''")
		safeVlan := strings.ReplaceAll(details.VlanID, "'", "''")
		safeType := strings.ReplaceAll(details.Type, "'", "''")

		row := fmt.Sprintf("('%s', '%s', '%s', '%s', '%s')",
			switch_id,
			safeIface,
			safeMac,
			safeVlan,
			safeType,
		)
		valueRows = append(valueRows, row)
	}

	// 5. Execute the insert via the global Execute_query function
	finalQuery := sqlStr + strings.Join(valueRows, ",")
	err := Execute_query(finalQuery)
	if err != nil {
		log.Errorf("Failed to execute bulk insert for %s: %v", switch_hostname, err)
	}

	log.Infof("%s :: %s :: Show MAC Address Table :: %d records inserted.", switch_id, switch_hostname, row_count)
}