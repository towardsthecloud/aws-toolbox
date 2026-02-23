package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newCompletionCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "completion [bash|zsh|fish|powershell]",
		Short: "Generate shell completion scripts",
		Long: `Generate shell completion scripts for awstbx.

To load completions for every new session, execute once:

Bash:

  # Linux:
  awstbx completion bash > /etc/bash_completion.d/awstbx

  # macOS:
  awstbx completion bash > $(brew --prefix)/etc/bash_completion.d/awstbx

Zsh:

  # Linux:
  awstbx completion zsh > "${fpath[1]}/_awstbx"

  # macOS:
  awstbx completion zsh > $(brew --prefix)/share/zsh/site-functions/_awstbx

Fish:

  awstbx completion fish > ~/.config/fish/completions/awstbx.fish

PowerShell:

  awstbx completion powershell > $PROFILE.CurrentUserAllHosts

You will need to start a new shell for this setup to take effect.`,
		Args:      cobra.ExactArgs(1),
		ValidArgs: []string{"bash", "zsh", "fish", "powershell"},
		RunE: func(cmd *cobra.Command, args []string) error {
			switch args[0] {
			case "bash":
				return cmd.Root().GenBashCompletionV2(cmd.OutOrStdout(), true)
			case "zsh":
				return cmd.Root().GenZshCompletion(cmd.OutOrStdout())
			case "fish":
				return cmd.Root().GenFishCompletion(cmd.OutOrStdout(), true)
			case "powershell":
				return cmd.Root().GenPowerShellCompletionWithDesc(cmd.OutOrStdout())
			default:
				return fmt.Errorf("unsupported shell %q", args[0])
			}
		},
		SilenceUsage: true,
	}

	return cmd
}
