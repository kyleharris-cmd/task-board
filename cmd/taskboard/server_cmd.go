package main

import (
	"net/http"
	"time"

	"github.com/kyleharris/task-board/internal/app"
	"github.com/kyleharris/task-board/internal/httpapi"
	"github.com/spf13/cobra"
)

func newServeCmd(repoRoot *string) *cobra.Command {
	var addr string
	var readTimeout time.Duration
	var writeTimeout time.Duration

	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Run local HTTP API for agent integrations",
		RunE: func(cmd *cobra.Command, args []string) error {
			return withService(*repoRoot, func(svc *app.Service) error {
				api := httpapi.NewServer(svc)
				httpServer := &http.Server{
					Addr:         addr,
					Handler:      api.Handler(),
					ReadTimeout:  readTimeout,
					WriteTimeout: writeTimeout,
				}
				cmd.Printf("taskboard HTTP API listening on %s\n", addr)
				return httpServer.ListenAndServe()
			})
		},
	}

	cmd.Flags().StringVar(&addr, "addr", "127.0.0.1:7327", "bind address")
	cmd.Flags().DurationVar(&readTimeout, "read-timeout", 10*time.Second, "HTTP server read timeout")
	cmd.Flags().DurationVar(&writeTimeout, "write-timeout", 10*time.Second, "HTTP server write timeout")

	return cmd
}
