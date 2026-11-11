package database

import (
	"fmt"
	"time"
	"strings"
	"cisco/parse"
	"charm.land/log/v2"
)

func database_show_interfaces(interfacesSlice []parse.InterfaceDetails, switch_id string, switch_hostname string) {
	// Check if data was returned
	if len(interfacesSlice) == 0 {
		log.Warnf("Show Interface :: Warning: Parsing completed for %s, but no interfaces were found.", switch_hostname)
		return
	}

	// Load Indiana Time
	loc, err := time.LoadLocation("America/Indiana/Indianapolis")
	if err != nil {
		fmt.Println("Could not load Indiana timezone: ", err)
	}
	time.Local = loc

	// Execute_query(fmt.Sprintf("DELETE FROM interfaces WHERE toDate(created_at) = today() AND switch_id = '%s'", switch_id))

	// ClickHouse performs best with a single large INSERT containing all rows.
	sqlStr := "INSERT INTO interfaces (switch_id, interface, description, ip_address, link_status, protocol_status, hardware_type, reliability, txload, rxload, mtu, duplex, speed, media_type, bandwidth, delay, encapsulation, last_input, last_output, last_output_hang, queue_strategy, input_rate, output_rate, input_packets, output_packets, runts, giants, throttles, input_errors, output_errors, crc_errors, collisions) VALUES "

	var valueRows []string

	for _, details := range interfacesSlice {
		// Escape single quotes in all string fields to prevent SQL syntax errors
		safeIface := strings.ReplaceAll(details.Interface, "'", "''")
		safeDesc := strings.ReplaceAll(details.Description, "'", "''")
		safeIP := strings.ReplaceAll(details.IPAddress, "'", "''")
		safeLStatus := strings.ReplaceAll(details.LinkStatus, "'", "''")
		safePStatus := strings.ReplaceAll(details.ProtocolStatus, "'", "''")
		safeHW := strings.ReplaceAll(details.Hardware, "'", "''")
		safeRel := strings.ReplaceAll(details.Reliability, "'", "''")
		safeTxL := strings.ReplaceAll(details.TxLoad, "'", "''")
		safeRxL := strings.ReplaceAll(details.RxLoad, "'", "''")
		safeMtu := strings.ReplaceAll(details.Mtu, "'", "''")
		safeDuplex := strings.ReplaceAll(details.Duplex, "'", "''")
		safeSpeed := strings.ReplaceAll(details.Speed, "'", "''")
		safeMedia := strings.ReplaceAll(details.MediaType, "'", "''")
		safeBW := strings.ReplaceAll(details.Bandwidth, "'", "''")
		safeDelay := strings.ReplaceAll(details.Delay, "'", "''")
		safeEnc := strings.ReplaceAll(details.Encapsulation, "'", "''")
		safeLIn := strings.ReplaceAll(details.LastInput, "'", "''")
		safeLOut := strings.ReplaceAll(details.LastOutput, "'", "''")
		safeOHang := strings.ReplaceAll(details.OutputHang, "'", "''")
		safeQS := strings.ReplaceAll(details.QueueStrategy, "'", "''")
		safeInRate := strings.ReplaceAll(details.InputRateBps, "'", "''")
		safeOutRate := strings.ReplaceAll(details.OutputRateBps, "'", "''")
		safeInPkts := strings.ReplaceAll(details.PacketsInput, "'", "''")
		safeOutPkts := strings.ReplaceAll(details.PacketsOutput, "'", "''")
		safeRunts := strings.ReplaceAll(details.Runts, "'", "''")
		safeGiants := strings.ReplaceAll(details.Giants, "'", "''")
		safeThrot := strings.ReplaceAll(details.Throttles, "'", "''")
		safeInErr := strings.ReplaceAll(details.InputErrors, "'", "''")
		safeOutErr := strings.ReplaceAll(details.OutputErrors, "'", "''")
		safeCrc := strings.ReplaceAll(details.CrcErrors, "'", "''")
		safeColl := strings.ReplaceAll(details.Collisions, "'", "''")

		// Format the row for the VALUES list
		row := fmt.Sprintf("('%s', '%s', '%s', '%s', '%s', '%s', '%s', '%s', '%s', '%s', '%s', '%s', '%s', '%s', '%s', '%s', '%s', '%s', '%s', '%s', '%s', '%s', '%s', '%s', '%s', '%s', '%s', '%s', '%s', '%s', '%s', '%s')",
			switch_id, safeIface, safeDesc, safeIP, safeLStatus, safePStatus, safeHW, safeRel,
			safeTxL, safeRxL, safeMtu, safeDuplex, safeSpeed, safeMedia, safeBW, safeDelay,
			safeEnc, safeLIn, safeLOut, safeOHang, safeQS, safeInRate, safeOutRate, safeInPkts,
			safeOutPkts, safeRunts, safeGiants, safeThrot, safeInErr, safeOutErr, safeCrc, safeColl,
		)
		valueRows = append(valueRows, row)
	}

	// 3. Execute the bulk insert
	finalQuery := sqlStr + strings.Join(valueRows, ",")
	finalQuery = strings.ReplaceAll(finalQuery, "\n", "")

	err = Execute_query(finalQuery)
	if err != nil {
		log.Errorf("Failed to execute bulk insert for %s: %v", switch_hostname, err)
		return
	}

	log.Infof("%s :: %s :: Show Interfaces :: %d records inserted.", switch_id, switch_hostname, len(interfacesSlice))
}
