package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/ahmedyounis/noema64/internal/appsvc"
)

func main() {
	cmd := flag.String("cmd", "state", "state, move, engine, pgn, fen")
	move := flag.String("move", "", "UCI move for -cmd move")
	flag.Parse()

	app := appsvc.NewApplication("")
	switch *cmd {
	case "state":
		state, err := app.GetGame()
		exitOnAppErr(err)
		printJSON(state)
	case "move":
		if *move == "" {
			fmt.Fprintln(os.Stderr, "-move is required")
			os.Exit(2)
		}
		state, err := app.MakeUserMove(*move)
		exitOnAppErr(err)
		printJSON(state)
	case "engine":
		result, err := app.RequestEngineMove()
		exitOnAppErr(err)
		printJSON(result)
	case "pgn":
		pgn, err := app.ExportPGN()
		exitOnAppErr(err)
		fmt.Println(pgn)
	case "fen":
		fen, err := app.ExportFEN()
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

func exitOnAppErr(err error) {
	if err != nil {
		var appErr *appsvc.AppError
		if errors.As(err, &appErr) {
			fmt.Fprintln(os.Stderr, appErr.Message)
			os.Exit(1)
		}
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
