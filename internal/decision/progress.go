package decision

import "time"

const DecisionStageEvent = "decision.stage"

type StageTrace struct {
	Name       string `json:"name"`
	Status     string `json:"status"`
	Message    string `json:"message,omitempty"`
	StartedAt  string `json:"started_at,omitempty"`
	FinishedAt string `json:"finished_at,omitempty"`
	DurationMS int64  `json:"duration_ms"`
}

type ProgressEvent struct {
	EventName  string `json:"event_name"`
	DecisionID string `json:"decision_id"`
	GameID     string `json:"game_id,omitempty"`
	Stage      string `json:"stage"`
	Status     string `json:"status"`
	Message    string `json:"message,omitempty"`
	ElapsedMS  int64  `json:"elapsed_ms"`
	Timestamp  string `json:"timestamp"`
}

type ProgressFunc func(ProgressEvent)

type stageRecorder struct {
	start      time.Time
	decisionID string
	gameID     string
	progress   ProgressFunc
	traces     []StageTrace
}

type activeStage struct {
	rec     *stageRecorder
	name    string
	message string
	started time.Time
}

func newStageRecorder(start time.Time, decisionID string, progress ProgressFunc) *stageRecorder {
	return &stageRecorder{start: start, decisionID: decisionID, progress: progress}
}

func (r *stageRecorder) setGameID(gameID string) {
	r.gameID = gameID
}

func (r *stageRecorder) begin(name, message string) activeStage {
	now := time.Now()
	r.emit(name, "started", message, now)
	return activeStage{rec: r, name: name, message: message, started: now}
}

func (s activeStage) finish(status, message string) {
	if s.rec == nil {
		return
	}
	if message == "" {
		message = s.message
	}
	now := time.Now()
	s.rec.traces = append(s.rec.traces, CompletedStage(s.name, status, message, s.started, now))
	s.rec.emit(s.name, status, message, now)
}

func (r *stageRecorder) snapshot() []StageTrace {
	return append([]StageTrace(nil), r.traces...)
}

func (r *stageRecorder) emit(stage, status, message string, at time.Time) {
	if r.progress == nil {
		return
	}
	r.progress(ProgressEvent{
		EventName:  DecisionStageEvent,
		DecisionID: r.decisionID,
		GameID:     r.gameID,
		Stage:      stage,
		Status:     status,
		Message:    message,
		ElapsedMS:  at.Sub(r.start).Milliseconds(),
		Timestamp:  at.UTC().Format(time.RFC3339Nano),
	})
}

func CompletedStage(name, status, message string, started, finished time.Time) StageTrace {
	if status == "" {
		status = "completed"
	}
	if finished.Before(started) {
		finished = started
	}
	return StageTrace{
		Name:       name,
		Status:     status,
		Message:    message,
		StartedAt:  started.UTC().Format(time.RFC3339Nano),
		FinishedAt: finished.UTC().Format(time.RFC3339Nano),
		DurationMS: finished.Sub(started).Milliseconds(),
	}
}
