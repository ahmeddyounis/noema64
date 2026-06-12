package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/ahmedyounis/noema64/internal/appsvc"
	"github.com/ahmedyounis/noema64/internal/experiments"
)

func main() {
	games := flag.Int("games", 0, "number of games; defaults to 100 random games or 20 games per mode")
	seed := flag.Int64("seed", 64, "random seed")
	modes := flag.Bool("modes", false, "run pure, blunderguard, and hybrid mode benchmarks")
	format := flag.String("format", "json", "output format: json or csv")
	timeout := flag.Duration("timeout", 2*time.Minute, "benchmark timeout")
	flag.Parse()
	outputFormat := strings.ToLower(*format)
	if outputFormat != "json" && outputFormat != "csv" {
		fmt.Fprintln(os.Stderr, "format must be json or csv")
		os.Exit(2)
	}

	app := appsvc.NewApplication("")
	type result struct {
		summary any
		err     error
	}
	done := make(chan result, 1)
	go func() {
		gameCount := *games
		if gameCount <= 0 && !*modes {
			gameCount = 100
		}
		if gameCount <= 0 && *modes {
			gameCount = 20
		}
		if *modes {
			summary, err := app.RunModeBenchmark(gameCount, *seed)
			done <- result{summary: summary, err: err}
			return
		}
		summary, err := app.RunRandomBenchmark(gameCount, *seed)
		done <- result{summary: summary, err: err}
	}()

	var res result
	select {
	case res = <-done:
	case <-time.After(*timeout):
		fmt.Fprintln(os.Stderr, "benchmark timed out")
		os.Exit(124)
	}
	if res.err != nil {
		var appErr *appsvc.AppError
		if errors.As(res.err, &appErr) {
			fmt.Fprintln(os.Stderr, appErr.Message)
			os.Exit(1)
		}
		fmt.Fprintln(os.Stderr, res.err)
		os.Exit(1)
	}
	switch outputFormat {
	case "json":
		b, _ := json.MarshalIndent(res.summary, "", "  ")
		fmt.Println(string(b))
	case "csv":
		out, err := benchmarkCSV(res.summary)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Print(out)
	}
}

func benchmarkCSV(summary any) (string, error) {
	switch typed := summary.(type) {
	case experiments.Summary:
		return experiments.SummaryCSV(typed)
	case experiments.ModeBenchmarkSummary:
		return experiments.ModeBenchmarkCSV(typed)
	default:
		return "", fmt.Errorf("unsupported benchmark summary type %T", summary)
	}
}
