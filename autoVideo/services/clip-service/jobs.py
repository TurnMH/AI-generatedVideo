"""
jobs.py — Job 状态存储（内存 + 磁盘双写，进程重启后可恢复）

每个 job 的状态持久化到 jobs/{job_id}/job_state.json；
启动时调用 JobStore.restore_from_disk() 加载历史任务。
"""

import json
import threading
import time
from pathlib import Path
from typing import Optional

# jobs/ 目录与 app.py 同级
_JOBS_ROOT = Path(__file__).parent / "jobs"


class JobStore:
    def __init__(self, max_age_hours: int = 24):
        self._lock = threading.Lock()
        self._jobs: dict[str, dict] = {}
        self._max_age_seconds = max_age_hours * 3600

    # ── 内部：磁盘读写 ──────────────────────────────────────────

    @staticmethod
    def _state_path(job_id: str) -> Path:
        return _JOBS_ROOT / job_id / "job_state.json"

    def _persist(self, job_id: str, state: dict) -> None:
        """将 state 写入 jobs/{job_id}/job_state.json，忽略 IO 错误。"""
        try:
            path = self._state_path(job_id)
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_text(json.dumps(state, ensure_ascii=False), encoding="utf-8")
        except Exception:
            pass

    # ── 公开接口 ────────────────────────────────────────────────

    def create(self, job_id: str):
        state = {
            "status": "pending",
            "created_at": time.time(),
            "result_ready": False,
        }
        with self._lock:
            self._jobs[job_id] = state
        self._persist(job_id, state)

    def update(self, job_id: str, **kwargs):
        with self._lock:
            if job_id in self._jobs:
                self._jobs[job_id].update(kwargs)
                state = dict(self._jobs[job_id])
            else:
                return
        self._persist(job_id, state)

    def get(self, job_id: str) -> Optional[dict]:
        with self._lock:
            if job_id in self._jobs:
                return self._jobs.get(job_id)
        # 内存中不存在时尝试从磁盘恢复（跨重启查询）
        path = self._state_path(job_id)
        if path.exists():
            try:
                state = json.loads(path.read_text(encoding="utf-8"))
                with self._lock:
                    self._jobs[job_id] = state
                return state
            except Exception:
                pass
        return None

    def restore_from_disk(self) -> int:
        """扫描 jobs/ 目录，将所有历史 job_state.json 加载到内存。返回恢复条数。"""
        count = 0
        if not _JOBS_ROOT.exists():
            return 0
        for state_file in _JOBS_ROOT.glob("*/job_state.json"):
            job_id = state_file.parent.name
            try:
                state = json.loads(state_file.read_text(encoding="utf-8"))
                # 进行中的任务视为失败（进程已重启）
                if state.get("status") == "running":
                    state["status"] = "failed"
                    state["error"] = "service restarted while job was running"
                    state_file.write_text(json.dumps(state, ensure_ascii=False), encoding="utf-8")
                with self._lock:
                    self._jobs[job_id] = state
                count += 1
            except Exception:
                pass
        return count

    def cleanup_old_jobs(self):
        """删除超过 max_age_hours 的已完成任务（内存 + 磁盘）。"""
        now = time.time()
        with self._lock:
            to_delete = [
                jid for jid, j in self._jobs.items()
                if j["status"] in ("done", "failed")
                and now - j.get("created_at", now) > self._max_age_seconds
            ]
            for jid in to_delete:
                del self._jobs[jid]
        # 同步清理磁盘
        for jid in to_delete:
            state_file = self._state_path(jid)
            try:
                if state_file.exists():
                    state_file.unlink()
            except Exception:
                pass
