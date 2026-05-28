import { useState } from 'react'
import { Plus, Trash2 } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Textarea } from '@/components/ui/textarea'
import type { StorySummary, Teller } from '../types'

interface StoryPickerProps {
  stories: StorySummary[]
  currentStoryId: string
  tellers: Teller[]
  onSelect: (storyId: string) => void
  onCreate: (input: { title: string; origin: string; story_teller_id: string }) => void
  onDelete: (storyId: string) => void
  layout?: 'inline' | 'sidebar'
}

export function StoryPicker({ stories, currentStoryId, tellers, onSelect, onCreate, onDelete, layout = 'inline' }: StoryPickerProps) {
  const [creating, setCreating] = useState(false)
  const [title, setTitle] = useState('')
  const [origin, setOrigin] = useState('')
  const defaultTeller = tellers[0]?.id || 'classic'
  const sidebar = layout === 'sidebar'

  const submit = () => {
    if (!title.trim()) return
    onCreate({ title: title.trim(), origin: origin.trim(), story_teller_id: defaultTeller })
    setTitle('')
    setOrigin('')
    setCreating(false)
  }

  const selector = (
    <Select value={currentStoryId || '__none'} onValueChange={(value) => { if (value !== '__none' && value !== '__empty') onSelect(value) }}>
      <SelectTrigger
        size="sm"
        className={`nova-field ${sidebar ? 'w-full' : 'w-[190px]'} px-3 py-0.5 text-xs focus:ring-0`}
        aria-label="选择故事线"
      >
        <SelectValue placeholder="选择故事线" />
      </SelectTrigger>
      <SelectContent className="nova-panel border text-[var(--nova-text)]">
        <SelectItem value="__none" disabled>选择故事线</SelectItem>
        {stories.length === 0 ? (
          <SelectItem value="__empty" disabled>暂无故事线</SelectItem>
        ) : stories.map((story) => (
          <SelectItem key={story.id} value={story.id}>{story.title}</SelectItem>
        ))}
      </SelectContent>
    </Select>
  )

  const createButton = (
    <Popover open={creating} onOpenChange={setCreating}>
      <PopoverTrigger asChild>
        <Button variant="ghost" size="xs" className="nova-nav-item">
          <Plus className="h-3 w-3" />
          新建
        </Button>
      </PopoverTrigger>
      <PopoverContent align="start" className="nova-panel w-80 border p-3 text-[var(--nova-text)] shadow-[var(--nova-shadow)]">
        <div className="mb-2 text-xs font-medium">创建故事线</div>
        <Input className="nova-field mb-2 text-xs" placeholder="故事标题" value={title} onChange={(event) => setTitle(event.target.value)} />
        <Textarea className="nova-field mb-3 h-20 min-h-20 resize-none text-xs" placeholder="开端描述" value={origin} onChange={(event) => setOrigin(event.target.value)} />
        <div className="flex justify-end gap-2">
          <Button variant="ghost" size="xs" onClick={() => setCreating(false)}>取消</Button>
          <Button size="xs" onClick={submit}>创建</Button>
        </div>
      </PopoverContent>
    </Popover>
  )

  const deleteButton = currentStoryId && (
    <Button variant="ghost" size="icon-xs" className="text-[var(--nova-text-faint)] hover:bg-[#4a2b2b] hover:text-[#ff8a8a]" onClick={() => onDelete(currentStoryId)} aria-label="删除故事线">
      <Trash2 className="h-3 w-3" />
    </Button>
  )

  if (sidebar) {
    return (
      <div className="flex min-w-0 flex-col gap-1.5">
        <div className="flex items-center justify-between gap-2">
          <span className="shrink-0 text-[11px] font-medium text-[var(--nova-text-faint)]">故事线</span>
          <div className="flex shrink-0 items-center gap-1">
            {createButton}
            {deleteButton}
          </div>
        </div>
        {selector}
      </div>
    )
  }

  return (
    <div className="flex min-w-0 items-center gap-1.5">
      <span className="shrink-0 text-[11px] font-medium text-[var(--nova-text-faint)]">故事线</span>
      {selector}
      {createButton}
      {deleteButton}
    </div>
  )
}
