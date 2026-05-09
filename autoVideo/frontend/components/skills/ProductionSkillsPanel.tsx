'use client'

import { useState, useEffect, useCallback } from 'react'
import { productionSkillAPI, type ProductionSkill } from '@/lib/api'
import { useToast } from '@/components/ui/toast'

// Department display config — icon + Chinese description
const DEPT_META: Record<string, { icon: string; desc: string; color: string }> = {
  // ── 真人影视部门 ──
  subtitle:  { icon: '💬', desc: '字幕/对白',  color: 'bg-blue-50 border-blue-200'   },
  sound:     { icon: '🎙️', desc: '录音/音效',  color: 'bg-purple-50 border-purple-200' },
  dp:        { icon: '🎬', desc: '摄影指导',  color: 'bg-orange-50 border-orange-200' },
  gaffer:    { icon: '💡', desc: '灯光师',    color: 'bg-yellow-50 border-yellow-200' },
  art:       { icon: '🎨', desc: '美术指导',  color: 'bg-green-50 border-green-200'  },
  prop:      { icon: '🗡️', desc: '道具师',    color: 'bg-red-50 border-red-200'      },
  costume:   { icon: '👘', desc: '服装/化妆', color: 'bg-pink-50 border-pink-200'    },
  director:  { icon: '🎭', desc: '场记/执行', color: 'bg-gray-50 border-gray-200'    },
  editor:    { icon: '✂️', desc: '剪辑师',    color: 'bg-indigo-50 border-indigo-200' },
  colorist:  { icon: '🌈', desc: '调色师',    color: 'bg-teal-50 border-teal-200'    },
  // ── 动画部门 ──
  layout:    { icon: '🖼️', desc: '分镜/Layout',  color: 'bg-orange-50 border-orange-200' },
  keyframe:  { icon: '✏️', desc: '原画设计',     color: 'bg-amber-50 border-amber-200'   },
  background:{ icon: '🏞️', desc: '背景美术',     color: 'bg-green-50 border-green-200'   },
  color:     { icon: '🎨', desc: '色彩设计',     color: 'bg-pink-50 border-pink-200'     },
  motion:    { icon: '🌀', desc: '动效设计',     color: 'bg-blue-50 border-blue-200'     },
  vfx:       { icon: '✨', desc: '特效合成',     color: 'bg-purple-50 border-purple-200' },
  voicedir:  { icon: '🎤', desc: '配音导演',     color: 'bg-rose-50 border-rose-200'     },
}

interface Props {
  projectId: number
}

export function ProductionSkillsPanel({ projectId }: Props) {
  const { toast } = useToast()
  const [skills, setSkills] = useState<ProductionSkill[]>([])
  const [loading, setLoading] = useState(true)
  const [seeding, setSeeding] = useState(false)
  const [editingId, setEditingId] = useState<number | null>(null)
  const [editPrompt, setEditPrompt] = useState('')

  const load = useCallback(async () => {
    setLoading(true)
    try {
      const res = await productionSkillAPI.list(projectId)
      setSkills(res.data?.items ?? [])
    } catch {
      toast({ title: '加载影视部门技能失败', variant: 'destructive' })
    } finally {
      setLoading(false)
    }
  }, [projectId])

  useEffect(() => { load() }, [load])

  const handleToggle = async (skill: ProductionSkill) => {
    try {
      await productionSkillAPI.update(projectId, skill.id, { is_active: !skill.is_active })
      setSkills(prev => prev.map(s => s.id === skill.id ? { ...s, is_active: !s.is_active } : s))
    } catch {
      toast({ title: '更新失败', variant: 'destructive' })
    }
  }

  const handleSeedDefaults = async () => {
    setSeeding(true)
    try {
      const res = await productionSkillAPI.seedDefaults(projectId)
      setSkills(res.data?.items ?? [])
      toast({ title: '已生成默认影视部门技能', variant: 'success' })
    } catch {
      toast({ title: '生成默认技能失败', variant: 'destructive' })
    } finally {
      setSeeding(false)
    }
  }

  const handleReseedDefaults = async () => {
    if (!confirm('重置所有部门的 Prompt 到系统默认值（保留开关状态）？')) return
    setSeeding(true)
    try {
      const res = await productionSkillAPI.reseedDefaults(projectId)
      setSkills(res.data?.items ?? [])
      toast({ title: '已重置到默认 Prompt', variant: 'success' })
    } catch {
      toast({ title: '重置失败', variant: 'destructive' })
    } finally {
      setSeeding(false)
    }
  }

  const startEdit = (skill: ProductionSkill) => {
    setEditingId(skill.id)
    setEditPrompt(skill.system_prompt)
  }

  const cancelEdit = () => {
    setEditingId(null)
    setEditPrompt('')
  }

  const saveEdit = async (skill: ProductionSkill) => {
    try {
      await productionSkillAPI.update(projectId, skill.id, { system_prompt: editPrompt })
      setSkills(prev => prev.map(s => s.id === skill.id ? { ...s, system_prompt: editPrompt } : s))
      setEditingId(null)
      toast({ title: 'Prompt 已保存', variant: 'success' })
    } catch {
      toast({ title: '保存失败', variant: 'destructive' })
    }
  }

  if (loading) {
    return (
      <div className="flex items-center justify-center py-12 text-gray-400">
        <span className="animate-spin mr-2">⏳</span> 加载中…
      </div>
    )
  }

  return (
    <div className="space-y-4">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h3 className="text-base font-semibold text-gray-900">影视部门标注技能</h3>
          <p className="text-xs text-gray-500 mt-0.5">
            开启后，分集润色时 LLM 会在剧本中嵌入对应的内联标注，如 <code className="bg-gray-100 px-1 rounded">[摄影:特写镜头]</code>
          </p>
        </div>
        <div className="flex gap-2">
          {skills.length === 0 && (
            <button
              onClick={handleSeedDefaults}
              disabled={seeding}
              className="text-sm px-3 py-1.5 bg-blue-600 text-white rounded-lg hover:bg-blue-700 disabled:opacity-50"
            >
              {seeding ? '生成中…' : '+ 生成默认技能'}
            </button>
          )}
          {skills.length > 0 && (
            <button
              onClick={handleReseedDefaults}
              disabled={seeding}
              className="text-sm px-3 py-1.5 border border-gray-200 text-gray-600 rounded-lg hover:bg-gray-50 disabled:opacity-50"
            >
              {seeding ? '重置中…' : '↺ 重置默认'}
            </button>
          )}
        </div>
      </div>

      {skills.length === 0 && (
        <div className="text-center py-10 border-2 border-dashed border-gray-200 rounded-xl text-gray-400">
          <div className="text-3xl mb-2">🎬</div>
          <p className="text-sm">暂无影视部门技能，点击上方按钮生成默认10个部门</p>
        </div>
      )}

      {/* Skills List */}
      <div className="grid gap-3">
        {skills.map(skill => {
          const meta = DEPT_META[skill.department] ?? { icon: '🔧', desc: skill.department, color: 'bg-gray-50 border-gray-200' }
          const isEditing = editingId === skill.id
          return (
            <div
              key={skill.id}
              className={`border rounded-xl p-4 transition-all ${meta.color} ${skill.is_active ? '' : 'opacity-50'}`}
            >
              <div className="flex items-start justify-between gap-3">
                {/* Left: icon + name + tag */}
                <div className="flex items-center gap-2 min-w-0">
                  <span className="text-xl">{meta.icon}</span>
                  <div>
                    <div className="flex items-center gap-2">
                      <span className="font-medium text-gray-800 text-sm">{skill.name}</span>
                      <code className="text-xs bg-white/60 border border-current/10 px-1.5 py-0.5 rounded text-gray-500">
                        [{skill.label_tag}:]
                      </code>
                    </div>
                    <p className="text-xs text-gray-400">{meta.desc}</p>
                  </div>
                </div>
                {/* Right: controls */}
                <div className="flex items-center gap-2 shrink-0">
                  {!isEditing && (
                    <button
                      onClick={() => startEdit(skill)}
                      className="text-xs text-gray-500 hover:text-gray-700 px-2 py-1 border border-gray-200 rounded bg-white/70"
                    >
                      编辑 Prompt
                    </button>
                  )}
                  {/* Toggle switch */}
                  <button
                    onClick={() => handleToggle(skill)}
                    className={`relative inline-flex h-5 w-9 items-center rounded-full transition-colors ${skill.is_active ? 'bg-blue-500' : 'bg-gray-300'}`}
                  >
                    <span
                      className={`inline-block h-3.5 w-3.5 transform rounded-full bg-white shadow transition-transform ${skill.is_active ? 'translate-x-4' : 'translate-x-1'}`}
                    />
                  </button>
                </div>
              </div>

              {/* Prompt preview / edit */}
              {!isEditing && skill.system_prompt && (
                <p className="mt-2 text-xs text-gray-500 line-clamp-2 leading-relaxed">
                  {skill.system_prompt}
                </p>
              )}
              {isEditing && (
                <div className="mt-3 space-y-2">
                  <textarea
                    value={editPrompt}
                    onChange={e => setEditPrompt(e.target.value)}
                    rows={5}
                    className="w-full text-xs border border-gray-300 rounded-lg p-2 bg-white focus:outline-none focus:ring-1 focus:ring-blue-400 resize-y"
                    placeholder="输入这个部门的注入指令…"
                  />
                  <div className="flex gap-2">
                    <button
                      onClick={() => saveEdit(skill)}
                      className="text-xs px-3 py-1.5 bg-blue-600 text-white rounded-lg hover:bg-blue-700"
                    >
                      保存
                    </button>
                    <button
                      onClick={cancelEdit}
                      className="text-xs px-3 py-1.5 border border-gray-200 text-gray-600 rounded-lg hover:bg-gray-50"
                    >
                      取消
                    </button>
                  </div>
                </div>
              )}
            </div>
          )
        })}
      </div>

      {skills.length > 0 && (
        <p className="text-xs text-gray-400 text-center pt-1">
          {skills.filter(s => s.is_active).length}/{skills.length} 个部门已启用 · 下次分集润色时生效
        </p>
      )}
    </div>
  )
}
