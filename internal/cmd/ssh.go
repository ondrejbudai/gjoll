package cmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"

	"github.com/ondrejbudai/gjoll/internal/engine"
	"github.com/ondrejbudai/gjoll/internal/paths"
	"github.com/ondrejbudai/gjoll/internal/remote"
	"github.com/ondrejbudai/gjoll/internal/state"
	"github.com/spf13/cobra"
)

var (
	sshWakeup         bool
	sshProxyFlag      bool
	sshReverseTunnels []string
)

var sshCmd = &cobra.Command{
	Use:   "ssh <name> [-- command...]",
	Short: "SSH into a running sandbox",
	Long: `Opens an interactive SSH session to the named sandbox.

If a command is given after --, it is executed on the sandbox instead of
starting an interactive shell.

With --wakeup, a stopped sandbox is started before connecting and stopped
again after the command finishes (requires a command).

With --proxy, credential-injecting proxies are started and reverse-tunneled
through the SSH connection. The proxies are stopped when SSH exits.

Additional reverse tunnels can be specified with -R, using the same syntax
as ssh(1). Can be combined with --proxy or used alone.

Examples:
  gjoll ssh mybox                              Interactive shell
  gjoll ssh mybox -- uname -a                  Run a command
  gjoll ssh mybox --wakeup -- uname -a         Start, run, stop
  gjoll ssh mybox --proxy                      Shell with proxies
  gjoll ssh mybox --proxy -- claude            Run command with proxies
  gjoll ssh mybox --wakeup --proxy -- cmd      Wakeup + proxies
  gjoll ssh mybox -R 8080:localhost:3000       Reverse tunnel
  gjoll ssh mybox --proxy -R 9090:localhost:80 Proxy + extra tunnel`,
	Args:               cobra.MinimumNArgs(1),
	DisableFlagParsing: false,
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		if sshWakeup && len(args) < 2 {
			return fmt.Errorf("--wakeup requires a command (e.g., gjoll ssh --wakeup %s -- uname -a)", name)
		}

		if sshWakeup {
			inst, err := state.Load(name)
			if err != nil {
				return err
			}

			if inst.Status == "stopped" {
				lock, err := state.Lock(name)
				if err != nil {
					return err
				}

				fmt.Println("Starting sandbox...")
				if err := engine.Start(name); err != nil {
					state.Unlock(lock)
					return err
				}
				state.Unlock(lock)

				defer func() {
					lock, err := state.Lock(name)
					if err != nil {
						fmt.Printf("Warning: could not lock for stop: %v\n", err)
						return
					}
					fmt.Println("Stopping sandbox...")
					if err := engine.Stop(name); err != nil {
						fmt.Printf("Warning: could not stop sandbox: %v\n", err)
					}
					state.Unlock(lock)
				}()
			}
		} else {
			if _, err := state.Load(name); err != nil {
				return err
			}
		}

		instanceDir, err := paths.InstanceDir(name)
		if err != nil {
			return err
		}
		configPath := remote.SSHConfigPath(instanceDir)

		extraTunnelArgs := reverseTunnelArgs(sshReverseTunnels)

		if sshProxyFlag || len(extraTunnelArgs) > 0 {
			return sshWithProxy(name, configPath, extraTunnelArgs, args[1:]...)
		}

		return remote.Connect(configPath, name, args[1:]...)
	},
}

// sshWithProxy starts proxies, opens SSH with reverse tunnels, and cleans up.
// Unlike remote.Connect, this always runs SSH as a subprocess so proxies can
// be stopped when the session ends. extraTunnelArgs are additional SSH -R flags
// specified via the command line.
func sshWithProxy(name, configPath string, extraTunnelArgs []string, command ...string) error {
	inst, err := state.Load(name)
	if err != nil {
		return err
	}

	if sshProxyFlag && len(inst.Proxies) == 0 {
		return fmt.Errorf("no proxies configured for instance %q — cannot use --proxy", name)
	}

	ctx := context.Background()
	ps, err := startProxies(ctx, inst.Proxies)
	if err != nil {
		return err
	}
	defer ps.stop(ctx)

	// Build SSH args: config, reverse tunnels, host, then command
	sshArgs := []string{"-F", configPath}
	sshArgs = append(sshArgs, ps.tunnelArgs...)
	sshArgs = append(sshArgs, extraTunnelArgs...)
	sshArgs = append(sshArgs, name)
	sshArgs = append(sshArgs, command...)

	sshProc := exec.Command("ssh", sshArgs...)
	sshProc.Stdin = os.Stdin
	sshProc.Stdout = os.Stdout
	sshProc.Stderr = os.Stderr

	if len(inst.Proxies) > 0 {
		fmt.Printf("Proxies active on %s:\n", name)
		for _, cfg := range inst.Proxies {
			fmt.Printf("  %s → http://localhost:%d\n", cfg.Name, cfg.Port)
		}
	}
	for _, rt := range sshReverseTunnels {
		fmt.Printf("  reverse tunnel: -R %s\n", rt)
	}
	if len(inst.Proxies) > 0 || len(sshReverseTunnels) > 0 {
		fmt.Println()
	}

	return sshProc.Run()
}

func init() {
	sshCmd.Flags().SetInterspersed(false)
	sshCmd.Flags().BoolVar(&sshWakeup, "wakeup", false, "start a stopped sandbox, run the command, then stop it")
	sshCmd.Flags().BoolVar(&sshProxyFlag, "proxy", false, "start proxies and tunnel them through the SSH connection")
	sshCmd.Flags().StringArrayVarP(&sshReverseTunnels, "reverse", "R", nil,
		"reverse port forwarding, same as ssh -R (can be specified multiple times)")
}
