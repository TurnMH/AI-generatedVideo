import type { Metadata } from 'next'
import './globals.css'
import { ToastContextProvider } from '@/components/ui/toast'
import { TooltipProvider } from '@/components/ui/tooltip'

export const metadata: Metadata = {
  title: 'AI Stream Media — 智能流媒体创作平台',
  description: '面向视频、图像与故事内容的 AI 流媒体创作中枢',
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
