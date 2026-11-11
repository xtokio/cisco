package database

import (
	"charm.land/log/v2"
	"net/http"
	"strings"
	"bufio"
	"fmt"
)

// MacVendor is a struct to hold the parsed data,
// similar to the Python dictionary.
type MacVendor struct {
	MacAddress string
	Vendor     string
}

// get_mac_vendor_table fetches and parses the IEEE OUI text file and stores it in ClickHouse.
func get_mac_vendor_table() error {
	const url = "https://standards-oui.ieee.org/oui/oui.txt"
	const userAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"

	// 1. Fetch the OUI file
	client := &http.Client{}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", userAgent)

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("received non-200 status code: %s", resp.Status)
	}

	// 2. Parse the content
	var macVendorList []MacVendor
	scanner := bufio.NewScanner(resp.Body)

	for scanner.Scan() {
		line := scanner.Text()

		if strings.TrimSpace(line) == "" || !strings.Contains(line, "(base 16)") {
			continue
		}

		parts := strings.Split(line, "(base 16)")
		if len(parts) < 2 {
			continue
		}

		macAddress := strings.ToLower(strings.TrimSpace(parts[0]))
		vendorName := strings.TrimSpace(parts[1])

		macVendorList = append(macVendorList, MacVendor{
			MacAddress: macAddress,
			Vendor:     vendorName,
		})
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading response body: %w", err)
	}

	if len(macVendorList) == 0 {
		log.Info("Get Vendors :: Warning: Parsing completed, but no records were found.")
		return nil
	}

	// 3. Clear existing table
	// ClickHouse handles TRUNCATE efficiently.
	Execute_query("TRUNCATE TABLE vendors")

	// 4. Build Bulk Insert String
	// Instead of manual batching, ClickHouse is optimized for very large single INSERT statements.
	sqlStr := "INSERT INTO vendors (mac_address, vendor) VALUES "
	var valueRows []string

	for _, v := range macVendorList {
		// Escape single quotes for SQL safety
		safeMAC := strings.ReplaceAll(v.MacAddress, "'", "''")
		safeVendor := strings.ReplaceAll(v.Vendor, "'", "''")

		row := fmt.Sprintf("('%s', '%s')", safeMAC, safeVendor)
		valueRows = append(valueRows, row)
	}

	finalQuery := sqlStr + strings.Join(valueRows, ",")

	// 5. Execute via database helper
	// Execute_query manages the connection lifecycle internally.
	err = Execute_query(finalQuery)
	if err != nil {
		log.Errorf("Failed to execute bulk insert for Vendors: %v", err)
		return err
	}

	log.Infof("Get vendors :: %d records inserted.", len(macVendorList))

	return nil
}
