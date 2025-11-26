package cisco

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

// Client holds the active SSH connection
type Client struct {
	*ssh.Client
	SwitchHostname string
}

// ConnectToSwitchWithCredentials creates and returns a new Client with an active SSH session
func connectToSwitchWithCredentials(switch_hostname string, username string, password string) (*Client, error) {
	sshConfig := &ssh.ClientConfig{
		User: username,
		Auth: []ssh.AuthMethod{
			ssh.Password(password),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // Use a proper HostKeyCallback in production!
		Timeout:         1 * time.Second,
		// Manually define all supported ciphers
		Config: ssh.Config{
			Ciphers: []string{
				// Modern ciphers (the defaults your other 9 switches use)
				"aes128-gcm@openssh.com",
				"aes256-gcm@openssh.com",
				"chacha20-poly1305@openssh.com",
				"aes128-ctr",
				"aes192-ctr",
				"aes256-ctr",

				// Add a legacy cipher that the old switch supports
				// (from the "peer offered" list in your error)
				"aes128-cbc",
			},
			// KeyExchanges for HANDSHAKE
			KeyExchanges: []string{
				// Modern Kex (defaults from your error's "we offered" list)
				"curve25519-sha256",
				"ecdh-sha2-nistp256",
				"ecdh-sha2-nistp384",
				"ecdh-sha2-nistp521",
				"diffie-hellman-group14-sha256",
				// Legacy Kex (the one your switch needs)
				"diffie-hellman-group1-sha1",
				"diffie-hellman-group14-sha1",
				"diffie-hellman-group1-sha256",
				"diffie-hellman-group14-sha256",
				"diffie-hellman-group1-sha512",
				"diffie-hellman-group14-sha512",
			},
		},
	}

	sshClient, err := ssh.Dial("tcp", switch_hostname+":22", sshConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to dial SSH to %s: %w", switch_hostname, err)
	}

	return &Client{
		Client:         sshClient,
		SwitchHostname: switch_hostname,
	}, nil
}

func RunCommandWithCredentials(switch_hostname string, switch_command string, username string, password string) (string, error) {
	client, err := connectToSwitchWithCredentials(switch_hostname, username, password)
	if err != nil {
		// Just return the connection error
		return "", err
	}
	// 3. Defer closing the *client*
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		log.Printf("%s :: %s :: Failed to create session :: %v", switch_hostname, switch_command, err)
		return "", fmt.Errorf("%s :: %s :: Failed to create session :: %v", switch_hostname, switch_command, err)
	}
	defer session.Close()

	modes := ssh.TerminalModes{
		ssh.ECHO:          0,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}

	if err := session.RequestPty("vt100", 80, 200, modes); err != nil {
		log.Printf("request for pseudo-terminal failed for %s: %v", switch_hostname, err)
		return "", fmt.Errorf("request for pseudo-terminal failed for %s: %v", switch_hostname, err)
	}

	stdin, err := session.StdinPipe()
	if err != nil {
		log.Printf("Unable to setup stdin for session on %s: %v", switch_hostname, err)
		return "", fmt.Errorf("unable to setup stdin for session on %s: %v", switch_hostname, err)
	}
	stdout, err := session.StdoutPipe()
	if err != nil {
		log.Printf("Unable to setup stdout for session on %s: %v", switch_hostname, err)
		return "", fmt.Errorf("unable to setup stdout for session on %s: %v", switch_hostname, err)
	}

	if err := session.Shell(); err != nil {
		log.Printf("failed to start shell on %s: %v", switch_hostname, err)
		return "", fmt.Errorf("failed to start shell on %s: %v", switch_hostname, err)
	}

	commands := []string{
		"terminal length 0",
		switch_command,
		"exit",
	}
	for _, cmd := range commands {
		_, err = fmt.Fprintf(stdin, "%s\n", cmd)
		if err != nil {
			log.Printf("Failed to write to stdin on %s: %v", switch_hostname, err)
			return "", fmt.Errorf("failed to write to stdin on %s: %v", switch_hostname, err)
		}
	}

	var buf bytes.Buffer
	// Channel to signal that session.Wait() has finished
	done := make(chan error, 1)

	// Goroutine to read stdout and wait for the session to close (after 'exit' command)
	go func() {
		// Reads from stdout until the session closes (EOF)
		// This must happen *before* session.Wait() for session.Wait() to be useful.
		buf.ReadFrom(stdout)
		done <- session.Wait() // Wait for the remote command/shell to exit
	}()

	// --- TIMEOUT MECHANISM ---
	// Give this command a generous 3 seconds to complete since 'show interface' can be long.
	const commandTimeout = 30 * time.Second

	select {
	case err := <-done:
		// Command execution finished successfully or with an error
		if err != nil && err != io.EOF {
			// io.EOF is often returned by session.Wait() on clean exit, which is fine
			log.Printf("Session wait failed on %s: %v", switch_hostname, err)
			return "", fmt.Errorf("session wait failed on %s: %w", switch_hostname, err)
		}
	case <-time.After(commandTimeout):
		// Timeout hit. Close the client connection to forcefully terminate the session.
		client.Close()
		log.Printf("Show Interfaces timed out after %s on %s", commandTimeout, switch_hostname)
		return "", fmt.Errorf("%s command timed out after %s", switch_command, commandTimeout)
	}

	outputString := buf.String()

	return outputString, nil
}

// ConnectToSwitch creates and returns a new Client with an active SSH session
func connectToSwitch(switch_hostname string) (*Client, error) {
	// Retrieve credentials from environment variables
	var username = os.Getenv("CISCO_USERNAME")
	var password = os.Getenv("CISCO_PASSWORD")

	sshConfig := &ssh.ClientConfig{
		User: username,
		Auth: []ssh.AuthMethod{
			ssh.Password(password),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // Use a proper HostKeyCallback in production!
		Timeout:         1 * time.Second,
		// Manually define all supported ciphers
		Config: ssh.Config{
			Ciphers: []string{
				// Modern ciphers (the defaults your other 9 switches use)
				"aes128-gcm@openssh.com",
				"aes256-gcm@openssh.com",
				"chacha20-poly1305@openssh.com",
				"aes128-ctr",
				"aes192-ctr",
				"aes256-ctr",

				// Add a legacy cipher that the old switch supports
				// (from the "peer offered" list in your error)
				"aes128-cbc",
			},
			// KeyExchanges for HANDSHAKE
			KeyExchanges: []string{
				// Modern Kex (defaults from your error's "we offered" list)
				"curve25519-sha256",
				"ecdh-sha2-nistp256",
				"ecdh-sha2-nistp384",
				"ecdh-sha2-nistp521",
				"diffie-hellman-group14-sha256",
				// Legacy Kex (the one your switch needs)
				"diffie-hellman-group1-sha1",
				"diffie-hellman-group14-sha1",
				"diffie-hellman-group1-sha256",
				"diffie-hellman-group14-sha256",
				"diffie-hellman-group1-sha512",
				"diffie-hellman-group14-sha512",
			},
		},
	}

	sshClient, err := ssh.Dial("tcp", switch_hostname+":22", sshConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to dial SSH to %s: %w", switch_hostname, err)
	}

	return &Client{
		Client:         sshClient,
		SwitchHostname: switch_hostname,
	}, nil
}

func RunCommand(switch_hostname string, switch_command string) (string, error) {
	client, err := connectToSwitch(switch_hostname)
	if err != nil {
		// Just return the connection error
		return "", err
	}
	// 3. Defer closing the *client*
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		log.Printf("%s :: %s :: Failed to create session :: %v", switch_hostname, switch_command, err)
		return "", fmt.Errorf("%s :: %s :: Failed to create session :: %v", switch_hostname, switch_command, err)
	}
	defer session.Close()

	modes := ssh.TerminalModes{
		ssh.ECHO:          0,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}

	if err := session.RequestPty("vt100", 80, 200, modes); err != nil {
		log.Printf("request for pseudo-terminal failed for %s: %v", switch_hostname, err)
		return "", fmt.Errorf("request for pseudo-terminal failed for %s: %v", switch_hostname, err)
	}

	stdin, err := session.StdinPipe()
	if err != nil {
		log.Printf("Unable to setup stdin for session on %s: %v", switch_hostname, err)
		return "", fmt.Errorf("unable to setup stdin for session on %s: %v", switch_hostname, err)
	}
	stdout, err := session.StdoutPipe()
	if err != nil {
		log.Printf("Unable to setup stdout for session on %s: %v", switch_hostname, err)
		return "", fmt.Errorf("unable to setup stdout for session on %s: %v", switch_hostname, err)
	}

	if err := session.Shell(); err != nil {
		log.Printf("failed to start shell on %s: %v", switch_hostname, err)
		return "", fmt.Errorf("failed to start shell on %s: %v", switch_hostname, err)
	}

	commands := []string{
		"terminal length 0",
		switch_command,
		"exit",
	}
	for _, cmd := range commands {
		_, err = fmt.Fprintf(stdin, "%s\n", cmd)
		if err != nil {
			log.Printf("Failed to write to stdin on %s: %v", switch_hostname, err)
			return "", fmt.Errorf("failed to write to stdin on %s: %v", switch_hostname, err)
		}
	}

	var buf bytes.Buffer
	// Channel to signal that session.Wait() has finished
	done := make(chan error, 1)

	// Goroutine to read stdout and wait for the session to close (after 'exit' command)
	go func() {
		// Reads from stdout until the session closes (EOF)
		// This must happen *before* session.Wait() for session.Wait() to be useful.
		buf.ReadFrom(stdout)
		done <- session.Wait() // Wait for the remote command/shell to exit
	}()

	// --- TIMEOUT MECHANISM ---
	// Give this command a generous 3 seconds to complete since 'show interface' can be long.
	const commandTimeout = 30 * time.Second

	select {
	case err := <-done:
		// Command execution finished successfully or with an error
		if err != nil && err != io.EOF {
			// io.EOF is often returned by session.Wait() on clean exit, which is fine
			log.Printf("Session wait failed on %s: %v", switch_hostname, err)
			return "", fmt.Errorf("session wait failed on %s: %w", switch_hostname, err)
		}
	case <-time.After(commandTimeout):
		// Timeout hit. Close the client connection to forcefully terminate the session.
		client.Close()
		log.Printf("Show Interfaces timed out after %s on %s", commandTimeout, switch_hostname)
		return "", fmt.Errorf("%s command timed out after %s", switch_command, commandTimeout)
	}

	outputString := buf.String()

	return outputString, nil
}

func RunCommands(switch_hostname string, switch_commands []string) (string, error) {
	client, err := connectToSwitch(switch_hostname)
	if err != nil {
		// Just return the connection error
		return "", err
	}
	// 3. Defer closing the *client*
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		log.Printf("%s :: %s :: Failed to create session :: %v", switch_hostname, switch_commands, err)
		return "", fmt.Errorf("%s :: %s :: Failed to create session :: %v", switch_hostname, switch_commands, err)
	}
	defer session.Close()

	modes := ssh.TerminalModes{
		ssh.ECHO:          0,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}

	if err := session.RequestPty("vt100", 80, 200, modes); err != nil {
		log.Printf("request for pseudo-terminal failed for %s: %v", switch_hostname, err)
		return "", fmt.Errorf("request for pseudo-terminal failed for %s: %v", switch_hostname, err)
	}

	stdin, err := session.StdinPipe()
	if err != nil {
		log.Printf("Unable to setup stdin for session on %s: %v", switch_hostname, err)
		return "", fmt.Errorf("unable to setup stdin for session on %s: %v", switch_hostname, err)
	}
	stdout, err := session.StdoutPipe()
	if err != nil {
		log.Printf("Unable to setup stdout for session on %s: %v", switch_hostname, err)
		return "", fmt.Errorf("unable to setup stdout for session on %s: %v", switch_hostname, err)
	}

	if err := session.Shell(); err != nil {
		log.Printf("failed to start shell on %s: %v", switch_hostname, err)
		return "", fmt.Errorf("failed to start shell on %s: %v", switch_hostname, err)
	}

	commands := []string{"terminal length 0"}
	commands = append(commands, switch_commands...)
	commands = append(commands, "exit")

	for _, cmd := range commands {
		_, err = fmt.Fprintf(stdin, "%s\n", cmd)
		if err != nil {
			log.Printf("Failed to write to stdin on %s: %v", switch_hostname, err)
			return "", fmt.Errorf("failed to write to stdin on %s: %v", switch_hostname, err)
		}
	}

	var buf bytes.Buffer
	// Channel to signal that session.Wait() has finished
	done := make(chan error, 1)

	// Goroutine to read stdout and wait for the session to close (after 'exit' command)
	go func() {
		// Reads from stdout until the session closes (EOF)
		// This must happen *before* session.Wait() for session.Wait() to be useful.
		buf.ReadFrom(stdout)
		done <- session.Wait() // Wait for the remote command/shell to exit
	}()

	// --- TIMEOUT MECHANISM ---
	// Give this command a generous 3 seconds to complete since 'show interface' can be long.
	const commandTimeout = 30 * time.Second

	select {
	case err := <-done:
		// Command execution finished successfully or with an error
		if err != nil && err != io.EOF {
			// io.EOF is often returned by session.Wait() on clean exit, which is fine
			log.Printf("Session wait failed on %s: %v", switch_hostname, err)
			return "", fmt.Errorf("session wait failed on %s: %w", switch_hostname, err)
		}
	case <-time.After(commandTimeout):
		// Timeout hit. Close the client connection to forcefully terminate the session.
		client.Close()
		log.Printf("Show Interfaces timed out after %s on %s", commandTimeout, switch_hostname)
		return "", fmt.Errorf("%s command timed out after %s", switch_commands, commandTimeout)
	}

	outputString := buf.String()

	return outputString, nil
}

func Interface_shutdown(switch_hostname string, switch_interface string) (string, error) {
	client, err := connectToSwitch(switch_hostname)
	if err != nil {
		// Just return the connection error
		return "", err
	}
	// 3. Defer closing the *client*
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		log.Printf("%s :: %s :: Failed to create session :: %v", switch_hostname, "shutdown", err)
		return "", fmt.Errorf("%s :: %s :: Failed to create session :: %v", switch_hostname, "shutdown", err)
	}
	defer session.Close()

	modes := ssh.TerminalModes{
		ssh.ECHO:          0,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}

	if err := session.RequestPty("vt100", 80, 200, modes); err != nil {
		log.Printf("request for pseudo-terminal failed for %s: %v", switch_hostname, err)
		return "", fmt.Errorf("request for pseudo-terminal failed for %s: %v", switch_hostname, err)
	}

	stdin, err := session.StdinPipe()
	if err != nil {
		log.Printf("Unable to setup stdin for session on %s: %v", switch_hostname, err)
		return "", fmt.Errorf("unable to setup stdin for session on %s: %v", switch_hostname, err)
	}
	stdout, err := session.StdoutPipe()
	if err != nil {
		log.Printf("Unable to setup stdout for session on %s: %v", switch_hostname, err)
		return "", fmt.Errorf("unable to setup stdout for session on %s: %v", switch_hostname, err)
	}

	if err := session.Shell(); err != nil {
		log.Printf("failed to start shell on %s: %v", switch_hostname, err)
		return "", fmt.Errorf("failed to start shell on %s: %v", switch_hostname, err)
	}

	commands := []string{
		"terminal length 0", // Prevents paging '--More--' prompts
		"configure terminal",
		fmt.Sprintf("interface %s", switch_interface),
		"shutdown",
		"end",
		"exit",
	}

	for _, cmd := range commands {
		_, err = fmt.Fprintf(stdin, "%s\n", cmd)
		if err != nil {
			log.Printf("Failed to write to stdin on %s: %v", switch_hostname, err)
			return "", fmt.Errorf("failed to write to stdin on %s: %v", switch_hostname, err)
		}
	}

	var buf bytes.Buffer
	// Channel to signal that session.Wait() has finished
	done := make(chan error, 1)

	// Goroutine to read stdout and wait for the session to close (after 'exit' command)
	go func() {
		// Reads from stdout until the session closes (EOF)
		// This must happen *before* session.Wait() for session.Wait() to be useful.
		buf.ReadFrom(stdout)
		done <- session.Wait() // Wait for the remote command/shell to exit
	}()

	// --- TIMEOUT MECHANISM ---
	// Give this command a generous 3 seconds to complete since 'show interface' can be long.
	const commandTimeout = 3 * time.Second

	select {
	case err := <-done:
		// Command execution finished successfully or with an error
		if err != nil && err != io.EOF {
			// io.EOF is often returned by session.Wait() on clean exit, which is fine
			log.Printf("Session wait failed on %s: %v", switch_hostname, err)
			return "", fmt.Errorf("session wait failed on %s: %w", switch_hostname, err)
		}
	case <-time.After(commandTimeout):
		// Timeout hit. Close the client connection to forcefully terminate the session.
		client.Close()
		log.Printf("Show Interfaces timed out after %s on %s", commandTimeout, switch_hostname)
		return "", fmt.Errorf("%s command timed out after %s", "shutdown", commandTimeout)
	}

	outputString := buf.String()

	log.Printf("Successfully applied '%s' to interface %s on %s.", "shutdown", switch_interface, switch_hostname)

	return outputString, nil
}

func Interface_no_shutdown(switch_hostname string, switch_interface string) (string, error) {
	client, err := connectToSwitch(switch_hostname)
	if err != nil {
		// Just return the connection error
		return "", err
	}
	// 3. Defer closing the *client*
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		log.Printf("%s :: %s :: Failed to create session :: %v", switch_hostname, "shutdown", err)
		return "", fmt.Errorf("%s :: %s :: Failed to create session :: %v", switch_hostname, "shutdown", err)
	}
	defer session.Close()

	modes := ssh.TerminalModes{
		ssh.ECHO:          0,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}

	if err := session.RequestPty("vt100", 80, 200, modes); err != nil {
		log.Printf("request for pseudo-terminal failed for %s: %v", switch_hostname, err)
		return "", fmt.Errorf("request for pseudo-terminal failed for %s: %v", switch_hostname, err)
	}

	stdin, err := session.StdinPipe()
	if err != nil {
		log.Printf("Unable to setup stdin for session on %s: %v", switch_hostname, err)
		return "", fmt.Errorf("unable to setup stdin for session on %s: %v", switch_hostname, err)
	}
	stdout, err := session.StdoutPipe()
	if err != nil {
		log.Printf("Unable to setup stdout for session on %s: %v", switch_hostname, err)
		return "", fmt.Errorf("unable to setup stdout for session on %s: %v", switch_hostname, err)
	}

	if err := session.Shell(); err != nil {
		log.Printf("failed to start shell on %s: %v", switch_hostname, err)
		return "", fmt.Errorf("failed to start shell on %s: %v", switch_hostname, err)
	}

	commands := []string{
		"terminal length 0", // Prevents paging '--More--' prompts
		"configure terminal",
		fmt.Sprintf("interface %s", switch_interface),
		"no shutdown",
		"end",
		"exit",
	}

	for _, cmd := range commands {
		_, err = fmt.Fprintf(stdin, "%s\n", cmd)
		if err != nil {
			log.Printf("Failed to write to stdin on %s: %v", switch_hostname, err)
			return "", fmt.Errorf("failed to write to stdin on %s: %v", switch_hostname, err)
		}
	}

	var buf bytes.Buffer
	// Channel to signal that session.Wait() has finished
	done := make(chan error, 1)

	// Goroutine to read stdout and wait for the session to close (after 'exit' command)
	go func() {
		// Reads from stdout until the session closes (EOF)
		// This must happen *before* session.Wait() for session.Wait() to be useful.
		buf.ReadFrom(stdout)
		done <- session.Wait() // Wait for the remote command/shell to exit
	}()

	// --- TIMEOUT MECHANISM ---
	// Give this command a generous 3 seconds to complete since 'show interface' can be long.
	const commandTimeout = 3 * time.Second

	select {
	case err := <-done:
		// Command execution finished successfully or with an error
		if err != nil && err != io.EOF {
			// io.EOF is often returned by session.Wait() on clean exit, which is fine
			log.Printf("Session wait failed on %s: %v", switch_hostname, err)
			return "", fmt.Errorf("session wait failed on %s: %w", switch_hostname, err)
		}
	case <-time.After(commandTimeout):
		// Timeout hit. Close the client connection to forcefully terminate the session.
		client.Close()
		log.Printf("Show Interfaces timed out after %s on %s", commandTimeout, switch_hostname)
		return "", fmt.Errorf("%s command timed out after %s", "shutdown", commandTimeout)
	}

	outputString := buf.String()

	log.Printf("Successfully applied '%s' to interface %s on %s.", "shutdown", switch_interface, switch_hostname)

	return outputString, nil
}

func Interface_change_description(switch_hostname string, switch_interface string, interface_description string) (string, error) {
	client, err := connectToSwitch(switch_hostname)
	if err != nil {
		// Just return the connection error
		return "", err
	}
	// 3. Defer closing the *client*
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		log.Printf("%s :: %s :: Failed to create session :: %v", switch_hostname, "shutdown", err)
		return "", fmt.Errorf("%s :: %s :: Failed to create session :: %v", switch_hostname, "shutdown", err)
	}
	defer session.Close()

	modes := ssh.TerminalModes{
		ssh.ECHO:          0,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}

	if err := session.RequestPty("vt100", 80, 200, modes); err != nil {
		log.Printf("request for pseudo-terminal failed for %s: %v", switch_hostname, err)
		return "", fmt.Errorf("request for pseudo-terminal failed for %s: %v", switch_hostname, err)
	}

	stdin, err := session.StdinPipe()
	if err != nil {
		log.Printf("Unable to setup stdin for session on %s: %v", switch_hostname, err)
		return "", fmt.Errorf("unable to setup stdin for session on %s: %v", switch_hostname, err)
	}
	stdout, err := session.StdoutPipe()
	if err != nil {
		log.Printf("Unable to setup stdout for session on %s: %v", switch_hostname, err)
		return "", fmt.Errorf("unable to setup stdout for session on %s: %v", switch_hostname, err)
	}

	if err := session.Shell(); err != nil {
		log.Printf("failed to start shell on %s: %v", switch_hostname, err)
		return "", fmt.Errorf("failed to start shell on %s: %v", switch_hostname, err)
	}

	commands := []string{
		"terminal length 0", // Prevents paging '--More--' prompts
		"configure terminal",
		fmt.Sprintf("interface %s", switch_interface),
		fmt.Sprintf("description %s", interface_description),
		"end",
		"exit",
	}

	for _, cmd := range commands {
		_, err = fmt.Fprintf(stdin, "%s\n", cmd)
		if err != nil {
			log.Printf("Failed to write to stdin on %s: %v", switch_hostname, err)
			return "", fmt.Errorf("failed to write to stdin on %s: %v", switch_hostname, err)
		}
	}

	var buf bytes.Buffer
	// Channel to signal that session.Wait() has finished
	done := make(chan error, 1)

	// Goroutine to read stdout and wait for the session to close (after 'exit' command)
	go func() {
		// Reads from stdout until the session closes (EOF)
		// This must happen *before* session.Wait() for session.Wait() to be useful.
		buf.ReadFrom(stdout)
		done <- session.Wait() // Wait for the remote command/shell to exit
	}()

	// --- TIMEOUT MECHANISM ---
	// Give this command a generous 3 seconds to complete since 'show interface' can be long.
	const commandTimeout = 3 * time.Second

	select {
	case err := <-done:
		// Command execution finished successfully or with an error
		if err != nil && err != io.EOF {
			// io.EOF is often returned by session.Wait() on clean exit, which is fine
			log.Printf("Session wait failed on %s: %v", switch_hostname, err)
			return "", fmt.Errorf("session wait failed on %s: %w", switch_hostname, err)
		}
	case <-time.After(commandTimeout):
		// Timeout hit. Close the client connection to forcefully terminate the session.
		client.Close()
		log.Printf("Show Interfaces timed out after %s on %s", commandTimeout, switch_hostname)
		return "", fmt.Errorf("%s command timed out after %s", "shutdown", commandTimeout)
	}

	outputString := buf.String()

	log.Printf("Successfully changed description '%s' to interface %s on %s.", interface_description, switch_interface, switch_hostname)

	return outputString, nil
}

// Close closes the underlying SSH connection
func (c *Client) Close() {
	c.Client.Close()
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
