package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/mayvqt/veyra/internal/app"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	var err error
	if len(os.Args) > 1 && os.Args[1] == "backup" {
		if len(os.Args) != 3 {
			_, _ = os.Stderr.WriteString("usage: veyra backup <destination>\n")
			os.Exit(2)
		}
		err = app.Backup(ctx, os.Args[2])
	} else {
		err = app.Run(ctx)
	}
	if err != nil {
		_, _ = os.Stderr.WriteString(err.Error() + "\n")
		os.Exit(1)
	}
}
