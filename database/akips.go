package database

import (
	"cisco/akips"
	"strings"
	"fmt"

	"charm.land/log/v2"
)

// Akips_get_switch_list checks AKIPS against the DB and adds missing switches.
func Add_missing_switches() error {
	log.Info("AKIPS Sync :: Add missing switches")

	var missing_switch_list []akips.SwitchList

	switch_list := akips.Switch_list()
	switch_records := Switches_all()

	// 1. Identify missing switches
	for _, current_switch := range switch_list {
		found := false
		for _, row := range switch_records {
			// strings.Contains(current_switch.Switch, row["switch"].(string))
			if current_switch.Switch == row["switch"].(string) {
				found = true
				break
			}
		}

		if !found {
			missing_switch_list = append(missing_switch_list, akips.SwitchList{
				Switch: current_switch.Switch,
				IP:     current_switch.IP,
			})
		}
	}

	// 2. Perform Bulk Insert if there are missing switches
	if len(missing_switch_list) > 0 {
		sqlStr := "INSERT INTO switches (switch, ip_address) VALUES "
		var valueRows []string

		for _, sw := range missing_switch_list {
			// Escape strings for ClickHouse safety
			safeFQDN := strings.ReplaceAll(sw.Switch, "'", "''")
			safeIP := strings.ReplaceAll(sw.IP, "'", "''")

			row := fmt.Sprintf("('%s', '%s')", safeFQDN, safeIP)
			valueRows = append(valueRows, row)
		}

		finalQuery := sqlStr + strings.Join(valueRows, ",")

		// Execute_query handles the connection internally
		err := Execute_query(finalQuery)
		if err != nil {
			log.Errorf("Failed to insert missing switches: %v", err)
			return err
		}
		log.Infof("AKIPS Sync :: %d new switches added to database.", len(missing_switch_list))
	}

	return nil
}

// Akips_usage fetches interface last-change data and updates ClickHouse.
func akips_get_interface_usage(switch_id string, switch_hostname string) error {
	interface_usage := akips.Interface_usage(switch_hostname)

	if len(interface_usage) == 0 {
		log.Warnf("AKIPS Usage :: Warning: No data found for %s.", switch_hostname)
		return nil
	}

	sqlStr := "INSERT INTO akips_interface_usage (switch_id, interface, status, last_change, days, hours, minutes) VALUES "
	var valueRows []string

	for _, usage := range interface_usage {
		// Escape interface name
		safeIface := strings.ReplaceAll(usage.Interface, "'", "''")

		row := fmt.Sprintf("('%s', '%s', '%s', '%s', '%s', '%s', '%s')",
			switch_id,
			safeIface,
			usage.Status,
			usage.Last_change,
			usage.Days,
			usage.Hours,
			usage.Minutes,
		)
		valueRows = append(valueRows, row)
	}

	finalQuery := sqlStr + strings.Join(valueRows, ",")

	// 3. Execute bulk insert
	err := Execute_query(finalQuery)
	if err != nil {
		log.Errorf("Failed to execute bulk insert for %s (AKIPS usage): %v", switch_hostname, err)
		return err
	}

	log.Infof("%s :: %s :: AKIPS Usage :: %d records inserted.", switch_id, switch_hostname, len(interface_usage))

	return nil
}
