package database

import (
	"strings"
	"fmt"

	"cisco/parse"
	"charm.land/log/v2"
)

type InterfaceConfig struct {
	Interface   string
	ConfigLines []string
}

// Show_running_config executes the command, parses the interface configs, and saves them to the DB.
func database_show_running_config(interfaceConfigs []parse.InterfaceConfig, switch_id string, switch_hostname string) error {
	if len(interfaceConfigs) == 0 {
		log.Warnf("Show Running-Config :: Warning: Parsing completed for %s, but no interfaces were found.", switch_hostname)
		return nil
	}

	// Execute_query(fmt.Sprintf("DELETE FROM show_running_config WHERE toDate(created_at) = today() AND switch_id = '%s'", switch_id))

	// We construct one large string of values for the ClickHouse insert.
	sqlStr := "INSERT INTO show_running_config (switch_id, interface, configuration) VALUES "
	var valueRows []string

	for _, cfg := range interfaceConfigs {
		// Join the config lines into a single text block
		configBlock := strings.Join(cfg.ConfigLines, "\n")

		// Escape single quotes in strings to prevent SQL syntax errors
		safeInterface := strings.ReplaceAll(cfg.Interface, "'", "''")
		safeConfig := strings.ReplaceAll(configBlock, "'", "''")

		// Format the row for the VALUES list
		row := fmt.Sprintf("('%s', '%s', '%s')", switch_id, safeInterface, safeConfig)
		valueRows = append(valueRows, row)
	}

	// 3. Execute the bulk insert
	finalQuery := sqlStr + strings.Join(valueRows, ",")
	err := Execute_query(finalQuery)
	if err != nil {
		log.Warnf("Failed to execute bulk insert for %s: %v", switch_hostname, err)
		return err
	}

	log.Infof("%s :: %s :: Show Running-Config :: %d records inserted.", switch_id, switch_hostname, len(interfaceConfigs))

	return nil
}
