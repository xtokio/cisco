package database

import (
	"charm.land/log/v2"
	"strings"
	"net"
)

func nslookup(ip_address string) (string, error) {
	fqdn := ""

	names, err := net.LookupAddr(ip_address)
	if err != nil {
		log.Errorf("Error performing reverse DNS lookup for %s: %v", ip_address, err)
		return "", err
	}

	if len(names) > 0 {
		for _, name := range names {
			fqdn = name
		}
	} else {
		log.Warnf("No FQDN found for this IP address: %s", ip_address)
	}
	return fqdn, nil
}

// normalizeInterfaceName shortens interface names to a standard format.
func normalizeInterfaceName(name string) string {
	name = strings.ReplaceAll(name, " ", "")

	// Using strings.NewReplacer is the most efficient way to do multiple replacements.
	replacer := strings.NewReplacer(
		"AppGigabitEthernet", "Ap",
		"FastEthernet", "Fa",
		"GigabitEthernet", "Gi",
		"FiveGigabitEthernet", "Fi",
		"FiveGi", "Fi",
		"Fiv", "Fi",
		"TenGigabitEthernet", "Te",
		"TenGi", "Te",
		"Ten", "Te",
		"TwentyGigabitEthernet", "Twe",
		"TwentyFiveGigE", "Twe",
		"TwentyFigE", "Twe",
		"FortyGigabitEthernet", "Fo",
		"FortyGi", "Fo",
		"HundredGigE", "Hu",
		"Gig", "Gi", // In case "Gig" is used instead of "GigabitEthernet"
	)
	return replacer.Replace(name)
}
