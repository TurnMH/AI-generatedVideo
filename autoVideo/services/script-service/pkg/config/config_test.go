package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
)

func TestLoadPreservesSharedKafkaBrokersWhenMergingScriptServiceConfig(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)

	baseDir := t.TempDir()
	basePath := filepath.Join(baseDir, "base.yaml")

	base := []byte(`kafka:
  brokers:
    - "kafka:29092"
script-service:
  kafka:
    consumer_topic: "script.analyze.request"
    producer_topic: "script.analyze.result"
`)
	if err := os.WriteFile(basePath, base, 0o600); err != nil {
		t.Fatalf("write base config: %v", err)
	}

	t.Setenv("AUTOVIDEO_CONFIG_FILE", basePath)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(cfg.Kafka.Brokers) != 1 || cfg.Kafka.Brokers[0] != "kafka:29092" {
		t.Fatalf("cfg.Kafka.Brokers = %v, want [kafka:29092]", cfg.Kafka.Brokers)
	}
	if cfg.Kafka.ConsumerTopic != "script.analyze.request" {
		t.Fatalf("cfg.Kafka.ConsumerTopic = %q, want script.analyze.request", cfg.Kafka.ConsumerTopic)
	}
	if cfg.Kafka.ProducerTopic != "script.analyze.result" {
		t.Fatalf("cfg.Kafka.ProducerTopic = %q, want script.analyze.result", cfg.Kafka.ProducerTopic)
	}
}