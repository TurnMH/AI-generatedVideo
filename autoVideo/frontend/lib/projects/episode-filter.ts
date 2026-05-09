'use client'

import React from 'react'

export interface ProjectEpisodeFilterValue {
  value: string
  setValue: (v: string) => void
}

export const ProjectEpisodeFilterContext = React.createContext<ProjectEpisodeFilterValue>({
  value: 'all',
  setValue: () => {},
})

export function useProjectEpisodeFilter() {
  return React.useContext(ProjectEpisodeFilterContext)
}
