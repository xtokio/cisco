package database

import (
	"fmt"
	"strings"
	"cisco/parse"
	"charm.land/log/v2"
)

// InterfaceStatus defines the structure for a single network interface entry.
type InterfaceStatus struct {
	Interface   string
	Description string
	Status      string
	VlanID      string
	Duplex      string
	Speed       string
	Type        string
}

func database_interfaces_status(show_interface_status_data []parse.InterfaceStatus, switch_id string, switch_hostname string) {
	// Check if data was returned
	if len(show_interface_status_data) == 0 {
		log.Warnf("Show Interface Status :: Warning: Parsing completed for %s, but no interfaces were found.", switch_hostname)
		return
	}

	// Execute_query(fmt.Sprintf("DELETE FROM interfaces_status WHERE toDate(created_at) = today() AND switch_id = '%s'", switch_id))

	// ClickHouse is optimized for large batch inserts in a single VALUES clause.
	sqlStr := "INSERT INTO interfaces_status (switch_id, interface, description, status, vlan_id, duplex, speed, type) VALUES "
	var valueRows []string

	for _, details := range show_interface_status_data {
		// Escape single quotes in strings to prevent SQL syntax errors
		safeIface := strings.ReplaceAll(details.Interface, "'", "''")
		safeDesc := strings.ReplaceAll(details.Description, "'", "''")
		safeStatus := strings.ReplaceAll(details.Status, "'", "''")
		safeVlan := strings.ReplaceAll(details.VlanID, "'", "''")
		safeDuplex := strings.ReplaceAll(details.Duplex, "'", "''")
		safeSpeed := strings.ReplaceAll(details.Speed, "'", "''")
		safeType := strings.ReplaceAll(details.Type, "'", "''")

		// Format the row for the VALUES list
		row := fmt.Sprintf("('%s', '%s', '%s', '%s', '%s', '%s', '%s', '%s')",
			switch_id,
			safeIface,
			safeDesc,
			safeStatus,
			safeVlan,
			safeDuplex,
			safeSpeed,
			safeType,
		)
		valueRows = append(valueRows, row)
	}

	finalQuery := sqlStr + strings.Join(valueRows, ",")

	// 3. Execute the bulk insert
	// We no longer need manual transactions (tx.Begin) for this multi-row operation.
	err := Execute_query(finalQuery)
	if err != nil {
		log.Errorf("Failed to execute bulk insert for %s: %v", switch_hostname, err)
	}

	log.Infof("%s :: %s :: Show Interface Status :: %d records inserted.", switch_id, switch_hostname, len(show_interface_status_data))
}
