package experiments

import (
	"bytes"
	"encoding/csv"
	"io"
	"strconv"
)

var benchmarkCSVHeader = []string{
	"benchmark",
	"mode",
	"game_index",
	"plies",
	"outcome",
	"adjudicated",
	"fallbacks_used",
	"engine_error",
	"games_requested",
	"games_completed",
	"adjudicated_games",
	"illegal_final_moves",
	"engine_errors",
	"total_plies",
	"duration_ms",
}

func SummaryCSV(summary Summary) (string, error) {
	var b bytes.Buffer
	if err := WriteSummaryCSV(&b, "random", "", summary); err != nil {
		return "", err
	}
	return b.String(), nil
}

func ModeBenchmarkCSV(summary ModeBenchmarkSummary) (string, error) {
	var b bytes.Buffer
	writer := csv.NewWriter(&b)
	if err := writer.Write(benchmarkCSVHeader); err != nil {
		return "", err
	}
	for _, result := range summary.Results {
		if err := writeSummaryRows(writer, "mode", string(result.Mode), result.Summary); err != nil {
			return "", err
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return "", err
	}
	return b.String(), nil
}

func WriteSummaryCSV(w io.Writer, benchmark string, mode string, summary Summary) error {
	writer := csv.NewWriter(w)
	if err := writer.Write(benchmarkCSVHeader); err != nil {
		return err
	}
	if err := writeSummaryRows(writer, benchmark, mode, summary); err != nil {
		return err
	}
	writer.Flush()
	return writer.Error()
}

func writeSummaryRows(writer *csv.Writer, benchmark string, mode string, summary Summary) error {
	if len(summary.Results) == 0 {
		return writer.Write(summaryCSVRow(benchmark, mode, summary, GameSummary{}))
	}
	for _, result := range summary.Results {
		if err := writer.Write(summaryCSVRow(benchmark, mode, summary, result)); err != nil {
			return err
		}
	}
	return nil
}

func summaryCSVRow(benchmark string, mode string, summary Summary, result GameSummary) []string {
	return []string{
		benchmark,
		mode,
		strconv.Itoa(result.GameIndex),
		strconv.Itoa(result.Plies),
		result.Outcome,
		strconv.FormatBool(result.Adjudicated),
		strconv.Itoa(result.FallbacksUsed),
		result.EngineError,
		strconv.Itoa(summary.GamesRequested),
		strconv.Itoa(summary.GamesCompleted),
		strconv.Itoa(summary.AdjudicatedGames),
		strconv.Itoa(summary.IllegalFinalMoves),
		strconv.Itoa(summary.EngineErrors),
		strconv.Itoa(summary.TotalPlies),
		strconv.FormatInt(summary.DurationMS, 10),
	}
}
