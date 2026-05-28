package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"gorm.io/datatypes"

	"github.com/autovideo/project-service/internal/model"
	"github.com/autovideo/project-service/internal/stylepreset"
)

const defaultAdCopyOptimizationPrompt = `你是广告短视频编剧、导演统筹和连续性审校。你的任务不是直接分集，而是先把整篇广告文案优化成更适合后续“自动切分成多个视频片段”的中间稿，并补出后续生成时必须遵守的一致性前提。

必须遵守：
- 保留原始产品卖点、人物设定、核心承诺与事实信息，不得胡编功效。
- 按当前目标风格重写语言与镜头感，使文案更适合后续广告视频生成。
- 必须主动补全并澄清以下 14 个维度：1）世界观/故事发生的视觉宇宙；2）空间（在哪里）；3）时间（几点/昼夜/时序）；4）人物（谁）；5）服装（穿什么）；6）动作（做什么）；7）核心物件/镜头重点；8）光线（怎么打光）；9）色彩（什么色调）；10）材质（表面质感）；11）镜头运动（怎么拍）；12）情绪（传达什么感觉）；13）转场（怎么切）；14）字幕/屏幕文字、配音/口播内容、以及最终给 AI 的生成 Prompt 描述。
- optimized_script 必须是可直接用于后续自动分集的广告正文；但文中要自然包含这些维度所需的信息，不要只给抽象概念。
- consistency_premise 必须单独总结以上 14 个维度里“后续不得漂移”的硬约束，写成清晰条目。
- 把长段落整理成更自然的口播 / 画面节奏单元，让后续系统更容易按时长自动切分。
- 段落之间要有清楚转场，避免一句话承载过多镜头。
- 如果是写实风格，优先真实场景、生活化表达、自然口语；如果是动漫风格，允许更鲜明的视觉感，但不要失去广告转化目标。
- 不要输出分集编号，不要显式写“第一段/第二段”，只输出优化后的完整文案。
- 要明确区分：哪些是画面信息、哪些是台词/配音、哪些是屏幕字幕、哪些是最终喂给模型的视觉 Prompt 重点。`

type AdCopyOptimizeRequest struct {
	OriginalScript     string `json:"original_script"`
	OptimizationPrompt string `json:"optimization_prompt"`
	PersistOriginal    bool   `json:"persist_original"`
}

type AdCopySaveRequest struct {
	OriginalScript        string `json:"original_script"`
	OptimizationPrompt    string `json:"optimization_prompt"`
	OptimizedScript       string `json:"optimized_script"`
	StoryboardSplitPrompt string `json:"storyboard_split_prompt"`
	PersistOriginal       bool   `json:"persist_original"`
}

type AdCopyOptimizeResponse struct {
	OriginalScript            string `json:"original_script"`
	OptimizationPrompt        string `json:"optimization_prompt"`
	OptimizedScript           string `json:"optimized_script"`
	ConsistencyPremise        string `json:"consistency_premise,omitempty"`
	ScriptLength              int    `json:"script_length,omitempty"`
	StoryboardSplitPrompt     string `json:"storyboard_split_prompt,omitempty"`
	StoryboardSplitPromptHint string `json:"storyboard_split_prompt_hint,omitempty"`
}

func normalizeAdCopyPrompt(raw string) string {
	if trimmed := strings.TrimSpace(raw); trimmed != "" {
		return trimmed
	}
	return defaultAdCopyOptimizationPrompt
}

func (s *EpisodeService) currentStoryboardConfigMap(project *model.Project) map[string]interface{} {
	cfg := map[string]interface{}{}
	if project == nil || len(project.StoryboardConfig) == 0 {
		return cfg
	}
	_ = json.Unmarshal(project.StoryboardConfig, &cfg)
	return cfg
}

func (s *EpisodeService) currentAdCopyOptimizationPrompt(project *model.Project) string {
	cfg := s.currentStoryboardConfigMap(project)
	if value, ok := cfg["ad_copy_optimization_prompt"].(string); ok {
		return normalizeAdCopyPrompt(value)
	}
	return defaultAdCopyOptimizationPrompt
}

func (s *EpisodeService) currentStoryboardSplitPrompt(project *model.Project) string {
	cfg := s.currentStoryboardConfigMap(project)
	if value, ok := cfg["storyboard_split_prompt"].(string); ok {
		return strings.TrimSpace(value)
	}
	return ""
}

func (s *EpisodeService) persistStoryboardPromptConfig(project *model.Project, updates map[string]string) error {
	if project == nil {
		return errors.New("project is nil")
	}
	cfg := s.currentStoryboardConfigMap(project)
	for key, value := range updates {
		if key == "ad_copy_optimization_prompt" {
			cfg[key] = normalizeAdCopyPrompt(value)
			continue
		}
		cfg[key] = strings.TrimSpace(value)
	}
	data, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	project.StoryboardConfig = datatypes.JSON(data)
	return s.projectRepo.Update(project)
}

func (s *EpisodeService) updateAdCopyProgress(project *model.Project, originalScript, optimizationPrompt string, result *optimizedAdScriptResult) error {
	if project == nil {
		return errors.New("project is nil")
	}
	var progress ProgressInfo
	if len(project.Progress) > 0 {
		_ = json.Unmarshal(project.Progress, &progress)
	}
	trimmedOriginal := strings.TrimSpace(originalScript)
	trimmedPrompt := normalizeAdCopyPrompt(optimizationPrompt)
	trimmedOptimized := ""
	trimmedConsistency := ""
	if result != nil {
		trimmedOptimized = strings.TrimSpace(result.OptimizedScript)
		trimmedConsistency = strings.TrimSpace(result.ConsistencyPremise)
	}

	runtimeCfg := parseStoryboardRuntimeConfig(project)
	meta := buildAutoSplitMeta(trimmedOptimized, runtimeCfg)
	meta.Enabled = runtimeCfg.AutoSplitAfterOptimization
	meta.VideoModel = runtimeCfg.VideoModel
	meta.StylePreset = stylepreset.Canonical(runtimeCfg.StylePreset)
	if meta.StylePreset == "" {
		meta.StylePreset = stylepreset.Default
	}
	if runtimeCfg.Duration > 0 {
		meta.Duration = runtimeCfg.Duration
	}
	meta.OriginalScript = trimmedOriginal
	meta.OptimizedScript = trimmedOptimized
	meta.ConsistencyPremise = trimmedConsistency
	meta.ScriptLength = utf8.RuneCountInString(trimmedOptimized)
	meta.OptimizationPrompt = trimmedPrompt

	progress.AutoSplit = &meta
	if progress.Stage == "" {
		progress.Stage = "idle"
	}
	progress.UpdatedAt = ""
	s.updateProgress(project.ID, progress)
	return nil
}

func (s *EpisodeService) saveAdCopyDraft(project *model.Project, originalScript, optimizationPrompt, optimizedScript string, keepConsistency string) error {
	if project == nil {
		return errors.New("project is nil")
	}
	var progress ProgressInfo
	if len(project.Progress) > 0 {
		_ = json.Unmarshal(project.Progress, &progress)
	}
	trimmedOriginal := strings.TrimSpace(originalScript)
	trimmedPrompt := normalizeAdCopyPrompt(optimizationPrompt)
	trimmedOptimized := strings.TrimSpace(optimizedScript)
	trimmedConsistency := strings.TrimSpace(keepConsistency)
	if progress.AutoSplit != nil && trimmedConsistency == "" {
		trimmedConsistency = strings.TrimSpace(progress.AutoSplit.ConsistencyPremise)
	}

	runtimeCfg := parseStoryboardRuntimeConfig(project)
	meta := buildAutoSplitMeta(trimmedOptimized, runtimeCfg)
	meta.Enabled = runtimeCfg.AutoSplitAfterOptimization
	meta.VideoModel = runtimeCfg.VideoModel
	meta.StylePreset = stylepreset.Canonical(runtimeCfg.StylePreset)
	if meta.StylePreset == "" {
		meta.StylePreset = stylepreset.Default
	}
	if runtimeCfg.Duration > 0 {
		meta.Duration = runtimeCfg.Duration
	}
	meta.OriginalScript = trimmedOriginal
	meta.OptimizedScript = trimmedOptimized
	meta.ConsistencyPremise = trimmedConsistency
	meta.ScriptLength = utf8.RuneCountInString(trimmedOptimized)
	meta.OptimizationPrompt = trimmedPrompt

	progress.AutoSplit = &meta
	if progress.Stage == "" {
		progress.Stage = "idle"
	}
	progress.UpdatedAt = ""
	s.updateProgress(project.ID, progress)
	return nil
}

func (s *EpisodeService) GetAdCopyOptimizationState(projectID uint64) (*AdCopyOptimizeResponse, error) {
	project, err := s.projectRepo.FindByIDNoAuth(projectID)
	if err != nil {
		return nil, fmt.Errorf("project not found: %w", err)
	}
	if err := s.hydrateScriptTextIfNeeded(project); err != nil {
		return nil, err
	}
	prompt := s.currentAdCopyOptimizationPrompt(project)
	var progress ProgressInfo
	if len(project.Progress) > 0 {
		_ = json.Unmarshal(project.Progress, &progress)
	}
	optimized := strings.TrimSpace(progress.AutoSplit.GetOptimizedScript())
	consistency := strings.TrimSpace(progress.AutoSplit.GetConsistencyPremise())
	originalScript := strings.TrimSpace(progress.AutoSplit.GetOriginalScript())
	if originalScript == "" {
		originalScript = strings.TrimSpace(project.ScriptText)
	}
	return &AdCopyOptimizeResponse{
		OriginalScript:            originalScript,
		OptimizationPrompt:        prompt,
		OptimizedScript:           optimized,
		ConsistencyPremise:        consistency,
		ScriptLength:              utf8.RuneCountInString(optimized),
		StoryboardSplitPrompt:     s.currentStoryboardSplitPrompt(project),
		StoryboardSplitPromptHint: "这里填写的是‘步骤 1 文本重拆分前，给分镜拆分模型的附加规则’。默认会叠加系统内置的广告口播拆分规则；你在这里写的是项目级补充规则，会在真正 scene split 前注入。",
	}, nil
}

func (s *EpisodeService) SaveAdCopyDraft(projectID uint64, req AdCopySaveRequest) (*AdCopyOptimizeResponse, error) {
	project, err := s.projectRepo.FindByIDNoAuth(projectID)
	if err != nil {
		return nil, fmt.Errorf("project not found: %w", err)
	}
	if err := s.hydrateScriptTextIfNeeded(project); err != nil {
		return nil, err
	}
	originalScript := strings.TrimSpace(req.OriginalScript)
	if originalScript == "" {
		originalScript = strings.TrimSpace(project.ScriptText)
	}
	prompt := normalizeAdCopyPrompt(req.OptimizationPrompt)
	optimizedScript := strings.TrimSpace(req.OptimizedScript)

	if err := s.persistStoryboardPromptConfig(project, map[string]string{"ad_copy_optimization_prompt": prompt, "storyboard_split_prompt": req.StoryboardSplitPrompt}); err != nil {
		return nil, err
	}
	if req.PersistOriginal {
		project.ScriptText = originalScript
		if err := s.projectRepo.Update(project); err != nil {
			return nil, err
		}
	}
	var progress ProgressInfo
	if len(project.Progress) > 0 {
		_ = json.Unmarshal(project.Progress, &progress)
	}
	if err := s.saveAdCopyDraft(project, originalScript, prompt, optimizedScript, progress.AutoSplit.GetConsistencyPremise()); err != nil {
		return nil, err
	}
	return &AdCopyOptimizeResponse{
		OriginalScript:            originalScript,
		OptimizationPrompt:        prompt,
		OptimizedScript:           optimizedScript,
		ConsistencyPremise:        progress.AutoSplit.GetConsistencyPremise(),
		ScriptLength:              utf8.RuneCountInString(optimizedScript),
		StoryboardSplitPrompt:     s.currentStoryboardSplitPrompt(project),
		StoryboardSplitPromptHint: "这里填写的是‘步骤 1 文本重拆分前，给分镜拆分模型的附加规则’。默认会叠加系统内置的广告口播拆分规则；你在这里写的是项目级补充规则，会在真正 scene split 前注入。",
	}, nil
}

func (s *EpisodeService) OptimizeAdCopy(ctx context.Context, projectID uint64, req AdCopyOptimizeRequest) (*AdCopyOptimizeResponse, error) {
	project, err := s.projectRepo.FindByIDNoAuth(projectID)
	if err != nil {
		return nil, fmt.Errorf("project not found: %w", err)
	}
	if err := s.hydrateScriptTextIfNeeded(project); err != nil {
		return nil, err
	}
	originalScript := strings.TrimSpace(req.OriginalScript)
	if originalScript == "" {
		originalScript = strings.TrimSpace(project.ScriptText)
	}
	if originalScript == "" {
		return nil, errors.New("original script is empty")
	}
	prompt := normalizeAdCopyPrompt(req.OptimizationPrompt)
	result, err := s.optimizeProjectScriptForAutoSplit(ctx, project, originalScript, prompt)
	if err != nil {
		return nil, err
	}

	if req.PersistOriginal {
		project.ScriptText = originalScript
	}
	if err := s.persistStoryboardPromptConfig(project, map[string]string{"ad_copy_optimization_prompt": prompt}); err != nil {
		return nil, err
	}
	if req.PersistOriginal {
		project.ScriptText = originalScript
		if err := s.projectRepo.Update(project); err != nil {
			return nil, err
		}
	}
	if err := s.updateAdCopyProgress(project, originalScript, prompt, result); err != nil {
		return nil, err
	}

	return &AdCopyOptimizeResponse{
		OriginalScript:            originalScript,
		OptimizationPrompt:        prompt,
		OptimizedScript:           strings.TrimSpace(result.OptimizedScript),
		ConsistencyPremise:        strings.TrimSpace(result.ConsistencyPremise),
		ScriptLength:              utf8.RuneCountInString(strings.TrimSpace(result.OptimizedScript)),
		StoryboardSplitPrompt:     s.currentStoryboardSplitPrompt(project),
		StoryboardSplitPromptHint: "这里填写的是‘步骤 1 文本重拆分前，给分镜拆分模型的附加规则’。默认会叠加系统内置的广告口播拆分规则；你在这里写的是项目级补充规则，会在真正 scene split 前注入。",
	}, nil
}

func (s *EpisodeService) hydrateScriptTextIfNeeded(project *model.Project) error {
	if project == nil || project.ScriptFileURL == "" || project.ScriptText != "" {
		return nil
	}
	return s.fetchAndPersistProjectScript(project)
}

func (s *EpisodeService) fetchAndPersistProjectScript(project *model.Project) error {
	if project == nil || project.ScriptFileURL == "" {
		return nil
	}
	req, err := http.NewRequest(http.MethodGet, project.ScriptFileURL, nil)
	if err != nil {
		return fmt.Errorf("build script fetch request: %w", err)
	}
	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("fetch script file: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("fetch script file: status %d", resp.StatusCode)
	}
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read script file: %w", err)
	}
	decoded := string(bytes.TrimSpace(raw))
	project.ScriptText = decoded
	if err := s.projectRepo.Update(project); err != nil {
		return fmt.Errorf("persist decoded script text: %w", err)
	}
	return nil
}

func (m *AutoSplitMeta) GetOriginalScript() string {
	if m == nil {
		return ""
	}
	return m.OriginalScript
}

func (m *AutoSplitMeta) GetOptimizedScript() string {
	if m == nil {
		return ""
	}
	return m.OptimizedScript
}

func (m *AutoSplitMeta) GetConsistencyPremise() string {
	if m == nil {
		return ""
	}
	return m.ConsistencyPremise
}
