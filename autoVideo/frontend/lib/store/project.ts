import { create } from 'zustand'
import type { Project, Episode } from '@/types'

interface ProjectState {
  projects: Project[]
  currentProject: Project | null
  currentEpisode: Episode | null
  setProjects: (projects: Project[]) => void
  setCurrentProject: (project: Project | null) => void
  setCurrentEpisode: (episode: Episode | null) => void
  updateProject: (project: Project) => void
  removeProject: (id: number) => void
}

export const useProjectStore = create<ProjectState>((set) => ({
  projects: [],
  currentProject: null,
  currentEpisode: null,
  setProjects: (projects) => set({ projects }),
  setCurrentProject: (project) => set({ currentProject: project }),
  setCurrentEpisode: (episode) => set({ currentEpisode: episode }),
  updateProject: (project) =>
    set((s) => ({
      projects: s.projects.map((p) => (p.id === project.id ? project : p)),
      currentProject:
        s.currentProject?.id === project.id ? project : s.currentProject,
    })),
  removeProject: (id) =>
    set((s) => ({
      projects: s.projects.filter((p) => p.id !== id),
      currentProject: s.currentProject?.id === id ? null : s.currentProject,
    })),
}))
