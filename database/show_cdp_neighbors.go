package database

import (
	"fmt"
	"strings"
	"cisco/parse"
	"charm.land/log/v2"
)

func database_show_cdp_neighbors(show_cdp_neighbors_data []parse.CdpNeighbor, switch_id string, switch_hostname string) {
	if len(show_cdp_neighbors_data) == 0 {
		log.Warnf("Warning: Parsing completed for %s, but no cdp_neighbors were found.", switch_hostname)
		return
	}

	// Execute_query(fmt.Sprintf("DELETE FROM cdp_neighbors WHERE toDate(created_at) = today() AND switch_id = '%s'", switch_id))

	// 2. Build the Bulk INSERT statement
	// ClickHouse is optimized for large VALUES blocks in a single query.
	sqlStr := "INSERT INTO cdp_neighbors (switch_id, interface, neighbor_name, neighbor_interface, capabilities, platform) VALUES "
	var valueRows []string

	for _, details := range show_cdp_neighbors_data {
		// Escape single quotes in strings for safety
		safeIface := strings.ReplaceAll(details.Interface, "'", "''")
		safeNeighbor := strings.ReplaceAll(details.Neighbor, "'", "''")
		safeNeighborIface := strings.ReplaceAll(details.NeighborInterface, "'", "''")
		safeCapability := strings.ReplaceAll(details.Capability, "'", "''")
		safePlatform := strings.ReplaceAll(details.Platform, "'", "''")

		// Format the row for the VALUES list
		row := fmt.Sprintf("('%s', '%s', '%s', '%s', '%s', '%s')",
			switch_id,
			safeIface,
			safeNeighbor,
			safeNeighborIface,
			safeCapability,
			safePlatform,
		)
		valueRows = append(valueRows, row)
	}

	finalQuery := sqlStr + strings.Join(valueRows, ",")

	// 3. Execute the bulk insert
	// We no longer need manual transactions (tx.Begin) for this operation.
	err := Execute_query(finalQuery)
	if err != nil {
		log.Errorf("Failed to execute bulk insert for %s: %v", switch_hostname, err)
	}

	log.Infof("%s :: %s :: Show CDP Neighbors :: %d records inserted.", switch_id, switch_hostname, len(show_cdp_neighbors_data))
}
