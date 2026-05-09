"""
whisper-sidecar — Fast transcription microservice (feat-4)
Provides a REST API to transcribe audio using faster-whisper (large-v3, INT8 CPU).

POST /transcribe
  Body: { "audio_url": "...", "language": "zh" }
  Returns: { "srt": "...", "segments": [...] }

GET /health
  Returns: { "status": "ok" }
"""

import io
import os
import re
import tempfile
import urllib.request
from pathlib import Path
from typing import Optional

from fastapi import FastAPI, HTTPException
from pydantic import BaseModel

# faster-whisper is the inference backend (pip install faster-whisper)
try:
    from faster_whisper import WhisperModel
    _model_available = True
except ImportError:
    _model_available = False

app = FastAPI(title="whisper-sidecar", version="1.0.0")

# Lazy-load the model on first request to avoid startup delay
_whisper_model: Optional["WhisperModel"] = None

MODEL_SIZE = os.getenv("WHISPER_MODEL_SIZE", "large-v3")
COMPUTE_TYPE = os.getenv("WHISPER_COMPUTE_TYPE", "int8")  # int8 for CPU, float16 for GPU
DEVICE = os.getenv("WHISPER_DEVICE", "cpu")


def get_model() -> "WhisperModel":
    global _whisper_model
    if _whisper_model is None:
        if not _model_available:
            raise RuntimeError("faster-whisper not installed: pip install faster-whisper")
        _whisper_model = WhisperModel(MODEL_SIZE, device=DEVICE, compute_type=COMPUTE_TYPE)
    return _whisper_model


class TranscribeRequest(BaseModel):
    audio_url: str
    language: str = "zh"
    # Optional: known sentence text to align against (for better accuracy)
    reference_text: Optional[str] = None


class SegmentInfo(BaseModel):
    start: float
    end: float
    text: str


class TranscribeResponse(BaseModel):
    srt: str
    segments: list[SegmentInfo]
    language: str


def _download_audio(url: str) -> str:
    """Download audio from URL to a temp file, return path."""
    suffix = Path(url.split("?")[0]).suffix or ".mp3"
    fd, path = tempfile.mkstemp(suffix=suffix)
    try:
        with urllib.request.urlopen(url, timeout=60) as resp:
            with os.fdopen(fd, "wb") as f:
                f.write(resp.read())
    except Exception as e:
        os.unlink(path)
        raise HTTPException(status_code=400, detail=f"Failed to download audio: {e}")
    return path


def _format_srt_time(seconds: float) -> str:
    """Convert seconds to SRT timestamp format HH:MM:SS,mmm"""
    h = int(seconds // 3600)
    m = int((seconds % 3600) // 60)
    s = int(seconds % 60)
    ms = int((seconds - int(seconds)) * 1000)
    return f"{h:02d}:{m:02d}:{s:02d},{ms:03d}"


def _segments_to_srt(segments) -> str:
    """Convert faster-whisper segments to SRT format string."""
    lines = []
    for i, seg in enumerate(segments, 1):
        start = _format_srt_time(seg.start)
        end = _format_srt_time(seg.end)
        text = seg.text.strip()
        if text:
            lines.append(f"{i}\n{start} --> {end}\n{text}\n")
    return "\n".join(lines)


@app.get("/health")
def health():
    return {"status": "ok", "model": MODEL_SIZE if _model_available else "unavailable"}


@app.post("/transcribe", response_model=TranscribeResponse)
def transcribe(req: TranscribeRequest):
    """Transcribe audio from URL, return SRT and segment list."""
    model = get_model()

    audio_path = _download_audio(req.audio_url)
    try:
        segments_iter, info = model.transcribe(
            audio_path,
            language=req.language if req.language else None,
            beam_size=5,
            vad_filter=True,
            vad_parameters=dict(min_silence_duration_ms=500),
        )
        segments = list(segments_iter)
    except Exception as e:
        raise HTTPException(status_code=500, detail=f"Transcription failed: {e}")
    finally:
        try:
            os.unlink(audio_path)
        except Exception:
            pass

    srt = _segments_to_srt(segments)
    seg_infos = [
        SegmentInfo(start=s.start, end=s.end, text=s.text.strip())
        for s in segments if s.text.strip()
    ]

    return TranscribeResponse(
        srt=srt,
        segments=seg_infos,
        language=info.language,
    )


if __name__ == "__main__":
    import uvicorn
    port = int(os.getenv("PORT", "8010"))
    uvicorn.run(app, host="0.0.0.0", port=port)
