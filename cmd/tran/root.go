package tran

import (
	"fmt"
	"log"

	"github.com/MakeNowJust/heredoc"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/scmn-dev/tran/app"
	configCmd "github.com/scmn-dev/tran/app/config"
	"github.com/scmn-dev/tran/cmd/factory"
	"github.com/scmn-dev/tran/config"
	"github.com/scmn-dev/tran/tui"
	"github.com/spf13/cobra"
)

// Execute start the CLI
func Execute(f *factory.Factory, version string, buildDate string) *cobra.Command {
	const desc = `🖥️ Securely transfer and send anything between computers with TUI.`

	// Root command
	var rootCmd = &cobra.Command{
		Use:   "tran <subcommand> [flags]",
		Short:  desc,
		Long: desc,
		SilenceErrors: true,
		Example: heredoc.Doc(`
			# Open Tran UI
			tran

			# Open with specific path
			tran --start-dir $PATH

			# Send files to a remote computer
			tran send <FILE || DIRECTORY>

			# Receive files from a remote computer
			tran receive <PASSWORD>
			
			# Authenticate
			tran auth login
			
			# Sync your tran config file
			tran sync start
		`),
		Annotations: map[string]string{
			"help:tellus": heredoc.Doc(`
				Open an issue at https://github.com/scmn-dev/tran/issues
			`),
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			startDir := cmd.Flags().Lookup("start-dir")

			config.LoadConfig(startDir)
			cfg := config.GetConfig()

			m := tui.New()
			var opts []tea.ProgramOption

			// Always append alt screen program option.
			opts = append(opts, tea.WithAltScreen())

			// If mousewheel is enabled, append it to the program options.
			if cfg.Tran.EnableMouseWheel {
				opts = append(opts, tea.WithMouseAllMotion())
			}

			// Initialize and start app.
			p := tea.NewProgram(m, opts...)

			if err := p.Start(); err != nil {
				log.Fatal("Failed to start tran", err)
			}

			return nil
		},
	}

	versionCmd := &cobra.Command{
		Use:   "version",
		Aliases: []string{"ver"},
		Short: "Print the version of your tran binary.",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("tran version " + version + " " + buildDate)
		},
	}

	rootCmd.SetOut(f.IOStreams.Out)
	rootCmd.SetErr(f.IOStreams.ErrOut)

	cs := f.IOStreams.ColorScheme()

	helpHelper := func(command *cobra.Command, args []string) {
		rootHelpFunc(cs, command, args)
	}

	rootCmd.PersistentFlags().Bool("help", false, "Help for tran")
	rootCmd.PersistentFlags().String("start-dir", "", "Starting directory for Tran")
	rootCmd.SetHelpFunc(helpHelper)
	rootCmd.SetUsageFunc(rootUsageFunc)
	rootCmd.SetFlagErrorFunc(rootFlagErrorFunc)

	// Add sub-commands to root command
	rootCmd.AddCommand(
		app.NewAuthCmd,
		app.NewSendCmd,
		app.NewReceiveCmd,
		app.NewGHConfigCmd,
		app.NewGHRepoCmd,
		app.Sync(),
		configCmd.NewConfigCmd(),
		versionCmd,
	)

	return rootCmd
}
