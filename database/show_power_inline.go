package database

import (
	"strings"
	"fmt"
	"charm.land/log/v2"
	"cisco/parse"
)

// Show_power_inline fetches and processes "show power inline" output.
func database_power_inline(show_power_inline_modules_data []parse.PowerModuleInfo, show_power_inline_interfaces_data []parse.PowerInterfaceInfo, switch_id string, switch_hostname string) {
	// Process Modules: ClickHouse handles these via the global Execute_query function.
	if len(show_power_inline_modules_data) > 0 {
		processPowerModules(switch_id, switch_hostname, show_power_inline_modules_data)
	} else {
		log.Warnf("Warning: No power modules found for %s.", switch_hostname)
		return
	}

	// Process Interfaces
	if len(show_power_inline_interfaces_data) > 0 {
		processPowerInterfaces(switch_id, switch_hostname, show_power_inline_interfaces_data)
	} else {
		log.Warnf("Warning: No power interfaces found for %s.", switch_hostname)
		return
	}
}

// processPowerModules handles the bulk insert for power modules using ClickHouse syntax.
func processPowerModules(switch_id string, switch_hostname string, modules []parse.PowerModuleInfo) {
	// Execute_query(fmt.Sprintf("DELETE FROM power_modules WHERE toDate(created_at) = today() AND switch_id = '%s'", switch_id))

	sqlStr := "INSERT INTO power_modules (switch_id, module, available, used, remaining) VALUES "
	var valueRows []string

	for _, mod := range modules {
		// Escape strings to prevent SQL errors
		safeModule := strings.ReplaceAll(mod.Module, "'", "''")

		row := fmt.Sprintf("('%s', '%s', '%s', '%s', '%s')",
			switch_id,
			safeModule,
			mod.Available,
			mod.Used,
			mod.Remaining,
		)
		valueRows = append(valueRows, row)
	}

	finalQuery := sqlStr + strings.Join(valueRows, ",")
	err := Execute_query(finalQuery)
	if err != nil {
		log.Errorf("Failed to execute bulk insert for %s (modules): %v", switch_hostname, err)
		return
	}

	log.Infof("%s :: %s :: Show Power Modules :: %d records inserted.", switch_id, switch_hostname, len(modules))
}

// processPowerInterfaces handles the bulk insert for power interfaces using ClickHouse syntax.
func processPowerInterfaces(switch_id string, switch_hostname string, interfaces []parse.PowerInterfaceInfo) {
	// Execute_query(fmt.Sprintf("DELETE FROM power_interfaces WHERE toDate(created_at) = today() AND switch_id = '%s'", switch_id))

	// 2. Build Bulk Insert String
	sqlStr := "INSERT INTO power_interfaces (switch_id, interface, admin, oper, power, device, class, max) VALUES "
	var valueRows []string

	for _, iface := range interfaces {
		// Escape strings for safety
		safeIface := strings.ReplaceAll(iface.Interface, "'", "''")
		safeDevice := strings.ReplaceAll(iface.Device, "'", "''")

		row := fmt.Sprintf("('%s', '%s', '%s', '%s', '%s', '%s', '%s', '%s')",
			switch_id,
			safeIface,
			iface.Admin,
			iface.Oper,
			iface.Power,
			safeDevice,
			iface.Class,
			iface.Max,
		)
		valueRows = append(valueRows, row)
	}

	finalQuery := sqlStr + strings.Join(valueRows, ",")
	err := Execute_query(finalQuery)
	if err != nil {
		log.Errorf("Failed to execute bulk insert for %s (interfaces): %v", switch_hostname, err)
		return
	}

	log.Infof("%s :: %s :: Show Power Interfaces :: %d records inserted.", switch_id, switch_hostname, len(interfaces))
}
