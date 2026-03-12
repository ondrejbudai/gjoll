package cmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"

	"github.com/obudai/gjoll/internal/config"
	"github.com/obudai/gjoll/internal/paths"
	"github.com/obudai/gjoll/internal/proxy"
	"github.com/obudai/gjoll/internal/remote"
	"github.com/obudai/gjoll/internal/state"
	"github.com/spf13/cobra"
)

var proxyCmd = &cobra.Command{
	Use:   "proxy <name>",
	Short: "Start credential-injecting proxies with SSH reverse tunnels",
	Long: `Start HTTP reverse proxies that optionally inject authentication headers
(GCP, API key, or none) and create SSH reverse tunnels to the remote VM.
This allows API clients on the VM to make authenticated requests without
having credentials on the VM.

The proxy configuration comes from the 'proxies' output in the terraform file.`,
	Args: cobra.ExactArgs(1),
	RunE: runProxy,
}

// proxySet holds running proxies and their associated SSH tunnel args.
type proxySet struct {
	proxies    []*proxy.Proxy
	tunnelArgs []string // SSH -R flags
}

// startProxies starts all configured proxies and returns them along with the
// SSH reverse tunnel arguments. Call stop() when done.
func startProxies(ctx context.Context, proxies []config.ProxyConfig) (*proxySet, error) {
	ps := &proxySet{}

	for _, cfg := range proxies {
		if cfg.Auth != "" && cfg.Auth != "gcp" && cfg.Auth != "api-key" {
			ps.stop(ctx)
			return nil, fmt.Errorf("proxy %q: invalid auth mode %q (must be 'gcp', 'api-key', or empty)", cfg.Name, cfg.Auth)
		}

		var apiKey string
		if cfg.Auth == "api-key" {
			if cfg.APIKeyFile == "" {
				ps.stop(ctx)
				return nil, fmt.Errorf("proxy %q: api_key_file not set for api-key auth", cfg.Name)
			}

			apiKeyPath, err := remote.ExpandTilde(cfg.APIKeyFile)
			if err != nil {
				ps.stop(ctx)
				return nil, fmt.Errorf("proxy %q: expanding api_key_file path: %w", cfg.Name, err)
			}

			apiKeyBytes, err := os.ReadFile(apiKeyPath)
			if err != nil {
				ps.stop(ctx)
				return nil, fmt.Errorf("proxy %q: reading api key from %s: %w", cfg.Name, cfg.APIKeyFile, err)
			}
			apiKey = strings.TrimSpace(string(apiKeyBytes))
		}

		authDesc := cfg.Auth
		if authDesc == "" {
			authDesc = "none"
		}
		fmt.Printf("Starting proxy %q to %s (auth: %s)...\n", cfg.Name, cfg.Target, authDesc)

		p, err := proxy.New(cfg.Target, cfg.Auth, apiKey)
		if err != nil {
			ps.stop(ctx)
			return nil, fmt.Errorf("proxy %q: creating proxy: %w", cfg.Name, err)
		}

		localPort, err := p.Start(ctx)
		if err != nil {
			ps.stop(ctx)
			return nil, fmt.Errorf("proxy %q: starting proxy: %w", cfg.Name, err)
		}

		ps.proxies = append(ps.proxies, p)
		ps.tunnelArgs = append(ps.tunnelArgs, "-R", fmt.Sprintf("%d:127.0.0.1:%d", cfg.Port, localPort))
		fmt.Printf("  %s listening on localhost:%d → remote port %d\n", cfg.Name, localPort, cfg.Port)
	}

	return ps, nil
}

func (ps *proxySet) stop(ctx context.Context) {
	for _, p := range ps.proxies {
		if err := p.Stop(ctx); err != nil {
			fmt.Fprintf(os.Stderr, "warning: stopping proxy: %v\n", err)
		}
	}
}

func runProxy(cmd *cobra.Command, args []string) error {
	name := args[0]

	inst, err := state.Load(name)
	if err != nil {
		return fmt.Errorf("loading instance: %w", err)
	}

	if len(inst.Proxies) == 0 {
		return fmt.Errorf("no proxies configured for instance %q", name)
	}

	instanceDir, err := paths.InstanceDir(name)
	if err != nil {
		return err
	}
	sshConfigPath := remote.SSHConfigPath(instanceDir)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ps, err := startProxies(ctx, inst.Proxies)
	if err != nil {
		return err
	}
	defer ps.stop(ctx)

	// Start a single SSH connection with all reverse tunnels
	fmt.Printf("Starting SSH reverse tunnel to %s...\n", name)
	sshArgs := []string{"-F", sshConfigPath}
	sshArgs = append(sshArgs, ps.tunnelArgs...)
	sshArgs = append(sshArgs, "-N", name) // no remote command

	sshProc := exec.Command("ssh", sshArgs...)
	sshProc.Stdout = os.Stdout
	sshProc.Stderr = os.Stderr

	if err := sshProc.Start(); err != nil {
		return fmt.Errorf("starting SSH tunnel: %w", err)
	}

	fmt.Printf("\nProxies running!\n")
	for _, cfg := range inst.Proxies {
		fmt.Printf("  %s → http://localhost:%d on %s\n", cfg.Name, cfg.Port, name)
	}
	fmt.Printf("  Press Ctrl+C to stop\n\n")

	// Wait for interrupt signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	<-sigCh
	fmt.Println("\nShutting down...")

	if err := sshProc.Process.Kill(); err != nil {
		fmt.Fprintf(os.Stderr, "warning: killing SSH process: %v\n", err)
	}
	_ = sshProc.Wait()

	// ps.stop() is called by defer
	fmt.Println("Stopped.")
	return nil
}
