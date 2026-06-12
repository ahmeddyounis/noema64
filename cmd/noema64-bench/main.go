package main

import (
	"context"
	"encoding/json"
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

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()
	app := appsvc.NewApplication("")
	summary, appErr := app.RunRandomBenchmark(ctx, *games, *seed)
	if appErr != nil {
		fmt.Fprintln(os.Stderr, appErr.Message)
		os.Exit(1)
	}
	b, _ := json.MarshalIndent(summary, "", "  ")
	fmt.Println(string(b))
}
