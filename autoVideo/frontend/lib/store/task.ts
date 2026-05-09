import { create } from 'zustand'
import type { Task, TaskProgress } from '@/types'

interface TaskState {
  tasks: Task[]
  progressMap: Record<number, TaskProgress>
  setTasks: (tasks: Task[]) => void
  updateTask: (task: Task) => void
  updateTaskProgress: (taskId: number, progress: TaskProgress) => void
}

export const useTaskStore = create<TaskState>((set) => ({
  tasks: [],
  progressMap: {},
  setTasks: (tasks) => set({ tasks }),
  updateTask: (task) =>
    set((s) => ({ tasks: s.tasks.map((t) => (t.id === task.id ? task : t)) })),
  updateTaskProgress: (taskId, progress) =>
    set((s) => ({ progressMap: { ...s.progressMap, [taskId]: progress } })),
}))
