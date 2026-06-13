package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/ahmedyounis/noema64/internal/appsvc"
	"github.com/ahmedyounis/noema64/internal/chesscore"
	"github.com/ahmedyounis/noema64/internal/engine"
)

func main() {
	cmd := flag.String("cmd", "state", "state, move, engine, analyze, pgn, fen, trace, debug-trace")
	move := flag.String("move", "", "UCI move for -cmd move")
	fen := flag.String("fen", "", "FEN to load before running the command")
	pgn := flag.String("pgn", "", "PGN to load before running the command")
	variant := flag.String("variant", "", "starting variant for a new game: standard, chess960, or custom")
	seed := flag.Int64("seed", 0, "Chess960 start index/seed; normalized to 0-959")
	flag.Parse()

	app := appsvc.NewApplication("")
	if *variant != "" {
		state, err := app.NewGame(engine.NewGameOptions{Variant: chesscore.Variant(*variant), Seed: *seed, FEN: *fen, Side: "auto"})
		exitOnAppErr(err)
		if *cmd == "state" {
			printJSON(state)
			return
		}
	} else if *fen != "" {
		_, err := app.ImportFEN(*fen)
		exitOnAppErr(err)
	}
	if *pgn != "" {
		_, err := app.ImportPGN(*pgn)
		exitOnAppErr(err)
	}
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
	case "analyze":
		decision, err := app.AnalyzeCurrentPosition()
		exitOnAppErr(err)
		printJSON(decision)
	case "pgn":
		pgn, err := app.ExportPGN()
		exitOnAppErr(err)
		fmt.Println(pgn)
	case "fen":
		fen, err := app.ExportFEN()
		exitOnAppErr(err)
		fmt.Println(fen)
	case "trace":
		trace, err := app.ExportTrace()
		exitOnAppErr(err)
		fmt.Print(trace)
	case "debug-trace":
		trace, err := app.ExportDebugTrace()
		exitOnAppErr(err)
		fmt.Print(trace)
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
