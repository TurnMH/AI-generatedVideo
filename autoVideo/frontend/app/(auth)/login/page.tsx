'use client'

import { Suspense, useState, useEffect, useRef } from 'react'
import { useRouter, useSearchParams } from 'next/navigation'
import { BookOpen, Clapperboard, ImageIcon, Lock, Mail, Video } from 'lucide-react'
import { authAPI } from '@/lib/api'
import { useAuthStore } from '@/lib/store/auth'
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
  email: string
  password: string
}

interface FieldErrors {
  email?: string
  password?: string
}

const creativeItems: MarketingItem[] = [
  {
    title: '图片灵感',
    description: '高质感封面、海报、角色形象，一站式生成与优化',
    icon: ImageIcon,
    accent: 'from-cyan-400/30 via-sky-500/20 to-transparent',
  },
  {
    title: '视频表达',
    description: '脚本、分镜、视频成片流转更顺滑，创作链路更完整',
    icon: Video,
    accent: 'from-violet-400/30 via-fuchsia-500/20 to-transparent',
  },
  {
    title: '小说世界观',
    description: '从文字设定到剧情演绎，把故事快速转为内容资产',
    icon: BookOpen,
    accent: 'from-amber-300/30 via-orange-500/20 to-transparent',
  },
  {
    title: '漫画分镜感',
    description: '章节节奏、画面情绪、人物表现更有“胶圈”氛围',
    icon: Clapperboard,
    accent: 'from-pink-400/30 via-rose-500/20 to-transparent',
  },
]

function validate(values: FormState): FieldErrors {
  const errors: FieldErrors = {}
  if (!values.email) {
    errors.email = '请输入邮箱'
  } else if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(values.email)) {
    errors.email = '邮箱格式不正确'
  }
  if (!values.password) errors.password = '请输入密码'
  return errors
}

function LoginPageContent() {
  const router = useRouter()
  const searchParams = useSearchParams()
  const { setAuth } = useAuthStore()
  const [form, setForm] = useState<FormState>({ email: '', password: '' })
  const [errors, setErrors] = useState<FieldErrors>({})
  const [serverError, setServerError] = useState('')
  const [loading, setLoading] = useState(false)
  const [slowNetwork, setSlowNetwork] = useState(false)
  const slowTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  const justRegistered = searchParams.get('registered') === '1'
  const redirectTo = searchParams.get('redirect') || '/video'

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
      const res = (await authAPI.login(form.email, form.password)) as {
        data: { user: import('@/types').User; access_token: string; refresh_token: string }
      }
      setAuth(res.data.user, res.data.access_token, res.data.refresh_token)
      router.push(redirectTo)
    } catch (err: any) {
      console.error('Login error:', err)
      const errMsg = err?.response?.data?.error || err?.response?.data?.message || err?.message || '登录失败，请检查邮箱和密码'
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
            badge="更时尚的 AI 创作入口"
            title={
              <>
                把
                <span className="bg-gradient-to-r from-cyan-300 via-violet-300 to-pink-300 bg-clip-text text-transparent">
                  图片、视频、小说、漫画
                </span>
                创作汇聚到一个工作台
              </>
            }
            description="登录后即可进入统一创作空间，快速在视觉、剧情、分镜和内容生产之间切换，让创意更具潮流感与完成度。"
            items={creativeItems}
            footer={
              <div className="flex items-center gap-4 rounded-3xl border border-white/10 bg-white/[0.04] p-4 backdrop-blur-xl">
                <div className="flex -space-x-2">
                  <div className="h-10 w-10 rounded-full border border-white/10 bg-cyan-400/30" />
                  <div className="h-10 w-10 rounded-full border border-white/10 bg-violet-400/30" />
                  <div className="h-10 w-10 rounded-full border border-white/10 bg-pink-400/30" />
                </div>
                <div>
                  <p className="text-sm font-medium text-white">灵感正在并行流动</p>
                  <p className="text-sm text-surface-400">
                    从故事文本到视觉成片，让每个创作模块更有表达力。
                  </p>
                </div>
              </div>
            }
          />

          <section className="mx-auto w-full max-w-md">
            <EntryCard
              title="欢迎回来"
              subtitle="继续你的灵感流转、项目推进与多模态内容生产。"
              tags={['图片', '视频', '小说', '漫画']}
            >
              {justRegistered && (
                <div className="mb-4 rounded-2xl border border-emerald-500/20 bg-emerald-500/10 px-4 py-3 text-sm text-emerald-300">
                  注册成功，请登录
                </div>
              )}

              <form onSubmit={handleSubmit} noValidate className="space-y-5">
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
                      autoComplete="current-password"
                      placeholder="••••••••"
                      value={form.password}
                      onChange={handleChange}
                      error={!!errors.password}
                      style={{ backgroundColor: 'rgba(15, 23, 42, 0.75)' }}
                      className="border-white/10 backdrop-blur-none pl-10 text-white shadow-[inset_0_1px_0_rgba(255,255,255,0.06)] placeholder:text-slate-500 focus-visible:border-cyan-300/70 focus-visible:ring-2 focus-visible:ring-violet-400/30 [&:-webkit-autofill]:shadow-[0_0_0_1000px_rgba(15,23,42,0.75)_inset] [&:-webkit-autofill]:[-webkit-text-fill-color:white]"
                    />
                  </div>
                  {errors.password && <p className="text-xs text-red-400">{errors.password}</p>}
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
                        登录中…
                      </span>
                    ) : (
                      '进入创作台'
                    )}
                  </Button>
                  {slowNetwork && (
                    <p className="text-center text-xs text-amber-300/80">
                      网络较慢，请耐心等待…
                    </p>
                  )}
                </div>

                <EntryFeatureStrip
                  items={['快速进入项目台', '统一管理图片/视频', '延续你的剧情与分镜']}
                />
              </form>

              <EntryFooterLink prefix="还没有账户？" href="/register" label="注册" />
            </EntryCard>

            <EntryCopyright />
          </section>
        </div>
      </div>
    </EntryPageLayout>
  )
}

function LoginPageFallback() {
  return (
    <EntryPageLayout animatedBackground className="px-4">
      <div className="flex min-h-screen items-center justify-center">
        <div className="relative z-10 flex w-full max-w-md justify-center rounded-[32px] border border-white/10 bg-white/[0.08] p-8 shadow-2xl backdrop-blur-2xl">
          <div className="flex items-center gap-2 text-sm text-surface-300">
            <LoadingSpinner size="sm" />
            页面加载中…
          </div>
        </div>
      </div>
    </EntryPageLayout>
  )
}

export default function LoginPage() {
  return (
    <Suspense fallback={<LoginPageFallback />}>
      <LoginPageContent />
    </Suspense>
  )
}
