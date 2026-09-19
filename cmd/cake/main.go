package main

import (
	"fmt"
	"os"

	"github.com/it1ro/cake/internal/cli"

	// Регистрация процессоров через init().
	// Порядок не важен, но перечислить все нужно здесь явно,
	// чтобы линтер не удалял import.
	_ "github.com/it1ro/cake/internal/processor/golang"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	if err := cli.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
