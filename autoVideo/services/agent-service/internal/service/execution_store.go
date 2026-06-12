package service

import (
	"context"
	"encoding/json"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/autovideo/agent-service/internal/model"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

type ExecutionRecord struct {
	ExecutionID string             `json:"execution_id"`
	PlanID      string             `json:"plan_id"`
	Status      string             `json:"status"`
	Goal        string             `json:"goal,omitempty"`
	Plan        model.Plan         `json:"plan"`
	Results     []model.StepResult `json:"results"`
	CreatedAt   time.Time          `json:"created_at"`
	UpdatedAt   time.Time          `json:"updated_at"`
	Metadata    map[string]any     `json:"metadata,omitempty"`
}

type ExecutionStore interface {
	Start(plan model.Plan) *ExecutionRecord
	Update(executionID string, status string, results []model.StepResult) *ExecutionRecord
	Get(executionID string) (*ExecutionRecord, bool)
	List(planID string, filter ExecutionListFilter) []ExecutionRecord
}

const defaultExecutionListLimit = 50

type ExecutionListFilter struct {
	Status string
	Limit  int
}

type MemoryExecutionStore struct {
	mu       sync.RWMutex
	byExecID map[string]*ExecutionRecord
	byPlanID map[string][]*ExecutionRecord
}

func NewExecutionStore() ExecutionStore {
	return NewMemoryExecutionStore()
}

func NewMemoryExecutionStore() *MemoryExecutionStore {
	return &MemoryExecutionStore{
		byExecID: make(map[string]*ExecutionRecord),
		byPlanID: make(map[string][]*ExecutionRecord),
	}
}

func (s *MemoryExecutionStore) Start(plan model.Plan) *ExecutionRecord {
	now := time.Now().UTC()
	record := &ExecutionRecord{
		ExecutionID: uuid.NewString(),
		PlanID:      plan.PlanID,
		Status:      "running",
		Goal:        plan.Goal,
		Plan:        plan,
		Results:     []model.StepResult{},
		CreatedAt:   now,
		UpdatedAt:   now,
		Metadata: map[string]any{
			"step_count": len(plan.Steps),
		},
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.byExecID[record.ExecutionID] = record
	s.byPlanID[plan.PlanID] = append(s.byPlanID[plan.PlanID], record)
	return cloneExecutionRecord(record)
}

func (s *MemoryExecutionStore) Update(executionID string, status string, results []model.StepResult) *ExecutionRecord {
	s.mu.Lock()
	defer s.mu.Unlock()
	record, ok := s.byExecID[executionID]
	if !ok {
		return nil
	}
	record.Status = status
	record.Results = cloneStepResults(results)
	record.UpdatedAt = time.Now().UTC()
	return cloneExecutionRecord(record)
}

func (s *MemoryExecutionStore) Get(executionID string) (*ExecutionRecord, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	record, ok := s.byExecID[executionID]
	if !ok {
		return nil, false
	}
	return cloneExecutionRecord(record), true
}

func (s *MemoryExecutionStore) List(planID string, filter ExecutionListFilter) []ExecutionRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()
	src := s.byPlanID[planID]
	items := make([]ExecutionRecord, 0, len(src))
	for _, record := range src {
		if !matchesExecutionFilter(record, filter) {
			continue
		}
		items = append(items, *cloneExecutionRecord(record))
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].CreatedAt.After(items[j].CreatedAt)
	})
	return applyExecutionListLimit(items, filter.Limit)
}

type RedisExecutionStore struct {
	client    *redis.Client
	keyPrefix string
	ttl       time.Duration
}

func NewRedisExecutionStore(client *redis.Client, keyPrefix string, ttl time.Duration) *RedisExecutionStore {
	prefix := strings.TrimSpace(keyPrefix)
	if prefix == "" {
		prefix = "agent:execution:"
	}
	if ttl <= 0 {
		ttl = 48 * time.Hour
	}
	return &RedisExecutionStore{
		client:    client,
		keyPrefix: prefix,
		ttl:       ttl,
	}
}

func (s *RedisExecutionStore) Start(plan model.Plan) *ExecutionRecord {
	now := time.Now().UTC()
	record := &ExecutionRecord{
		ExecutionID: uuid.NewString(),
		PlanID:      plan.PlanID,
		Status:      "running",
		Goal:        plan.Goal,
		Plan:        plan,
		Results:     []model.StepResult{},
		CreatedAt:   now,
		UpdatedAt:   now,
		Metadata: map[string]any{
			"step_count": len(plan.Steps),
		},
	}
	s.save(record)
	return cloneExecutionRecord(record)
}

func (s *RedisExecutionStore) Update(executionID string, status string, results []model.StepResult) *ExecutionRecord {
	record, ok := s.Get(executionID)
	if !ok {
		return nil
	}
	record.Status = status
	record.Results = cloneStepResults(results)
	record.UpdatedAt = time.Now().UTC()
	s.save(record)
	return cloneExecutionRecord(record)
}

func (s *RedisExecutionStore) Get(executionID string) (*ExecutionRecord, bool) {
	ctx := context.Background()
	data, err := s.client.Get(ctx, s.executionKey(executionID)).Bytes()
	if err == redis.Nil || err != nil {
		return nil, false
	}
	var record ExecutionRecord
	if err := json.Unmarshal(data, &record); err != nil {
		return nil, false
	}
	return cloneExecutionRecord(&record), true
}

func (s *RedisExecutionStore) List(planID string, filter ExecutionListFilter) []ExecutionRecord {
	ctx := context.Background()
	ids, err := s.client.SMembers(ctx, s.planKey(planID)).Result()
	if err != nil {
		return []ExecutionRecord{}
	}
	items := make([]ExecutionRecord, 0, len(ids))
	for _, id := range ids {
		record, ok := s.Get(id)
		if ok && matchesExecutionFilter(record, filter) {
			items = append(items, *record)
		}
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].CreatedAt.After(items[j].CreatedAt)
	})
	return applyExecutionListLimit(items, filter.Limit)
}

func (s *RedisExecutionStore) save(record *ExecutionRecord) {
	if s.client == nil || record == nil {
		return
	}
	ctx := context.Background()
	data, err := json.Marshal(record)
	if err != nil {
		return
	}
	pipe := s.client.TxPipeline()
	pipe.Set(ctx, s.executionKey(record.ExecutionID), data, s.ttl)
	pipe.SAdd(ctx, s.planKey(record.PlanID), record.ExecutionID)
	pipe.Expire(ctx, s.planKey(record.PlanID), s.ttl)
	_, _ = pipe.Exec(ctx)
}

func (s *RedisExecutionStore) executionKey(executionID string) string {
	return s.keyPrefix + executionID
}

func (s *RedisExecutionStore) planKey(planID string) string {
	return s.keyPrefix + "plan:" + planID
}

func matchesExecutionFilter(record *ExecutionRecord, filter ExecutionListFilter) bool {
	if record == nil {
		return false
	}
	status := strings.TrimSpace(strings.ToLower(filter.Status))
	if status != "" && strings.ToLower(record.Status) != status {
		return false
	}
	return true
}

func applyExecutionListLimit(items []ExecutionRecord, limit int) []ExecutionRecord {
	if limit <= 0 {
		limit = defaultExecutionListLimit
	}
	if len(items) <= limit {
		return items
	}
	return items[:limit]
}

func cloneExecutionRecord(src *ExecutionRecord) *ExecutionRecord {
	if src == nil {
		return nil
	}
	copy := *src
	copy.Results = cloneStepResults(src.Results)
	copy.Metadata = cloneMap(src.Metadata)
	copy.Plan = clonePlan(src.Plan)
	return &copy
}

func clonePlan(src model.Plan) model.Plan {
	copy := src
	copy.Context = cloneMap(src.Context)
	copy.Constraints = cloneMap(src.Constraints)
	copy.Steps = make([]model.PlanStep, len(src.Steps))
	for i, step := range src.Steps {
		stepCopy := step
		stepCopy.Input = cloneMap(step.Input)
		stepCopy.UseResultFields = cloneMap(step.UseResultFields)
		stepCopy.DependsOn = append([]string(nil), step.DependsOn...)
		stepCopy.Expected = append([]string(nil), step.Expected...)
		stepCopy.FailureHints = append([]string(nil), step.FailureHints...)
		copy.Steps[i] = stepCopy
	}
	return copy
}

func cloneStepResults(src []model.StepResult) []model.StepResult {
	if len(src) == 0 {
		return []model.StepResult{}
	}
	out := make([]model.StepResult, len(src))
	for i, item := range src {
		copy := item
		copy.Output = cloneMap(item.Output)
		copy.ResolvedInput = cloneMap(item.ResolvedInput)
		copy.DependencyInfo = append([]string(nil), item.DependencyInfo...)
		out[i] = copy
	}
	return out
}
