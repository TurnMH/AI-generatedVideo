package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/autovideo/video-service/internal/model"
	whisperClient "github.com/autovideo/video-service/pkg/whisper"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

var ErrActiveTaskExists = errors.New("active dubbing task exists")
var ErrTaskRetryNotAllowed = errors.New("task retry not allowed")

const dubbingTaskStallThreshold = 2 * time.Minute
const edgeTTSCommandTimeout = 2 * time.Minute
const autoVoiceModel = "auto"

var speakerLinePattern = regexp.MustCompile(`^(?:[【\[(（]\s*)?([^:：\]）)】]{1,24})(?:\s*[】\])）])?\s*[:：]\s*(.+)$`)

// reInlineAnnotation matches [tag:content] or [tag：content] inline production annotations.
var reInlineAnnotation = regexp.MustCompile(`[（(【\[]([^:：\[\]（）【】]{1,12})[：:]([^\]）】)]*)[）)\]】]`)

// scriptStripTags are inline annotation tag names whose content should be silently dropped.
var scriptStripTags = map[string]bool{
	// Camera / visuals
	"摄影": true, "镜头": true, "画面": true, "景别": true, "视角": true, "构图": true, "机位": true, "运镜": true,
	// Sound
	"音效": true, "声效": true, "配乐": true, "背景音": true, "音乐": true, "音响": true, "拟音": true,
	// Transitions
	"转场": true, "过渡": true, "淡入": true, "淡出": true, "剪辑": true, "衔接": true,
	// Scene / set / art
	"场景": true, "地点": true, "背景": true, "布景": true, "道具": true, "美术": true, "置景": true,
	"时间": true, "时段": true, "时代": true, "朝代": true, "年代": true, "环境": true, "天气": true,
	// Action / emotion / staging
	"动作": true, "表情": true, "肢体": true, "走位": true, "调度": true, "情绪": true, "氛围": true, "基调": true,
	// Lighting / color
	"灯光": true, "光线": true, "打光": true, "调色": true, "色彩": true, "色调": true,
	// Cast / costume / makeup
	"服化": true, "服装": true, "妆造": true, "造型": true, "发型": true, "化妆": true, "服饰": true, "人物": true, "演员": true,
	// Crew / department
	"导演": true, "场记": true, "制片": true, "录音": true, "剧本": true, "动画": true, "特效": true, "后期": true, "监制": true, "灯光师": true,
	// Production notes / meta
	"旁注": true, "说明": true, "备注": true, "注意": true, "提示": true, "字幕特效": true, "字幕样式": true, "标注": true, "注释": true,
	"节奏": true, "时长": true, "秒数": true, "镜头时长": true,
}

// scriptSpeechTags are inline annotation tags whose content IS dialogue and should be preserved.
var scriptSpeechTags = map[string]bool{
	"字幕": true, "对白": true, "台词": true, "独白": true, "旁白": true, "内心独白": true, "画外音": true, "解说": true,
}

// linePrefixStripPattern matches lines that start with a production-direction word plus colon,
// e.g. "环境：...", "场景: ...", "摄影：...", "时间：...". These are stage descriptions, not dialogue.
var linePrefixStripPattern = regexp.MustCompile(`^(?:摄影|镜头|画面|景别|视角|构图|机位|运镜|音效|声效|配乐|背景音|音乐|音响|拟音|转场|过渡|淡入|淡出|剪辑|衔接|场景|地点|背景|布景|道具|美术|置景|时间|时段|时代|朝代|年代|环境|天气|动作|表情|肢体|走位|调度|情绪|氛围|基调|灯光|光线|打光|调色|色彩|色调|服化|服装|妆造|造型|发型|化妆|服饰|演员|导演|场记|制片|录音|剧本|动画|特效|后期|监制|旁注|说明|备注|注意|提示|标注|注释|节奏|时长|秒数|字幕特效|字幕样式)\s*[：:]`)

// chapterTitlePattern matches chapter/episode/scene title lines.
var chapterTitlePattern = regexp.MustCompile(`^第[一二三四五六七八九十百千零〇两0-9\d]+[章集场幕回部卷节篇]`)

// cleanScriptForSpeech strips production-direction annotations and stage directions
// from screenplay/script text so only speakable dialogue reaches TTS/subtitle generation.
// Rules:
//  1. Inline [tag:content] — if tag is a speech tag, keep content; if a strip tag, drop entirely.
//  2. Pure-parenthetical lines (entire line wrapped in （）, (  ), 【】) are dropped.
//  3. Markdown headings / horizontal rules are dropped.
func cleanScriptForSpeech(text string) string {
	// 1. Handle inline bracket annotations.
	text = reInlineAnnotation.ReplaceAllStringFunc(text, func(match string) string {
		parts := reInlineAnnotation.FindStringSubmatch(match)
		if len(parts) != 3 {
			return match
		}
		tag := strings.TrimSpace(parts[1])
		content := strings.TrimSpace(parts[2])
		if scriptSpeechTags[tag] {
			return content // extract dialogue content
		}
		if scriptStripTags[tag] {
			return "" // drop production direction
		}
		// Unknown tag — drop conservatively (avoids TTS reading "特写镜头" etc.)
		return ""
	})

	// 2. Filter lines.
	lines := strings.Split(text, "\n")
	out := lines[:0]
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Drop pure stage-direction parentheticals.
		if (strings.HasPrefix(line, "(") && strings.HasSuffix(line, ")")) ||
			(strings.HasPrefix(line, "（") && strings.HasSuffix(line, "）")) ||
			(strings.HasPrefix(line, "【") && strings.HasSuffix(line, "】")) {
			continue
		}
		// Drop markdown headings and horizontal rules.
		if strings.HasPrefix(line, "#") || strings.HasPrefix(line, "---") || strings.HasPrefix(line, "===") || strings.HasPrefix(line, "***") {
			continue
		}
		// Drop chapter / episode / scene title lines (e.g. "第一章 …", "第3集：…").
		if chapterTitlePattern.MatchString(line) {
			continue
		}
		// Drop lines starting with production-direction prefix + colon (e.g. "环境：…", "场景：…").
		if linePrefixStripPattern.MatchString(line) {
			continue
		}
		out = append(out, line)
	}
	return strings.TrimSpace(strings.Join(out, "\n"))
}

var autoVoiceStagePrefixes = []string{
	"场景", "镜头", "画面", "时间", "地点", "动作", "音效", "提示", "说明", "备注", "转场", "环境", "布景",
	"摄影", "景别", "灯光", "光线", "道具", "背景", "字幕样式", "字幕特效", "声效", "配乐",
}

var autoVoiceNarratorHints = []string{
	"旁白", "内心", "独白", "画外音", "解说", "系统", "广播",
}

var autoVoiceFemaleHints = []string{
	"女", "妈妈", "母亲", "妻子", "姑娘", "女孩", "女生", "小姐", "女士", "阿姨", "奶奶", "姐姐", "妹妹", "太太", "皇后", "公主",
}

var autoVoiceMaleHints = []string{
	"男", "爸爸", "父亲", "丈夫", "男孩", "男生", "先生", "叔叔", "爷爷", "哥哥", "弟弟", "皇帝", "王子",
}

var autoVoiceNeutralCycle = []string{"female1", "male2", "female2", "male3", "default"}
var autoVoiceFemaleCycle = []string{"female1", "female2"}
var autoVoiceMaleCycle = []string{"default", "male2", "male3"}

// Supported Edge TTS voices for Chinese content
var EdgeVoices = map[string]string{
	"default":           "zh-CN-YunjianNeural",  // Male, passionate (storytelling)
	"male1":             "zh-CN-YunjianNeural",  // Male, passionate
	"male2":             "zh-CN-YunxiNeural",    // Male, lively
	"male3":             "zh-CN-YunyangNeural",  // Male, professional
	"female1":           "zh-CN-XiaoxiaoNeural", // Female, warm
	"female2":           "zh-CN-XiaoyiNeural",   // Female, lively
	"dialect":           "zh-CN-liaoning-XiaobeiNeural",
	"dialect-northeast": "zh-CN-liaoning-XiaobeiNeural", // Northeastern Mandarin
	"dialect-shaanxi":   "zh-CN-shaanxi-XiaoniNeural",   // Shaanxi dialect
	"cantonese-female1": "zh-HK-HiuGaaiNeural",          // Cantonese female
	"cantonese-female2": "zh-HK-HiuMaanNeural",          // Cantonese female
	"cantonese-male1":   "zh-HK-WanLungNeural",          // Cantonese male
	"taiwan-female1":    "zh-TW-HsiaoChenNeural",        // Taiwanese Mandarin female
	"taiwan-female2":    "zh-TW-HsiaoYuNeural",          // Taiwanese Mandarin female
	"taiwan-male1":      "zh-TW-YunJheNeural",           // Taiwanese Mandarin male
}

type DubbingResult struct {
	AudioURL    string  `json:"audio_url"`
	SubtitleURL string  `json:"subtitle_url"`
	Duration    float64 `json:"duration_sec"`
}

type dubbingChunk struct {
	Speaker  string
	Text     string
	VoiceKey string
}

type speakerSegment struct {
	Speaker string
	Text    string
}

type autoVoiceAssigner struct {
	speakerVoices map[string]string
	neutralIndex  int
	femaleIndex   int
	maleIndex     int
}

type DubbingService struct {
	logger            *zap.Logger
	storageBaseURL    string
	tempDir           string
	db                *gorm.DB
	sem               chan struct{} // concurrency limiter for edge-tts
	azureSpeechKey    string
	azureSpeechRegion string
	siliconflowAPIKey string
	characterBaseURL  string // optional, for character voice model bindings (char-c8)
	whisperURL        string // optional, for Whisper-based subtitle generation (feat-4)
}

// NewDubbingService —— 创建配音服务实例，初始化临时目录和并发限制器
func NewDubbingService(logger *zap.Logger, storageBaseURL string, db *gorm.DB) *DubbingService {
	tmpDir := filepath.Join(os.TempDir(), "autovideo-dubbing")
	os.MkdirAll(tmpDir, 0755)
	azureRegion := os.Getenv("AZURE_SPEECH_REGION")
	if azureRegion == "" {
		azureRegion = "eastasia"
	}
	return &DubbingService{
		logger:            logger,
		storageBaseURL:    storageBaseURL,
		tempDir:           tmpDir,
		db:                db,
		sem:               make(chan struct{}, 3), // max 3 concurrent edge-tts tasks
		azureSpeechKey:    os.Getenv("AZURE_SPEECH_KEY"),
		azureSpeechRegion: azureRegion,
		siliconflowAPIKey: os.Getenv("SILICONFLOW_API_KEY"),
	}
}

// SetCharacterBaseURL configures the character-service URL so dubbing can
// look up per-character voice model bindings (char-c8).
func (s *DubbingService) SetCharacterBaseURL(url string) {
	s.characterBaseURL = url
}

// SetWhisperURL configures the whisper-sidecar URL for Whisper-based subtitles (feat-4).
func (s *DubbingService) SetWhisperURL(url string) {
	s.whisperURL = url
}

// CreateTask —— 持久化配音/字幕任务并在后台启动处理
// CreateTask persists a dubbing/subtitle task and starts processing in background.
// When task.CustomAudioURL is non-empty, TTS is skipped and the audio is used directly.
func (s *DubbingService) CreateTask(ctx context.Context, task *model.DubbingTask, text string) error {
	// If custom audio is provided, mark complete immediately — no TTS needed.
	if strings.TrimSpace(task.CustomAudioURL) != "" {
		task.Status = model.StatusSucceeded
		task.AudioURL = strings.TrimSpace(task.CustomAudioURL)
		task.ChunksTotal = 1
		task.ChunksDone = 1
		task.SourceText = text
		if err := s.db.WithContext(ctx).Create(task).Error; err != nil {
			return fmt.Errorf("create custom audio task: %w", err)
		}
		s.logger.Info("dubbing task created with custom audio",
			zap.Int64("task_id", task.ID),
			zap.String("audio_url", task.AudioURL),
		)
		// feat-4: if Whisper is configured, generate subtitle for the custom audio asynchronously
		if s.whisperURL != "" {
			go s.generateWhisperSubtitle(task.ID, task.ProjectID, task.EpisodeID, task.AudioURL)
		}
		return nil
	}

	// Estimate chunks
	runes := []rune(strings.TrimSpace(text))
	chunks := splitTextChunks(text, maxChunkRunes)
	task.ChunksTotal = len(chunks)
	task.Status = model.StatusPending
	task.SourceText = text

	var existing model.DubbingTask
	err := s.db.WithContext(ctx).
		Where("project_id = ? AND episode_id = ? AND task_type = ? AND status IN ?", task.ProjectID, task.EpisodeID, task.TaskType, []string{model.StatusPending, model.StatusProcessing}).
		Order("id DESC").
		First(&existing).Error
	if err == nil {
		return ErrActiveTaskExists
	}
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return fmt.Errorf("query active %s task: %w", task.TaskType, err)
	}

	if err := s.db.WithContext(ctx).Create(task).Error; err != nil {
		return fmt.Errorf("create dubbing task: %w", err)
	}

	s.logger.Info("dubbing task created",
		zap.Int64("task_id", task.ID),
		zap.String("type", task.TaskType),
		zap.Int("text_runes", len(runes)),
		zap.Int("chunks", len(chunks)),
	)

	// Process in background
	go s.processTask(task.ID, task.ProjectID, task.EpisodeID, task.UserID, text, task.VoiceModel, task.VoiceRate, task.VoicePitch, task.VoiceVolume, task.TaskType)
	return nil
}

// ListTasks —— 查询项目下每个(episode_id, task_type)的最新任务
// ListTasks returns the latest dubbing task per (episode_id, task_type) for a project.
// Only the most recent task per combination is returned, avoiding stale duplicates.
// Storyboard-scoped tasks (storyboard_id IS NOT NULL) are excluded.
func (s *DubbingService) ListTasks(ctx context.Context, projectID int64) ([]model.DubbingTask, error) {
	var tasks []model.DubbingTask
	subq := s.db.Model(&model.DubbingTask{}).
		Select("MAX(id)").
		Where("project_id = ? AND storyboard_id IS NULL", projectID).
		Group("episode_id, task_type")
	err := s.db.WithContext(ctx).
		Where("id IN (?)", subq).
		Order("created_at DESC").
		Find(&tasks).Error
	return tasks, err
}

// ListStoryboardTasks —— 查询项目下每个分镜的最新配音任务
// ListStoryboardTasks returns the latest dubbing task per storyboard_id for a project.
func (s *DubbingService) ListStoryboardTasks(ctx context.Context, projectID int64) ([]model.DubbingTask, error) {
	var tasks []model.DubbingTask
	subq := s.db.Model(&model.DubbingTask{}).
		Select("MAX(id)").
		Where("project_id = ? AND storyboard_id IS NOT NULL", projectID).
		Group("storyboard_id, task_type")
	err := s.db.WithContext(ctx).
		Where("id IN (?)", subq).
		Order("created_at DESC").
		Find(&tasks).Error
	return tasks, err
}

// CreateStoryboardTask —— 创建分镜级别的配音任务
// CreateStoryboardTask creates a dubbing task scoped to a specific storyboard.
func (s *DubbingService) CreateStoryboardTask(ctx context.Context, task *model.DubbingTask, text string) error {
	if task.StoryboardID == nil {
		return fmt.Errorf("storyboard_id is required")
	}

	runes := []rune(strings.TrimSpace(text))
	chunks := splitTextChunks(text, maxChunkRunes)
	task.ChunksTotal = len(chunks)
	task.Status = model.StatusPending
	task.SourceText = text

	// Check for an active task for this storyboard
	var existing model.DubbingTask
	err := s.db.WithContext(ctx).
		Where("project_id = ? AND storyboard_id = ? AND task_type = ? AND status IN ?",
			task.ProjectID, *task.StoryboardID, task.TaskType, []string{model.StatusPending, model.StatusProcessing}).
		Order("id DESC").
		First(&existing).Error
	if err == nil {
		return ErrActiveTaskExists
	}
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return fmt.Errorf("query active storyboard %s task: %w", task.TaskType, err)
	}

	if err := s.db.WithContext(ctx).Create(task).Error; err != nil {
		return fmt.Errorf("create storyboard dubbing task: %w", err)
	}

	s.logger.Info("storyboard dubbing task created",
		zap.Int64("task_id", task.ID),
		zap.Int64("storyboard_id", *task.StoryboardID),
		zap.Int("text_runes", len(runes)),
		zap.Int("chunks", len(chunks)),
	)

	go s.processTask(task.ID, task.ProjectID, task.EpisodeID, task.UserID, text, task.VoiceModel, task.VoiceRate, task.VoicePitch, task.VoiceVolume, task.TaskType)
	return nil
}


// GetTask —— 根据 ID 查询单个配音任务，返回 *DubbingTask
// GetTask returns a single task by ID.
func (s *DubbingService) GetTask(ctx context.Context, taskID int64) (*model.DubbingTask, error) {
	var task model.DubbingTask
	err := s.db.WithContext(ctx).First(&task, taskID).Error
	if err != nil {
		return nil, err
	}
	return &task, nil
}

func (s *DubbingService) RetryTask(ctx context.Context, taskID int64, fallbackText string) (*model.DubbingTask, error) {
	task, err := s.GetTask(ctx, taskID)
	if err != nil {
		return nil, err
	}
	sourceText := strings.TrimSpace(task.SourceText)
	if sourceText == "" {
		sourceText = strings.TrimSpace(fallbackText)
	}
	if sourceText == "" {
		return nil, ErrTaskRetryNotAllowed
	}

	if task.Status == model.StatusPending || task.Status == model.StatusProcessing {
		if time.Since(task.UpdatedAt) < dubbingTaskStallThreshold {
			return nil, ErrTaskRetryNotAllowed
		}
		if err := s.db.WithContext(ctx).Model(&model.DubbingTask{}).Where("id = ?", task.ID).
			Updates(map[string]any{
				"status":    model.StatusFailed,
				"error_msg": "marked as stalled and retried",
			}).Error; err != nil {
			return nil, err
		}
	}

	newTask := &model.DubbingTask{
		ProjectID:   task.ProjectID,
		EpisodeID:   task.EpisodeID,
		UserID:      task.UserID,
		TaskType:    task.TaskType,
		SourceText:  sourceText,
		VoiceModel:  task.VoiceModel,
		VoiceRate:   task.VoiceRate,
		VoicePitch:  task.VoicePitch,
		VoiceVolume: task.VoiceVolume,
	}
	if err := s.CreateTask(ctx, newTask, sourceText); err != nil {
		return nil, err
	}
	return newTask, nil
}

// DeleteProjectData —— 删除项目下全部配音/字幕任务
func (s *DubbingService) DeleteProjectData(ctx context.Context, projectID int64) error {
	return s.db.WithContext(ctx).Where("project_id = ?", projectID).Delete(&model.DubbingTask{}).Error
}

// processTask —— 后台执行配音/字幕生成流程，完成后更新任务状态
func (s *DubbingService) processTask(taskID, projectID, episodeID, userID int64, text, voiceModel, voiceRate, voicePitch, voiceVolume, taskType string) {
	// Acquire semaphore — limits concurrent edge-tts processes
	s.sem <- struct{}{}
	defer func() { <-s.sem }()

	// Use a bounded timeout so goroutines don't run forever on hung external calls.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	// Mark processing
	if err := s.db.WithContext(ctx).Model(&model.DubbingTask{}).Where("id = ?", taskID).
		Updates(map[string]any{"status": model.StatusProcessing}).Error; err != nil {
		s.logger.Error("mark dubbing task processing failed", zap.Int64("task_id", taskID), zap.Error(err))
	}

	var result *DubbingResult
	var err error

	if taskType == "subtitle" {
		result, err = s.generateSubtitleAsync(ctx, taskID, uint64(projectID), uint64(episodeID), text, voiceModel, voiceRate, voicePitch, voiceVolume)
	} else {
		result, err = s.generateDubbingAsync(ctx, taskID, uint64(projectID), uint64(episodeID), text, voiceModel, voiceRate, voicePitch, voiceVolume)
	}

	if err != nil {
		s.logger.Error("dubbing task failed", zap.Int64("task_id", taskID), zap.Error(err))
		if dbErr := s.db.WithContext(ctx).Model(&model.DubbingTask{}).Where("id = ?", taskID).
			Updates(map[string]any{
				"status":    model.StatusFailed,
				"error_msg": err.Error(),
			}).Error; dbErr != nil {
			s.logger.Error("mark dubbing task failed: DB error", zap.Int64("task_id", taskID), zap.Error(dbErr))
		}
		return
	}

	updates := map[string]any{
		"status":       model.StatusSucceeded,
		"audio_url":    result.AudioURL,
		"subtitle_url": result.SubtitleURL,
		"duration_sec": result.Duration,
	}
	if dbErr := s.db.WithContext(ctx).Model(&model.DubbingTask{}).Where("id = ?", taskID).
		Updates(updates).Error; dbErr != nil {
		s.logger.Error("mark dubbing task succeeded: DB error", zap.Int64("task_id", taskID), zap.Error(dbErr))
	}

	s.logger.Info("dubbing task completed",
		zap.Int64("task_id", taskID),
		zap.String("audio_url", result.AudioURL),
		zap.String("subtitle_url", result.SubtitleURL),
	)
}

// ResumeOrphanedTasks —— 将服务重启时仍在处理中的任务标记为失败
// ResumeOrphanedTasks restarts tasks that were processing when the service stopped.
func (s *DubbingService) ResumeOrphanedTasks(ctx context.Context) {
	// Mark orphaned processing tasks as failed (we lost their edge-tts processes)
	s.db.Model(&model.DubbingTask{}).
		Where("status = ?", model.StatusProcessing).
		Updates(map[string]any{
			"status":    model.StatusFailed,
			"error_msg": "service restarted during processing",
		})
	s.logger.Info("marked orphaned dubbing tasks as failed")
}

// ResumePendingTasks —— 服务重启后重新拉起仍处于 pending 的配音/字幕任务
// ResumePendingTasks re-dispatches pending dubbing tasks that were persisted but never resumed.
func (s *DubbingService) ResumePendingTasks(ctx context.Context) {
	var tasks []model.DubbingTask
	if err := s.db.WithContext(ctx).
		Where("status = ?", model.StatusPending).
		Order("id ASC").
		Limit(1000).
		Find(&tasks).Error; err != nil {
		s.logger.Error("find pending dubbing tasks", zap.Error(err))
		return
	}
	if len(tasks) == 0 {
		return
	}

	s.logger.Info("resuming pending dubbing tasks", zap.Int("count", len(tasks)))
	for _, task := range tasks {
		go s.processTask(
			task.ID,
			task.ProjectID,
			task.EpisodeID,
			task.UserID,
			task.SourceText,
			task.VoiceModel,
			task.VoiceRate,
			task.VoicePitch,
			task.VoiceVolume,
			task.TaskType,
		)
	}
}

// maxChunkRunes is the max rune count per edge-tts chunk to avoid timeouts.
const maxChunkRunes = 800

// generateDubbingAsync —— 异步执行配音生成：分块调用 edge-tts、合并音频和字幕并上传
// generateDubbingAsync is the background worker for dubbing generation with DB progress updates.
func (s *DubbingService) generateDubbingAsync(ctx context.Context, taskID int64, projectID, episodeID uint64, text, voiceModel, voiceRate, voicePitch, voiceVolume string) (*DubbingResult, error) {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.TrimSpace(text)
	text = cleanScriptForSpeech(text)

	// Fetch per-character voice bindings when using auto-voice mode (char-c8).
	var charVoiceMap map[string]string
	if isAutoVoiceModel(voiceModel) {
		charVoiceMap = s.fetchCharacterVoiceBindings(ctx, int64(projectID))
	}
	chunks := buildDubbingChunksWithCharVoices(text, voiceModel, charVoiceMap)
	if len(chunks) == 0 {
		return nil, fmt.Errorf("dubbing text is empty")
	}

	s.logger.Info("async dubbing start",
		zap.Int64("task_id", taskID),
		zap.Uint64("episode_id", episodeID),
		zap.String("voice_model", voiceModel),
		zap.Bool("auto_cast", isAutoVoiceModel(voiceModel)),
		zap.Int("text_runes", len([]rune(text))),
	)

	ts := time.Now().UnixMilli()

	// Update total chunks in DB
	s.db.Model(&model.DubbingTask{}).Where("id = ?", taskID).
		Update("chunks_total", len(chunks))

	var chunkAudioPaths []string
	var chunkSubPaths []string
	defer func() {
		for _, p := range chunkAudioPaths {
			os.Remove(p)
		}
		for _, p := range chunkSubPaths {
			os.Remove(p)
		}
	}()

	for i, chunk := range chunks {
		audioPath := filepath.Join(s.tempDir, fmt.Sprintf("dub_%d_%d_%d_c%d.mp3", projectID, episodeID, ts, i))
		subPath := filepath.Join(s.tempDir, fmt.Sprintf("dub_%d_%d_%d_c%d.vtt", projectID, episodeID, ts, i))

		var chunkErr error
		switch {
		case strings.HasPrefix(voiceModel, "azure:"):
			azureVoice := strings.TrimPrefix(voiceModel, "azure:")
			chunkErr = s.runAzureTTS(ctx, azureVoice, chunk.Text, audioPath)
		case strings.HasPrefix(voiceModel, "siliconflow:"):
			sfStr := strings.TrimPrefix(voiceModel, "siliconflow:")
			sfParts := strings.SplitN(sfStr, ":", 2)
			sfModel := sfParts[0]
			sfVoice := ""
			if len(sfParts) > 1 {
				sfVoice = sfParts[1]
			}
			chunkErr = s.runSiliconFlowTTS(ctx, sfModel, sfVoice, chunk.Text, audioPath)
		case strings.HasPrefix(voiceModel, "comfyui:"):
			// opt-p1: ComfyUI TTS voice cloning
			comfyEndpoint := strings.TrimPrefix(voiceModel, "comfyui:")
			chunkErr = s.runComfyUITTS(ctx, comfyEndpoint, chunk.Text, audioPath)
		default:
			textPath := filepath.Join(s.tempDir, fmt.Sprintf("text_%d_%d_%d_c%d.txt", projectID, episodeID, ts, i))
			if err := os.WriteFile(textPath, []byte(chunk.Text), 0644); err != nil {
				return nil, fmt.Errorf("write chunk %d: %w", i, err)
			}
			defer os.Remove(textPath)
			chunkErr = s.runEdgeTTS(ctx, resolveEdgeVoice(chunk.VoiceKey), voiceRate, voicePitch, voiceVolume, textPath, audioPath, subPath)
			if chunkErr == nil {
				if fi, err := os.Stat(subPath); err == nil && fi.Size() > 0 {
					chunkSubPaths = append(chunkSubPaths, subPath)
				}
			}
		}

		if chunkErr != nil {
			s.logger.Warn("tts chunk failed",
				zap.Int("chunk", i),
				zap.String("speaker", chunk.Speaker),
				zap.String("voice_key", chunk.VoiceKey),
				zap.Error(chunkErr),
			)
			continue
		}
		if fi, err := os.Stat(audioPath); err == nil && fi.Size() > 0 {
			chunkAudioPaths = append(chunkAudioPaths, audioPath)
		}

		// Update progress in DB
		s.db.Model(&model.DubbingTask{}).Where("id = ?", taskID).
			Update("chunks_done", i+1)

		s.logger.Info("chunk done",
			zap.Int64("task_id", taskID),
			zap.Int("chunk", i+1),
			zap.Int("total", len(chunks)),
			zap.String("speaker", chunk.Speaker),
			zap.String("voice_key", chunk.VoiceKey),
		)
	}

	if len(chunkAudioPaths) == 0 {
		return nil, fmt.Errorf("tts produced no audio for any chunk")
	}

	// Merge audio
	finalAudioPath := filepath.Join(s.tempDir, fmt.Sprintf("dub_%d_%d_%d_final.mp3", projectID, episodeID, ts))
	defer os.Remove(finalAudioPath)

	if len(chunkAudioPaths) == 1 {
		finalAudioPath = chunkAudioPaths[0]
	} else {
		if err := s.concatAudio(ctx, chunkAudioPaths, finalAudioPath); err != nil {
			return nil, fmt.Errorf("concat audio: %w", err)
		}
	}

	// Merge VTT
	finalSubPath := filepath.Join(s.tempDir, fmt.Sprintf("dub_%d_%d_%d_final.vtt", projectID, episodeID, ts))
	defer os.Remove(finalSubPath)
	mergeVTTFiles(chunkSubPaths, finalSubPath)

	audioInfo, err := os.Stat(finalAudioPath)
	if err != nil || audioInfo.Size() == 0 {
		return nil, fmt.Errorf("final audio is empty")
	}
	durationSec := float64(audioInfo.Size()) / 16000.0
	if probedDuration, probeErr := probeMediaDuration(ctx, finalAudioPath); probeErr == nil && probedDuration > 0 {
		durationSec = probedDuration
	}

	// Upload
	audioURL, err := s.uploadFile(ctx, finalAudioPath, "audio/mpeg",
		fmt.Sprintf("dubbing/project_%d/ep_%d_audio.mp3", projectID, episodeID), int64(projectID), "dubbing")
	if err != nil {
		return nil, fmt.Errorf("upload audio: %w", err)
	}

	var subtitleURL string
	if fi, err := os.Stat(finalSubPath); err == nil && fi.Size() > 0 {
		subtitleURL, err = s.uploadFile(ctx, finalSubPath, "text/vtt",
			fmt.Sprintf("dubbing/project_%d/ep_%d_subtitle.vtt", projectID, episodeID), int64(projectID), "subtitle")
		if err != nil {
			s.logger.Warn("failed to upload subtitle", zap.Error(err))
		}
	}

	return &DubbingResult{
		AudioURL:    audioURL,
		SubtitleURL: subtitleURL,
		Duration:    durationSec,
	}, nil
}

// generateSubtitleAsync —— 异步生成纯字幕（不含音频），通过 edge-tts 生成 VTT 并上传
// generateSubtitleAsync generates only subtitles (no audio) for a task.
func (s *DubbingService) generateSubtitleAsync(ctx context.Context, taskID int64, projectID, episodeID uint64, text, voiceModel, voiceRate, voicePitch, voiceVolume string) (*DubbingResult, error) {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.TrimSpace(text)
	text = cleanScriptForSpeech(text)
	var charVoiceMap map[string]string
	if isAutoVoiceModel(voiceModel) {
		charVoiceMap = s.fetchCharacterVoiceBindings(ctx, int64(projectID))
	}
	chunks := buildDubbingChunksWithCharVoices(text, voiceModel, charVoiceMap)
	if len(chunks) == 0 {
		return nil, fmt.Errorf("subtitle text is empty")
	}

	s.db.Model(&model.DubbingTask{}).Where("id = ?", taskID).
		Updates(map[string]any{"chunks_total": len(chunks), "chunks_done": 0})

	ts := time.Now().UnixMilli()
	finalSubtitlePath := filepath.Join(s.tempDir, fmt.Sprintf("sub_%d_%d_%d_final.vtt", projectID, episodeID, ts))
	defer os.Remove(finalSubtitlePath)

	var chunkSubtitlePaths []string
	var chunkAudioPathsSub []string
	var cleanupPaths []string
	defer func() {
		for _, path := range cleanupPaths {
			_ = os.Remove(path)
		}
	}()

	isAzureSub := strings.HasPrefix(voiceModel, "azure:")
	isComfyUISub := strings.HasPrefix(voiceModel, "comfyui:")

	for i, chunk := range chunks {
		subtitlePath := filepath.Join(s.tempDir, fmt.Sprintf("sub_%d_%d_%d_c%d.vtt", projectID, episodeID, ts, i))
		textPath := filepath.Join(s.tempDir, fmt.Sprintf("subtext_%d_%d_%d_c%d.txt", projectID, episodeID, ts, i))
		audioPath := filepath.Join(s.tempDir, fmt.Sprintf("sub_%d_%d_%d_c%d.mp3", projectID, episodeID, ts, i))
		cleanupPaths = append(cleanupPaths, subtitlePath, textPath, audioPath)

		var chunkErr error
		switch {
		case isAzureSub:
			azureVoice := strings.TrimPrefix(voiceModel, "azure:")
			chunkErr = s.runAzureTTS(ctx, azureVoice, chunk.Text, audioPath)
			if chunkErr == nil {
				if fi, err := os.Stat(audioPath); err == nil && fi.Size() > 0 {
					chunkAudioPathsSub = append(chunkAudioPathsSub, audioPath)
				}
			}
		case isComfyUISub:
			// opt-p1: ComfyUI TTS for subtitle generation
			comfyEndpoint := strings.TrimPrefix(voiceModel, "comfyui:")
			chunkErr = s.runComfyUITTS(ctx, comfyEndpoint, chunk.Text, audioPath)
			if chunkErr == nil {
				if fi, err := os.Stat(audioPath); err == nil && fi.Size() > 0 {
					chunkAudioPathsSub = append(chunkAudioPathsSub, audioPath)
				}
			}
		default:
			if err := os.WriteFile(textPath, []byte(chunk.Text), 0644); err != nil {
				return nil, fmt.Errorf("write subtitle chunk %d: %w", i+1, err)
			}
			chunkErr = s.runEdgeTTS(ctx, resolveEdgeVoice(chunk.VoiceKey), voiceRate, voicePitch, voiceVolume, textPath, audioPath, subtitlePath)
			if chunkErr == nil {
				if fi, err := os.Stat(subtitlePath); err == nil && fi.Size() > 0 {
					chunkSubtitlePaths = append(chunkSubtitlePaths, subtitlePath)
				}
			}
		}

		if chunkErr != nil {
			return nil, fmt.Errorf("tts subtitle chunk %d/%d: %w", i+1, len(chunks), chunkErr)
		}

		s.db.Model(&model.DubbingTask{}).Where("id = ?", taskID).
			Update("chunks_done", i+1)
	}

	// Azure/ComfyUI path: return merged audio, no VTT
	if isAzureSub || isComfyUISub {
		if len(chunkAudioPathsSub) == 0 {
			return nil, fmt.Errorf("tts produced no audio for subtitle task")
		}
		finalAudioSubPath := filepath.Join(s.tempDir, fmt.Sprintf("sub_%d_%d_%d_final.mp3", projectID, episodeID, ts))
		defer os.Remove(finalAudioSubPath)
		if len(chunkAudioPathsSub) == 1 {
			finalAudioSubPath = chunkAudioPathsSub[0]
		} else {
			if err := s.concatAudio(ctx, chunkAudioPathsSub, finalAudioSubPath); err != nil {
				return nil, fmt.Errorf("concat azure audio: %w", err)
			}
		}
		audioURL, err := s.uploadFile(ctx, finalAudioSubPath, "audio/mpeg",
			fmt.Sprintf("dubbing/project_%d/ep_%d_audio.mp3", projectID, episodeID), int64(projectID), "dubbing")
		if err != nil {
			return nil, fmt.Errorf("upload azure audio: %w", err)
		}
		return &DubbingResult{AudioURL: audioURL}, nil
	}

	if len(chunkSubtitlePaths) == 0 {
		return nil, fmt.Errorf("tts subtitle produced no subtitle output")
	}

	mergeVTTFiles(chunkSubtitlePaths, finalSubtitlePath)

	var subtitleURL string
	if fi, err := os.Stat(finalSubtitlePath); err == nil && fi.Size() > 0 {
		subtitleURL, err = s.uploadFile(ctx, finalSubtitlePath, "text/vtt",
			fmt.Sprintf("dubbing/project_%d/ep_%d_subtitle.vtt", projectID, episodeID), int64(projectID), "subtitle")
		if err != nil {
			return nil, fmt.Errorf("upload subtitle: %w", err)
		}
	}

	return &DubbingResult{SubtitleURL: subtitleURL}, nil
}

func (s *DubbingService) runEdgeTTS(ctx context.Context, voice, voiceRate, voicePitch, voiceVolume, textPath, audioPath, subtitlePath string) error {
	cmdCtx, cancel := context.WithTimeout(ctx, edgeTTSCommandTimeout)
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, "edge-tts",
		"--voice", voice,
		"--rate", normalizeVoiceRate(voiceRate),
		"--pitch", normalizeVoicePitch(voicePitch),
		"--volume", normalizeVoiceVolume(voiceVolume),
		"--file", textPath,
		"--write-media", audioPath,
		"--write-subtitles", subtitlePath,
	)
	output, err := cmd.CombinedOutput()
	if err == nil {
		return nil
	}
	trimmedOutput := strings.TrimSpace(string(output))
	if errors.Is(cmdCtx.Err(), context.DeadlineExceeded) {
		if trimmedOutput == "" {
			return fmt.Errorf("edge-tts timed out after %s", edgeTTSCommandTimeout)
		}
		return fmt.Errorf("edge-tts timed out after %s: %s", edgeTTSCommandTimeout, trimmedOutput)
	}
	if trimmedOutput == "" {
		return err
	}
	return fmt.Errorf("%w output=%s", err, trimmedOutput)
}

// runAzureTTS calls the Azure Speech REST API to synthesise speech and writes
// the raw MP3 response to audioPath.
func (s *DubbingService) runAzureTTS(ctx context.Context, voice, text, audioPath string) error {
	ssml := fmt.Sprintf(`<speak version='1.0' xml:lang='zh-CN'><voice name='%s'>%s</voice></speak>`, voice, text)
	apiURL := fmt.Sprintf("https://%s.tts.speech.microsoft.com/cognitiveservices/v1", s.azureSpeechRegion)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, strings.NewReader(ssml))
	if err != nil {
		return fmt.Errorf("azure tts: create request: %w", err)
	}
	req.Header.Set("Ocp-Apim-Subscription-Key", s.azureSpeechKey)
	req.Header.Set("Content-Type", "application/ssml+xml")
	req.Header.Set("X-Microsoft-OutputFormat", "audio-48khz-192kbitrate-mono-mp3")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("azure tts: request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("azure tts: status %d: %s", resp.StatusCode, string(body))
	}

	f, err := os.Create(audioPath)
	if err != nil {
		return fmt.Errorf("azure tts: create file: %w", err)
	}
	defer f.Close()

	_, err = io.Copy(f, resp.Body)
	return err
}

// runSiliconFlowTTS calls the SiliconFlow TTS API and writes the raw MP3
// response to audioPath. model is e.g. "FunAudioLLM/CosyVoice2-0.5B".
func (s *DubbingService) runSiliconFlowTTS(ctx context.Context, model, voice, text, audioPath string) error {
	payload := map[string]string{
		"model":           model,
		"input":           text,
		"voice":           voice,
		"response_format": "mp3",
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("siliconflow tts: marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.siliconflow.cn/v1/audio/speech", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("siliconflow tts: create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+s.siliconflowAPIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("siliconflow tts: request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("siliconflow tts: status %d: %s", resp.StatusCode, string(respBody))
	}

	f, err := os.Create(audioPath)
	if err != nil {
		return fmt.Errorf("siliconflow tts: create file: %w", err)
	}
	defer f.Close()

	_, err = io.Copy(f, resp.Body)
	return err
}

// runComfyUITTS calls a ComfyUI instance with a TTS workflow node (opt-p1).
// voiceModel format: "comfyui:http://host:port/workflow_id"
// The ComfyUI workflow must have nodes with names:
//   __TTS_TEXT__ (text input), __TTS_SPEAKER__ (speaker/voice reference)
// and output node producing an audio file.
func (s *DubbingService) runComfyUITTS(ctx context.Context, endpointURL, text, audioPath string) error {
	workflowTemplate := `{
		"1": {"inputs": {"text": __TTS_TEXT_JSON__}, "class_type": "CLIPTextEncode"},
		"2": {"inputs": {"audio_path": __TTS_TEXT_JSON__}, "class_type": "TTSNode"},
		"9": {"inputs": {"filename_prefix": "tts_out", "audio": ["2", 0]}, "class_type": "SaveAudio"}
	}`
	textJSON, err := json.Marshal(text)
	if err != nil {
		return fmt.Errorf("comfyui tts: marshal text: %w", err)
	}
	workflow := strings.ReplaceAll(workflowTemplate, "__TTS_TEXT_JSON__", string(textJSON))

	payload := map[string]interface{}{"prompt": json.RawMessage(workflow)}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("comfyui tts: marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpointURL+"/prompt", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("comfyui tts: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("comfyui tts: submit workflow: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		PromptID string `json:"prompt_id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil || result.PromptID == "" {
		return fmt.Errorf("comfyui tts: invalid submit response")
	}

	// Poll for completion (up to 5 minutes)
	deadline := time.Now().Add(5 * time.Minute)
	for {
		if time.Now().After(deadline) {
			return fmt.Errorf("comfyui tts: timeout after 5 minutes")
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(3 * time.Second):
		}

		histReq, _ := http.NewRequestWithContext(ctx, http.MethodGet, endpointURL+"/history/"+result.PromptID, nil)
		histResp, err := http.DefaultClient.Do(histReq)
		if err != nil {
			continue
		}
		var history map[string]struct {
			Status struct {
				StatusStr string `json:"status_str"`
				Completed bool   `json:"completed"`
			} `json:"status"`
			Outputs map[string]struct {
				Audio []struct {
					Filename  string `json:"filename"`
					Subfolder string `json:"subfolder"`
					Type      string `json:"type"`
				} `json:"audio"`
			} `json:"outputs"`
		}
		json.NewDecoder(histResp.Body).Decode(&history)
		histResp.Body.Close()

		entry, ok := history[result.PromptID]
		if !ok || !entry.Status.Completed {
			continue
		}
		if entry.Status.StatusStr != "success" {
			return fmt.Errorf("comfyui tts: workflow failed: %s", entry.Status.StatusStr)
		}
		for _, out := range entry.Outputs {
			if len(out.Audio) > 0 {
				audio := out.Audio[0]
				viewURL := fmt.Sprintf("%s/view?filename=%s&subfolder=%s&type=%s",
					endpointURL, audio.Filename, audio.Subfolder, audio.Type)
				dlReq, _ := http.NewRequestWithContext(ctx, http.MethodGet, viewURL, nil)
				dlResp, err := http.DefaultClient.Do(dlReq)
				if err != nil {
					return fmt.Errorf("comfyui tts: download audio: %w", err)
				}
				defer dlResp.Body.Close()
				f, err := os.Create(audioPath)
				if err != nil {
					return fmt.Errorf("comfyui tts: create file: %w", err)
				}
				defer f.Close()
				_, err = io.Copy(f, dlResp.Body)
				return err
			}
		}
		return fmt.Errorf("comfyui tts: no audio output found in completed workflow")
	}
}

// generateWhisperSubtitle calls the Whisper sidecar to transcribe custom audio
// and stores the resulting SRT as a subtitle VTT URL on the dubbing task (feat-4).
func (s *DubbingService) generateWhisperSubtitle(taskID, projectID, episodeID int64, audioURL string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	wc := whisperClient.NewClient(s.whisperURL)
	result, err := wc.Transcribe(ctx, audioURL, "zh")
	if err != nil {
		s.logger.Warn("whisper transcription failed",
			zap.Int64("task_id", taskID),
			zap.Error(err))
		return
	}
	if result.SRT == "" {
		return
	}
	// Save SRT to temp file and upload
	srtPath := filepath.Join(s.tempDir, fmt.Sprintf("whisper_%d_%d_%d.srt", projectID, episodeID, taskID))
	if err := os.WriteFile(srtPath, []byte(result.SRT), 0644); err != nil {
		s.logger.Warn("whisper: write srt file", zap.Error(err))
		return
	}
	defer os.Remove(srtPath)

	subtitleURL, err := s.uploadFile(ctx, srtPath, "text/plain",
		fmt.Sprintf("dubbing/project_%d/ep_%d_whisper.srt", projectID, episodeID), projectID, "subtitle")
	if err != nil {
		s.logger.Warn("whisper: upload subtitle", zap.Error(err))
		return
	}
	// Update the task with the new subtitle URL
	s.db.Model(&model.DubbingTask{}).Where("id = ?", taskID).Update("subtitle_url", subtitleURL)
	s.logger.Info("whisper subtitle generated",
		zap.Int64("task_id", taskID),
		zap.String("subtitle_url", subtitleURL))
}

func normalizeVoiceRate(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "+0%"
	}
	return value
}

func normalizeVoicePitch(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "+0Hz"
	}
	return value
}

func normalizeVoiceVolume(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "+0%"
	}
	return value
}

func isAutoVoiceModel(voiceModel string) bool {
	return strings.EqualFold(strings.TrimSpace(voiceModel), autoVoiceModel)
}

func resolveEdgeVoice(voiceKey string) string {
	if voice, ok := EdgeVoices[strings.TrimSpace(voiceKey)]; ok {
		return voice
	}
	return EdgeVoices["default"]
}

func buildDubbingChunks(text, voiceModel string) []dubbingChunk {
	return buildDubbingChunksWithCharVoices(text, voiceModel, nil)
}

// buildDubbingChunksWithCharVoices is like buildDubbingChunks but accepts an optional
// charVoiceMap (lowercase speaker name → voice model string) to honour per-character
// voice bindings set in the character management UI (char-c8).
func buildDubbingChunksWithCharVoices(text, voiceModel string, charVoiceMap map[string]string) []dubbingChunk {
	text = strings.TrimSpace(strings.ReplaceAll(text, "\r\n", "\n"))
	if text == "" {
		return nil
	}
	if !isAutoVoiceModel(voiceModel) {
		parts := splitTextChunks(text, maxChunkRunes)
		chunks := make([]dubbingChunk, 0, len(parts))
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			chunks = append(chunks, dubbingChunk{Text: part, VoiceKey: voiceModel})
		}
		return chunks
	}

	segments := parseSpeakerSegments(text)
	if len(segments) == 0 {
		return nil
	}
	assigner := newAutoVoiceAssigner()
	chunks := make([]dubbingChunk, 0, len(segments))
	for _, segment := range segments {
		// Check character-bound voice first (char-c8).
		boundVoice := ""
		if len(charVoiceMap) > 0 && segment.Speaker != "" {
			boundVoice = charVoiceMap[strings.ToLower(strings.TrimSpace(segment.Speaker))]
		}
		for _, part := range splitTextChunks(segment.Text, maxChunkRunes) {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			voiceKey := boundVoice
			if voiceKey == "" {
				voiceKey = assigner.voiceForSpeaker(segment.Speaker)
			}
			chunks = append(chunks, dubbingChunk{
				Speaker:  segment.Speaker,
				Text:     part,
				VoiceKey: voiceKey,
			})
		}
	}
	if len(chunks) == 0 {
		return []dubbingChunk{{Text: text, VoiceKey: "default"}}
	}
	return chunks
}

// fetchCharacterVoiceBindings fetches character voice model bindings from
// character-service for the given project. Returns map of lowercase name → voice model.
// Merges bindings from both the characters table and character-type assets.
func (s *DubbingService) fetchCharacterVoiceBindings(ctx context.Context, projectID int64) map[string]string {
	if s.characterBaseURL == "" {
		return nil
	}
	bindings := make(map[string]string)

	// 1. Fetch from characters table (manually created characters)
	charsURL := fmt.Sprintf("%s/api/v1/characters?project_id=%d&page=1&page_size=50", s.characterBaseURL, projectID)
	if req, err := http.NewRequestWithContext(ctx, http.MethodGet, charsURL, nil); err == nil {
		client := &http.Client{Timeout: 5 * time.Second}
		if resp, err := client.Do(req); err == nil {
			defer resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				var result struct {
					Data struct {
						Items []struct {
							Name       string `json:"name"`
							VoiceModel string `json:"voice_model"`
						} `json:"items"`
					} `json:"data"`
				}
				if body, err := io.ReadAll(resp.Body); err == nil {
					if err := json.Unmarshal(body, &result); err == nil {
						for _, item := range result.Data.Items {
							if item.Name != "" && item.VoiceModel != "" {
								bindings[strings.ToLower(strings.TrimSpace(item.Name))] = item.VoiceModel
							}
						}
					}
				}
			}
		}
	}

	// 2. Fetch from character-type assets (extracted characters) — asset voice bindings take precedence
	assetsURL := fmt.Sprintf("%s/api/v1/projects/%d/assets?type=character&page_size=200", s.characterBaseURL, projectID)
	if req, err := http.NewRequestWithContext(ctx, http.MethodGet, assetsURL, nil); err == nil {
		client := &http.Client{Timeout: 5 * time.Second}
		if resp, err := client.Do(req); err == nil {
			defer resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				var result struct {
					Data []struct {
						Name       string `json:"name"`
						VoiceModel string `json:"voice_model"`
					} `json:"data"`
				}
				if body, err := io.ReadAll(resp.Body); err == nil {
					if err := json.Unmarshal(body, &result); err == nil {
						for _, item := range result.Data {
							if item.Name != "" && item.VoiceModel != "" {
								bindings[strings.ToLower(strings.TrimSpace(item.Name))] = item.VoiceModel
							}
						}
					}
				}
			}
		}
	}

	if len(bindings) == 0 {
		return nil
	}
	return bindings
}

func parseSpeakerSegments(text string) []speakerSegment {
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	segments := make([]speakerSegment, 0, len(lines))
	appendSegment := func(speaker, content string) {
		content = strings.TrimSpace(content)
		if content == "" {
			return
		}
		if len(segments) > 0 && segments[len(segments)-1].Speaker == speaker {
			segments[len(segments)-1].Text = strings.TrimSpace(segments[len(segments)-1].Text + "\n" + content)
			return
		}
		segments = append(segments, speakerSegment{Speaker: speaker, Text: content})
	}

	for _, rawLine := range lines {
		line := strings.TrimSpace(rawLine)
		if line == "" {
			continue
		}
		matches := speakerLinePattern.FindStringSubmatch(line)
		if len(matches) == 3 {
			speaker := normalizeSpeakerLabel(matches[1])
			content := strings.TrimSpace(matches[2])
			if speaker != "" && content != "" && isLikelySpeakerLabel(speaker) {
				appendSegment(speaker, content)
				continue
			}
		}
		appendSegment("", line)
	}

	if len(segments) == 0 {
		return []speakerSegment{{Text: strings.TrimSpace(text)}}
	}
	return segments
}

func normalizeSpeakerLabel(label string) string {
	label = strings.TrimSpace(label)
	label = strings.Trim(label, "[]()（）【】")
	label = strings.TrimSpace(label)
	return strings.ReplaceAll(label, " ", "")
}

func isLikelySpeakerLabel(label string) bool {
	if label == "" {
		return false
	}
	for _, disallowed := range []string{"，", "。", "！", "？", ";", "；", ",", ".", "、", "/"} {
		if strings.Contains(label, disallowed) {
			return false
		}
	}
	for _, prefix := range autoVoiceStagePrefixes {
		if strings.HasPrefix(label, prefix) {
			return false
		}
	}
	return true
}

func newAutoVoiceAssigner() *autoVoiceAssigner {
	return &autoVoiceAssigner{
		speakerVoices: make(map[string]string),
	}
}

func (a *autoVoiceAssigner) voiceForSpeaker(speaker string) string {
	speaker = normalizeSpeakerLabel(speaker)
	if speaker == "" || containsAny(speaker, autoVoiceNarratorHints) {
		return "male3"
	}
	if voiceKey, ok := a.speakerVoices[speaker]; ok {
		return voiceKey
	}

	var voiceKey string
	switch {
	case containsAny(speaker, autoVoiceFemaleHints):
		voiceKey = autoVoiceFemaleCycle[a.femaleIndex%len(autoVoiceFemaleCycle)]
		a.femaleIndex++
	case containsAny(speaker, autoVoiceMaleHints):
		voiceKey = autoVoiceMaleCycle[a.maleIndex%len(autoVoiceMaleCycle)]
		a.maleIndex++
	default:
		voiceKey = autoVoiceNeutralCycle[a.neutralIndex%len(autoVoiceNeutralCycle)]
		a.neutralIndex++
	}
	a.speakerVoices[speaker] = voiceKey
	return voiceKey
}

func containsAny(value string, hints []string) bool {
	for _, hint := range hints {
		if hint != "" && strings.Contains(value, hint) {
			return true
		}
	}
	return false
}

func probeMediaDuration(ctx context.Context, mediaPath string) (float64, error) {
	cmd := exec.CommandContext(ctx, "ffprobe",
		"-v", "error",
		"-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1",
		mediaPath,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return 0, fmt.Errorf("ffprobe: %w output=%s", err, strings.TrimSpace(string(out)))
	}
	var duration float64
	if _, err := fmt.Sscanf(strings.TrimSpace(string(out)), "%f", &duration); err != nil {
		return 0, fmt.Errorf("parse duration: %w", err)
	}
	return duration, nil
}

// splitTextChunks —— 将文本按最大 maxRunes 字符数拆分为多个块，优先在句段边界处分割
// splitTextChunks splits text into chunks of at most maxRunes runes,
// preferring to split at paragraph/sentence boundaries.
func splitTextChunks(text string, maxRunes int) []string {
	runes := []rune(strings.TrimSpace(text))
	if len(runes) <= maxRunes {
		return []string{string(runes)}
	}

	var chunks []string
	for len(runes) > 0 {
		end := maxRunes
		if end > len(runes) {
			end = len(runes)
		}
		// Try to find a good split point (paragraph, period, comma)
		if end < len(runes) {
			best := -1
			for _, sep := range []rune{'\n', '。', '；', '！', '？', '，', '.', ',', ' '} {
				for i := end - 1; i >= end/2; i-- {
					if runes[i] == sep {
						best = i + 1
						break
					}
				}
				if best > 0 {
					break
				}
			}
			if best > 0 {
				end = best
			}
		}
		chunk := strings.TrimSpace(string(runes[:end]))
		if chunk != "" {
			chunks = append(chunks, chunk)
		}
		runes = runes[end:]
	}
	return chunks
}

// concatAudio —— 使用 ffmpeg 将多个 MP3 文件合并为一个
// concatAudio merges multiple MP3 files into one using ffmpeg.
func (s *DubbingService) concatAudio(ctx context.Context, inputs []string, output string) error {
	// Create ffmpeg concat list file
	listPath := output + ".list"
	var lines []string
	for _, p := range inputs {
		lines = append(lines, fmt.Sprintf("file '%s'", p))
	}
	if err := os.WriteFile(listPath, []byte(strings.Join(lines, "\n")), 0644); err != nil {
		return err
	}
	defer os.Remove(listPath)

	cmd := exec.CommandContext(ctx, "ffmpeg", "-y",
		"-f", "concat", "-safe", "0",
		"-i", listPath,
		"-c", "copy",
		output,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ffmpeg concat: %w output=%s", err, string(out))
	}
	return nil
}

// mergeVTTFiles —— 合并多个 VTT 字幕文件，自动调整时间戳偏移
// mergeVTTFiles concatenates multiple VTT files, adjusting timestamps.
func mergeVTTFiles(inputs []string, output string) {
	var buf strings.Builder
	buf.WriteString("WEBVTT\n\n")

	var offsetMs int64
	seq := 1

	for _, path := range inputs {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		lines := strings.Split(string(data), "\n")
		var maxEndMs int64

		for i := 0; i < len(lines); i++ {
			line := strings.TrimSpace(lines[i])
			// Look for timestamp lines: 00:00:00.000 --> 00:00:02.037
			if idx := strings.Index(line, " --> "); idx > 0 {
				startStr := line[:idx]
				endStr := line[idx+5:]
				startMs := parseVTTMs(startStr)
				endMs := parseVTTMs(endStr)
				// Adjust by offset
				startMs += offsetMs
				endMs += offsetMs
				if endMs > maxEndMs {
					maxEndMs = endMs
				}
				// Read the text line(s) after timestamp
				var textLines []string
				for i++; i < len(lines); i++ {
					tl := strings.TrimSpace(lines[i])
					if tl == "" {
						break
					}
					textLines = append(textLines, tl)
				}
				if len(textLines) > 0 {
					buf.WriteString(fmt.Sprintf("%d\n", seq))
					buf.WriteString(fmt.Sprintf("%s --> %s\n", formatVTTTimeMs(startMs), formatVTTTimeMs(endMs)))
					buf.WriteString(strings.Join(textLines, "\n"))
					buf.WriteString("\n\n")
					seq++
				}
			}
		}
		// Next chunk starts after this chunk's last cue + small gap
		offsetMs = maxEndMs + 100
	}

	os.WriteFile(output, []byte(buf.String()), 0644)
}

// parseVTTMs —— 将 VTT 时间格式字符串解析为毫秒数
func parseVTTMs(s string) int64 {
	// Parse "HH:MM:SS.mmm" or "MM:SS.mmm"
	s = strings.TrimSpace(s)
	parts := strings.Split(s, ":")
	var h, m int64
	var secStr string
	switch len(parts) {
	case 3:
		fmt.Sscanf(parts[0], "%d", &h)
		fmt.Sscanf(parts[1], "%d", &m)
		secStr = parts[2]
	case 2:
		fmt.Sscanf(parts[0], "%d", &m)
		secStr = parts[1]
	default:
		return 0
	}
	secParts := strings.SplitN(secStr, ".", 2)
	var sec, ms int64
	fmt.Sscanf(secParts[0], "%d", &sec)
	if len(secParts) > 1 {
		msStr := secParts[1]
		for len(msStr) < 3 {
			msStr += "0"
		}
		fmt.Sscanf(msStr[:3], "%d", &ms)
	}
	return h*3600000 + m*60000 + sec*1000 + ms
}

// formatVTTTimeMs —— 将毫秒数格式化为 VTT 时间字符串 "HH:MM:SS.mmm"
func formatVTTTimeMs(ms int64) string {
	h := ms / 3600000
	ms %= 3600000
	m := ms / 60000
	ms %= 60000
	sec := ms / 1000
	milli := ms % 1000
	return fmt.Sprintf("%02d:%02d:%02d.%03d", h, m, sec, milli)
}

// GenerateSubtitle —— 根据纯文本生成 VTT 字幕文件并上传，返回 DubbingResult
// GenerateSubtitle generates a VTT subtitle file from text (without audio).
func (s *DubbingService) GenerateSubtitle(ctx context.Context, projectID, episodeID uint64, text string) (*DubbingResult, error) {
	s.logger.Info("generating subtitle from text",
		zap.Uint64("project_id", projectID),
		zap.Uint64("episode_id", episodeID),
		zap.Int("text_len", len(text)),
	)

	// Split text into lines and generate simple VTT
	lines := strings.Split(strings.TrimSpace(text), "\n")
	var vtt strings.Builder
	vtt.WriteString("WEBVTT\n\n")

	secPerChar := 0.15 // ~150ms per character
	currentTime := 0.0
	seq := 1

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		dur := float64(len([]rune(line))) * secPerChar
		if dur < 1.0 {
			dur = 1.0
		}
		endTime := currentTime + dur

		vtt.WriteString(fmt.Sprintf("%d\n", seq))
		vtt.WriteString(fmt.Sprintf("%s --> %s\n", formatVTTTime(currentTime), formatVTTTime(endTime)))
		vtt.WriteString(line + "\n\n")

		currentTime = endTime + 0.3 // small gap
		seq++
	}

	// Write to temp file and upload
	ts := time.Now().UnixMilli()
	vttPath := filepath.Join(s.tempDir, fmt.Sprintf("sub_%d_%d_%d.vtt", projectID, episodeID, ts))
	if err := os.WriteFile(vttPath, []byte(vtt.String()), 0644); err != nil {
		return nil, fmt.Errorf("write vtt: %w", err)
	}
	defer os.Remove(vttPath)

	subtitleURL, err := s.uploadFile(ctx, vttPath, "text/vtt",
		fmt.Sprintf("dubbing/project_%d/ep_%d_subtitle.vtt", projectID, episodeID), int64(projectID), "subtitle")
	if err != nil {
		return nil, fmt.Errorf("upload subtitle: %w", err)
	}

	return &DubbingResult{
		SubtitleURL: subtitleURL,
	}, nil
}

// formatVTTTime —— 将秒数格式化为 VTT 时间字符串 "HH:MM:SS.mmm"
func formatVTTTime(sec float64) string {
	h := int(sec) / 3600
	m := (int(sec) % 3600) / 60
	s := int(sec) % 60
	ms := int((sec - float64(int(sec))) * 1000)
	return fmt.Sprintf("%02d:%02d:%02d.%03d", h, m, s, ms)
}

// uploadFile —— 通过 multipart POST 将本地文件上传到存储服务，返回文件 URL
// uploadFile uploads a local file to the storage service via multipart POST.
func (s *DubbingService) uploadFile(ctx context.Context, localPath, contentType, remoteName string, projectID int64, category string) (string, error) {
	f, err := os.Open(localPath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	writer.WriteField("bucket", "dubbing")
	writer.WriteField("user_id", "1") // system user
	writer.WriteField("project_id", fmt.Sprintf("%d", projectID))
	writer.WriteField("category", category)

	part, err := writer.CreateFormFile("file", filepath.Base(remoteName))
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(part, f); err != nil {
		return "", err
	}
	writer.Close()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		s.storageBaseURL+"/api/v1/storage/upload", &body)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("storage upload: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("storage returned %d: %s", resp.StatusCode, string(respBody))
	}

	// Parse URL from response — storage returns {"data":{"cdn_url":"...","object_key":"..."}}
	var storageResp struct {
		Data struct {
			CdnURL    string `json:"cdn_url"`
			ObjectKey string `json:"object_key"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &storageResp); err == nil && storageResp.Data.CdnURL != "" {
		return storageResp.Data.CdnURL, nil
	}
	if storageResp.Data.ObjectKey != "" {
		return s.storageBaseURL + "/api/v1/storage/url/" + storageResp.Data.ObjectKey, nil
	}

	return "", fmt.Errorf("cannot parse upload response: %s", string(respBody))
}
