package appsvc

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/ahmedyounis/noema64/internal/decision"
	"github.com/ahmedyounis/noema64/internal/experiments"
	"github.com/ahmedyounis/noema64/internal/finetune"
	"github.com/ahmedyounis/noema64/internal/storage"
)

type FineTuneWorkflow struct {
	DatasetJSONL string                `json:"dataset_jsonl"`
	Workflow     finetune.WorkflowSpec `json:"workflow"`
}

func (a *Application) CreateBackup(outputDir string) (storage.BackupManifest, error) {
	if strings.TrimSpace(outputDir) == "" {
		outputDir = filepath.Join(a.settings.Logging.OutputDir, "backups")
	}
	manifest, err := storage.CreateBackup(context.Background(), storage.BackupRequest{
		SettingsPath: resolvedSettingsPath(a.settingsPath),
		LogDir:       a.settings.Logging.OutputDir,
		OutputDir:    outputDir,
	})
	return manifest, appErr("ERR_BACKUP", err, true)
}

func (a *Application) RestoreBackup(archivePath string, targetDir string) (storage.BackupManifest, error) {
	manifest, err := storage.RestoreBackup(context.Background(), archivePath, targetDir)
	return manifest, appErr("ERR_BACKUP_RESTORE", err, true)
}

func (a *Application) ExportFineTuneDataset() (FineTuneWorkflow, error) {
	state, err := a.engine.State(context.Background())
	if err != nil {
		return FineTuneWorkflow{}, appErr("ERR_GAME_STATE", err, true)
	}
	trace, err := a.traces.ReadGame(context.Background(), state.Snapshot.GameID)
	if err == nil && strings.TrimSpace(trace) != "" {
		jsonl, workflow, exportErr := finetune.ExportTraceJSONL(trace)
		if exportErr == nil && workflow.ExampleCount > 0 {
			return FineTuneWorkflow{DatasetJSONL: jsonl, Workflow: workflow}, nil
		}
	}
	if state.LastDecision == nil {
		return FineTuneWorkflow{Workflow: finetune.NewWorkflowSpec(0)}, nil
	}
	jsonl, workflow, err := finetune.ExportDecisionsJSONL([]decision.MoveDecision{*state.LastDecision})
	return FineTuneWorkflow{DatasetJSONL: jsonl, Workflow: workflow}, appErr("ERR_FINE_TUNE_EXPORT", err, true)
}

func (a *Application) RunTournament(gamesPerPair int, seed int64) (experiments.TournamentSummary, error) {
	if gamesPerPair <= 0 {
		gamesPerPair = 2
	}
	opts := a.engineOptions()
	runner := experiments.Runner{Options: opts, MaxPlies: 80}
	summary, err := runner.Tournament(context.Background(), experiments.DefaultTournamentEntrants(), gamesPerPair, seed)
	return summary, appErr("ERR_EXPERIMENT", err, true)
}

func resolvedSettingsPath(path string) string {
	if strings.TrimSpace(path) != "" {
		return path
	}
	dir, err := storage.ConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "config.yaml")
}
