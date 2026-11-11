package database

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"charm.land/log/v2"
	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/jmoiron/sqlx"
)

var (
	chDB *sqlx.DB
	once sync.Once
)

// GetDB returns a thread-safe, global singleton connection pool
func GetDB() *sqlx.DB {
	once.Do(func() {
		loc, err := time.LoadLocation("America/Indiana/Indianapolis")
		if err == nil {
			time.Local = loc
		}

		chServer := os.Getenv("CLICKHOUSE_SERVER")
		chPort := os.Getenv("CLICKHOUSE_PORT")
		chUser := os.Getenv("CLICKHOUSE_USERNAME")
		chPass := os.Getenv("CLICKHOUSE_PASSWORD")

		conn := clickhouse.OpenDB(&clickhouse.Options{
			Addr: []string{chServer + ":" + chPort},
			Auth: clickhouse.Auth{
				Database: "switches",
				Username: chUser,
				Password: chPass,
			},
			DialTimeout: 10 * time.Second,
			// Enable MaxOpenConns to throttle concurrent TCP socket creation
			MaxOpenConns: 20,
			MaxIdleConns: 10,
			ConnMaxLifetime: 10 * time.Minute,
			Settings: clickhouse.Settings{
        "max_execution_time": 15, // Cancel queries exceeding 15 seconds on the CH engine
    	},
		})

		chDB = sqlx.NewDb(conn, "clickhouse")
	})

	return chDB
}

func Truncate_table(table_name string) {
	Execute_query("TRUNCATE TABLE IF EXISTS " + table_name)
}

func Delete_today(table string) {
	today := time.Now().Format("2006-01-02")
	sqlQuery := fmt.Sprintf("ALTER TABLE %s DROP PARTITION '%s'", table, today)

	db := GetDB()

	// Give partition drops 60 seconds to clear background locks
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	_, err := db.ExecContext(ctx, sqlQuery)
	if err != nil {
		// Log warning but allow startup to continue if partition didn't exist yet
		log.Warnf("Drop partition warning for %s: %v", table, err)
	}
}

func Truncate_tables() {
	log.Info("Truncating tables...")
	Truncate_table("interfaces")
	Truncate_table("interfaces_status")
	Truncate_table("power_modules")
	Truncate_table("power_interfaces")
	Truncate_table("vlans")
	Truncate_table("cdp_neighbors")
	Truncate_table("lldp_neighbors")
	Truncate_table("mac_address_table")
	Truncate_table("show_running_config")
	Truncate_table("akips_interface_usage")
	Truncate_table("vendors")
}

func Drop_table_partition() {
	log.Info("Dropping table partition...")
	Delete_today("interfaces")
	Delete_today("interfaces_status")
	Delete_today("power_modules")
	Delete_today("power_interfaces")
	Delete_today("vlans")
	Delete_today("cdp_neighbors")
	Delete_today("lldp_neighbors")
	Delete_today("mac_address_table")
	Delete_today("show_running_config")
	Delete_today("akips_interface_usage")
	Delete_today("arp_table")
	// Delete_today("vendors")
}

func Return_query(query string) []map[string]any {
	db := GetDB()

	// Add 60s Context Timeout so queries never hang indefinitely
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	rows, err := db.QueryxContext(ctx, query)
	if err != nil {
		log.Errorf("Return_query error: %v | Query: %s", err, query)
		return nil
	}
	defer rows.Close()

	var results []map[string]any
	for rows.Next() {
		m := make(map[string]any)
		if err := rows.MapScan(m); err != nil {
			log.Errorf("MapScan error: %v", err)
			return nil
		}
		results = append(results, m)
	}

	return results
}

// Execute_query reuses the global ClickHouse pool with safety timeouts
func Execute_query(query string) error {
	db := GetDB()

	// 15-second timeout per query prevents hanging goroutines
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_, err := db.ExecContext(ctx, query)
	if err != nil {
		log.Errorf("Error executing query: %v | Query: %s", err, query)
		return err
	}

	return nil
}
