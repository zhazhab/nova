import { useEffect, useState } from 'react'
import { readFile, saveFile } from '@/lib/api'
import { Button } from '@/components/ui/button'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Textarea } from '@/components/ui/textarea'

const FILES = [
  { label: '角色', path: 'setting/characters.md' },
  { label: '世界观', path: 'setting/world-building.md' },
  { label: 'CREATOR', path: 'CREATOR.md' },
]

export function SettingPanel() {
  const [active, setActive] = useState(FILES[0].path)
  const [content, setContent] = useState('')
  const [saving, setSaving] = useState(false)

  useEffect(() => {
    let cancelled = false
    readFile(active)
      .then((data) => { if (!cancelled) setContent(data.content) })
      .catch(() => { if (!cancelled) setContent('') })
    return () => { cancelled = true }
  }, [active])

  const handleSave = async () => {
    setSaving(true)
    try {
      await saveFile(active, content)
    } finally {
      setSaving(false)
    }
  }

  return (
    <aside className="flex h-full w-[260px] shrink-0 flex-col border-r border-[#30343b] bg-[#1c1e22] p-3">
      <div className="mb-3 flex h-7 items-center">
        <span className="text-xs font-medium text-[#c5c9d1]">设定</span>
      </div>
      <div className="mb-3">
        <Tabs value={active} onValueChange={setActive}>
          <TabsList className="h-8 w-full bg-[#252831]">
            {FILES.map((file) => (
              <TabsTrigger key={file.path} value={file.path} className="text-xs">{file.label}</TabsTrigger>
            ))}
          </TabsList>
        </Tabs>
      </div>
      <ScrollArea className="min-h-0 flex-1 rounded-xl border border-[#333842] bg-[#141519]">
        <Textarea
          className="min-h-[calc(100vh-260px)] resize-none border-0 bg-transparent font-mono text-xs leading-6 text-[#aeb6c6] shadow-none focus-visible:ring-0"
          value={content}
          onChange={(event) => setContent(event.target.value)}
        />
      </ScrollArea>
      <div className="pt-3">
        <Button className="w-full border-[#333842] bg-[#20242b] text-[#d7dbe2] hover:bg-[#252831]" variant="outline" size="sm" disabled={saving} onClick={handleSave}>
          {saving ? '保存中...' : '保存设定'}
        </Button>
      </div>
    </aside>
  )
}
