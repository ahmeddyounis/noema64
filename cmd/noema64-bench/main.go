package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/ahmedyounis/noema64/internal/appsvc"
)

func main() {
	games := flag.Int("games", 100, "number of random legal benchmark games")
	seed := flag.Int64("seed", 64, "random seed")
	timeout := flag.Duration("timeout", 2*time.Minute, "benchmark timeout")
	flag.Parse()

	app := appsvc.NewApplication("")
	type result struct {
		summary any
		err     error
	}
	done := make(chan result, 1)
	go func() {
		summary, err := app.RunRandomBenchmark(*games, *seed)
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
	b, _ := json.MarshalIndent(res.summary, "", "  ")
	fmt.Println(string(b))
}
