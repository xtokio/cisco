package connection

import (
	"strings"
	"slices"
	"bytes"
	"time"
	"fmt"
	"os"
	"io"

	"charm.land/log/v2"
	"golang.org/x/crypto/ssh"
)

// Client holds the active SSH connection
type Client struct {
	*ssh.Client
	SwitchHostname string
}

// Close closes the underlying SSH connection
func (c *Client) Close() {
	c.Client.Close()
}

// CommandResult holds the execution outcome of a single command
type CommandResult struct {
	Command  string        `json:"command"`
	Response string        `json:"response"`
	Duration time.Duration `json:"duration"`
	Error    string        `json:"error,omitempty"`
}

// ConnectToSwitch creates and returns a new Client with an active SSH session
func Connect(switch_hostname string) (*ssh.Client, error) {
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
		return nil, fmt.Errorf("Failed to dial SSH to %s: %v", switch_hostname, err)
	}

	return sshClient, nil
}

// readUntilPrompt reads from the SSH stream until the switch prompt `#` or `>` is received
func readUntilPrompt(stdout io.Reader, timeout time.Duration) (string, error) {
	buf := make([]byte, 1024)
	var output bytes.Buffer
	deadline := time.Now().Add(timeout)

	for {
		if time.Now().After(deadline) {
			return output.String(), fmt.Errorf("timeout waiting for switch prompt")
		}

		n, err := stdout.Read(buf)
		if n > 0 {
			output.Write(buf[:n])
			str := output.String()

			// Detect prompt character `#` or `>` indicating Cisco CLI is ready for next input
			trimmed := strings.TrimSpace(str)
			if strings.HasSuffix(trimmed, "#") || strings.HasSuffix(trimmed, ">") {
				return str, nil
			}
		}

		if err != nil {
			if err == io.EOF {
				break
			}
			return output.String(), err
		}
	}

	return output.String(), nil
}

// ExecuteCommandsOnClient executes commands in sequence and returns a struct array
func ExecuteCommandsOnClient(client *ssh.Client, switch_hostname string, commands []string) ([]CommandResult, error) {
	if client == nil {
		return nil, fmt.Errorf("ssh client is nil for %s", switch_hostname)
	}

	session, err := client.NewSession()
	if err != nil {
		return nil, fmt.Errorf("%s :: Failed to create session :: %w", switch_hostname, err)
	}
	defer session.Close()

	stdin, err := session.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("stdin pipe failed: %w", err)
	}

	stdout, err := session.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("stdout pipe failed: %w", err)
	}

	if err := session.Shell(); err != nil {
		return nil, fmt.Errorf("failed to start shell: %w", err)
	}

	// 1. Wait for initial prompt after banner/login
	_, _ = readUntilPrompt(stdout, 5*time.Second)

	// 2. Disable pagination once
	fmt.Fprintln(stdin, "terminal length 0")
	_, _ = readUntilPrompt(stdout, 5*time.Second)

	var results []CommandResult

	// 3. Execute each command and append the CommandResult struct to the array
	for _, cmd := range commands {
		startTime := time.Now()
		fmt.Fprintln(stdin, cmd)

		output, err := readUntilPrompt(stdout, 15*time.Second)
		duration := time.Since(startTime)

		res := CommandResult{
			Command:  cmd,
			Duration: duration,
		}

		if err != nil {
			res.Error = err.Error()
			res.Response = cleanOutput(output, cmd)
		} else {
			res.Response = cleanOutput(output, cmd)
		}

		results = append(results, res)
	}

	// Graceful termination
	fmt.Fprintln(stdin, "exit")

	return results, nil
}

// cleanOutput strips the command echo and trailing prompt from the result
func cleanOutput(raw, cmd string) string {
	lines := strings.Split(raw, "\n")
	var cleaned []string

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		// Skip initial command echo line
		if i == 0 && strings.Contains(trimmed, cmd) {
			continue
		}
		// Skip trailing switch prompt line (e.g. lgomezreswitch#)
		if strings.HasSuffix(trimmed, "#") || strings.HasSuffix(trimmed, ">") {
			continue
		}
		cleaned = append(cleaned, line)
	}

	return strings.Join(cleaned, "\n")
}

// ConnectToSwitch creates and returns a new Client with an active SSH session
func connectToSwitch(switch_hostname string) (*Client, error) {
	var username string
	var password string

	london_switches := []string{"lonb10-a.hub.nd.edu", "lon107-a.hub.nd.edu", "lon301-a.hub.nd.edu","lon-hob-a.hub.nd.edu","conway-mdf.hub.nd.edu","conway-idf2-west.hub.nd.edu","conway-idf4-west.hub.nd.edu","conway-idf4-east.hub.nd.edu"}
	if slices.Contains(london_switches, switch_hostname) {
		username = os.Getenv("CISCO_ROME_USERNAME")
		password = os.Getenv("CISCO_ROME_PASSWORD")
	} else {
		username = os.Getenv("CISCO_USERNAME")
		password = os.Getenv("CISCO_PASSWORD")
	}

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
		log.Errorf("%s :: %s :: Failed to create session :: %v", switch_hostname, switch_command, err)
		return "", fmt.Errorf("%s :: %s :: Failed to create session :: %v", switch_hostname, switch_command, err)
	}
	defer session.Close()

	modes := ssh.TerminalModes{
		ssh.ECHO:          0,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}

	if err := session.RequestPty("vt100", 80, 200, modes); err != nil {
		log.Errorf("request for pseudo-terminal failed for %s: %v", switch_hostname, err)
		return "", fmt.Errorf("request for pseudo-terminal failed for %s: %v", switch_hostname, err)
	}

	stdin, err := session.StdinPipe()
	if err != nil {
		log.Errorf("Unable to setup stdin for session on %s: %v", switch_hostname, err)
		return "", fmt.Errorf("unable to setup stdin for session on %s: %v", switch_hostname, err)
	}
	stdout, err := session.StdoutPipe()
	if err != nil {
		log.Errorf("Unable to setup stdout for session on %s: %v", switch_hostname, err)
		return "", fmt.Errorf("unable to setup stdout for session on %s: %v", switch_hostname, err)
	}

	if err := session.Shell(); err != nil {
		log.Errorf("failed to start shell on %s: %v", switch_hostname, err)
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
			log.Errorf("Failed to write to stdin on %s: %v", switch_hostname, err)
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
			log.Errorf("Session wait failed on %s: %v", switch_hostname, err)
			return "", fmt.Errorf("session wait failed on %s: %w", switch_hostname, err)
		}
	case <-time.After(commandTimeout):
		// Timeout hit. Close the client connection to forcefully terminate the session.
		client.Close()
		log.Errorf("Show Interfaces timed out after %s on %s", commandTimeout, switch_hostname)
		return "", fmt.Errorf("%s command timed out after %s", switch_command, commandTimeout)
	}

	outputString := buf.String()

	return outputString, nil
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

func RunCommandWithCredentials(switch_hostname string, username string, password string, switch_command string) (string, error) {
	client, err := connectToSwitchWithCredentials(switch_hostname, username, password)
	if err != nil {
		// Just return the connection error
		return "", err
	}
	// 3. Defer closing the *client*
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		log.Errorf("%s :: %s :: Failed to create session :: %v", switch_hostname, switch_command, err)
		return "", fmt.Errorf("%s :: %s :: Failed to create session :: %v", switch_hostname, switch_command, err)
	}
	defer session.Close()

	modes := ssh.TerminalModes{
		ssh.ECHO:          0,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}

	if err := session.RequestPty("vt100", 80, 200, modes); err != nil {
		log.Errorf("request for pseudo-terminal failed for %s: %v", switch_hostname, err)
		return "", fmt.Errorf("request for pseudo-terminal failed for %s: %v", switch_hostname, err)
	}

	stdin, err := session.StdinPipe()
	if err != nil {
		log.Errorf("Unable to setup stdin for session on %s: %v", switch_hostname, err)
		return "", fmt.Errorf("unable to setup stdin for session on %s: %v", switch_hostname, err)
	}
	stdout, err := session.StdoutPipe()
	if err != nil {
		log.Errorf("Unable to setup stdout for session on %s: %v", switch_hostname, err)
		return "", fmt.Errorf("unable to setup stdout for session on %s: %v", switch_hostname, err)
	}

	if err := session.Shell(); err != nil {
		log.Errorf("failed to start shell on %s: %v", switch_hostname, err)
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
			log.Errorf("Failed to write to stdin on %s: %v", switch_hostname, err)
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
			log.Errorf("Session wait failed on %s: %v", switch_hostname, err)
			return "", fmt.Errorf("session wait failed on %s: %w", switch_hostname, err)
		}
	case <-time.After(commandTimeout):
		// Timeout hit. Close the client connection to forcefully terminate the session.
		client.Close()
		log.Errorf("Show Interfaces timed out after %s on %s", commandTimeout, switch_hostname)
		return "", fmt.Errorf("%s command timed out after %s", switch_command, commandTimeout)
	}

	outputString := buf.String()

	return outputString, nil
}
