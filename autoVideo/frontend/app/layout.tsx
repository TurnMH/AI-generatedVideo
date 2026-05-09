import type { Metadata } from 'next'
import './globals.css'
import { ToastContextProvider } from '@/components/ui/toast'
import { TooltipProvider } from '@/components/ui/tooltip'

export const metadata: Metadata = {
  title: 'AutoVideo — AI 视频生成平台',
  description: '智能 AI 视频生成工作台',
}

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="zh">
      <body className="bg-surface-50 text-surface-900 antialiased">
        <TooltipProvider delayDuration={300}>
          <ToastContextProvider>{children}</ToastContextProvider>
        </TooltipProvider>
      </body>
    </html>
  )
}
