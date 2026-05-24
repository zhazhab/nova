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
}

export function StoryPicker({ stories, currentStoryId, tellers, onSelect, onCreate, onDelete }: StoryPickerProps) {
  const [creating, setCreating] = useState(false)
  const [title, setTitle] = useState('')
  const [origin, setOrigin] = useState('')
  const defaultTeller = tellers[0]?.id || 'classic'

  const submit = () => {
    if (!title.trim()) return
    onCreate({ title: title.trim(), origin: origin.trim(), story_teller_id: defaultTeller })
    setTitle('')
    setOrigin('')
    setCreating(false)
  }

  return (
    <div className="flex min-w-0 items-center gap-1.5">
      <span className="shrink-0 text-[11px] font-medium text-[#858b96]">故事线</span>
      <Select value={currentStoryId || '__none'} onValueChange={(value) => { if (value !== '__none' && value !== '__empty') onSelect(value) }}>
        <SelectTrigger
          size="sm"
          className="w-[190px] border-[#303238] bg-[#25262a] px-2 py-0.5 text-xs text-[#d7dbe2] hover:bg-[#303238] focus:ring-0"
          aria-label="选择故事线"
        >
          <SelectValue placeholder="选择故事线" />
        </SelectTrigger>
        <SelectContent className="border-[#303238] bg-[#25262a] text-[#d7dbe2]">
          <SelectItem value="__none" disabled>选择故事线</SelectItem>
          {stories.length === 0 ? (
            <SelectItem value="__empty" disabled>暂无故事线</SelectItem>
          ) : stories.map((story) => (
            <SelectItem key={story.id} value={story.id}>{story.title}</SelectItem>
          ))}
        </SelectContent>
      </Select>
      <Popover open={creating} onOpenChange={setCreating}>
        <PopoverTrigger asChild>
          <Button variant="ghost" size="xs" className="text-[#8fb5ff] hover:bg-[#303238] hover:text-[#d7dbe2]">
            <Plus className="h-3 w-3" />
            新建
          </Button>
        </PopoverTrigger>
        <PopoverContent align="start" className="w-80 border-[#303238] bg-[#25262a] p-3 text-[#d7dbe2]">
          <div className="mb-2 text-xs font-medium">创建故事线</div>
          <Input className="mb-2 border-[#3a3d45] bg-[#1b1c1f] text-xs" placeholder="故事标题" value={title} onChange={(event) => setTitle(event.target.value)} />
          <Textarea className="mb-3 h-20 min-h-20 resize-none border-[#3a3d45] bg-[#1b1c1f] text-xs" placeholder="开端描述" value={origin} onChange={(event) => setOrigin(event.target.value)} />
          <div className="flex justify-end gap-2">
            <Button variant="ghost" size="xs" onClick={() => setCreating(false)}>取消</Button>
            <Button size="xs" onClick={submit}>创建</Button>
          </div>
        </PopoverContent>
      </Popover>
      {currentStoryId && (
        <Button variant="ghost" size="icon-xs" className="text-[#858b96] hover:bg-[#4a2b2b] hover:text-[#ff8a8a]" onClick={() => onDelete(currentStoryId)} aria-label="删除故事线">
          <Trash2 className="h-3 w-3" />
        </Button>
      )}
    </div>
  )
}
