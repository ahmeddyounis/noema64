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

type PolicyPriorTrainingResult struct {
	ModelPath string                    `json:"model_path"`
	Model     finetune.PolicyPriorModel `json:"model"`
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

func (a *Application) TrainLocalPolicyPrior(datasetJSONL string, outputPath string) (PolicyPriorTrainingResult, error) {
	datasetJSONL = strings.TrimSpace(datasetJSONL)
	if datasetJSONL == "" {
		return PolicyPriorTrainingResult{}, &AppError{Code: "ERR_FINE_TUNE_TRAIN", Message: "Fine-tune JSONL is required", Recoverable: true}
	}
	outputPath = strings.TrimSpace(outputPath)
	if outputPath == "" {
		outputPath = filepath.Join(a.settings.Logging.OutputDir, "policy-prior-model.json")
	}
	model, err := finetune.TrainPolicyPriorJSONL(datasetJSONL)
	if err != nil {
		return PolicyPriorTrainingResult{}, appErr("ERR_FINE_TUNE_TRAIN", err, true)
	}
	if err := finetune.SavePolicyPriorModel(outputPath, model); err != nil {
		return PolicyPriorTrainingResult{}, appErr("ERR_FINE_TUNE_TRAIN", err, true)
	}
	return PolicyPriorTrainingResult{ModelPath: outputPath, Model: model}, nil
}

func (a *Application) EnablePolicyPriorModel(modelPath string) (storage.Settings, error) {
	modelPath = strings.TrimSpace(modelPath)
	if modelPath == "" {
		return storage.Settings{}, &AppError{Code: "ERR_POLICY_PRIOR", Message: "Policy-prior model path is required", Recoverable: true}
	}
	if _, err := finetune.LoadPolicyPriorModel(modelPath); err != nil {
		return storage.Settings{}, appErr("ERR_POLICY_PRIOR", err, true)
	}
	settings := cloneSettings(a.settings)
	settings.LLM.Provider = "policy_prior"
	settings.LLM.Model = modelPath
	settings.LLM.Endpoint = ""
	settings.LLM.ProfileID = "policy-prior"
	settings.LLM.Profiles = upsertProviderProfile(settings.LLM.Profiles, storage.ProviderProfile{
		ID:          "policy-prior",
		Provider:    "policy_prior",
		Mode:        string(settings.Engine.DefaultMode),
		IntendedUse: "local_distilled_policy_prior",
		Model:       modelPath,
		Temperature: settings.LLM.Temperature,
		MaxTokens:   settings.LLM.MaxTokens,
		TimeoutMS:   settings.LLM.TimeoutMS,
		Retries:     settings.LLM.Retries,
	})
	if err := storage.SaveSettings(a.settingsPath, settings); err != nil {
		return storage.Settings{}, appErr("ERR_SETTINGS_INVALID", err, true)
	}
	a.settings = storage.NormalizeSettings(settings)
	a.engine.SetOptions(a.engineOptions())
	return a.GetSettings()
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

func upsertProviderProfile(profiles []storage.ProviderProfile, profile storage.ProviderProfile) []storage.ProviderProfile {
	out := append([]storage.ProviderProfile(nil), profiles...)
	for i := range out {
		if out[i].ID == profile.ID {
			out[i] = profile
			return out
		}
	}
	return append(out, profile)
}
