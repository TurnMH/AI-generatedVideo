package service

import (
	"github.com/autovideo/project-service/internal/model"
	"github.com/autovideo/project-service/internal/repository"
)

// CostEstimate holds the estimated production cost for a project.
type CostEstimate struct {
	TextCost  float64 `json:"text_cost"`
	ImageCost float64 `json:"image_cost"`
	VideoCost float64 `json:"video_cost"`
	TTSCost   float64 `json:"tts_cost"`
	TotalMin  float64 `json:"total_min"`
	TotalMax  float64 `json:"total_max"`
}

// Placeholder per-unit prices (USD).
const (
	priceTextPerEpisode  = 0.05  // ~2k tokens per episode
	priceImagePerShot    = 0.02  // per image generation
	priceVideoPerShot    = 0.10  // per video clip generation
	priceTTSPerEpisode   = 0.06  // ~500 chars per episode
	estimatedShotsPerEp  = 8     // average storyboard shots per episode
	costVariancePercent  = 0.20  // ±20% for min/max range
)

// CostService calculates cost estimates for projects.
type CostService struct {
	projectRepo *repository.ProjectRepo
	episodeRepo *repository.EpisodeRepo
}

// NewCostService —— 创建成本预估服务实例
// NewCostService creates a CostService.
func NewCostService(projectRepo *repository.ProjectRepo, episodeRepo *repository.EpisodeRepo) *CostService {
	return &CostService{
		projectRepo: projectRepo,
		episodeRepo: episodeRepo,
	}
}

// EstimateCost —— 根据项目配置计算预估生产成本，返回各项费用及总计范围
// EstimateCost calculates a rough cost estimate based on project configuration.
func (s *CostService) EstimateCost(project *model.Project) (*CostEstimate, error) {
	episodes := project.TargetEpisodes
	if episodes <= 0 {
		episodes = len(project.Episodes)
	}
	if episodes <= 0 {
		episodes = 1
	}

	totalShots := episodes * estimatedShotsPerEp

	textCost := float64(episodes) * priceTextPerEpisode
	imageCost := float64(totalShots) * priceImagePerShot
	videoCost := float64(totalShots) * priceVideoPerShot
	ttsCost := 0.0
	if project.EnableDubbing {
		ttsCost = float64(episodes) * priceTTSPerEpisode
	}

	base := textCost + imageCost + videoCost + ttsCost
	totalMin := base * (1 - costVariancePercent)
	totalMax := base * (1 + costVariancePercent)

	return &CostEstimate{
		TextCost:  textCost,
		ImageCost: imageCost,
		VideoCost: videoCost,
		TTSCost:   ttsCost,
		TotalMin:  totalMin,
		TotalMax:  totalMax,
	}, nil
}
