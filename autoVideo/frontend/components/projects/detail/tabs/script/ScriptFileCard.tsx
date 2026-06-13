'use client'

import React, { type RefObject } from 'react'
import { format } from 'date-fns'
import { Download, Eye, FileText, Upload } from 'lucide-react'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import type { Project } from '@/types'
import { formatBytes } from '@/lib/projects/utils'

type ScriptFileCardProps = {
  project: Project
  fileRef: RefObject<HTMLInputElement | null>
  onTriggerUpload: () => void
  hasScriptText: boolean
  onUpload: (e: React.ChangeEvent<HTMLInputElement>) => void
  onShowPreview: () => void
}

export function ScriptFileCard({
  project,
  fileRef,
  hasScriptText,
  onUpload,
  onShowPreview,
  onTriggerUpload,
}: ScriptFileCardProps) {
  return (
    <Card>
      <CardHeader className="pb-3">
        <div className="flex items-center justify-between">
          <CardTitle className="text-base">剧本文件</CardTitle>
          <div className="flex gap-2">
            <input ref={fileRef as React.RefObject<HTMLInputElement>} type="file" accept=".txt,.pdf,.docx,.md" className="hidden" onChange={onUpload} />
            <Button size="sm" variant="outline" onClick={onTriggerUpload} title="上传新版本的剧本文件">
              <Upload className="mr-1.5 h-3.5 w-3.5" />
              上传新版本
            </Button>
          </div>
        </div>
      </CardHeader>
      <CardContent>
        {project.script_file_url ? (
          <div className="space-y-4">
            <div className="flex flex-wrap items-center justify-between gap-3 text-sm text-surface-600">
              <div className="flex flex-wrap items-center gap-4">
                <span className="flex items-center gap-1.5">
                  <FileText className="h-4 w-4 text-primary-500" />
                  {project.script_file_url.split('/').pop() || '剧本文件'}
                </span>
                <span>{formatBytes(project.script_file_size || 0)}</span>
                <span>上传于 {format(new Date(project.updated_at), 'yyyy-MM-dd HH:mm')}</span>
              </div>
              <div className="flex flex-wrap gap-2">
                {hasScriptText && (
                  <Button size="sm" variant="outline" onClick={onShowPreview} title="查看剧本全文">
                    <Eye className="mr-1.5 h-3.5 w-3.5" />
                    查看全文
                  </Button>
                )}
                <Button
                  size="sm"
                  variant="outline"
                  onClick={() => window.open(project.script_file_url, '_blank', 'noopener,noreferrer')}
                  title="打开原始剧本文件"
                >
                  <Download className="mr-1.5 h-3.5 w-3.5" />
                  打开原文件
                </Button>
              </div>
            </div>
          </div>
        ) : (
          <p className="text-sm text-surface-400">尚未上传剧本文件</p>
        )}
      </CardContent>
    </Card>
  )
}
