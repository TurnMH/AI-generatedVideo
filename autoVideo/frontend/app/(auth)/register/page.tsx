'use client'

import { useState, useEffect, useRef } from 'react'
import { useRouter } from 'next/navigation'
import { BookOpen, Clapperboard, ImageIcon, Lock, Mail, User, UserPlus, Video } from 'lucide-react'
import { authAPI } from '@/lib/api'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { LoadingSpinner } from '@/components/common/LoadingSpinner'
import {
  EntryCard,
  EntryCopyright,
  EntryFeatureStrip,
  EntryFooterLink,
  EntryPageLayout,
  MarketingShowcase,
  type MarketingItem,
} from '@/components/marketing/entry-ui'

interface FormState {
  username: string
  email: string
  password: string
  confirmPassword: string
}

interface FieldErrors {
  username?: string
  email?: string
  password?: string
  confirmPassword?: string
}

const creativeItems: MarketingItem[] = [
  {
    title: '图片创作',
    description: '让封面、插画、角色图和物料视觉更统一、更高级。',
    icon: ImageIcon,
    accent: 'from-cyan-400/30 via-sky-500/20 to-transparent',
  },
  {
    title: '视频成片',
    description: '脚本、分镜、镜头、配音到成片的链路集中管理。',
    icon: Video,
    accent: 'from-violet-400/30 via-fuchsia-500/20 to-transparent',
  },
  {
    title: '小说设定',
    description: '把人物关系、剧情结构与世界观快速沉淀为生产素材。',
    icon: BookOpen,
    accent: 'from-amber-300/30 via-orange-500/20 to-transparent',
  },
  {
    title: '漫画表达',
    description: '强化镜头感、节奏感和画面情绪，形成更强的内容风格。',
    icon: Clapperboard,
    accent: 'from-pink-400/30 via-rose-500/20 to-transparent',
  },
]

function validate(values: FormState): FieldErrors {
  const errors: FieldErrors = {}
  if (!values.username) {
    errors.username = '请输入用户名'
  } else if (values.username.length < 3) {
    errors.username = '用户名至少 3 个字符'
  }
  if (!values.email) {
    errors.email = '请输入邮箱'
  } else if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(values.email)) {
    errors.email = '邮箱格式不正确'
  }
  if (!values.password) {
    errors.password = '请输入密码'
  } else if (values.password.length < 8) {
    errors.password = '密码至少 8 个字符'
  }
  if (!values.confirmPassword) {
    errors.confirmPassword = '请确认密码'
  } else if (values.password !== values.confirmPassword) {
    errors.confirmPassword = '两次密码不一致'
  }
  return errors
}

export default function RegisterPage() {
  const router = useRouter()
  const [form, setForm] = useState<FormState>({
    username: '',
    email: '',
    password: '',
    confirmPassword: '',
  })
  const [errors, setErrors] = useState<FieldErrors>({})
  const [serverError, setServerError] = useState('')
  const [loading, setLoading] = useState(false)
  const [slowNetwork, setSlowNetwork] = useState(false)
  const slowTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)

  // Show slow-network hint after 3 s of loading
  useEffect(() => {
    if (loading) {
      slowTimerRef.current = setTimeout(() => setSlowNetwork(true), 3000)
    } else {
      if (slowTimerRef.current) clearTimeout(slowTimerRef.current)
      setSlowNetwork(false)
    }
    return () => { if (slowTimerRef.current) clearTimeout(slowTimerRef.current) }
  }, [loading])

  const handleChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const { name, value } = e.target
    setForm((prev) => ({ ...prev, [name]: value }))
    if (errors[name as keyof FieldErrors]) {
      setErrors((prev) => ({ ...prev, [name]: undefined }))
    }
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    const fieldErrors = validate(form)
    if (Object.keys(fieldErrors).length > 0) {
      setErrors(fieldErrors)
      return
    }
    setLoading(true)
    setServerError('')
    try {
      await authAPI.register({
        username: form.username,
        email: form.email,
        password: form.password,
      })
      router.push('/login?registered=1')
    } catch (err: any) {
      console.error('Register error:', err)
      const errMsg = err?.response?.data?.error || err?.response?.data?.message || err?.message || '注册失败，请稍后重试'
      setServerError(errMsg)
    } finally {
      setLoading(false)
    }
  }

  return (
    <EntryPageLayout animatedBackground>
      <div className="flex min-h-screen items-center">
        <div className="grid w-full gap-8 lg:grid-cols-[1.15fr_0.85fr] lg:items-center">
          <MarketingShowcase
            badge="从注册开始进入创作生态"
            title={
              <>
                创建你的
                <span className="bg-gradient-to-r from-cyan-300 via-violet-300 to-pink-300 bg-clip-text text-transparent">
                  AI 内容工作台
                </span>
              </>
            }
            description="用一个账户连接图片、视频、小说、漫画等创作能力，让灵感沉淀、生产和输出都在同一套系统里完成。"
            items={creativeItems}
            footer={
              <div className="rounded-3xl border border-white/10 bg-white/[0.04] p-5 backdrop-blur-xl">
                <div className="flex items-center gap-3">
                  <div className="flex h-12 w-12 items-center justify-center rounded-2xl border border-white/10 bg-white/10">
                    <UserPlus className="h-6 w-6 text-white" />
                  </div>
                  <div>
                    <p className="text-sm font-medium text-white">注册后立即开始</p>
                    <p className="text-sm text-surface-400">
                      建项目、写剧本、出图、做视频，统一从这里启程。
                    </p>
                  </div>
                </div>
              </div>
            }
          />

          <section className="mx-auto w-full max-w-md">
            <EntryCard
              title="创建账户"
              subtitle="一步接入图片、视频、剧情、分镜与后续成片工作流。"
              tags={['图片', '视频', '小说', '漫画']}
            >
              <form onSubmit={handleSubmit} noValidate className="space-y-5">
                <div className="space-y-1.5">
                  <Label htmlFor="username" className="text-surface-200">用户名</Label>
                  <div className="group relative">
                    <User className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-slate-300 transition-colors group-focus-within:text-cyan-200" />
                    <Input
                      id="username"
                      name="username"
                      type="text"
                      autoComplete="username"
                      placeholder="你的用户名"
                      value={form.username}
                      onChange={handleChange}
                      error={!!errors.username}
                      style={{ backgroundColor: 'rgba(15, 23, 42, 0.75)' }}
                      className="border-white/10 backdrop-blur-none pl-10 text-white shadow-[inset_0_1px_0_rgba(255,255,255,0.06)] placeholder:text-slate-500 focus-visible:border-cyan-300/70 focus-visible:ring-2 focus-visible:ring-violet-400/30 [&:-webkit-autofill]:shadow-[0_0_0_1000px_rgba(15,23,42,0.75)_inset] [&:-webkit-autofill]:[-webkit-text-fill-color:white]"
                    />
                  </div>
                  {errors.username && <p className="text-xs text-red-400">{errors.username}</p>}
                </div>

                <div className="space-y-1.5">
                  <Label htmlFor="email" className="text-surface-200">邮箱</Label>
                  <div className="group relative">
                    <Mail className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-slate-300 transition-colors group-focus-within:text-cyan-200" />
                    <Input
                      id="email"
                      name="email"
                      type="email"
                      autoComplete="email"
                      placeholder="you@example.com"
                      value={form.email}
                      onChange={handleChange}
                      error={!!errors.email}
                      style={{ backgroundColor: 'rgba(15, 23, 42, 0.75)' }}
                      className="border-white/10 backdrop-blur-none pl-10 text-white shadow-[inset_0_1px_0_rgba(255,255,255,0.06)] placeholder:text-slate-500 focus-visible:border-cyan-300/70 focus-visible:ring-2 focus-visible:ring-violet-400/30 [&:-webkit-autofill]:shadow-[0_0_0_1000px_rgba(15,23,42,0.75)_inset] [&:-webkit-autofill]:[-webkit-text-fill-color:white]"
                    />
                  </div>
                  {errors.email && <p className="text-xs text-red-400">{errors.email}</p>}
                </div>

                <div className="space-y-1.5">
                  <Label htmlFor="password" className="text-surface-200">密码</Label>
                  <div className="group relative">
                    <Lock className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-slate-300 transition-colors group-focus-within:text-violet-200" />
                    <Input
                      id="password"
                      name="password"
                      type="password"
                      autoComplete="new-password"
                      placeholder="至少 8 个字符"
                      value={form.password}
                      onChange={handleChange}
                      error={!!errors.password}
                      style={{ backgroundColor: 'rgba(15, 23, 42, 0.75)' }}
                      className="border-white/10 backdrop-blur-none pl-10 text-white shadow-[inset_0_1px_0_rgba(255,255,255,0.06)] placeholder:text-slate-500 focus-visible:border-cyan-300/70 focus-visible:ring-2 focus-visible:ring-violet-400/30 [&:-webkit-autofill]:shadow-[0_0_0_1000px_rgba(15,23,42,0.75)_inset] [&:-webkit-autofill]:[-webkit-text-fill-color:white]"
                    />
                  </div>
                  {errors.password && <p className="text-xs text-red-400">{errors.password}</p>}
                </div>

                <div className="space-y-1.5">
                  <Label htmlFor="confirmPassword" className="text-surface-200">确认密码</Label>
                  <div className="group relative">
                    <Lock className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-slate-300 transition-colors group-focus-within:text-pink-200" />
                    <Input
                      id="confirmPassword"
                      name="confirmPassword"
                      type="password"
                      autoComplete="new-password"
                      placeholder="再次输入密码"
                      value={form.confirmPassword}
                      onChange={handleChange}
                      error={!!errors.confirmPassword}
                      style={{ backgroundColor: 'rgba(15, 23, 42, 0.75)' }}
                      className="border-white/10 backdrop-blur-none pl-10 text-white shadow-[inset_0_1px_0_rgba(255,255,255,0.06)] placeholder:text-slate-500 focus-visible:border-cyan-300/70 focus-visible:ring-2 focus-visible:ring-violet-400/30 [&:-webkit-autofill]:shadow-[0_0_0_1000px_rgba(15,23,42,0.75)_inset] [&:-webkit-autofill]:[-webkit-text-fill-color:white]"
                    />
                  </div>
                  {errors.confirmPassword && <p className="text-xs text-red-400">{errors.confirmPassword}</p>}
                </div>

                {serverError && (
                  <div className="rounded-2xl border border-red-500/20 bg-red-500/10 px-4 py-3 text-sm text-red-300">
                    {serverError}
                  </div>
                )}

                <div className="space-y-2">
                  <Button
                    type="submit"
                    className="h-11 w-full border border-white/10 bg-[linear-gradient(135deg,rgba(56,189,248,0.95),rgba(139,92,246,0.95),rgba(236,72,153,0.92))] shadow-[0_12px_40px_rgba(14,165,233,0.28)] transition-all duration-300 hover:-translate-y-0.5 hover:shadow-[0_18px_46px_rgba(168,85,247,0.32)] hover:brightness-110 active:translate-y-0"
                    disabled={loading}
                  >
                    {loading ? (
                      <span className="flex items-center gap-2">
                        <LoadingSpinner size="sm" />
                        注册中…
                      </span>
                    ) : (
                      '立即创建'
                    )}
                  </Button>
                  {slowNetwork && (
                    <p className="text-center text-xs text-amber-300/80">
                      网络较慢，请耐心等待…
                    </p>
                  )}
                </div>

                <EntryFeatureStrip
                  items={['创建后直达登录', '统一连接创作模块', '后续项目与资产持续沉淀']}
                />
              </form>

              <EntryFooterLink prefix="已有账户？" href="/login" label="去登录" />
            </EntryCard>

            <EntryCopyright />
          </section>
        </div>
      </div>
    </EntryPageLayout>
  )
}
