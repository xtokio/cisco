package main

import (
	"gopkg.in/gomail.v2"
	"charm.land/log/v2"
	"cisco/database"
	"sync/atomic"
	"strings"
	"sync"
	"time"
	"fmt"
	"net"
)

func send_email(message_body string) {
	// --- Configuration ---
	from := "noreply+neteng-scripts@nd.edu"
	smtpHost := "smtp.nd.edu"
	smtpPort := 587 // gomail uses an int for the port

	to := []string{
		"lgomezre@nd.edu",
	}

	subject := "Device monitor script"

	// Your HTML content
	htmlBody := "<html>" +
		"<head></head>" +
		"<body>" +
		"<p>" + message_body + "</p>" +
		"</body>" +
		"</html>"

	// --- Create the Message ---
	message := gomail.NewMessage()

	// 2. Set headers
	message.SetHeader("From", from)
	message.SetHeader("To", to...)
	message.SetHeader("Subject", subject)

	message.SetBody("text/html", htmlBody)
	dialer := gomail.NewDialer(smtpHost, smtpPort, "", "")
	dialer.SSL = false

	// 2. Dial and send the email
	if err := dialer.DialAndSend(message); err != nil {
		log.Printf("Failed to send email: %v", err)
	}

	log.Info("Email sent successfully!")
}

func check_device(host string) int {
	port := "22"
	address := net.JoinHostPort(host, port)

	conn, err := net.DialTimeout("tcp", address, 1*time.Second)
	if err != nil {
		return 0
	}
	conn.Close()
	return 1
}

func process_switch(switch_hostname string) {
	start := time.Now()

	switch_records := database.Switches_by_switch(switch_hostname)
	total_records := len(switch_records)

	if total_records == 0 {
		log.Errorf("No switch records retrieved from database!")
		return
	}

	for _, row := range switch_records {
		switch_id, _ := row["id"].(string)
		switch_hostname, _ := row["switch"].(string)

		database.Process_switch(switch_id, switch_hostname)
	}

	log.Infof("Finished! Processed: %d | Time: %v",len(switch_records),time.Since(start))
}

func process_switches() {
	database.Drop_table_partition()
	database.Add_missing_switches()
	database.Reachable()

	start := time.Now()

	switch_records := database.Switches_reachable()
	total_records := len(switch_records)

	if total_records == 0 {
		log.Errorf("No switch records retrieved from database!")
		return
	}

	const maxConcurrency = 25
	jobs := make(chan map[string]any, total_records)

	// Feed jobs into the queue with a 5ms rate-limiting throttle to prevent SYN flooding
	go func() {
		for _, row := range switch_records {
			jobs <- row
			time.Sleep(5 * time.Millisecond) // Paces out TCP connection bursts cleanly
		}
		close(jobs)
	}()

	var wg sync.WaitGroup
	var processedCount int64
	var skippedCount int64

	log.Infof("Processing %d switches with %d continuous workers...", total_records, maxConcurrency)

	for w := 1; w <= maxConcurrency; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			for row := range jobs {
				switch_id, _ := row["id"].(string)
				switch_hostname, _ := row["switch"].(string)

				switch_hostname = strings.TrimSpace(switch_hostname)
				if switch_hostname == "" {
					atomic.AddInt64(&skippedCount, 1)
					continue
				}

				atomic.AddInt64(&processedCount, 1)

				done := make(chan struct{})
				go func(id, host string) {
					database.Process_switch(id, host)
					close(done)
				}(switch_id, switch_hostname)

				select {
				case <-done:
					// Completed
				case <-time.After(60 * time.Second):
					log.Errorf("%s :: TIMED OUT (Exceeded 60s hard worker limit)", switch_hostname)
				}
			}
		}(w)
	}
	wg.Wait()

	database.Process_vendors()
	database.Process_arp_table()
	database.Process_fqdn()
	database.Process_history()

	log.Infof("Finished! Processed: %d | Skipped/Unreachable: %d | Time: %v",atomic.LoadInt64(&processedCount),atomic.LoadInt64(&skippedCount),time.Since(start))

	message := fmt.Sprintf("Devices: %d | Process duration: %v \n",atomic.LoadInt64(&processedCount),time.Since(start))
	send_email(message)
}

func main() {
	process_switches()

	// switch_hostname := "lgomezreswitch.hub.nd.edu"
	// process_switch(switch_hostname)

}
