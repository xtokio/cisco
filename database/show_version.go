package database

// import (
// 	"charm.land/log/v2"
// 	"fmt"
// )

// VersionInfo defines the structure for the parsed "show version" output.
// It's used as an intermediate struct within the parsing function.
type VersionInfo struct {
	Hardware      string
	Version       string
	Release       string
	SoftwareImage string
	SerialNumber  string
	Uptime        string
	Restarted     string
	ReloadReason  string
	Rommon        string
}

func database_show_version(locationData string, versionData map[string]string, switch_id string, switch_hostname string) {
	if len(locationData) < 5 {
		locationData = ""
	}

	// updateQuery := fmt.Sprintf("UPDATE switches SET `location` = '%s', `hardware` = '%s', `version` = '%s', `release` = '%s', `software_image` = '%s', `serial` = '%s', `uptime` = '%s', `restarted` = '%s', `reload_reason` = '%s', `rommon` = '%s' WHERE id = '%s'",
	// 	locationData,
	// 	versionData["Hardware"],
	// 	versionData["Version"],
	// 	versionData["Release"],
	// 	versionData["SoftwareImage"],
	// 	versionData["SerialNumber"],
	// 	versionData["Uptime"],
	// 	versionData["Restarted"],
	// 	versionData["ReloadReason"],
	// 	versionData["Rommon"],
	// 	switch_id,
	// )

	// err := Execute_query(updateQuery)
	// if err != nil {
	// 	log.Errorf("Update query failed: %v", err)
	// } else {
	// 	log.Infof("%s :: %s :: Show Version :: UPDATE successful.", switch_id, switch_hostname)
	// }
}
