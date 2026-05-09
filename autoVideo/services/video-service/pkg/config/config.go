package config

import (
	"os"
	"strings"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	HTTP struct {
		Port int `mapstructure:"port"`
	} `mapstructure:"http"`

	DB struct {
		DSN string `mapstructure:"dsn"`
	} `mapstructure:"db"`

	Kafka struct {
		Brokers       []string `mapstructure:"brokers"`
		ConsumerGroup string   `mapstructure:"consumer_group"`
		ConsumerTopic string   `mapstructure:"consumer_topic"`
		ProducerTopic string   `mapstructure:"producer_topic"`
	} `mapstructure:"kafka"`

	JWT struct {
		Secret string `mapstructure:"secret"`
	} `mapstructure:"jwt"`

	Storage struct {
		BaseURL string `mapstructure:"base_url"`
	} `mapstructure:"storage"`

	Character struct {
		BaseURL string `mapstructure:"base_url"`
	} `mapstructure:"character"`

	// 视频串行生成：末帧提取服务
	FrameExtractor struct {
		BaseURL string `mapstructure:"base_url"` // e.g. http://localhost:8010
	} `mapstructure:"frame_extractor"`

	// feat-4: Whisper subtitle sidecar
	Whisper struct {
		URL string `mapstructure:"url"` // e.g. http://whisper-sidecar:8010
	} `mapstructure:"whisper"`

	Models struct {
		KlingKey        string   `mapstructure:"kling_key"`
		KlingKeys       []string `mapstructure:"kling_keys"` // multi-key pool for rotation
		KlingBase       string   `mapstructure:"kling_base"`
		KlingSecret     string   `mapstructure:"kling_secret"` // Tencent Cloud SecretKey (paired with kling_key as SecretId)
		WanKey          string `mapstructure:"wan_key"`    // Volcengine Access Key ID
		WanSecret       string `mapstructure:"wan_secret"` // Volcengine Secret Access Key
		WanBase         string `mapstructure:"wan_base"`
		ComfyUIURL      string `mapstructure:"comfyui_url"`
		ComfyUIWorkflow string `mapstructure:"comfyui_workflow"`
		// char-c5: IP-Adapter model for character reference in ComfyUI workflow
		ComfyUIIPAdapter string `mapstructure:"comfyui_ipadapter"`
		// char-c6: LoRA model and weight for character style binding
		ComfyUILoRAModel  string  `mapstructure:"comfyui_lora_model"`
		ComfyUILoRAWeight float64 `mapstructure:"comfyui_lora_weight"`
		Sora2Key        string `mapstructure:"sora2_key"`
		Sora2Base       string `mapstructure:"sora2_base"`
		HubagiKey       string `mapstructure:"hubagi_key"`
		HubagiBase      string `mapstructure:"hubagi_base"`
		HubagiModel     string `mapstructure:"hubagi_model"`
		VeoKey          string `mapstructure:"veo_key"`
		VeoBase         string `mapstructure:"veo_base"`
		VeoModel        string `mapstructure:"veo_model"`
		DoubaoKey              string `mapstructure:"doubao_key"`
		DoubaoBase             string `mapstructure:"doubao_base"`
		DoubaoModel            string `mapstructure:"doubao_model"`
		DoubaoSeedanceKey      string `mapstructure:"doubao_seedance_key"`
		DoubaoSeedanceBase     string `mapstructure:"doubao_seedance_base"`
		DoubaoSeedanceModel    string `mapstructure:"doubao_seedance_model"`
		ViduKey                string `mapstructure:"vidu_key"`
		ViduOffpeakKey         string `mapstructure:"vidu_offpeak_key"`
		ViduBase               string `mapstructure:"vidu_base"`
		ViduModel              string `mapstructure:"vidu_model"`
		ViduMixModel           string `mapstructure:"vidu_mix_model"`
		SuannengKey     string `mapstructure:"suanneng_key"`
		SuannengBase    string `mapstructure:"suanneng_base"`
		SuannengModel   string `mapstructure:"suanneng_model"`
		GagaKey         string `mapstructure:"gaga_key"`
		GagaBase        string `mapstructure:"gaga_base"`
		// 百度 BCE 视频生成（BCE-AUTH-V1 签名）
		BaiduBCEKey    string `mapstructure:"baidu_bce_key"`
		BaiduBCESecret string `mapstructure:"baidu_bce_secret"`
		BaiduBCEModel  string `mapstructure:"baidu_bce_model"` // V10/V15/V20/VQ1
		ReplicateKey    string `mapstructure:"replicate_key"`
		// RunningHub cloud ComfyUI (opt-p3)
		RunningHubKey      string `mapstructure:"runninghub_key"`
		RunningHubBase     string `mapstructure:"runninghub_base"`
		RunningHubWorkflow string `mapstructure:"runninghub_workflow"`
		RunningHubNodeID   string `mapstructure:"runninghub_node_id"`
		// feat-11: social media publishing via Upload-Post API
		UploadPostKey string `mapstructure:"upload_post_key"`
		UploadPostURL string `mapstructure:"upload_post_url"`
		// Kling 3.0 (星澜3.0) — same APPID/Secret as kling_key/kling_secret but with v3 model names
		KlingModel      string `mapstructure:"kling_model"`      // e.g. "kling-v3" for v3.0
		KlingOmniModel  string `mapstructure:"kling_omni_model"` // e.g. "kling-v3-omni" for fusion
		// aiping channel (Kling-compatible API for high-concurrency scenarios)
		AipingKey  string `mapstructure:"aiping_key"`
		AipingBase string `mapstructure:"aiping_base"`
		// Tencent VCLM (vclm.tencentcloudapi.com) — TC3-HMAC-SHA256 signing
		VclmSecretID  string `mapstructure:"vclm_secret_id"`
		VclmSecretKey string `mapstructure:"vclm_secret_key"`
		VclmRegion    string `mapstructure:"vclm_region"` // default: ap-guangzhou
		// LLM for video motion prompt refinement (opt-motion-llm)
		// Uses any OpenAI-compatible endpoint; defaults to sora2 channel if unset.
		LLMKey   string `mapstructure:"llm_key"`
		LLMBase  string `mapstructure:"llm_base"`
		LLMModel string `mapstructure:"llm_model"` // defaults to "gpt-4.1-mini"
		// Music generation API (music-worker)
		// Compatible with SiliconFlow /v1/audio/speech or any configurable audio endpoint.
		MusicKey   string `mapstructure:"music_key"`
		MusicBase  string `mapstructure:"music_base"`
		MusicModel string `mapstructure:"music_model"` // e.g. "fishaudio/fish-speech-1.5"
	} `mapstructure:"models"`

	FFmpeg struct {
		TempDir string `mapstructure:"temp_dir"`
		Bin     string `mapstructure:"bin"`
	} `mapstructure:"ffmpeg"`

	Concurrency struct {
		MaxClips      int `mapstructure:"max_clips"`
		LocalMaxClips int `mapstructure:"local_max_clips"`  // clips per task for local models (ComfyUI)
		MaxKafkaTasks int `mapstructure:"max_kafka_tasks"`  // concurrent Kafka video tasks
	} `mapstructure:"concurrency"`
	AuthService struct {
		BaseURL string `mapstructure:"base_url"`
	} `mapstructure:"auth_service"`
	Gateway struct {
		Addr     string `mapstructure:"addr"`
		SelfAddr string `mapstructure:"self_addr"`
	} `mapstructure:"gateway"`
}

// Load —— 加载配置文件和环境变量，返回 *Config
func Load() (*Config, error) {
	viper.SetConfigType("yaml")

	// Env variable overrides: VIDEO_SERVICE_DB_DSN → db.dsn
	viper.SetEnvPrefix("VIDEO_SERVICE")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	// Defaults
	viper.SetDefault("http.port", 8006)
	viper.SetDefault("character.base_url", "http://localhost:8004")
	viper.SetDefault("ffmpeg.bin", "ffmpeg")
	viper.SetDefault("ffmpeg.temp_dir", "/tmp/video-service")
	viper.SetDefault("kafka.consumer_group", "video-service")
	viper.SetDefault("kafka.consumer_topic", "video.generate.request")
	viper.SetDefault("kafka.producer_topic", "video.generate.result")
	viper.SetDefault("models.kling_base", "https://api.klingai.com")
	viper.SetDefault("models.wan_base", "https://visual.volcengineapi.com")
	viper.SetDefault("models.llm_model", "gpt-4.1-mini") // opt-motion-llm
	viper.SetDefault("models.music_base", "https://api.siliconflow.cn")
	viper.SetDefault("models.music_model", "fishaudio/fish-speech-1.5")
	viper.SetDefault("concurrency.max_clips", 10)
	viper.SetDefault("concurrency.local_max_clips", 1)  // local GPU: 1 clip at a time
	viper.SetDefault("concurrency.max_kafka_tasks", 3)
	viper.SetDefault("auth_service.base_url", "http://localhost:8001")
	viper.SetDefault("gateway.addr", "http://localhost:8000")
	viper.SetDefault("gateway.self_addr", "")

	if configFile := os.Getenv("AUTOVIDEO_CONFIG_FILE"); configFile != "" {
		viper.SetConfigFile(configFile)
	} else {
		// Load unified config from project root
		viper.SetConfigName("config")
		viper.AddConfigPath("../../")
		viper.AddConfigPath(".")
		viper.AddConfigPath("./config")
		viper.AddConfigPath("/etc/video-service")
	}
	_ = viper.ReadInConfig()

	// Merge service-specific section on top of shared values
	if sub := viper.Sub("video-service"); sub != nil {
		viper.MergeConfigMap(sub.AllSettings())
	}

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// StartWatcher polls the config file at path every 30 seconds and calls onChange
// when the file's modification time changes. It runs in a background goroutine.
func StartWatcher(path string, onChange func(*Config)) {
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		var lastMod time.Time
		if info, err := os.Stat(path); err == nil {
			lastMod = info.ModTime()
		}
		for range ticker.C {
			info, err := os.Stat(path)
			if err != nil {
				continue
			}
			if info.ModTime().After(lastMod) {
				lastMod = info.ModTime()
				if cfg, err := Load(); err == nil {
					onChange(cfg)
				}
			}
		}
	}()
}
