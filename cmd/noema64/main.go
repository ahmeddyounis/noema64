package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/ahmedyounis/noema64/internal/appsvc"
)

func main() {
	cmd := flag.String("cmd", "state", "state, move, engine, pgn, fen")
	move := flag.String("move", "", "UCI move for -cmd move")
	flag.Parse()

	ctx := context.Background()
	app := appsvc.NewApplication("")
	switch *cmd {
	case "state":
		state, err := app.GetGame(ctx)
		exitOnAppErr(err)
		printJSON(state)
	case "move":
		if *move == "" {
			fmt.Fprintln(os.Stderr, "-move is required")
			os.Exit(2)
		}
		state, err := app.MakeUserMove(ctx, *move)
		exitOnAppErr(err)
		printJSON(state)
	case "engine":
		result, err := app.RequestEngineMove(ctx)
		exitOnAppErr(err)
		printJSON(result)
	case "pgn":
		pgn, err := app.ExportPGN(ctx)
		exitOnAppErr(err)
		fmt.Println(pgn)
	case "fen":
		fen, err := app.ExportFEN(ctx)
		exitOnAppErr(err)
		fmt.Println(fen)
	default:
		fmt.Fprintln(os.Stderr, "unknown command:", *cmd)
		os.Exit(2)
	}
}

func printJSON(v any) {
	b, _ := json.MarshalIndent(v, "", "  ")
	fmt.Println(string(b))
}

func exitOnAppErr(err *appsvc.AppError) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Message)
		os.Exit(1)
	}
}
