package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.uber.org/zap"
)

// SynthesizeClipAudios produces one local audio file (mp3) per storyboard clip.
//
// dialogues[i] is the spoken text for clip i. Empty/whitespace entries produce
// ""  in the returned slice so the caller can decide whether to pad with silence
// or leave the clip without audio.
//
// The returned local paths live under DubbingService.tempDir. The caller is
// responsible for removing them when composition finishes.
//
// Multi-speaker lines within a single clip (e.g. "角色名：..." label rows) are
// preserved via buildDubbingChunksWithCharVoices, then concatenated into a
// single per-clip mp3, so that a clip with dialogue between two characters
// still produces ONE aligned audio file.
func (s *DubbingService) SynthesizeClipAudios(
	ctx context.Context,
	projectID, episodeID int64,
	dialogues []string,
	voiceModel, voiceRate, voicePitch, voiceVolume string,
) ([]string, error) {
	if len(dialogues) == 0 {
		return nil, nil
	}

	var charVoiceMap map[string]string
	if isAutoVoiceModel(voiceModel) {
		charVoiceMap = s.fetchCharacterVoiceBindings(ctx, projectID)
	}

	results := make([]string, len(dialogues))
	ts := time.Now().UnixMilli()

	for clipIdx, raw := range dialogues {
		text := cleanScriptForSpeech(strings.ReplaceAll(raw, "\r\n", "\n"))
		text = strings.TrimSpace(text)
		if text == "" {
			continue
		}

		chunks := buildDubbingChunksWithCharVoices(text, voiceModel, charVoiceMap)
		if len(chunks) == 0 {
			continue
		}

		var chunkPaths []string
		cleanup := func() {
			for _, p := range chunkPaths {
				os.Remove(p)
			}
		}

		for chunkIdx, chunk := range chunks {
			audioPath := filepath.Join(s.tempDir,
				fmt.Sprintf("pcdub_%d_%d_%d_clip%d_c%d.mp3", projectID, episodeID, ts, clipIdx, chunkIdx))

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
				endpoint := strings.TrimPrefix(voiceModel, "comfyui:")
				chunkErr = s.runComfyUITTS(ctx, endpoint, chunk.Text, audioPath)
			default:
				textPath := filepath.Join(s.tempDir,
					fmt.Sprintf("pctxt_%d_%d_%d_clip%d_c%d.txt", projectID, episodeID, ts, clipIdx, chunkIdx))
				if err := os.WriteFile(textPath, []byte(chunk.Text), 0644); err != nil {
					cleanup()
					return nil, fmt.Errorf("write clip%d chunk%d text: %w", clipIdx, chunkIdx, err)
				}
				subPath := audioPath + ".vtt" // discarded — per-clip offsets built separately
				chunkErr = s.runEdgeTTS(ctx, resolveEdgeVoice(chunk.VoiceKey),
					voiceRate, voicePitch, voiceVolume, textPath, audioPath, subPath)
				os.Remove(textPath)
				os.Remove(subPath)
			}

			if chunkErr != nil {
				s.logger.Warn("per-clip tts chunk failed",
					zap.Int("clip", clipIdx),
					zap.Int("chunk", chunkIdx),
					zap.String("speaker", chunk.Speaker),
					zap.Error(chunkErr),
				)
				continue
			}
			if fi, err := os.Stat(audioPath); err == nil && fi.Size() > 0 {
				chunkPaths = append(chunkPaths, audioPath)
			}
		}

		if len(chunkPaths) == 0 {
			s.logger.Warn("per-clip tts produced no audio", zap.Int("clip", clipIdx))
			continue
		}

		if len(chunkPaths) == 1 {
			results[clipIdx] = chunkPaths[0]
			continue
		}

		merged := filepath.Join(s.tempDir,
			fmt.Sprintf("pcdub_%d_%d_%d_clip%d.mp3", projectID, episodeID, ts, clipIdx))
		if err := s.concatAudio(ctx, chunkPaths, merged); err != nil {
			cleanup()
			return nil, fmt.Errorf("concat clip %d audio: %w", clipIdx, err)
		}
		cleanup()
		results[clipIdx] = merged
	}

	return results, nil
}
