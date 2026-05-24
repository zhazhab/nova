import { useState } from 'react'
import { Send } from 'lucide-react'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Textarea } from '@/components/ui/textarea'
import { sendInteractiveMessage } from '../api'
import type { Snapshot } from '../types'

interface StoryStageProps {
  storyId: string
  branchId: string
  snapshot: Snapshot | null
  onDone: () => void
}

export function StoryStage({ storyId, branchId, snapshot, onDone }: StoryStageProps) {
  const [input, setInput] = useState('')
  const [streaming, setStreaming] = useState(false)
  const [draft, setDraft] = useState('')

  const send = async () => {
    const message = input.trim()
    if (!message || !storyId || streaming) return
    setInput('')
    setDraft('')
    setStreaming(true)
    try {
      const stream = await sendInteractiveMessage({ mode: 'story', story_id: storyId, branch: branchId, message })
      const reader = stream.getReader()
      while (true) {
        const { done, value } = await reader.read()
        if (done) break
        if (value.event === 'chunk') {
          const data = JSON.parse(value.data)
          setDraft((prev) => prev + (data.content || ''))
        }
      }
      await onDone()
    } finally {
      setStreaming(false)
    }
  }

  return (
    <main className="flex min-w-0 flex-1 flex-col bg-[#18191b] p-3">
      <div data-testid="story-stage-card" className="flex min-h-0 flex-1 flex-col overflow-hidden rounded-xl border border-[#333842] bg-[#141519]">
        <div className="flex h-10 items-center justify-between px-4">
          <div className="text-xs font-medium text-[#7f8898]">故事舞台 · 当前分支 {branchId || 'main'}</div>
          <Badge variant="outline" className="border-[#333842] bg-[#20242b] text-[#7f8898]">{snapshot?.turns?.length || 0} 回合</Badge>
        </div>
        <ScrollArea className="min-h-0 flex-1">
          <div className="px-4 pb-4">
            <div className="space-y-3">
              {(snapshot?.turns || []).map((turn) => (
                <div key={turn.id} className="space-y-2">
                  <div className="ml-auto max-w-[75%] rounded-lg bg-[#20242b] px-3 py-2 text-sm leading-6 text-[#d7dbe2] shadow-sm">{turn.user}</div>
                  <div className="max-w-[80%] whitespace-pre-wrap rounded-lg border border-[#355845] bg-[#222b25] px-3 py-2 text-sm leading-7 text-[#d7dbe2] shadow-sm">{turn.narrative}</div>
                </div>
              ))}
              {draft && <div className="max-w-[80%] whitespace-pre-wrap rounded-lg border border-[#355845] bg-[#222b25] px-3 py-2 text-sm leading-7 text-[#d7dbe2] shadow-sm">{draft}</div>}
              {!snapshot?.turns?.length && !draft && (
                <div className="flex h-64 items-center justify-center rounded-xl border border-dashed border-[#333842] bg-[#18191b]/80 text-sm text-[#858b96]">
                  输入第一句话，开始互动故事
                </div>
              )}
            </div>
          </div>
        </ScrollArea>
      </div>
      <div className="mt-3 rounded-xl border border-[#333842] bg-[#141519] p-3">
        <div className="flex items-center gap-3">
          <Textarea
            className="h-14 min-h-14 flex-1 resize-none border-[#333842] bg-[#1f2228] text-sm text-[#d7dbe2] placeholder:text-[#778091] focus-visible:ring-1"
            value={input}
            placeholder="你要做什么？"
            onChange={(event) => setInput(event.target.value)}
            onKeyDown={(event) => {
              if (event.key === 'Enter' && (event.metaKey || event.ctrlKey)) void send()
            }}
          />
          <Button className="h-14 w-24" disabled={!storyId || streaming || !input.trim()} onClick={() => void send()}>
            <Send className="h-4 w-4" />
            {streaming ? '生成中' : '发送'}
          </Button>
        </div>
      </div>
    </main>
  )
}
