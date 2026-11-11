package database

import (
	"fmt"
	"strings"
	"cisco/parse"
	"charm.land/log/v2"
)

func database_show_lldp_neighbors(show_lldp_neighbors_data []parse.LldpNeighbor, switch_id string, switch_hostname string) {
	// Check if data exists before proceeding
	if len(show_lldp_neighbors_data) == 0 {
		log.Warnf("Show LLDP Neighbors :: Warning: Parsing completed for %s, but no interfaces were found.", switch_hostname)
		return
	}

	// Execute_query(fmt.Sprintf("DELETE FROM lldp_neighbors WHERE toDate(created_at) = today() AND switch_id = '%s'", switch_id))

	// ClickHouse performs best with single, large inserts rather than multiple small ones.
	sqlStr := "INSERT INTO lldp_neighbors (switch_id, interface, neighbor_name, neighbor_interface, capabilities) VALUES "
	var valueRows []string

	for _, details := range show_lldp_neighbors_data {
		// Escape single quotes in strings to prevent SQL syntax errors
		safeInterface := strings.ReplaceAll(details.Interface, "'", "''")
		safeNeighbor := strings.ReplaceAll(details.Neighbor, "'", "''")
		safeNeighborIface := strings.ReplaceAll(details.NeighborInterface, "'", "''")
		safeCapability := strings.ReplaceAll(details.Capability, "'", "''")

		// Format the row for the VALUES list
		row := fmt.Sprintf("('%s', '%s', '%s', '%s', '%s')",
			switch_id,
			safeInterface,
			safeNeighbor,
			safeNeighborIface,
			safeCapability,
		)
		valueRows = append(valueRows, row)
	}

	finalQuery := sqlStr + strings.Join(valueRows, ",")

	// 3. Execute the bulk insert
	// Transactions (tx.Begin) are not required for this single multi-row insert in ClickHouse.
	err := Execute_query(finalQuery)
	if err != nil {
		log.Errorf("Failed to execute bulk insert for %s: %v", switch_hostname, err)
	}

	log.Infof("%s :: %s :: Show LLDP Neighbors :: %d records inserted.", switch_id, switch_hostname, len(show_lldp_neighbors_data))
}
