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
	cmd := flag.String("cmd", "state", "state, move, engine, analyze, pgn, fen, trace, debug-trace, study, agents, backup, restore-backup, finetune, tournament")
	move := flag.String("move", "", "UCI move for -cmd move")
	fen := flag.String("fen", "", "FEN to load before running the command")
	pgn := flag.String("pgn", "", "PGN to load before running the command")
	variant := flag.String("variant", "", "starting variant for a new game: standard, chess960, or custom")
	seed := flag.Int64("seed", 0, "Chess960 start index/seed; normalized to 0-959")
	backupDir := flag.String("backup-dir", "", "Directory for -cmd backup output")
	restoreArchive := flag.String("restore-archive", "", "Backup archive path for -cmd restore-backup")
	restoreTarget := flag.String("restore-target", "", "Restore target directory for -cmd restore-backup")
	gamesPerPair := flag.Int("games-per-pair", 1, "Tournament games per pairing for -cmd tournament")
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
	case "study":
		dashboard, err := app.StudyDashboard()
		exitOnAppErr(err)
		printJSON(dashboard)
	case "agents":
		review, err := app.MultiAgentAnalysis()
		exitOnAppErr(err)
		printJSON(review)
	case "backup":
		manifest, err := app.CreateBackup(*backupDir)
		exitOnAppErr(err)
		printJSON(manifest)
	case "restore-backup":
		if *restoreArchive == "" || *restoreTarget == "" {
			fmt.Fprintln(os.Stderr, "-restore-archive and -restore-target are required")
			os.Exit(2)
		}
		manifest, err := app.RestoreBackup(*restoreArchive, *restoreTarget)
		exitOnAppErr(err)
		printJSON(manifest)
	case "finetune":
		workflow, err := app.ExportFineTuneDataset()
		exitOnAppErr(err)
		printJSON(workflow)
	case "tournament":
		summary, err := app.RunTournament(*gamesPerPair, *seed)
		exitOnAppErr(err)
		printJSON(summary)
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
