package database

import (
	"fmt"
	"strings"
	"cisco/parse"
	"charm.land/log/v2"
)

// VlanInfo defines the structure for a single VLAN entry.
type VlanInfo struct {
	VLANID   string
	VLANName string
	Status   string
	Ports    []string
}

func database_show_vlan(show_vlan_data []parse.VlanInfo, switch_id string, switch_hostname string) {
	if len(show_vlan_data) == 0 {
		log.Warnf("Show VLAN :: Warning: No interfaces found for %s.", switch_hostname)
		return
	}

	// Execute_query(fmt.Sprintf("DELETE FROM vlans WHERE toDate(created_at) = today() AND switch_id = '%s'", switch_id))

	// Since ClickHouse is highly optimized for bulk inserts, we will
	// construct one large string of values.
	sqlStr := "INSERT INTO vlans (switch_id, vlan_id, vlan_name, status, interfaces) VALUES "
	var valueRows []string

	for _, details := range show_vlan_data {
		portsString := strings.Join(details.Ports, ",")

		// Escape single quotes in strings to prevent SQL errors
		vlanName := strings.ReplaceAll(details.VLANName, "'", "''")
		status := strings.ReplaceAll(details.Status, "'", "''")

		// Format the row for the VALUES list
		row := fmt.Sprintf("('%s', '%s', '%s', '%s', '%s')",
			switch_id,
			details.VLANID,
			vlanName,
			status,
			portsString,
		)
		valueRows = append(valueRows, row)
	}

	finalQuery := sqlStr + strings.Join(valueRows, ",")

	// 3. Execute the bulk insert
	// We no longer need db.Begin() or tx.Commit() because ClickHouse
	// processes this single INSERT statement atomically.
	err := Execute_query(finalQuery)
	if err != nil {
		log.Errorf("Failed to execute bulk insert for %s: %v", switch_hostname, err)
	}

	log.Infof("%s :: %s :: Show Vlans :: %d records inserted.", switch_id, switch_hostname, len(show_vlan_data))
}
