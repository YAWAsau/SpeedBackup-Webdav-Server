package main

import (
	"os"
	"speedbackup-server/internal/desktop"
)

func main() {
	if len(os.Args) != 2 {
		return
	}
	if err := desktop.Run(os.Args[1]); err != nil {
		desktop.ShowError(err)
		os.Exit(1)
	}
}
