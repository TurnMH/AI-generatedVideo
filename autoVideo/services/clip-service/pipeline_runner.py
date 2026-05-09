"""
pipeline_runner.py — 在独立子进程中运行 MVP 9 步流水线

每个 job 有自己的 workspace:
  jobs/{job_id}/
    input/
      shots_metadata.json
      script.txt
      videos/  -> 软链接到 video_base_dir
    output/    (各步中间产物)
    config.py  (per-job 配置，覆盖全局 config)
    run.py     (子进程入口)

step09 (剪映 MCP, Windows-only) 替换为打包 zip 输出模式。
"""

import json
import os
import subprocess
import sys
import textwrap
import urllib.request
import zipfile
from pathlib import Path

from jobs import JobStore

# MVP 源码绝对路径
MVP_SRC = Path(__file__).parent.parent.parent.parent / "自动视频处理" / "MVP项目"
MVP_STEPS_DIR = MVP_SRC / "src"


def _workspace(job_id: str) -> Path:
    return Path(__file__).parent / "jobs" / job_id


def _download_videos(shots_metadata: dict, dest_dir: Path) -> dict:
    """将 shots_metadata 中每个 shot 的 url 字段下载到 dest_dir，返回更新后的 metadata。

    shot 结构示例:
        {"shot_id": "001", "file": "001.mp4", "url": "https://..."}
    若 shot 无 url 字段，则跳过（调用方负责保证本地文件已存在）。
    """
    dest_dir.mkdir(parents=True, exist_ok=True)
    updated_shots = []
    for shot in shots_metadata.get("shots", []):
        url = shot.get("url", "")
        if url:
            # 用 shot_id 拼出文件名，保留原扩展名
            ext = Path(url.split("?")[0]).suffix or ".mp4"
            filename = f"{shot['shot_id']}{ext}"
            dest_path = dest_dir / filename
            if not dest_path.exists():
                urllib.request.urlretrieve(url, dest_path)
            shot = {**shot, "file": filename}  # 覆盖 file 字段为本地名
        updated_shots.append(shot)
    return {**shots_metadata, "shots": updated_shots}


def _write_job_config(workspace: Path, video_base_dir: str, overrides: dict) -> None:
    """在 workspace 写一个覆盖全局路径的 config.py。"""
    video_dir = Path(video_base_dir).resolve()

    cfg = textwrap.dedent(f"""\
        # AUTO-GENERATED per-job config — do not edit manually
        import platform
        from pathlib import Path

        PROJECT_ROOT = Path(r"{workspace}")
        INPUT_DIR    = PROJECT_ROOT / "input"
        OUTPUT_DIR   = PROJECT_ROOT / "output"
        VIDEO_DIR    = Path(r"{video_dir}")
        PROMPTS_DIR  = Path(r"{MVP_SRC / 'prompts'}")
        LOG_DIR      = OUTPUT_DIR / "logs"

        BOUNDARIES_DIR  = OUTPUT_DIR / "02_boundaries"
        ANALYSIS_DIR    = OUTPUT_DIR / "03_analysis"
        BGM_DIR         = OUTPUT_DIR / "06_bgm"
        PLACEHOLDER_DIR = OUTPUT_DIR / "07_placeholders"

        SHOTS_METADATA_PATH    = INPUT_DIR / "shots_metadata.json"
        SCRIPT_PATH            = INPUT_DIR / "script.txt"
        BOUNDARIES_PATH        = BOUNDARIES_DIR / "boundaries.json"
        ANALYSIS_PATH          = ANALYSIS_DIR / "analysis.json"
        SHOT_SCRIPT_MAPPING_PATH = ANALYSIS_DIR / "shot_script_mapping.json"
        CLEAN_PLAN_PATH        = OUTPUT_DIR / "04_clean_plan.json"
        JUNCTION_PLAN_PATH     = OUTPUT_DIR / "05_junction_plan.json"
        BGM_PLAN_PATH          = BGM_DIR / "bgm_plan.json"
        MCP_SCRIPT_PATH        = OUTPUT_DIR / "08_mcp_script.json"
        DRAFT_RESULT_PATH      = OUTPUT_DIR / "09_draft_result.json"
        REPORT_PATH            = OUTPUT_DIR / "REPORT.md"

        # 剪映 MCP (不使用，保留变量防止导入报错)
        JIANYING_MCP_DIR   = Path(".")
        JIANYING_DRAFT_PATH = Path(".")

        DRAFT_WIDTH  = {overrides.get('draft_width', 720)}
        DRAFT_HEIGHT = {overrides.get('draft_height', 1280)}
        DRAFT_FPS    = {overrides.get('draft_fps', 30)}

        TRANSITION_MAP = {{
            "fade":        "叠化",
            "dissolve":    "叠化",
            "white_flash": "泛白",
            "black_flash": "泛光",
        }}

        OPENROUTER_BASE_URL = "https://openrouter.ai/api/v1"
        GEMINI_MODEL  = "google/gemini-2.5-pro"
        CLAUDE_MODEL  = "anthropic/claude-sonnet-4.6"

        SCENE_CHANGE_THRESHOLD = {overrides.get('scene_change_threshold', 0.3)}
        MAX_RETRIES            = 2
        RETRY_WAIT_SECONDS     = 5
        JUNCTION_WINDOW_SECONDS = 2.0
        PSNR_THRESHOLD         = 38.0
        BGM_VOLUME             = 0.3
        BGM_FADE_IN_MS         = 1000
        BGM_FADE_OUT_MS        = 1000

        def _detect_cjk_font():
            import platform
            from pathlib import Path as P
            system = platform.system()
            if system == "Darwin":
                candidates = [P("/System/Library/Fonts/PingFang.ttc"),
                              P("/Library/Fonts/Arial Unicode.ttf")]
            elif system == "Windows":
                candidates = [P("C:/Windows/Fonts/msyh.ttc")]
            else:
                candidates = [P("/usr/share/fonts/truetype/noto/NotoSansCJKsc-Regular.otf")]
            for p in candidates:
                if p.exists():
                    return p
            return P("msyh.ttc")

        FONT_PATH = _detect_cjk_font()
    """)
    (workspace / "config.py").write_text(cfg, encoding="utf-8")


def _write_runner_script(workspace: Path) -> None:
    """写子进程入口：import per-job config，依次执行 step01-step08，最后打 zip。"""
    # step09 替换为 zip 打包
    runner = textwrap.dedent(f"""\
        #!/usr/bin/env python3
        \"\"\"Per-job pipeline runner — runs inside the job workspace.\"\"\"
        import sys, os, json, zipfile
        from pathlib import Path

        # 1. config 优先级: workspace > MVP_SRC (workspace 的 config.py 覆盖 MVP 的)
        workspace = Path(__file__).parent
        sys.path.insert(0, str(workspace))          # per-job config.py
        sys.path.insert(1, r"{MVP_SRC}")             # MVP 根目录 (utils.py 等)
        sys.path.insert(2, r"{MVP_STEPS_DIR.parent}") # src/__init__.py

        import config  # noqa: E402 — per-job config

        STATUS_PATH = workspace / "status.json"

        def update_status(step: str, status: str, error: str = ""):
            STATUS_PATH.write_text(
                json.dumps({{"step": step, "status": status, "error": error}}),
                encoding="utf-8",
            )

        steps = [
            ("step01_prepare",   "step01_prepare",   "01-素材准备"),
            ("step02_boundaries","step02_boundaries", "02-镜头切换"),
            ("step03_analysis",  "step03_analysis",  "03-AI拉片"),
            ("step04_clean_plan","step04_clean_plan", "04-废镜过滤"),
            ("step05_junction_plan","step05_junction_plan","05-衔接规划"),
            ("step06_bgm",       "step06_bgm",        "06-BGM生成"),
            ("step07_placeholder","step07_placeholder","07-占位视频"),
            ("step08_mcp_script","step08_mcp_script", "08-MCP清单"),
        ]

        for module_name, run_func_module, label in steps:
            update_status(label, "running")
            try:
                import importlib
                mod = importlib.import_module(f"src.{{module_name}}")
                importlib.reload(mod)   # 确保使用 per-job config
                mod.run()
                update_status(label, "done")
            except Exception as exc:
                update_status(label, "failed", str(exc))
                raise

        # ── step09 替代: 打包 zip ─────────────────────────────────────
        update_status("09-zip打包", "running")
        zip_path = workspace / "result.zip"
        with zipfile.ZipFile(zip_path, "w", zipfile.ZIP_DEFLATED) as zf:
            mcp_script = config.MCP_SCRIPT_PATH
            if mcp_script.exists():
                zf.write(mcp_script, "08_mcp_script.json")
            report = config.REPORT_PATH
            if report.exists():
                zf.write(report, "REPORT.md")
            bgm = config.BGM_DIR / "bgm.mp3"
            if bgm.exists():
                zf.write(bgm, "bgm.mp3")

        update_status("09-zip打包", "done")
        print(f"[clip-pipeline] done, zip: {{zip_path}}")
    """)
    (workspace / "run.py").write_text(runner, encoding="utf-8")


def run_pipeline_job(
    job_id: str,
    store: JobStore,
    episode_id: str,
    shots_metadata: dict,
    script_text: str,
    video_base_dir: str,  # 空字符串时从 shot.url 自动下载
    overrides: dict,
):
    """FastAPI BackgroundTask 入口，在 threadpool 中执行。"""
    workspace = _workspace(job_id)

    try:
        # ── 1. 准备 workspace ──────────────────────────────────────
        videos_dir = workspace / "input" / "videos"
        (workspace / "input").mkdir(parents=True, exist_ok=True)
        (workspace / "output").mkdir(exist_ok=True)

        # 若未指定 video_base_dir，则从 shot.url 下载到本地 videos/ 目录
        if not video_base_dir:
            store.update(job_id, step="00-下载视频", status="running")
            shots_metadata = _download_videos(shots_metadata, videos_dir)
            video_base_dir = str(videos_dir)

        # 写输入文件
        (workspace / "input" / "shots_metadata.json").write_text(
            json.dumps(shots_metadata, ensure_ascii=False, indent=2), encoding="utf-8"
        )
        (workspace / "input" / "script.txt").write_text(script_text, encoding="utf-8")

        # ── 2. 写 per-job config 和 runner ─────────────────────────
        _write_job_config(workspace, video_base_dir, overrides)
        _write_runner_script(workspace)

        store.update(job_id, status="running", step="initializing")

        # ── 3. 子进程运行流水线 ─────────────────────────────────────
        env = os.environ.copy()
        # 传递 API Keys（从宿主进程继承 .env 已 load 的环境变量）

        proc = subprocess.Popen(
            [sys.executable, str(workspace / "run.py")],
            stdout=subprocess.PIPE,
            stderr=subprocess.STDOUT,
            text=True,
            encoding="utf-8",
            env=env,
        )

        log_path = workspace / "pipeline.log"
        with open(log_path, "w", encoding="utf-8") as log_f:
            for line in proc.stdout:
                log_f.write(line)
                # 同步 status.json 进度到 store
                status_file = workspace / "status.json"
                if status_file.exists():
                    try:
                        s = json.loads(status_file.read_text(encoding="utf-8"))
                        store.update(job_id, step=s.get("step"), status="running")
                    except Exception:
                        pass

        proc.wait()

        if proc.returncode != 0:
            store.update(job_id, status="failed", error=f"exit code {proc.returncode}")
            return

        # ── 4. 完成 ─────────────────────────────────────────────────
        zip_path = workspace / "result.zip"
        store.update(
            job_id,
            status="done",
            result_ready=zip_path.exists(),
            zip_path=str(zip_path),
            step="completed",
        )

    except Exception as exc:
        store.update(job_id, status="failed", error=str(exc))
