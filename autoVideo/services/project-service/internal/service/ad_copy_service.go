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


const defaultStoryboardSplitBuiltinPrompt = `你是一位专业的广告分镜师和摄影指导。当前步骤 1 的分镜拆分必须遵守以下内置规则：

1. 最高优先级：当规则发生冲突时，一律以“时长优先、台词 / 口播承载量优先”为最高准则。
2. 本项目目标单分镜时长以当前用户所选值为准；如当前未显式指定，则按模型默认允许时长执行。
3. 核心原则：优先判断一段台词 / 口播是否能在当前目标单分镜时长内完整表达，并同时追求视觉单位连贯性、空间方位一致性与整体表达稳定性，而不是追求最小视觉单位。
4. 如果同一段卖点说明、同一段口播、同一段连续动作在当前目标时长内可以完整表达，应优先合并为一个主分镜或少量连续分镜；即使进入新卖点，也只有在当前时长已经承载不下时才拆镜。
5. 判断是否拆镜的唯一依据是：观众是否会在该镜头内获得新的信息或新的情绪锚点；若没有，则不拆。
6. 口播内容必须在一个完整分镜内说完，不得为动作细节拆散口播。无 dialogue 分镜只能作为极短辅助镜头（建议不超过总分镜数的 20%），不可连续出现，不可单独承担卖点传达；除最后一个分镜外，若当前分镜没有台词，或台词长度明显不足以支撑当前目标时长，就必须继续合并、重写或调整拆分。
7. 只有最后一个分镜允许在确有必要时作为收束镜头例外，但即便如此也应尽量带有一句完整收尾口播、CTA 或字幕，不要轻易留空。
8. dialogue 只能放真的会被念出来或打上字幕的文字；如果某段只有动作或镜头说明、没有可念文本，优先继续调整拆分，让它并回前后有台词的分镜，而不是直接保留。
9. description 必须使用结构化格式：[景别] + [人物/主体位置与动作] + [环境与光线] + [关键道具或视觉锚点]；每条尽量不超过 60 字。`

const defaultAdCopyOptimizationPrompt = `你是广告短视频编剧、导演统筹和连续性审校。你的任务不是直接分集，也不是直接写成逐镜头分镜稿，而是先把整篇广告文案优化成更适合后续“按台词 / 口播为主自动切分成多个视频片段”的中间稿，并补出后续生成时必须遵守的一致性前提。

必须遵守：
- 保留原始产品卖点、人物设定、核心承诺与事实信息，不得胡编功效。
- 按当前目标风格重写语言与镜头感，使文案更适合后续广告视频生成，但绝不能提前把它写成 storyboard / shotlist / 分镜脚本。
- 必须主动补全并澄清以下 14 个维度：1）世界观/故事发生的视觉宇宙；2）空间（在哪里）；3）时间（几点/昼夜/时序）；4）人物（谁）；5）服装（穿什么）；6）动作（做什么）；7）核心物件/镜头重点；8）光线（怎么打光）；9）色彩（什么色调）；10）材质（表面质感）；11）镜头运动（怎么拍）；12）情绪（传达什么感觉）；13）转场（怎么切）；14）字幕/屏幕文字、配音/口播内容、以及最终给 AI 的生成 Prompt 描述。
- optimized_script 必须是“可继续拆分的广告中间稿”，核心是口播 / 台词 / 信息块顺序清楚，而不是已经拆好的镜头列表。
- consistency_premise 必须单独总结以上 14 个维度里“后续不得漂移”的硬约束，写成清晰条目。
- 把长段落整理成更自然的台词 / 口播句群，让后续系统更容易按单分镜时长进行台词拆分；优先保证一句口播能在一个完整镜头里说完。
- 每个段落优先围绕“一个卖点 / 一个信息推进 / 一个情绪动作”来写，不要为了增加画面感把一句话拆成多个视觉段。
- 可以补充必要的视觉约束，但只能轻量嵌入同一段中；不要给每段都单独展开“画面 / 字幕 / 配音 / Prompt”四件套。
- 严禁使用类似“【画面1】/【镜头1】/【字幕】/【口播】/【Prompt】”的逐段标签式输出；不要显式编号，不要写成 shot-by-shot 结构。
- 除收尾 CTA 外，不要主动新增无台词视觉段；不要为了渲染镜头感平白增加多个空镜、转场镜头、补充动作镜头。
- 优化后的正文总长度应尽量克制，通常控制在原文的 1.2x~1.6x 内；若明显超过，优先压缩视觉描述，而不是继续扩写。
- 如果是写实风格，优先真实场景、生活化表达、自然口语；如果是动漫风格，允许更鲜明的视觉感，但不要失去广告转化目标。
- 不要输出分集编号，不要显式写“第一段/第二段”，只输出优化后的完整文案和 consistency_premise。`

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
	OriginalScript                string `json:"original_script"`
	OptimizationPrompt            string `json:"optimization_prompt"`
	OptimizedScript               string `json:"optimized_script"`
	ConsistencyPremise            string `json:"consistency_premise,omitempty"`
	ScriptLength                  int    `json:"script_length,omitempty"`
	StoryboardSplitPrompt         string `json:"storyboard_split_prompt,omitempty"`
	StoryboardSplitPromptHint     string `json:"storyboard_split_prompt_hint,omitempty"`
	StoryboardSplitPromptBuiltin  string `json:"storyboard_split_prompt_builtin,omitempty"`
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
		OriginalScript:               originalScript,
		OptimizationPrompt:           prompt,
		OptimizedScript:              optimized,
		ConsistencyPremise:           consistency,
		ScriptLength:                 utf8.RuneCountInString(optimized),
		StoryboardSplitPrompt:        s.currentStoryboardSplitPrompt(project),
		StoryboardSplitPromptHint:    "这里填写的是‘步骤 1 文本重拆分前，给台词 / 分镜拆分模型的附加规则’。系统内置规则已经明确优先级、目标时长、拆镜判断标准、无台词限制和 description 结构；你在这里写的是项目级补充规则，会在真正 scene split 前注入。",
		StoryboardSplitPromptBuiltin: defaultStoryboardSplitBuiltinPrompt,
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
		OriginalScript:               originalScript,
		OptimizationPrompt:           prompt,
		OptimizedScript:              optimizedScript,
		ConsistencyPremise:           progress.AutoSplit.GetConsistencyPremise(),
		ScriptLength:                 utf8.RuneCountInString(optimizedScript),
		StoryboardSplitPrompt:        s.currentStoryboardSplitPrompt(project),
		StoryboardSplitPromptHint:    "这里填写的是‘步骤 1 文本重拆分前，给台词 / 分镜拆分模型的附加规则’。系统内置规则已经明确优先级、目标时长、拆镜判断标准、无台词限制和 description 结构；你在这里写的是项目级补充规则，会在真正 scene split 前注入。",
		StoryboardSplitPromptBuiltin: defaultStoryboardSplitBuiltinPrompt,
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
		OriginalScript:               originalScript,
		OptimizationPrompt:           prompt,
		OptimizedScript:              strings.TrimSpace(result.OptimizedScript),
		ConsistencyPremise:           strings.TrimSpace(result.ConsistencyPremise),
		ScriptLength:                 utf8.RuneCountInString(strings.TrimSpace(result.OptimizedScript)),
		StoryboardSplitPrompt:        s.currentStoryboardSplitPrompt(project),
		StoryboardSplitPromptHint:    "这里填写的是‘步骤 1 文本重拆分前，给台词 / 分镜拆分模型的附加规则’。系统内置规则已经明确优先级、目标时长、拆镜判断标准、无台词限制和 description 结构；你在这里写的是项目级补充规则，会在真正 scene split 前注入。",
		StoryboardSplitPromptBuiltin: defaultStoryboardSplitBuiltinPrompt,
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
