package akips

import (
	"charm.land/log/v2"
	"crypto/tls"
	"net/http"
	"strconv"
	"strings"
	"time"
	"fmt"
	"net"
	"io"
	"os"
)

type SwitchList struct {
	Switch string
	IP     string
}

type InterfaceUsage struct {
	Interface   string
	Status      string
	Last_change string
	Days        string
	Hours       string
	Minutes     string
}

func get_endpoint(api_type, endpoint string) []string {
	// 1. Create a custom HTTP client that skips SSL verification
	//    This is the Go equivalent of `verify=False`
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{
		Transport: tr,
		Timeout:   30 * time.Second, // Set a 30-second timeout
	}

	var akips_url = os.Getenv("AKIPS_URL")
	var akips_password = os.Getenv("AKIPS_PASSWORD")
	url := akips_url + "/" + api_type + "?password=" + akips_password + ";" + endpoint

	// Make the HTTP GET request
	resp, err := client.Get(url)
	if err != nil {
		log.Errorf("Error fetching URL: %v", err)
	}
	// Make sure to close the response body
	defer resp.Body.Close()

	// Check if the request was successful (HTTP status code 200)
	if resp.StatusCode != http.StatusOK {
		log.Warnf("Error: received non-200 status code: %d", resp.StatusCode)
	}

	// Read the entire response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Errorf("Error reading response body: %v", err)
	}

	// Convert the byte slice to a string
	bodyString := string(body)

	// Split the string into a slice of strings by the newline character
	// This creates the list you wanted.
	// strings.TrimSpace removes leading/trailing whitespace which is good practice.
	lines := strings.Split(strings.TrimSpace(bodyString), "\n")

	return lines
}

// getIPFromFQDN resolves a hostname to a single IP address.
// It prefers IPv4 but will return an IPv6 address if no IPv4 is found.
// It returns the IP as a string and an error if lookup fails.
func getIPFromFQDN(host string) (string, error) {
	ips, err := net.LookupIP(host)
	if err != nil {
		// Return an empty string and the error
		return "", fmt.Errorf("DNS lookup failed for %s: %w", host, err)
	}

	// Loop through IPs to find the first IPv4
	for _, ip := range ips {
		if ip.To4() != nil {
			return ip.String(), nil // Found IPv4, return it
		}
	}

	// If no IPv4 was found, just return the first IP in the list (likely IPv6)
	if len(ips) > 0 {
		return ips[0].String(), nil
	}

	// This case is unlikely if err was nil, but good to have
	return "", fmt.Errorf("no IPs found for %s", host)
}

func Switch_list() []SwitchList {
	api_type := "api-db"
	api_endpoint := "cmds=mlist+device+/.*hub.nd.edu/"

	response := get_endpoint(api_type, api_endpoint)

	var switch_list []SwitchList

	// Iterate through each line from the URL list
	for _, line := range response {
		single_switch := SwitchList{}
		single_switch.Switch = line
		single_switch.IP, _ = getIPFromFQDN(line)
		switch_list = append(switch_list, single_switch)
	}

	return switch_list
}

func Interface_usage(switch_hostname string) []InterfaceUsage {
	api_type := "api-db"
	api_endpoint := "cmds=mget%20*%20" + switch_hostname + "%20*%20IF-MIB.ifOperStatus"

	response := get_endpoint(api_type, api_endpoint)

	var interface_usage []InterfaceUsage

	for _, line := range response {
		interface_usage_single := parseAndCalculateDuration(line)
		interface_usage = append(interface_usage, interface_usage_single)
	}

	return interface_usage
}

// parseAndCalculateDuration parses a single line of AKIPS output
// and prints the formatted duration of its state.
func parseAndCalculateDuration(line string) InterfaceUsage {
	interface_usage := InterfaceUsage{}

	// Skip empty lines or "ok:" messages
	if line == "" || strings.HasPrefix(line, "ok:") {
		return InterfaceUsage{}
	}

	// 1. Split the line into key and value at " = "
	//    Example: "dusc047-a... Gi0/0 IF-MIB.ifOperStatus = 2,down,1626408520,1626408520,"
	parts := strings.SplitN(line, " = ", 2)
	if len(parts) != 2 {
		// fmt.Printf("Skipping malformed line (no ' = '): %s\n", line)
		return InterfaceUsage{}
	}
	key := parts[0]
	valueString := parts[1]

	// 2. Parse the key to get the interface name
	//    keyParts = ["dusc047-a.hub.nd.edu", "Gi0/0", "IF-MIB.ifOperStatus"]
	keyParts := strings.Fields(key) // strings.Fields handles whitespace
	if len(keyParts) < 2 {
		// fmt.Printf("Skipping malformed key: %s\n", key)
		return InterfaceUsage{}
	}
	interfaceName := keyParts[1]

	// 3. Parse the value string to get status and timestamp
	//    valueParts = ["2", "down", "1626408520", "1626408520", ""]
	valueParts := strings.Split(valueString, ",")
	if len(valueParts) < 4 {
		// fmt.Printf("Skipping malformed value: %s\n", valueString)
		return InterfaceUsage{}
	}

	status := valueParts[1]
	// The last change timestamp is the 4th item (index 3)
	lastChangeTimestampStr := valueParts[3]

	// 4. Convert timestamp string to an integer
	lastChangeTimestamp, err := strconv.ParseInt(lastChangeTimestampStr, 10, 64)
	if err != nil {
		fmt.Printf("Skipping line with invalid timestamp: %s\n", line)
		return InterfaceUsage{}
	}

	// 5. Convert Unix timestamp to time.Time object and format it // NEW/CHANGED
	lastChangeTime := time.Unix(lastChangeTimestamp, 0)
	formattedTime := lastChangeTime.Format("2006-01-02 15:04:05") // Go's specific layout string

	// 6. Get the current time as a Unix timestamp
	currentTimestamp := time.Now().Unix()

	// 7. Calculate the duration in seconds
	durationSeconds := currentTimestamp - lastChangeTimestamp

	if durationSeconds < 0 {
		// Also add the formatted time to the error message
		fmt.Printf("%-15s is %-4s (Timestamp is in the future: %s)\n", interfaceName, status, formattedTime) // NEW/CHANGED
		return InterfaceUsage{}
	}

	// 8. Convert seconds to days, hours, and minutes
	days := durationSeconds / 86400 // 60*60*24

	remainder := durationSeconds % 86400

	hours := remainder / 3600 // 60*60

	remainder = remainder % 3600

	minutes := remainder / 60

	interface_usage.Interface = interfaceName
	interface_usage.Status = status
	interface_usage.Last_change = formattedTime
	interface_usage.Days = strconv.FormatInt(days, 10)
	interface_usage.Hours = strconv.FormatInt(hours, 10)
	interface_usage.Minutes = strconv.FormatInt(minutes, 10)

	return interface_usage
}
