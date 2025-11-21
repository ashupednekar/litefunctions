package cmd

import (
	"log"

	"github.com/ashupednekar/litewebservices-portal/pkg/server"
	"github.com/spf13/cobra"
)

var listenCmd = &cobra.Command{
	Use:   "listen",
	Short: "starts http server",
	Long: `
	starts lws portal, a full stack stateless(local state) server
	`,
	Run: func(cmd *cobra.Command, args []string) {
		s, err := server.NewServer()
		if err != nil {
			log.Fatal(err)
		}
		s.Start()
	},
}

func init() {
	rootCmd.AddCommand(listenCmd)
}
