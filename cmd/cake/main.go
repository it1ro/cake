package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/it1ro/cake/internal/cli"
	"github.com/it1ro/cake/internal/pipeline"

	_ "github.com/it1ro/cake/internal/processor/golang"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	cli.SetVersion(version, commit, date)
	if err := cli.Execute(); err != nil {
		// Переполнение контекста — отчёт уже напечатан pipeline'ом
		// в stderr, exit 3 (review §D2). Отличается от обычной
		// ошибки (exit 1) на уровне скриптов.
		//
		// Если --report-file не записался, ReportErr печатается
		// следом, но код выхода всё равно 3: «не влезли» — главный
		// факт, «файл не записался» — сопутствующий. Иначе CI,
		// различающий 1 и 3, не увидит 3.
		var oe *pipeline.OverflowError
		if errors.As(err, &oe) {
			if oe.ReportErr != nil {
				fmt.Fprintln(os.Stderr, oe.ReportErr)
			}
			os.Exit(3)
		}
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
