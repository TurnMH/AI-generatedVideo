'use client'

import { Suspense, useState, useEffect, useRef } from 'react'
import Link from 'next/link'
import { useRouter, useSearchParams } from 'next/navigation'
import {
  Lock,
  Mail,
} from 'lucide-react'
import { authAPI } from '@/lib/api'
import { useAuthStore } from '@/lib/store/auth'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { LoadingSpinner } from '@/components/common/LoadingSpinner'
import {
  BrandLockup,
  EntryCopyright,
  EntryFooterLink,
  EntryPageLayout,
} from '@/components/marketing/entry-ui'

interface FormState {
  email: string
  password: string
}

interface FieldErrors {
  email?: string
  password?: string
}

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
      <div className="flex min-h-screen flex-col">
        <header className="flex items-center justify-between gap-4 py-3 md:py-5">
          <BrandLockup />
          <div className="flex items-center gap-2">
            <Link
              href="/"
              className="rounded-full border border-white/10 bg-white/5 px-4 py-2 text-sm text-slate-200 backdrop-blur-xl transition hover:border-white/20 hover:bg-white/10"
            >
              返回首页
            </Link>
            <Link
              href="/register"
              className="rounded-full border border-cyan-200/15 bg-[linear-gradient(135deg,rgba(34,211,238,0.18),rgba(245,158,11,0.12))] px-4 py-2 text-sm font-medium text-white transition hover:-translate-y-0.5 hover:bg-[linear-gradient(135deg,rgba(34,211,238,0.22),rgba(245,158,11,0.16))]"
            >
              创建账户
            </Link>
          </div>
        </header>

        <div className="flex flex-1 items-center py-6 lg:py-8 xl:py-10">
          <div className="mx-auto w-full max-w-xl">
            <section>
              <div className="relative overflow-hidden rounded-[40px] border border-cyan-200/15 bg-slate-950/78 p-6 shadow-[0_30px_100px_rgba(2,6,23,0.48)] backdrop-blur-2xl sm:p-8 lg:p-10">
                <div className="pointer-events-none absolute inset-0 bg-[linear-gradient(135deg,rgba(255,255,255,0.14),transparent_22%,transparent_72%,rgba(255,255,255,0.05))]" />
                <div className="pointer-events-none absolute inset-x-10 top-0 h-px bg-gradient-to-r from-transparent via-white/70 to-transparent opacity-80" />
                <div className="pointer-events-none absolute -right-12 top-10 h-40 w-40 rounded-full bg-cyan-300/14 blur-3xl" />
                <div className="pointer-events-none absolute -left-6 bottom-8 h-28 w-28 rounded-full bg-amber-300/12 blur-3xl" />

                <div className="relative">
                  <div>
                    <p className="text-xs uppercase tracking-[0.28em] text-slate-400">Access Gate</p>
                    <h2 className="mt-3 text-3xl font-semibold text-white sm:text-[2.2rem]">欢迎回到 AI Stream Media</h2>
                    <p className="mt-3 max-w-lg text-sm leading-6 text-slate-300">
                      输入账号信息并继续你的项目工作流。
                    </p>
                  </div>

                  {justRegistered && (
                    <div className="mt-6 rounded-2xl border border-emerald-500/20 bg-emerald-500/10 px-4 py-3 text-sm text-emerald-300">
                      注册成功，请登录
                    </div>
                  )}

                  <form onSubmit={handleSubmit} noValidate className="mt-6 space-y-5">
                    <div className="space-y-1.5">
                      <Label htmlFor="email" className="text-slate-200">邮箱</Label>
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
                          style={{ backgroundColor: 'rgba(15, 23, 42, 0.72)' }}
                          className="h-12 rounded-2xl border-white/10 pl-10 text-white shadow-[inset_0_1px_0_rgba(255,255,255,0.06)] placeholder:text-slate-500 focus-visible:border-cyan-300/70 focus-visible:ring-2 focus-visible:ring-cyan-300/20 [&:-webkit-autofill]:shadow-[0_0_0_1000px_rgba(15,23,42,0.75)_inset] [&:-webkit-autofill]:[-webkit-text-fill-color:white]"
                        />
                      </div>
                      {errors.email && <p className="text-xs text-red-400">{errors.email}</p>}
                    </div>

                    <div className="space-y-1.5">
                      <Label htmlFor="password" className="text-slate-200">密码</Label>
                      <div className="group relative">
                        <Lock className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-slate-300 transition-colors group-focus-within:text-amber-200" />
                        <Input
                          id="password"
                          name="password"
                          type="password"
                          autoComplete="current-password"
                          placeholder="••••••••"
                          value={form.password}
                          onChange={handleChange}
                          error={!!errors.password}
                          style={{ backgroundColor: 'rgba(15, 23, 42, 0.72)' }}
                          className="h-12 rounded-2xl border-white/10 pl-10 text-white shadow-[inset_0_1px_0_rgba(255,255,255,0.06)] placeholder:text-slate-500 focus-visible:border-amber-300/70 focus-visible:ring-2 focus-visible:ring-amber-300/20 [&:-webkit-autofill]:shadow-[0_0_0_1000px_rgba(15,23,42,0.75)_inset] [&:-webkit-autofill]:[-webkit-text-fill-color:white]"
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
                        className="h-13 w-full rounded-2xl border border-white/10 bg-[linear-gradient(135deg,rgba(34,211,238,0.96),rgba(45,212,191,0.92),rgba(245,158,11,0.9))] text-base font-semibold shadow-[0_20px_52px_rgba(8,145,178,0.34)] transition-all duration-300 hover:-translate-y-0.5 hover:shadow-[0_26px_58px_rgba(245,158,11,0.24)] hover:brightness-110 active:translate-y-0"
                        disabled={loading}
                      >
                        {loading ? (
                          <span className="flex items-center gap-2">
                            <LoadingSpinner size="sm" />
                            登录中…
                          </span>
                        ) : (
                          '进入 AI Stream Media'
                        )}
                      </Button>
                      {slowNetwork && (
                        <p className="text-center text-xs text-amber-300/80">
                          网络较慢，请耐心等待…
                        </p>
                      )}
                    </div>
                  </form>

                  <EntryFooterLink prefix="还没有账户？" href="/register" label="注册" />
                </div>
              </div>

              <EntryCopyright />
            </section>
          </div>
        </div>
      </div>
    </EntryPageLayout>
  )
}

function LoginPageFallback() {
  return (
    <EntryPageLayout animatedBackground className="px-4">
      <div className="flex min-h-screen items-center justify-center">
        <div className="relative z-10 flex w-full max-w-lg justify-center rounded-[36px] border border-white/10 bg-slate-950/40 p-10 shadow-[0_24px_80px_rgba(2,6,23,0.38)] backdrop-blur-2xl">
          <div className="flex items-center gap-2 text-sm text-slate-300">
            <LoadingSpinner size="sm" />
            登录舱准备中…
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
