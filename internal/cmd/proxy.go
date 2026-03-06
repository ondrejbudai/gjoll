package cmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"syscall"

	"github.com/obudai/gjoll/internal/paths"
	"github.com/obudai/gjoll/internal/proxy"
	"github.com/obudai/gjoll/internal/remote"
	"github.com/obudai/gjoll/internal/state"
	"github.com/spf13/cobra"
)

var proxyCmd = &cobra.Command{
	Use:   "proxy <name>",
	Short: "Start credential-injecting proxy with SSH reverse tunnel",
	Long: `Start an HTTP reverse proxy that injects authentication headers (GCP or API key)
and creates an SSH reverse tunnel to the remote VM. This allows API clients on the
VM to make authenticated requests without having credentials on the VM.

The proxy configuration comes from the 'proxy' output in the terraform file.`,
	Args: cobra.ExactArgs(1),
	RunE: runProxy,
}

func runProxy(cmd *cobra.Command, args []string) error {
	name := args[0]

	// Load instance state
	inst, err := state.Load(name)
	if err != nil {
		return fmt.Errorf("loading instance: %w", err)
	}

	// Check if proxy is configured
	if inst.Proxy == nil {
		return fmt.Errorf("no proxy configured for instance %q", name)
	}

	cfg := inst.Proxy

	// Validate auth mode
	if cfg.Auth != "gcp" && cfg.Auth != "api-key" {
		return fmt.Errorf("invalid auth mode %q (must be 'gcp' or 'api-key')", cfg.Auth)
	}

	// Read API key if needed
	var apiKey string
	if cfg.Auth == "api-key" {
		if cfg.APIKeyFile == "" {
			return fmt.Errorf("api_key_file not set for api-key auth")
		}

		apiKeyPath, err := remote.ExpandTilde(cfg.APIKeyFile)
		if err != nil {
			return fmt.Errorf("expanding api_key_file path: %w", err)
		}

		apiKeyBytes, err := os.ReadFile(apiKeyPath)
		if err != nil {
			return fmt.Errorf("reading api key from %s: %w", cfg.APIKeyFile, err)
		}
		apiKey = string(apiKeyBytes)
	}

	// Create and start proxy
	fmt.Printf("Starting proxy to %s (auth: %s)...\n", cfg.Target, cfg.Auth)
	p, err := proxy.New(cfg.Target, cfg.Auth, apiKey)
	if err != nil {
		return fmt.Errorf("creating proxy: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	localPort, err := p.Start(ctx)
	if err != nil {
		return fmt.Errorf("starting proxy: %w", err)
	}
	fmt.Printf("Proxy listening on localhost:%d\n", localPort)

	// Get SSH config path
	instanceDir, err := paths.InstanceDir(name)
	if err != nil {
		return err
	}
	sshConfigPath := remote.SSHConfigPath(instanceDir)

	// Start SSH reverse tunnel
	remotePort := cfg.Port
	fmt.Printf("Starting SSH reverse tunnel to %s:%d...\n", name, remotePort)

	sshArgs := []string{
		"-F", sshConfigPath,
		"-R", fmt.Sprintf("%d:127.0.0.1:%d", remotePort, localPort),
		"-N", // no remote command
		name,
	}

	sshCmd := exec.Command("ssh", sshArgs...)
	sshCmd.Stdout = os.Stdout
	sshCmd.Stderr = os.Stderr

	if err := sshCmd.Start(); err != nil {
		_ = p.Stop(ctx)
		return fmt.Errorf("starting SSH tunnel: %w", err)
	}

	fmt.Printf("\n✓ Proxy running!\n")
	fmt.Printf("  Remote endpoint: http://localhost:%d on %s\n", remotePort, name)
	fmt.Printf("  Press Ctrl+C to stop\n\n")

	// Wait for interrupt signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	<-sigCh
	fmt.Println("\nShutting down...")

	// Stop SSH tunnel
	if err := sshCmd.Process.Kill(); err != nil {
		fmt.Fprintf(os.Stderr, "warning: killing SSH process: %v\n", err)
	}
	_ = sshCmd.Wait()

	// Stop proxy
	if err := p.Stop(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "warning: stopping proxy: %v\n", err)
	}

	fmt.Println("Stopped.")
	return nil
}
