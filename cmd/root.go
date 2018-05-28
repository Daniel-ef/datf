package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"os/exec"
	"os"
	"log"
	"io/ioutil"
	"time"
	"strings"
)

func RunCmd(cmdName string, cmdArgs []string) {
	env := os.Environ()
	env[7] = strings.Replace(env[7], "/test", "", 1)
	log.Printf(env[7])

	cmd := exec.Command(cmdName, cmdArgs...)
	cmd.Env = env

	cmdReader, err := cmd.StdoutPipe()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error creating StdoutPipe for Cmd", err)
		os.Exit(1)
	}


	err = cmd.Start()
	log.Printf("Command started")
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error starting Cmd", err)
		os.Exit(1)
	}

	grepBytes, _ := ioutil.ReadAll(cmdReader)
	time.Sleep(1000 * time.Millisecond)
	log.Printf(string(grepBytes))

	stdoutStderr, err := cmd.CombinedOutput()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%s\n", stdoutStderr)


	err = cmd.Wait()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error waiting for Cmd", err)
		os.Exit(1)
	}
}

var RootCmd = &cobra.Command{
	Use:   "hse-dss",
	Short: "HSE Distributed Systems Seminar",
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
}

var cfgFile string

func init() {
	cobra.OnInitialize(initConfig)

	RootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "configuration file (default: $HOME/.hse-dss-efimov.yaml)")
}

func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	}

	viper.SetConfigName("config")
	viper.AddConfigPath("/etc/hse-dss-efimov")
	viper.AddConfigPath("$HOME/.hse-dss-efimov")
	viper.AddConfigPath(".")

	viper.SetEnvPrefix("HSEDSS")
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err == nil {
		fmt.Println("Using configuration file: " + viper.ConfigFileUsed())
	}
}
