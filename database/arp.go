package database

import (
	"encoding/xml"
	"crypto/tls"
	"net/http"
	"strings"
	"sync"
	"time"
	"fmt"
	"io"
	"os"

	"charm.land/log/v2"
	"cisco/connection"
)

// ArpInfo defines the structure for a single ARP entry.
type ArpInfo struct {
	Protocol   string
	Address    string
	Age        string
	MacAddress string
	Type       string
	Interface  string
}

// Show_ip_arp runs 'show ip arp' on the switch, parses the output, and stores it in ClickHouse.
func show_ip_arp(switch_hostname string) error {
	outputString, err := connection.RunCommand(switch_hostname, "show ip arp")
	if err != nil {
		return err
	}

	arpList, err := parseIpArpInfo(outputString)
	if err != nil {
		log.Errorf("%s :: Show IP ARP :: Error during parsing: %v", switch_hostname, err)
		return err
	}

	if len(arpList) == 0 {
		log.Warnf("Show IP ARP :: Warning: Parsing completed for %s, but no arp entries were found.", switch_hostname)
		return nil
	}

	// 1. ClickHouse Mutation: Delete records from today
	// Replaced standard DELETE with ALTER TABLE ... DELETE and toDate(created_at) = today()
	safeHostname := strings.ReplaceAll(switch_hostname, "'", "''")

	// ClickHouse performs best when receiving all rows in a single VALUES block.
	sqlStr := "INSERT INTO arp_table (source, mac_address, ip_address, protocol, age, type, interface) VALUES "
	var valueRows []string

	for _, details := range arpList {
		// Escape strings for ClickHouse safety
		safeMac := strings.ReplaceAll(details.MacAddress, "'", "''")
		safeAddr := strings.ReplaceAll(details.Address, "'", "''")
		safeProto := strings.ReplaceAll(details.Protocol, "'", "''")
		safeAge := strings.ReplaceAll(details.Age, "'", "''")
		safeType := strings.ReplaceAll(details.Type, "'", "''")
		safeIface := strings.ReplaceAll(details.Interface, "'", "''")

		row := fmt.Sprintf("('%s', '%s', '%s', '%s', '%s', '%s', '%s')",
			safeHostname,
			safeMac,
			safeAddr,
			safeProto,
			safeAge,
			safeType,
			safeIface,
		)
		valueRows = append(valueRows, row)
	}

	finalQuery := sqlStr + strings.Join(valueRows, ",")

	// 3. Execute via database helper
	err = Execute_query(finalQuery)
	if err != nil {
		log.Errorf("Failed to execute bulk insert for %s: %v", switch_hostname, err)
		return err
	}

	log.Infof("%s :: Show IP ARP :: %d records inserted.", switch_hostname, len(arpList))
	return nil
}

// Show_ip_arp_vrf runs 'show ip arp vrf' on the switch, parses the output, and stores it in ClickHouse.
func show_ip_arp_vrf(switch_hostname string, vrf_name string) error {
	outputString, err := connection.RunCommand(switch_hostname, "sh ip arp vrf "+vrf_name)
	if err != nil {
		return err
	}

	arpList, err := parseIpArpInfo(outputString)
	if err != nil {
		log.Errorf("%s :: Show IP ARP VRF %s :: Error during parsing: %v", switch_hostname, vrf_name, err)
	}

	if len(arpList) == 0 {
		log.Warnf("Show IP ARP VRF %s :: Warning: Parsing completed for %s, but no arp entries were found.", vrf_name, switch_hostname)
		return nil
	}

	source_name := switch_hostname + "_vrf_" + vrf_name
	safeSource := strings.ReplaceAll(source_name, "'", "''")

	sqlStr := "INSERT INTO arp_table (source, mac_address, ip_address, protocol, age, type, interface) VALUES "
	var valueRows []string

	for _, details := range arpList {
		safeMac := strings.ReplaceAll(details.MacAddress, "'", "''")
		safeAddr := strings.ReplaceAll(details.Address, "'", "''")
		safeProto := strings.ReplaceAll(details.Protocol, "'", "''")
		safeAge := strings.ReplaceAll(details.Age, "'", "''")
		safeType := strings.ReplaceAll(details.Type, "'", "''")
		safeIface := strings.ReplaceAll(details.Interface, "'", "''")

		row := fmt.Sprintf("('%s', '%s', '%s', '%s', '%s', '%s', '%s')",
			safeSource,
			safeMac,
			safeAddr,
			safeProto,
			safeAge,
			safeType,
			safeIface,
		)
		valueRows = append(valueRows, row)
	}

	finalQuery := sqlStr + strings.Join(valueRows, ",")

	// 3. Execute bulk insert
	err = Execute_query(finalQuery)
	if err != nil {
		log.Errorf("Failed to execute bulk insert for %s: %v", source_name, err)
		return err
	}

	log.Infof("%s :: Show IP ARP :: %d records inserted.", source_name, len(arpList))
	return nil
}

// parseIpArpInfo processes the raw CLI output from "show ip arp".
// It skips lines with "Incomplete" MAC addresses.
func parseIpArpInfo(rawOutput string) ([]ArpInfo, error) {
	var arps []ArpInfo
	lines := strings.Split(rawOutput, "\n")

	dataStartIndex := -1
	// Find the start of the data, which is 1 line after the header "Protocol  Address..."
	for i, line := range lines {
		if strings.HasPrefix(line, "Protocol  Address") {
			dataStartIndex = i + 1 // Data starts on the next line
			break
		}
	}

	if dataStartIndex == -1 {
		return nil, fmt.Errorf("could not find ARP header in output")
	}

	for i := dataStartIndex; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}

		fields := strings.Fields(line)

		// A valid line must have at least 5 fields (Proto, Addr, Age, MAC, Type)
		if len(fields) < 5 {
			continue
		}

		// Requirement: Skip lines with "Incomplete"
		// The MAC address is the 4th field (index 3)
		if fields[3] == "Incomplete" {
			continue
		}

		// Create the entry
		arp := ArpInfo{
			Protocol:   fields[0],
			Address:    fields[1],
			Age:        fields[2],
			MacAddress: fields[3],
			Type:       fields[4],
			Interface:  "", // Default to empty string
		}

		// If the 6th field (Interface) exists, add it
		if len(fields) == 6 {
			arp.Interface = fields[5]
		}

		arps = append(arps, arp)
	}

	return arps, nil
}

// ArpInfoRome defines the structure for a single ARP entry.
type ArpInfoRome struct {
	Protocol   string
	Address    string
	Age        string
	MacAddress string
	Type       string
	Interface  string
}

// Show_ip_arp_rome runs 'show ip arp' on the switch using credentials and stores it in ClickHouse.
func show_ip_arp_rome(switch_hostname, username, password string) error {
	outputString, err := connection.RunCommandWithCredentials(switch_hostname, username, password, "show ip arp")
	if err != nil {
		return err
	}

	// Parse the output using the existing parser
	arpList, err := parseIpArpInfoRome(outputString)
	if err != nil {
		log.Errorf("%s :: Show IP ARP :: Error during parsing: %v", switch_hostname, err)
		return err
	}

	if len(arpList) == 0 {
		log.Warnf("Show IP ARP :: Warning: Parsing completed for %s, but no arp entries were found.", switch_hostname)
		return nil
	}

	safeHostname := strings.ReplaceAll(switch_hostname, "'", "''")

	// ClickHouse performs best with a single large INSERT containing all rows.
	sqlStr := "INSERT INTO arp_table (source, mac_address, ip_address, protocol, age, type, interface) VALUES "
	var valueRows []string

	for _, details := range arpList {
		// Escape single quotes for SQL safety
		safeMac := strings.ReplaceAll(details.MacAddress, "'", "''")
		safeAddr := strings.ReplaceAll(details.Address, "'", "''")
		safeProto := strings.ReplaceAll(details.Protocol, "'", "''")
		safeAge := strings.ReplaceAll(details.Age, "'", "''")
		safeType := strings.ReplaceAll(details.Type, "'", "''")
		safeIface := strings.ReplaceAll(details.Interface, "'", "''")

		// Construct the row string
		row := fmt.Sprintf("('%s', '%s', '%s', '%s', '%s', '%s', '%s')",
			safeHostname,
			safeMac,
			safeAddr,
			safeProto,
			safeAge,
			safeType,
			safeIface,
		)
		valueRows = append(valueRows, row)
	}

	finalQuery := sqlStr + strings.Join(valueRows, ",")

	// 3. Execute via database helper
	err = Execute_query(finalQuery)
	if err != nil {
		log.Errorf("Failed to execute bulk insert for %s: %v", switch_hostname, err)
		return err
	}

	log.Infof("%s :: Show IP ARP :: %d records inserted.", switch_hostname, len(arpList))
	return nil
}

// parseIpArpInfoRome processes the raw CLI output from "show ip arp".
// It skips lines with "Incomplete" MAC addresses.
func parseIpArpInfoRome(rawOutput string) ([]ArpInfo, error) {
	var arps []ArpInfo
	lines := strings.Split(rawOutput, "\n")

	dataStartIndex := -1
	// Find the start of the data, which is 1 line after the header "Protocol  Address..."
	for i, line := range lines {
		if strings.HasPrefix(line, "Protocol  Address") {
			dataStartIndex = i + 1 // Data starts on the next line
			break
		}
	}

	if dataStartIndex == -1 {
		return nil, fmt.Errorf("could not find ARP header in output")
	}

	for i := dataStartIndex; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}

		fields := strings.Fields(line)

		// A valid line must have at least 5 fields (Proto, Addr, Age, MAC, Type)
		if len(fields) < 5 {
			continue
		}

		// Requirement: Skip lines with "Incomplete"
		// The MAC address is the 4th field (index 3)
		if fields[3] == "Incomplete" {
			continue
		}

		// Create the entry
		arp := ArpInfo{
			Protocol:   fields[0],
			Address:    fields[1],
			Age:        fields[2],
			MacAddress: fields[3],
			Type:       fields[4],
			Interface:  "", // Default to empty string
		}

		// If the 6th field (Interface) exists, add it
		if len(fields) == 6 {
			arp.Interface = fields[5]
		}

		arps = append(arps, arp)
	}

	return arps, nil
}

// --- Structs for XML Parsing ---

// TokenResponse maps to the XML from the keygen API
type TokenResponse struct {
	XMLName xml.Name `xml:"response"`
	Result  struct {
		Key string `xml:"key"`
	} `xml:"result"`
}

// ArpResponseXML maps to the XML from the 'show arp' command
type ArpResponseXML struct {
	XMLName xml.Name `xml:"response"`
	Result  struct {
		Entries struct {
			EntryList []ArpEntryXML `xml:"entry"`
		} `xml:"entries"`
	} `xml:"result"`
}

// ArpEntryXML maps to a single <entry> tag
type ArpEntryXML struct {
	Interface string `xml:"interface"`
	IP        string `xml:"ip"`
	MAC       string `xml:"mac"`
	Port      string `xml:"port"`
	Status    string `xml:"status"`
	TTL       string `xml:"ttl"`
}

// --- Internal Struct ---

// ArpRecord holds the cleaned data, similar to the Python dictionary
type ArpRecord struct {
	Interface string
	IP        string
	MAC       string
	Port      string
	Status    string
	TTL       string
}

// --- Helper Functions ---

// formatMacAddress converts "aa:bb:cc:dd:ee:ff" to "aabb.ccdd.eeff"
func formatMacAddress(macString string) string {
	cleanMac := strings.ReplaceAll(macString, ":", "")
	if len(cleanMac) != 12 {
		return macString // Return original if not a standard 12-char MAC
	}
	return fmt.Sprintf("%s.%s.%s", cleanMac[0:4], cleanMac[4:8], cleanMac[8:12])
}

// --- API Functions ---

// getToken fetches the API token from the Palo Alto device
func getToken(client *http.Client, fqdn, user, pass string) (string, error) {
	url := fmt.Sprintf("https://%s/api/?type=keygen&user=%s&password=%s", fqdn, user, pass)

	resp, err := client.Get(url)
	if err != nil {
		return "", fmt.Errorf("failed to get token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("bad status code %d from token API", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read token response body: %w", err)
	}

	var tokenResponse TokenResponse
	if err := xml.Unmarshal(body, &tokenResponse); err != nil {
		return "", fmt.Errorf("failed to parse token XML: %w", err)
	}

	if tokenResponse.Result.Key == "" {
		return "", fmt.Errorf("received empty token")
	}

	return tokenResponse.Result.Key, nil
}

// getArpRecords fetches and parses the ARP table from the device
func getArpRecords(client *http.Client, fqdn, token string) ([]ArpRecord, error) {
	url := fmt.Sprintf("https://%s/api/?type=op&cmd=<show><arp><entry+name='all'/></arp></show>&key=%s", fqdn, token)

	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to get ARP records: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bad status code %d from ARP API", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read ARP response body: %w", err)
	}

	var arpResponse ArpResponseXML
	if err := xml.Unmarshal(body, &arpResponse); err != nil {
		return nil, fmt.Errorf("failed to parse ARP XML: %w", err)
	}

	// Convert from XML struct to internal struct, applying formatting
	records := make([]ArpRecord, 0, len(arpResponse.Result.Entries.EntryList))
	for _, entry := range arpResponse.Result.Entries.EntryList {
		records = append(records, ArpRecord{
			Interface: entry.Interface,
			IP:        entry.IP,
			MAC:       formatMacAddress(entry.MAC),
			Port:      entry.Port,
			Status:    strings.TrimSpace(entry.Status), // Match Python's .strip()
			TTL:       entry.TTL,
		})
	}

	return records, nil
}

// --- Database Functions ---
// bulkInsertArpRecords inserts new records in a single ClickHouse bulk operation.
func bulkInsertArpRecords(fqdn string, records []ArpRecord) (int64, error) {
	if len(records) == 0 {
		return 0, nil
	}

	// ClickHouse performs best with a single large INSERT containing all rows.
	sqlStr := "INSERT INTO arp_table (source, mac_address, ip_address, protocol, age, type, interface) VALUES "
	var valueRows []string

	for _, r := range records {
		// Escape single quotes for SQL safety
		safeSource := strings.ReplaceAll(fqdn, "'", "''")
		safeMac := strings.ReplaceAll(r.MAC, "'", "''")
		safeIP := strings.ReplaceAll(r.IP, "'", "''")
		safeTTL := strings.ReplaceAll(r.TTL, "'", "''")
		safeStatus := strings.ReplaceAll(r.Status, "'", "''")
		safeInterface := strings.ReplaceAll(r.Interface, "'", "''")

		// Construct the row string
		row := fmt.Sprintf("('%s', '%s', '%s', 'ARP', '%s', '%s', '%s')",
			safeSource,
			safeMac,
			safeIP,
			safeTTL,
			safeStatus,
			safeInterface,
		)
		valueRows = append(valueRows, row)
	}

	finalQuery := sqlStr + strings.Join(valueRows, ",")

	// 3. Execute the bulk insert via your global helper
	// This replaces the Begin/Prepare/Commit logic used in MySQL.
	err := Execute_query(finalQuery)
	if err != nil {
		return 0, fmt.Errorf("failed to execute bulk insert for %s: %w", fqdn, err)
	}

	return int64(len(records)), nil
}

// --- Main Execution ---

// processFirewall handles the full logic for a single firewall
// This function is designed to be run as a goroutine
func processFirewall(wg *sync.WaitGroup, client *http.Client, fqdn, user, pass string) {
	defer wg.Done() // Tell the WaitGroup we are finished when this function exits

	log.Infof("[%s] Getting token...", fqdn)
	token, err := getToken(client, fqdn, user, pass)
	if err != nil {
		log.Errorf("[%s] ERROR: %v", fqdn, err)
		return
	}

	log.Infof("[%s] Token acquired. Getting ARP records...", fqdn)
	arpRecords, err := getArpRecords(client, fqdn, token)
	if err != nil {
		log.Errorf("[%s] ERROR: %v", fqdn, err)
		return
	}
	log.Infof("[%s] Found %d records.", fqdn, len(arpRecords))

	if len(arpRecords) > 0 {
		log.Infof("[%s] Starting bulk insert into ClickHouse...", fqdn)
		insertedCount, err := bulkInsertArpRecords(fqdn, arpRecords)
		if err != nil {
			log.Errorf("[%s] ERROR: %v", fqdn, err)
			return
		}
		log.Infof("[%s] Successfully inserted %d records.", fqdn, insertedCount)
	}
}

func palo_alto_arp_records() {
	// Get the start time
	processStartTime := time.Now()

	paloUser := os.Getenv("PALO_ALTO_API_USERNAME")
	paloPass := os.Getenv("PALO_ALTO_API_PASSWORD")

	// Create a single, reusable HTTP client
	// This replicates `urllib3.disable_warnings()`
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	fqdnRecords := []string{"hesb-como-pa-a.fw.nd.edu", "res-keo-pa-a.fw.nd.edu", "shub-stad-pa-a.fw.nd.edu"}

	// A WaitGroup is used to wait for all concurrent goroutines to finish
	var wg sync.WaitGroup

	for _, fqdn := range fqdnRecords {
		wg.Add(1) // Increment the WaitGroup counter
		// Start a new goroutine for each firewall
		go processFirewall(&wg, client, fqdn, paloUser, paloPass)
	}

	// Wait for all goroutines (launched with wg.Add(1)) to call wg.Done()
	wg.Wait()

	processDuration := time.Since(processStartTime)
	log.Infof("Finished palo_alto_arp_records() process duration: %s", processDuration)
}
