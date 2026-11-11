package parse

import (
	"strings"
)
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
