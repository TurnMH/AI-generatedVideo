"""
clip-service — 自动剪辑 HTTP 服务

端口: 8092
API:
  POST  /api/v1/clips/process   提交剪辑任务
  GET   /api/v1/clips/{job_id}  查询任务状态
  GET   /api/v1/clips/{job_id}/result  下载 MCP 清单 zip
"""

import os
import uuid
from contextlib import asynccontextmanager
from pathlib import Path
from typing import Optional

from dotenv import load_dotenv
from fastapi import BackgroundTasks, FastAPI, HTTPException
from fastapi.responses import FileResponse
from pydantic import BaseModel

from jobs import JobStore
from pipeline_runner import run_pipeline_job

# 优先加载 clip-service 自身目录的 .env，再加载 MVP 项目 .env（补充 API keys）
_svc_dir = Path(__file__).parent
load_dotenv(_svc_dir / ".env", override=False)
_mvp_env = _svc_dir.parent.parent.parent / "自动视频处理" / "MVP项目" / ".env"
if _mvp_env.exists():
    load_dotenv(_mvp_env, override=False)

store = JobStore()


@asynccontextmanager
async def lifespan(app: FastAPI):
    restored = store.restore_from_disk()
    if restored:
        import logging
        logging.getLogger("clip-service").info("restored %d jobs from disk", restored)
    yield
    store.cleanup_old_jobs()


app = FastAPI(title="clip-service", version="1.0.0", lifespan=lifespan)


# ── 请求/响应模型 ──────────────────────────────────────────────

class ProcessRequest(BaseModel):
    episode_id: str
    shots_metadata: dict          # shots_metadata.json 内容；shot 可含 url 字段（云存储地址）
    script_text: str              # 剧本文本
    video_base_dir: Optional[str] = None  # 本机视频目录；为空时从 shot.url 自动下载
    # 可选覆盖参数
    scene_change_threshold: Optional[float] = None
    draft_width: Optional[int] = None
    draft_height: Optional[int] = None
    draft_fps: Optional[int] = None


class JobStatus(BaseModel):
    job_id: str
    status: str           # pending | running | done | failed
    step: Optional[str] = None
    error: Optional[str] = None
    result_ready: bool = False


# ── 路由 ──────────────────────────────────────────────────────

@app.post("/api/v1/clips/process", response_model=JobStatus, status_code=202)
async def process(req: ProcessRequest, background_tasks: BackgroundTasks):
    job_id = str(uuid.uuid4())
    store.create(job_id)
    background_tasks.add_task(
        run_pipeline_job,
        job_id=job_id,
        store=store,
        episode_id=req.episode_id,
        shots_metadata=req.shots_metadata,
        script_text=req.script_text,
        video_base_dir=req.video_base_dir or "",
        overrides={
            k: v for k, v in {
                "scene_change_threshold": req.scene_change_threshold,
                "draft_width": req.draft_width,
                "draft_height": req.draft_height,
                "draft_fps": req.draft_fps,
            }.items() if v is not None
        },
    )
    return JobStatus(job_id=job_id, status="pending")


@app.get("/api/v1/clips/{job_id}", response_model=JobStatus)
def get_status(job_id: str):
    job = store.get(job_id)
    if job is None:
        raise HTTPException(status_code=404, detail="job not found")
    return JobStatus(
        job_id=job_id,
        status=job["status"],
        step=job.get("step"),
        error=job.get("error"),
        result_ready=job.get("result_ready", False),
    )


@app.get("/api/v1/clips/{job_id}/result")
def get_result(job_id: str):
    job = store.get(job_id)
    if job is None:
        raise HTTPException(status_code=404, detail="job not found")
    if job["status"] != "done":
        raise HTTPException(status_code=409, detail=f"job status: {job['status']}")
    zip_path = job.get("zip_path")
    if not zip_path or not os.path.exists(zip_path):
        raise HTTPException(status_code=500, detail="result file missing")
    return FileResponse(
        path=zip_path,
        media_type="application/zip",
        filename=f"clip_result_{job_id[:8]}.zip",
    )


@app.get("/health")
def health():
    return {"status": "ok"}


if __name__ == "__main__":
    import uvicorn
    port = int(os.getenv("CLIP_SERVICE_PORT", "8092"))
    uvicorn.run("app:app", host="0.0.0.0", port=port, reload=False)
