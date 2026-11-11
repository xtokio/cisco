package database

import (
	"charm.land/log/v2"
	"cisco/connection"
	"cisco/parse"
	"strings"
	"time"
	"fmt"
	"os"
)

func Process_arp_table(){
	// Record the start time.
	start := time.Now()

	// Get ARP records
	log.Info("Processing ARP records...")
	arp_host_records := [...]string{"core-9500-mgmnt.gw.nd.edu", "rk-9600-mgmnt.gw.nd.edu", "hc-9600-mgmnt.gw.nd.edu", "shub-stad.gw.nd.edu", "non-nd.gw.nd.edu"}
	arp_vrf_host_records := [...]string{"student", "ath-guest", "COGENT", "GUEST", "DMZ", "default", "management"}
	for _, arp_host_record := range arp_host_records {
		err := show_ip_arp(arp_host_record)
		if err != nil {
			log.Errorf("ERROR [Show_ip_arp] %s: %v", arp_host_record, err)
		}

		// Check for VRF records
		for _, arp_vrf_host_record := range arp_vrf_host_records {
			err := show_ip_arp_vrf(arp_host_record, arp_vrf_host_record)
			if err != nil {
				log.Errorf("ERROR [Show_ip_arp_vrf] %s: %v", arp_host_record, err)
			}
		}
	}

	// Process Cisco arp records from Rome
	log.Info("Processing Rome ARP records...")
	// Retrieve credentials from environment variables
	var rome_username = os.Getenv("CISCO_ROME_USERNAME")
	var rome_password = os.Getenv("CISCO_ROME_PASSWORD")
	show_ip_arp_rome("rome-wlc9k-a.ap.nd.edu", rome_username, rome_password)

	// Process Palo Alto arp records
	log.Info("Processing Palo Alto ARP records...")
	palo_alto_arp_records()

	duration := time.Since(start)
	log.Info("ARP Table Process duration: " + duration.String())

}

// Process_fqdn performs reverse lookups for all active interfaces and updates ClickHouse.
type FQDN struct {
	MACAddress string
	IPAddress  string
	FQDN       string
}

func Process_fqdn() {
	fqdns := []FQDN{}

	// 1. Fetch records from the view
	// Signature updated to match your new go (no db parameter).
	interfacesQuery := "SELECT mac_address, ip_address FROM view_interfaces WHERE mac_address IS NOT NULL AND ip_address != ''"
	interface_records := Return_query(interfacesQuery)

	log.Infof("Interfaces retrieved from view_interfaces: %d", len(interface_records))

	for _, row := range interface_records {
		mac_address := row["mac_address"].(string)
		ip_address := row["ip_address"].(string)

		fqdn, _ := nslookup(ip_address)
		fqdn = strings.TrimSuffix(fqdn, ".")

		if fqdn != "" {
			fqdns = append(fqdns, FQDN{MACAddress: mac_address, IPAddress: ip_address, FQDN: fqdn})
		}
	}

	if len(fqdns) == 0 {
		log.Warn("No FQDN's were found.")
		return
	}

	// 2. Clear the table
	// ClickHouse supports TRUNCATE TABLE.
	Truncate_table("fqdn_table")

	// 3. Build Bulk Insert
	// Replaced manual batching/transactions with a single high-performance string.
	sqlStr := "INSERT INTO fqdn_table (mac_address, ip_address, fqdn) VALUES "
	var valueRows []string

	for _, item := range fqdns {
		// Escape strings for ClickHouse safety
		safeMAC := strings.ReplaceAll(item.MACAddress, "'", "''")
		safeIP := strings.ReplaceAll(item.IPAddress, "'", "''")
		safeFQDN := strings.ReplaceAll(item.FQDN, "'", "''")

		row := fmt.Sprintf("('%s', '%s', '%s')", safeMAC, safeIP, safeFQDN)
		valueRows = append(valueRows, row)
	}

	finalQuery := sqlStr + strings.Join(valueRows, ",")

	// 4. Execute via helper
	err := Execute_query(finalQuery)
	if err != nil {
		log.Errorf("Failed to execute bulk insert into fqdn_table: %v", err)
		return
	}

	log.Infof("Inserted %d records into fqdn_table", len(fqdns))
}

// Process_fqdn_by_switch performs reverse lookups for a specific switch.
func Process_fqdn_by_switch(switch_id string) {
	fqdns := []FQDN{}

	// 1. Fetch records for the specific switch
	interfacesQuery := fmt.Sprintf("SELECT mac_address, ip_address FROM view_interfaces WHERE switch_id = '%s' AND mac_address IS NOT NULL AND ip_address != ''",switch_id)
	interface_records := Return_query(interfacesQuery)

	for _, row := range interface_records {
		mac_address := row["mac_address"].(string)
		ip_address := row["ip_address"].(string)

		fqdn, _ := nslookup(ip_address)
		fqdn = strings.TrimSuffix(fqdn, ".")

		if fqdn != "" {
			fqdns = append(fqdns, FQDN{MACAddress: mac_address, IPAddress: ip_address, FQDN: fqdn})
		}
	}

	if len(fqdns) == 0 {
		log.Warn("No FQDN's were found.")
		return
	}

	// 2. Build Bulk Insert
	sqlStr := "INSERT INTO fqdn_table (mac_address, ip_address, fqdn) VALUES "
	var valueRows []string

	for _, item := range fqdns {
		safeMAC := strings.ReplaceAll(item.MACAddress, "'", "''")
		safeIP := strings.ReplaceAll(item.IPAddress, "'", "''")
		safeFQDN := strings.ReplaceAll(item.FQDN, "'", "''")

		row := fmt.Sprintf("('%s', '%s', '%s')", safeMAC, safeIP, safeFQDN)
		valueRows = append(valueRows, row)
	}

	finalQuery := sqlStr + strings.Join(valueRows, ",")

	// 3. Execute via helper
	err := Execute_query(finalQuery)
	if err != nil {
		log.Errorf("Failed to execute bulk insert (switch %s) into fqdn_table: %v", switch_id, err)
		return
	}

	log.Infof("Switch %s :: Inserted %d records into fqdn_table", switch_id, len(fqdns))
}

func Process_switch(switch_id string, switch_hostname string) {
	log.Infof("Process Switch :: %s :: %s", switch_id, switch_hostname)

	akips_get_interface_usage(switch_id, switch_hostname)

	// SSH Connection with Safe Guard
	client, client_err := connection.Connect(switch_hostname)
	if client_err != nil {
		log.Errorf("%s :: Connection failed: %v", switch_hostname, client_err)
		return // Safely skip to next switch without stopping the loop
	}
	defer client.Close()

	commands := []string{
		"show snmp location",
		"show version",
		"show running-config",
		"show interfaces",
		"show interface status",
		"show power inline",
		"show mac address-table",
		"show lldp neighbors",
		"show cdp neighbors",
		"show vlan",
	}

	// 3. Execute Commands (Fixed error bypass)
	results, results_err := connection.ExecuteCommandsOnClient(client, switch_hostname, commands)
	if results_err != nil {
		log.Errorf("%s :: Execution error: %v", switch_hostname, results_err)
		return // Stop processing this switch if command execution completely fails
	}

	// 4. Process Results cleanly
	locationData := ""
	var show_interface_status_data []parse.InterfaceStatus
	var err error

	for _, data := range results {
		// Skip processing if an individual command produced an execution error
		if data.Error != "" {
			log.Warnf("%s :: Command '%s' failed: %s", switch_hostname, data.Command, data.Error)
			continue
		}

		switch {
		case strings.HasPrefix(data.Command, "show snmp location"):
			if strings.Contains(data.Response, "Invalid") {
				locationData = ""
				continue
			} else {
				locationData = data.Response
			}

		case strings.HasPrefix(data.Command, "show version"):
			column, err := parse.Show_version(data.Response)
			if err != nil {
				log.Errorf("%s :: parse.Show_version error: %v", switch_hostname, err)
				continue
			}
			database_show_version(locationData, column, switch_id, switch_hostname)

		case strings.HasPrefix(data.Command, "show running-config"):
			column_data, err := parse.Show_running_config(data.Response)
			if err != nil {
				log.Errorf("%s :: parse.Show_running_config error: %v", switch_hostname, err)
				continue
			}
			database_show_running_config(column_data, switch_id, switch_hostname)

		case strings.HasPrefix(data.Command, "show interfaces") && !strings.Contains(data.Command, "status"):
			column_data, err := parse.Show_interfaces(data.Response)
			if err != nil {
				log.Infof("%s :: parse.Show_interfaces info: %v", switch_hostname, err)
				continue
			}
			database_show_interfaces(column_data, switch_id, switch_hostname)

		case strings.HasPrefix(data.Command, "show interface status"):
			show_interface_status_data, err = parse.Show_interfaces_status(data.Response)
			if err != nil {
				log.Errorf("%s :: parse.Show_interfaces_status error: %v", switch_hostname, err)
				continue
			}
			database_interfaces_status(show_interface_status_data, switch_id, switch_hostname)

		case strings.HasPrefix(data.Command, "show power inline"):
			power_inline_modules_data, power_inline_interfaces_data, err := parse.Show_power_inline(data.Response)
			if err != nil {
				log.Errorf("%s :: parse.Show_power_inline error: %v", switch_hostname, err)
				continue
			}
			database_power_inline(power_inline_modules_data, power_inline_interfaces_data, switch_id, switch_hostname)

		case strings.HasPrefix(data.Command, "show mac address-table"):
			column_data, err := parse.Show_mac_address_table(data.Response)
			if err != nil {
				log.Infof("%s :: parse.Show_mac_address_table info: %v", switch_hostname, err)
				continue
			}
			database_show_mac_address_table(column_data, show_interface_status_data, switch_id, switch_hostname)

		case strings.HasPrefix(data.Command, "show lldp neighbors"):
			column_data, err := parse.Show_lldp_neighbors(data.Response)
			if err != nil {
				log.Errorf("%s :: parse.Show_lldp_neighbors error: %v", switch_hostname, err)
				continue
			}
			database_show_lldp_neighbors(column_data, switch_id, switch_hostname)

		case strings.HasPrefix(data.Command, "show cdp neighbors"):
			column_data, err := parse.Show_cdp_neighbors(data.Response)
			if err != nil {
				log.Errorf("%s :: parse.Show_cdp_neighbors error: %v", switch_hostname, err)
				continue
			}
			database_show_cdp_neighbors(column_data, switch_id, switch_hostname)

		case strings.HasPrefix(data.Command, "show vlan"):
			column_data, err := parse.Show_vlan(data.Response)
			if err != nil {
				log.Errorf("%s :: parse.Show_vlan error: %v", switch_hostname, err)
				continue
			}
			database_show_vlan(column_data, switch_id, switch_hostname)
		}
	}
}

func Process_history() {
	Delete_today("interfaces_history")
	Delete_today("cdp_neighbors_history")
	Delete_today("lldp_neighbors_history")
	Delete_today("vlans_history")

	// Insert interfaces history
	log.Info("Inserting interfaces history...")
	Execute_query("INSERT INTO interfaces_history SELECT * FROM view_interfaces")

	// Insert cdp neighbors history
	log.Info("Inserting cdp neighbors history...")
	Execute_query("INSERT INTO cdp_neighbors_history SELECT * FROM cdp_neighbors")

	// Insert lldp neighbors history
	log.Info("Inserting lldp neighbors history...")
	Execute_query("INSERT INTO lldp_neighbors_history SELECT * FROM lldp_neighbors")

	// Insert vlans history
	log.Info("Inserting vlans history...")
	Execute_query("INSERT INTO vlans_history SELECT * FROM vlans")
}

func Process_vendors() {
	// Generate Vendors table
	log.Info("Processing Vendor records...")
	err := get_mac_vendor_table()
	if err != nil {
		log.Errorf("Error fetching vendor table: %v", err)
	}
}
