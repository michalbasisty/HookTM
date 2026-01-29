package main

import (
	"log"
	"os"
	"time"

	"hooktm/internal/cli"
)

func main() {
	app := cli.NewApp()
	app.Writer = os.Stdout
	app.ErrWriter = os.Stderr
	app.Compiled = time.Now()

	args := cli.NormalizeArgs(os.Args)
	if err := app.Run(args); err != nil {
		log.Println(err)
		os.Exit(1)
	}
}
