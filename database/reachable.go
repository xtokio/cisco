package database

import (
	"context"
	"fmt"
	"net"
	"os"
	"sync"
	"time"

	"charm.land/log/v2"
	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

type Switch struct {
	Switch_id   string
	Switch_hostname string
}

type SwitchResult struct {
	ID        string
	Reachable uint8
}

// checkDevice checks TCP port 22 reachability for a given host
func checkDevice(host string) uint8 {
	address := net.JoinHostPort(host, "22")

	conn, err := net.DialTimeout("tcp", address, 1*time.Second)
	if err != nil {
		return 0
	}
	conn.Close()
	return 1
}

// checkSwitchesConcurrently distributes reachability checks across worker goroutines
func checkSwitchesConcurrently(switches []Switch, workerCount int) []SwitchResult {
	jobs := make(chan Switch, len(switches))
	results := make(chan SwitchResult, len(switches))

	var wg sync.WaitGroup

	// Start worker pool
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for s := range jobs {
				results <- SwitchResult{
					ID:        s.Switch_id,
					Reachable: checkDevice(s.Switch_hostname),
				}
			}
		}()
	}

	// Feed jobs
	for _, s := range switches {
		jobs <- s
	}
	close(jobs)

	// Wait for workers to finish
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect results
	var statusList []SwitchResult
	for r := range results {
		statusList = append(statusList, r)
	}

	return statusList
}

// executeBulkUpdate streams results to a temp table on a single pinned connection
func executeBulkUpdate(ctx context.Context, conn driver.Conn, results []SwitchResult) error {
	// 1. Create temporary staging table (scoped to this specific TCP connection)
	err := conn.Exec(ctx, `
		CREATE TEMPORARY TABLE temp_switch_status (
			switch_id UUID,
			reachable UInt8
		) ENGINE = Join(ANY, LEFT, switch_id)
	`)
	if err != nil {
		return fmt.Errorf("failed to create temporary table: %w", err)
	}

	// 2. Prepare high-performance batch insert on the same connection
	batch, err := conn.PrepareBatch(ctx, "INSERT INTO temp_switch_status (switch_id, reachable)")
	if err != nil {
		return fmt.Errorf("failed to prepare batch insert: %w", err)
	}

	for _, res := range results {
		if err := batch.Append(res.ID, res.Reachable); err != nil {
			return fmt.Errorf("failed to append row to batch: %w", err)
		}
	}

	if err := batch.Send(); err != nil {
		return fmt.Errorf("failed to send batch to clickhouse: %w", err)
	}

	// 3. Execute bulk update JOIN on the same connection
	updateQuery := `
	ALTER TABLE switches
	UPDATE reachable = joinGet('temp_switch_status', 'reachable', id)
	WHERE id IN (SELECT switch_id FROM temp_switch_status)
	`
	if err := conn.Exec(ctx, updateQuery); err != nil {
		return fmt.Errorf("failed to execute lightweight update: %w", err)
	}

	return nil
}

func Reachable() {
	ctx := context.Background()

	chServer := os.Getenv("CLICKHOUSE_SERVER")
	chPort := os.Getenv("CLICKHOUSE_PORT")
	chUser := os.Getenv("CLICKHOUSE_USERNAME")
	chPass := os.Getenv("CLICKHOUSE_PASSWORD")

	// Connect to ClickHouse
	client, err := clickhouse.Open(&clickhouse.Options{
		Addr: []string{chServer + ":" + chPort},
		Auth: clickhouse.Auth{
			Database: "switches",
			Username: chUser,
			Password: chPass,
		},
	})
	if err != nil {
		log.Errorf("ClickHouse connection error: %v", err)
		return
	}
	defer client.Close()

	var switches []Switch
	err = client.Select(ctx, &switches, "SELECT id as Switch_id, switch as Switch_hostname FROM switches")
	if err != nil {
		log.Errorf("Failed to fetch switches: %v", err)
		return
	}

	if len(switches) == 0 {
		log.Warn("No switches retrieved from ClickHouse.")
		return
	}

	log.Infof("Checking reachability for %d switches...", len(switches))
	startTime := time.Now()

	workerCount := 50
	results := checkSwitchesConcurrently(switches, workerCount)

	log.Infof("Completed checks in %v. Updating ClickHouse...", time.Since(startTime))

	// Run staging update on the pinned connection
	if err := executeBulkUpdate(ctx, client, results); err != nil {
		log.Errorf("Bulk update failed: %v", err)
		return
	}

	log.Info("Bulk update completed successfully!")
}
