package database

func Switches_all() []map[string]any {
	rows := Return_query("SELECT * from switches") // 'malloy-spine-1.hub.nd.edu','drni-n7010-itc.hub.nd.edu','lgomezreswitch.hub.nd.edu'
	return rows
}

func Switches_reachable() []map[string]any {
	rows := Return_query("SELECT * from switches WHERE reachable = 1")
	return rows
}

func Switches_by_switch_id(switch_id string) []map[string]any {
	rows := Return_query("SELECT * from switches WHERE id = '"+switch_id+"'")
	return rows
}

func Switches_by_switch(switch_hostname string) []map[string]any {
	rows := Return_query("SELECT * from switches WHERE switch = '"+switch_hostname+"'")
	return rows
}

func Interfaces_by_switch_id(switch_id string) []map[string]any {
	rows := Return_query("SELECT * from view_interfaces WHERE switch_id = "+switch_id)
	return rows
}

func mac_address_table_interfaces(switch_id string) []map[string]any {
	rows := Return_query("SELECT concat(arrayStringConcat(groupArray(concat('_', interface, '_')), '|'), '|_CPU_') AS interfaces FROM interfaces_status WHERE switch_id = '"+switch_id+"' AND status = 'connected' AND vlan_id LIKE '%trunk%' AND toDate(created_at) = today()")
	return rows
}
