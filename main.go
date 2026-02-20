package main

import (
	"github.com/pocketbrain/pocketbrain/cmd"
	"github.com/pocketbrain/pocketbrain/internal/config"
)

func main() {
	_ = config.LoadDotEnvFile(".env")
	cmd.Execute()
}
