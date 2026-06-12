package model

import "time"

type PlanRequest struct {
	Goal        string                 `json:"goal"`
	ProjectID   int64                  `json:"project_id,omitempty"`
	EpisodeID   int64                  `json:"episode_id,omitempty"`
	UserID      int64                  `json:"user_id,omitempty"`
	Constraints map[string]any         `json:"constraints,omitempty"`
	Context     map[string]any         `json:"context,omitempty"`
}

type ToolDefinition struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	InputSchema []string `json:"input_schema,omitempty"`
}

type Plan struct {
	PlanID      string         `json:"plan_id"`
	Goal        string         `json:"goal"`
	ProjectID   int64          `json:"project_id,omitempty"`
	EpisodeID   int64          `json:"episode_id,omitempty"`
	Status      string         `json:"status"`
	Agent       string         `json:"agent"`
	Model       string         `json:"model"`
	CreatedAt   time.Time      `json:"created_at"`
	Constraints map[string]any `json:"constraints,omitempty"`
	Context     map[string]any `json:"context,omitempty"`
	Steps       []PlanStep     `json:"steps"`
}

type PlanStep struct {
	ID              string         `json:"id"`
	Agent           string         `json:"agent"`
	Tool            string         `json:"tool"`
	Description     string         `json:"description"`
	Status          string         `json:"status"`
	Input           map[string]any `json:"input,omitempty"`
	DependsOn       []string       `json:"depends_on,omitempty"`
	Expected        []string       `json:"expected_output,omitempty"`
	FailureHints    []string       `json:"failure_hints,omitempty"`
	UseResultFields map[string]any `json:"use_result_fields,omitempty"`
}

type ExecutePlanRequest struct {
	Plan Plan `json:"plan"`
}

type ExecutePlanResponse struct {
	PlanID        string         `json:"plan_id"`
	ExecutionID   string         `json:"execution_id,omitempty"`
	Status        string         `json:"status"`
	Results       []StepResult   `json:"results"`
	ExecutionMeta map[string]any `json:"execution_meta,omitempty"`
}

type RetryExecutionResponse struct {
	SourceExecutionID string               `json:"source_execution_id"`
	Retried           *ExecutePlanResponse `json:"retried"`
}

type ResumeExecutionResponse struct {
	SourceExecutionID string               `json:"source_execution_id"`
	ResumedFromStepID string               `json:"resumed_from_step_id,omitempty"`
	SkippedStepIDs    []string             `json:"skipped_step_ids,omitempty"`
	Resumed           *ExecutePlanResponse `json:"resumed"`
}

type ReplayStepExecutionResponse struct {
	SourceExecutionID string               `json:"source_execution_id"`
	ReplayFromStepID  string               `json:"replay_from_step_id"`
	SkippedStepIDs    []string             `json:"skipped_step_ids,omitempty"`
	Replayed          *ExecutePlanResponse `json:"replayed"`
}

type StepResult struct {
	StepID         string         `json:"step_id"`
	Tool           string         `json:"tool"`
	Status         string         `json:"status"`
	Message        string         `json:"message"`
	Output         map[string]any `json:"output,omitempty"`
	ResolvedInput  map[string]any `json:"resolved_input,omitempty"`
	DependencyInfo []string       `json:"dependency_info,omitempty"`
	StartedAt      time.Time      `json:"started_at"`
	EndedAt        time.Time      `json:"ended_at"`
}
