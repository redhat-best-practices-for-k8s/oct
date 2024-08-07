package main

import (
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/redhat-best-practices-for-k8s/oct/cmd/tnf/fetch"
)

var (
	rootCmd = &cobra.Command{
		Use:   "oct",
		Short: "Offline Catalog Tool for Red Hat's certified artifacts",
	}
)

func main() {
	rootCmd.AddCommand(fetch.NewCommand())

	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}
