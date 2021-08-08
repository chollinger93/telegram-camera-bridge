package cmd

import (
	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/spf13/viper"
)

var cfgFile string

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "telegram-camera-bridge",
	Short: "A bridge between a camera / MotionEye and Telegram",
	Long:  `A bridge between a camera / MotionEye and Telegram`,
	// Uncomment the following line if your bare application
	// has an action associated with it:
	// Run: func(cmd *cobra.Command, args []string) { },
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	cobra.CheckErr(rootCmd.Execute())
}

func init() {
	cobra.OnInitialize(initConfig)
	cobra.OnInitialize(initLogging)
	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "Path to config file yaml")

	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

func printUsage(msg string) {
	zap.S().Errorf(msg)
	zap.S().Fatalf("Usage: TBD")
}

func initLogging() {
	logger, _ := zap.NewProduction()
	zap.ReplaceGlobals(logger)
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		printUsage("--config is required")
	}

	viper.AutomaticEnv() // read in environment variables that match
}
