import { useEffect, useState, type ReactNode } from 'react'
import { BookMarked, Building2, ChevronDown, Database, FileText, Folder, Library, MapPin, Plus, Save, ScrollText, Search, SlidersHorizontal, Trash2, UserRound } from 'lucide-react'
import type { LucideIcon } from 'lucide-react'
import {
  createLoreItem,
  deleteLoreItem,
  getLoreItems,
  readFile,
  saveFile,
  updateLoreItem,
  type LoreItem,
} from '@/lib/api'
import { isSaveShortcut } from '@/lib/keyboard'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Textarea } from '@/components/ui/textarea'
import { createInteractiveTeller, deleteInteractiveTeller, getInteractiveTellers, updateInteractiveTeller } from '../api'
import type { Teller, TellerPromptSlot } from '../types'

const CREATOR_PATH = 'CREATOR.md'

const TYPE_OPTIONS = [
  { value: 'character', label: '角色' },
  { value: 'world', label: '世界观' },
  { value: 'location', label: '地点' },
  { value: 'faction', label: '势力' },
  { value: 'rule', label: '规则' },
  { value: 'item', label: '物品' },
  { value: 'other', label: '其他' },
] as const

const IMPORTANCE_OPTIONS = [
  { value: 'major', label: '主要' },
  { value: 'important', label: '重要' },
  { value: 'minor', label: '次要' },
] as const

const TELLER_TARGET_OPTIONS = [
  {
    value: 'system',
    label: '讲述者身份',
    summary: '定义这个讲述者是谁',
    detail: '随故事上下文注入，影响整体口吻、叙事偏好和主持人身份。',
  },
  {
    value: 'context',
    label: '背景上下文',
    summary: '补充长期设定和偏好',
    detail: '随标题、开端、资料库和当前状态一起注入，适合稳定生效的世界观、风格和偏好。',
  },
  {
    value: 'thinking',
    label: '内部思考',
    summary: '进入本轮 reasoning/thinking',
    detail: '会靠近本轮行动注入，要求 Agent 在内部推理时使用；这些内容不应出现在故事正文里。',
  },
  {
    value: 'private_instruction',
    label: '内部规则',
    summary: '隐式遵守，不直接展示',
    detail: '随讲述者上下文注入，适合放裁定原则、自检清单和禁止输出的分析规则。',
  },
  {
    value: 'turn',
    label: '本轮输出',
    summary: '约束故事舞台正文',
    detail: '每次生成下一回合时注入，适合控制篇幅、结尾方式、对白比例和正文呈现格式。',
  },
  {
    value: 'state_agent',
    label: '状态引擎',
    summary: '只影响右侧场景记忆',
    detail: '主 Agent 写完正文后，后端会让状态引擎把本回合转成 JSON 状态变化；这里配置它应该记录什么。',
  },
  {
    value: 'editor_agent',
    label: '边界引擎',
    summary: '预留给后续编辑/边界检查',
    detail: '当前故事生成链路暂不使用，保留给后续编辑 Agent 或边界检查 Agent 的专用规则。',
  },
] as const

type TellerTarget = TellerPromptSlot['target']
export type SettingPanelMode = 'lore' | 'creator' | 'teller'

type LoreType = LoreItem['type']

interface KnowledgeSection {
  id: string
  label: string
  icon: LucideIcon
  types: LoreType[]
  createType: LoreType
  createName: string
  tag?: string
  excludeTag?: string
}

const KNOWLEDGE_SECTIONS: KnowledgeSection[] = [
  { id: 'characters', label: '角色', icon: UserRound, types: ['character'], createType: 'character', createName: '新角色' },
  { id: 'locations', label: '地点', icon: MapPin, types: ['location'], createType: 'location', createName: '新地点' },
  { id: 'factions', label: '组织', icon: Building2, types: ['faction'], createType: 'faction', createName: '新组织' },
  { id: 'rules', label: '规则', icon: ScrollText, types: ['world', 'rule'], createType: 'rule', createName: '新规则' },
  { id: 'templates', label: '模板', icon: FileText, types: ['other'], createType: 'other', createName: '新模板', tag: '模板' },
  { id: 'assets', label: '素材库', icon: Library, types: ['item', 'other'], createType: 'item', createName: '新素材', excludeTag: '模板' },
]

interface SettingPanelProps {
  mode?: SettingPanelMode
  tellers?: Teller[]
  onTellersChange?: (tellers: Teller[]) => void
}

export function SettingPanel({ mode, tellers: externalTellers = [], onTellersChange }: SettingPanelProps) {
  const activeMode = mode || 'lore'
  const [items, setItems] = useState<LoreItem[]>([])
  const [activeId, setActiveId] = useState('')
  const [draft, setDraft] = useState<LoreItem | null>(null)
  const [tagDraft, setTagDraft] = useState('')
  const [query, setQuery] = useState('')
  const [creatorContent, setCreatorContent] = useState('')
  const [tellers, setTellers] = useState<Teller[]>(externalTellers)
  const [activeTellerId, setActiveTellerId] = useState('')
  const [tellerDraft, setTellerDraft] = useState<Teller | null>(null)
  const [tellerTagDraft, setTellerTagDraft] = useState('')
  const [activeSlotId, setActiveSlotId] = useState('')
  const [saving, setSaving] = useState(false)

  useEffect(() => {
    let cancelled = false
    getLoreItems()
      .then((data) => {
        if (cancelled) return
        setItems(data)
        setActiveId((current) => current || data[0]?.id || '')
      })
      .catch(() => {
        if (!cancelled) setItems([])
      })
    return () => { cancelled = true }
  }, [])

  useEffect(() => {
    const item = items.find((entry) => entry.id === activeId) || null
    setDraft(item ? { ...item, tags: [...(item.tags || [])] } : null)
    setTagDraft((item?.tags || []).join('，'))
  }, [activeId, items])

  useEffect(() => {
    if (activeMode !== 'creator') return
    let cancelled = false
    readFile(CREATOR_PATH)
      .then((data) => { if (!cancelled) setCreatorContent(data.content) })
      .catch(() => { if (!cancelled) setCreatorContent('') })
    return () => { cancelled = true }
  }, [activeMode])

  useEffect(() => {
    setTellers(externalTellers)
    setActiveTellerId((current) => current || externalTellers[0]?.id || '')
  }, [externalTellers])

  useEffect(() => {
    const teller = tellers.find((entry) => entry.id === activeTellerId) || null
    setTellerDraft(teller ? {
      ...teller,
      tags: [...(teller.tags || [])],
      slots: [...(teller.slots || [])],
      context_policy: { ...teller.context_policy },
    } : null)
    setTellerTagDraft((teller?.tags || []).join('，'))
    setActiveSlotId(teller?.slots?.[0]?.id || '')
  }, [activeTellerId, tellers])

  const refreshItems = async (nextActiveId?: string) => {
    const data = await getLoreItems()
    setItems(data)
    setActiveId(nextActiveId || data[0]?.id || '')
  }

  const refreshTellers = async (nextActiveId?: string) => {
    const data = await getInteractiveTellers()
    setTellers(data)
    onTellersChange?.(data)
    setActiveTellerId(nextActiveId || data[0]?.id || '')
  }

  const handleCreateLore = async (section: KnowledgeSection = KNOWLEDGE_SECTIONS[0]) => {
    setSaving(true)
    try {
      const item = await createLoreItem({
        type: section.createType,
        name: section.createName,
        importance: section.createType === 'character' ? 'major' : 'important',
        tags: section.tag ? [section.tag] : [],
        content: `## ${section.createName}\n\n`,
      })
      await refreshItems(item.id)
    } finally {
      setSaving(false)
    }
  }

  const handleCreateTeller = async () => {
    setSaving(true)
    try {
      const teller = await createInteractiveTeller(newTellerDraft())
      await refreshTellers(teller.id)
    } finally {
      setSaving(false)
    }
  }

  const handleDelete = async () => {
    if (activeMode === 'teller') {
      if (!tellerDraft || !tellerDraft.custom) return
      if (!window.confirm(`删除讲述者「${tellerDraft.name}」？`)) return
      setSaving(true)
      try {
        await deleteInteractiveTeller(tellerDraft.id)
        await refreshTellers()
      } finally {
        setSaving(false)
      }
      return
    }
    if (!draft) return
    if (!window.confirm(`删除资料「${draft.name}」？`)) return
    setSaving(true)
    try {
      await deleteLoreItem(draft.id)
      await refreshItems()
    } finally {
      setSaving(false)
    }
  }

  const handleSave = async () => {
    setSaving(true)
    try {
      if (activeMode === 'creator') {
        await saveFile(CREATOR_PATH, creatorContent)
        return
      }
      if (activeMode === 'teller') {
        if (!tellerDraft) return
        const teller = await updateInteractiveTeller(tellerDraft.id, {
          ...tellerDraft,
          tags: splitTags(tellerTagDraft),
        })
        await refreshTellers(teller.id)
        return
      }
      if (!draft) return
      const item = await updateLoreItem(draft.id, { ...draft, tags: splitTags(tagDraft) })
      await refreshItems(item.id)
    } finally {
      setSaving(false)
    }
  }

  return (
    <section className="flex h-full min-h-0 bg-[#1b1c1f] text-[#d7dbe2]">
      <aside className="flex w-[320px] shrink-0 flex-col border-r border-[#303238] bg-[#202124]">
        <div className="border-b border-[#303238] px-3 py-3">
          <div className="flex items-center gap-2">
            <ModeIcon mode={activeMode} />
            <div className="text-sm font-semibold text-[#e0e4ec]">{panelTitle(activeMode)}</div>
          </div>
          <div className="mt-1 text-[11px] text-[#858b96]">在目录中选择条目，右侧打开编辑。</div>
        </div>

        {activeMode === 'lore' ? (
          <LoreDirectory
            items={items}
            activeId={activeId}
            query={query}
            saving={saving}
            onQueryChange={setQuery}
            onSelect={setActiveId}
            onCreate={(section) => void handleCreateLore(section)}
          />
        ) : activeMode === 'creator' ? (
          <CreatorDirectory />
        ) : (
          <TellerDirectory
            tellers={tellers}
            activeTellerId={activeTellerId}
            saving={saving}
            onSelect={setActiveTellerId}
            onCreate={() => void handleCreateTeller()}
          />
        )}
      </aside>

      <main className="flex min-w-0 flex-1 flex-col bg-[#1b1c1f]">
        <div className="flex min-h-12 shrink-0 items-center justify-between gap-3 border-b border-[#303238] bg-[#1f2023] px-4">
          <div className="min-w-0">
            <div className="flex min-w-0 items-center gap-2">
              <ModeIcon mode={activeMode} />
              <h2 className="truncate text-sm font-semibold text-[#e0e4ec]">{editorTitle(activeMode, draft, tellerDraft)}</h2>
            </div>
            <p className="mt-0.5 truncate text-[11px] text-[#858b96]">{editorSubtitle(activeMode, draft, tellerDraft)}</p>
          </div>
          <div className="flex shrink-0 items-center gap-2">
            {activeMode === 'lore' && (
              <Button className="border-[#303238] bg-[#25262a] text-[#d7dbe2] hover:bg-[#303238]" variant="outline" size="icon" disabled={saving || !draft} onClick={handleDelete} aria-label="删除资料">
                <Trash2 className="h-4 w-4" />
              </Button>
            )}
            {activeMode === 'teller' && (
              <Button className="border-[#303238] bg-[#25262a] text-[#d7dbe2] hover:bg-[#303238]" variant="outline" size="icon" disabled={saving || !tellerDraft?.custom} onClick={handleDelete} aria-label="删除讲述者">
                <Trash2 className="h-4 w-4" />
              </Button>
            )}
            <Button className="gap-1.5 border-[#303238] bg-[#25262a] text-[#d7dbe2] hover:bg-[#303238]" variant="outline" size="sm" disabled={saving || (activeMode === 'lore' && !draft) || (activeMode === 'teller' && !tellerDraft)} onClick={handleSave}>
              <Save className="h-4 w-4" />
              {saving ? '保存中...' : '保存'}
            </Button>
          </div>
        </div>

        {activeMode === 'lore' ? (
          <LoreEditor draft={draft} tagDraft={tagDraft} setDraft={setDraft} setTagDraft={setTagDraft} onSave={handleSave} />
        ) : activeMode === 'creator' ? (
          <CreatorEditor content={creatorContent} setContent={setCreatorContent} onSave={handleSave} />
        ) : (
          <TellerEditor
            draft={tellerDraft}
            setDraft={setTellerDraft}
            tagDraft={tellerTagDraft}
            setTagDraft={setTellerTagDraft}
            activeSlotId={activeSlotId}
            setActiveSlotId={setActiveSlotId}
            onSave={handleSave}
          />
        )}
      </main>
    </section>
  )
}

function LoreDirectory({
  items,
  activeId,
  query,
  saving,
  onQueryChange,
  onSelect,
  onCreate,
}: {
  items: LoreItem[]
  activeId: string
  query: string
  saving: boolean
  onQueryChange: (value: string) => void
  onSelect: (id: string) => void
  onCreate: (section: KnowledgeSection) => void
}) {
  return (
    <>
      <div className="border-b border-[#303238] p-2">
        <div className="flex h-8 items-center gap-2 rounded-md border border-[#303238] bg-[#1b1c1f] px-2 text-xs text-[#858b96]">
          <Search className="h-3.5 w-3.5" />
          <input
            className="min-w-0 flex-1 bg-transparent text-[#c5c9d1] outline-none placeholder:text-[#6f7580]"
            value={query}
            onChange={(event) => onQueryChange(event.target.value)}
            placeholder="搜索资料"
          />
        </div>
      </div>
      <ScrollArea className="min-h-0 flex-1">
        <div className="p-2">
          {KNOWLEDGE_SECTIONS.map((section) => {
            const entries = sectionItems(items, section, query)
            const Icon = section.icon
            return (
              <section key={section.id} className="mb-2">
                <div className="flex h-8 items-center gap-2 rounded px-2 text-xs text-[#a8adb7]">
                  <ChevronDown className="h-3.5 w-3.5 text-[#858b96]" />
                  <Icon className="h-3.5 w-3.5 text-[#9aa0aa]" />
                  <span className="min-w-0 flex-1 truncate font-medium">{section.label}</span>
                  <span className="text-[11px] text-[#858b96]">{entries.length}</span>
                  <button
                    type="button"
                    className="rounded p-1 text-[#858b96] hover:bg-[#303238] hover:text-[#d7dbe2]"
                    disabled={saving}
                    onClick={() => onCreate(section)}
                    aria-label={`新建${section.label}`}
                  >
                    <Plus className="h-3.5 w-3.5" />
                  </button>
                </div>
                <div className="ml-5 space-y-0.5 border-l border-[#303238] pl-2">
                  {entries.length ? entries.map((item) => (
                    <button
                      key={item.id}
                      type="button"
                      onClick={() => onSelect(item.id)}
                      className={`flex h-8 w-full items-center gap-2 rounded-md px-2 text-left text-xs transition ${
                        activeId === item.id ? 'bg-[#303238] text-[#f0f2f5]' : 'text-[#aeb4bf] hover:bg-[#25262a] hover:text-[#d7dbe2]'
                      }`}
                    >
                      <FileText className="h-3.5 w-3.5 shrink-0 text-[#858b96]" />
                      <span className="min-w-0 flex-1 truncate">{item.name}</span>
                    </button>
                  )) : (
                    <div className="px-2 py-1.5 text-[11px] text-[#6f7580]">暂无条目</div>
                  )}
                </div>
              </section>
            )
          })}
        </div>
      </ScrollArea>
    </>
  )
}

function CreatorDirectory() {
  return (
    <div className="p-2">
      <div className="flex h-8 items-center gap-2 rounded px-2 text-xs text-[#a8adb7]">
        <ChevronDown className="h-3.5 w-3.5 text-[#858b96]" />
        <Folder className="h-3.5 w-3.5 text-[#9aa0aa]" />
        <span className="font-medium">作品根目录</span>
      </div>
      <div className="ml-5 border-l border-[#303238] pl-2">
        <div className="flex h-8 items-center gap-2 rounded-md bg-[#303238] px-2 text-xs text-[#f0f2f5]">
          <BookMarked className="h-3.5 w-3.5 text-[#a8adb7]" />
          <span className="truncate">{CREATOR_PATH}</span>
        </div>
      </div>
    </div>
  )
}

function TellerDirectory({
  tellers,
  activeTellerId,
  saving,
  onSelect,
  onCreate,
}: {
  tellers: Teller[]
  activeTellerId: string
  saving: boolean
  onSelect: (id: string) => void
  onCreate: () => void
}) {
  return (
    <>
      <div className="flex h-10 items-center justify-between border-b border-[#303238] px-3">
        <div className="text-xs font-medium text-[#a8adb7]">讲述者目录</div>
        <Button className="h-7 border-[#303238] bg-[#25262a] text-[#d7dbe2] hover:bg-[#303238]" variant="outline" size="icon" disabled={saving} onClick={onCreate} aria-label="新建讲述者">
          <Plus className="h-3.5 w-3.5" />
        </Button>
      </div>
      <ScrollArea className="min-h-0 flex-1">
        <div className="p-2">
          <div className="flex h-8 items-center gap-2 rounded px-2 text-xs text-[#a8adb7]">
            <ChevronDown className="h-3.5 w-3.5 text-[#858b96]" />
            <Folder className="h-3.5 w-3.5 text-[#9aa0aa]" />
            <span className="font-medium">规则包</span>
          </div>
          <div className="ml-5 space-y-0.5 border-l border-[#303238] pl-2">
            {tellers.map((teller) => (
              <button
                key={teller.id}
                type="button"
                onClick={() => onSelect(teller.id)}
                className={`flex min-h-9 w-full items-center gap-2 rounded-md px-2 py-1 text-left text-xs transition ${
                  activeTellerId === teller.id ? 'bg-[#303238] text-[#f0f2f5]' : 'text-[#aeb4bf] hover:bg-[#25262a] hover:text-[#d7dbe2]'
                }`}
              >
                <SlidersHorizontal className="h-3.5 w-3.5 shrink-0 text-[#858b96]" />
                <span className="min-w-0 flex-1">
                  <span className="block truncate">{teller.name}</span>
                  <span className="block truncate text-[11px] text-[#858b96]">{teller.custom ? '自定义' : '内置'} · {(teller.slots || []).filter((slot) => slot.enabled).length} 条启用规则</span>
                </span>
              </button>
            ))}
          </div>
        </div>
      </ScrollArea>
    </>
  )
}

function LoreEditor({
  draft,
  tagDraft,
  setDraft,
  setTagDraft,
  onSave,
}: {
  draft: LoreItem | null
  tagDraft: string
  setDraft: (draft: LoreItem | null) => void
  setTagDraft: (value: string) => void
  onSave: () => void
}) {
  if (!draft) {
    return <EmptyState title="未选择资料" description="从左侧资料库目录选择或新建一个条目。" />
  }

  return (
    <div className="flex min-h-0 flex-1 flex-col">
      <div className="grid shrink-0 gap-3 border-b border-[#303238] bg-[#202124] p-4 lg:grid-cols-[minmax(220px,1fr)_180px_180px]">
        <Field label="名称">
          <Input className={inputClassName} value={draft.name} onChange={(event) => setDraft({ ...draft, name: event.target.value })} />
        </Field>
        <Field label="类型">
          <Select value={draft.type} onValueChange={(value) => setDraft({ ...draft, type: value as LoreItem['type'] })}>
            <SelectTrigger size="sm" className={selectClassName}>
              <SelectValue />
            </SelectTrigger>
            <SelectContent className="border-[#303238] bg-[#25262a] text-[#d7dbe2]">
              {TYPE_OPTIONS.map((option) => (
                <SelectItem key={option.value} value={option.value}>{option.label}</SelectItem>
              ))}
            </SelectContent>
          </Select>
        </Field>
        <Field label="重要度">
          <Select value={draft.importance} onValueChange={(value) => setDraft({ ...draft, importance: value as LoreItem['importance'] })}>
            <SelectTrigger size="sm" className={selectClassName}>
              <SelectValue />
            </SelectTrigger>
            <SelectContent className="border-[#303238] bg-[#25262a] text-[#d7dbe2]">
              {IMPORTANCE_OPTIONS.map((option) => (
                <SelectItem key={option.value} value={option.value}>{option.label}</SelectItem>
              ))}
            </SelectContent>
          </Select>
        </Field>
        <Field label="标签">
          <Input className={inputClassName} value={tagDraft} onChange={(event) => setTagDraft(event.target.value)} placeholder="用逗号分隔" />
        </Field>
      </div>
      <div className="min-h-0 flex-1 p-4">
        <Textarea
          className="h-full min-h-[360px] resize-none border-[#303238] bg-[#202124] font-mono text-sm leading-7 text-[#d7dbe2] shadow-none focus-visible:ring-0"
          value={draft.content || ''}
          onChange={(event) => setDraft({ ...draft, content: event.target.value })}
          onKeyDown={(event) => {
            if (isSaveShortcut(event)) {
              event.preventDefault()
              event.stopPropagation()
              onSave()
            }
          }}
        />
      </div>
    </div>
  )
}

function CreatorEditor({
  content,
  setContent,
  onSave,
}: {
  content: string
  setContent: (value: string) => void
  onSave: () => void
}) {
  return (
    <div className="min-h-0 flex-1 p-4">
      <Textarea
        className="h-full min-h-[520px] resize-none border-[#303238] bg-[#202124] font-mono text-sm leading-7 text-[#d7dbe2] shadow-none focus-visible:ring-0"
        value={content}
        onChange={(event) => setContent(event.target.value)}
        placeholder="写下本书最高优先级的创作规则..."
        onKeyDown={(event) => {
          if (isSaveShortcut(event)) {
            event.preventDefault()
            event.stopPropagation()
            onSave()
          }
        }}
      />
    </div>
  )
}

function TellerEditor({
  draft,
  setDraft,
  tagDraft,
  setTagDraft,
  activeSlotId,
  setActiveSlotId,
  onSave,
}: {
  draft: Teller | null
  setDraft: (draft: Teller | null) => void
  tagDraft: string
  setTagDraft: (value: string) => void
  activeSlotId: string
  setActiveSlotId: (id: string) => void
  onSave: () => void
}) {
  const activeSlot = draft?.slots?.find((slot) => slot.id === activeSlotId) || draft?.slots?.[0] || null

  const updateSlot = (patch: Partial<TellerPromptSlot>) => {
    if (!draft || !activeSlot) return
    setDraft({
      ...draft,
      slots: draft.slots.map((slot) => slot.id === activeSlot.id ? { ...slot, ...patch } : slot),
    })
  }

  const addSlot = () => {
    if (!draft) return
    const id = `slot-${Date.now()}`
    const slot: TellerPromptSlot = {
      id,
      name: '新规则',
      target: 'context',
      enabled: true,
      content: '',
    }
    setDraft({ ...draft, slots: [...(draft.slots || []), slot] })
    setActiveSlotId(id)
  }

  const deleteSlot = () => {
    if (!draft || !activeSlot) return
    const nextSlots = draft.slots.filter((slot) => slot.id !== activeSlot.id)
    setDraft({ ...draft, slots: nextSlots })
    setActiveSlotId(nextSlots[0]?.id || '')
  }

  if (!draft) {
    return <EmptyState title="未选择讲述者" description="从左侧讲述者目录选择或新建一个规则包。" />
  }

  const selectedTarget = targetOption(activeSlot?.target || 'context')

  return (
    <div className="flex min-h-0 flex-1 flex-col">
      <div className="grid shrink-0 gap-3 border-b border-[#303238] bg-[#202124] p-4 lg:grid-cols-[minmax(220px,1fr)_minmax(220px,1fr)_150px]">
        <Field label="名称">
          <Input className={inputClassName} value={draft.name} onChange={(event) => setDraft({ ...draft, name: event.target.value })} />
        </Field>
        <Field label="描述">
          <Input className={inputClassName} value={draft.description} onChange={(event) => setDraft({ ...draft, description: event.target.value })} placeholder="选择时显示的简介" />
        </Field>
        <Field label="随机事件率">
          <Input className={inputClassName} value={String(draft.random_event_rate ?? 0)} onChange={(event) => setDraft({ ...draft, random_event_rate: Number(event.target.value) || 0 })} />
        </Field>
        <Field label="标签">
          <Input className={inputClassName} value={tagDraft} onChange={(event) => setTagDraft(event.target.value)} placeholder="用逗号分隔" />
        </Field>
        <Field label="上下文回合数">
          <Input
            className={inputClassName}
            value={String(draft.context_policy?.recent_turns ?? 0)}
            onChange={(event) => setDraft({
              ...draft,
              context_policy: { ...draft.context_policy, recent_turns: Number(event.target.value) || 0 },
            })}
          />
        </Field>
        <div className="flex items-end">
          <span className="rounded border border-[#303238] bg-[#1b1c1f] px-2 py-1 text-xs text-[#858b96]">{draft.custom ? '自定义' : '内置'}</span>
        </div>
      </div>

      <div className="grid min-h-0 flex-1 grid-cols-[280px_minmax(0,1fr)]">
        <aside className="flex min-h-0 flex-col border-r border-[#303238] bg-[#202124]">
          <div className="flex h-11 items-center justify-between border-b border-[#303238] px-3">
            <div className="text-xs font-medium text-[#c5c9d1]">注入规则</div>
            <Button className="h-7 border-[#303238] bg-[#25262a] text-[#d7dbe2] hover:bg-[#303238]" variant="outline" size="icon" onClick={addSlot} aria-label="新增注入规则">
              <Plus className="h-3.5 w-3.5" />
            </Button>
          </div>
          <ScrollArea className="min-h-0 flex-1">
            <div className="p-2">
              {(draft.slots || []).map((slot) => (
                <button
                  key={slot.id}
                  type="button"
                  onClick={() => setActiveSlotId(slot.id)}
                  className={`mb-1 flex min-h-10 w-full items-center gap-2 rounded-md px-2 py-1.5 text-left text-xs transition ${
                    activeSlot?.id === slot.id ? 'bg-[#303238] text-[#f0f2f5]' : 'text-[#aeb4bf] hover:bg-[#25262a] hover:text-[#d7dbe2]'
                  }`}
                >
                  <FileText className="h-3.5 w-3.5 shrink-0 text-[#858b96]" />
                  <span className="min-w-0 flex-1">
                    <span className="block truncate">{slot.name}</span>
                    <span className="block truncate text-[11px] text-[#858b96]">{targetLabel(slot.target)} · {slot.enabled ? '已启用' : '已停用'}</span>
                  </span>
                  <span className={`h-2 w-2 shrink-0 rounded-full ${slot.enabled ? 'bg-[#81b38d]' : 'bg-[#565c66]'}`} />
                </button>
              ))}
            </div>
          </ScrollArea>
        </aside>

        {activeSlot ? (
          <section className="flex min-h-0 flex-col">
            <div className="shrink-0 border-b border-[#303238] bg-[#202124] p-4">
              <div className="grid gap-3 lg:grid-cols-[minmax(220px,1fr)_minmax(260px,420px)]">
                <Field label="规则名称">
                  <Input className={inputClassName} value={activeSlot.name} onChange={(event) => updateSlot({ name: event.target.value })} />
                </Field>
                <div className="flex items-end justify-between gap-3 rounded-md border border-[#303238] bg-[#1b1c1f] px-3 py-2">
                  <div className="min-w-0">
                    <div className="text-[11px] text-[#858b96]">Prompt 效果</div>
                    <div className="mt-1 truncate text-xs font-medium text-[#d7dbe2]">{selectedTarget.label}</div>
                    <div className="mt-0.5 line-clamp-2 text-[11px] leading-4 text-[#858b96]">{selectedTarget.detail}</div>
                  </div>
                  <ToggleSwitch checked={activeSlot.enabled} onChange={(enabled) => updateSlot({ enabled })} />
                </div>
              </div>

              <div className="mt-4">
                <div className="mb-2 flex items-center justify-between gap-3">
                  <div>
                    <div className="text-xs font-medium text-[#c5c9d1]">注入位置</div>
                    <div className="mt-0.5 text-[11px] text-[#858b96]">选择这条规则交给哪一段 Agent 流程使用。</div>
                  </div>
                  <Button className="h-8 border-[#303238] bg-[#25262a] text-[#d7dbe2] hover:bg-[#303238]" variant="outline" size="icon" disabled={(draft.slots || []).length <= 1} onClick={deleteSlot} aria-label="删除注入规则">
                    <Trash2 className="h-4 w-4" />
                  </Button>
                </div>
                <div className="grid gap-2 xl:grid-cols-4 lg:grid-cols-3 md:grid-cols-2">
                  {TELLER_TARGET_OPTIONS.map((option) => {
                    const selected = activeSlot.target === option.value
                    return (
                      <button
                        key={option.value}
                        type="button"
                        onClick={() => updateSlot({ target: option.value as TellerTarget })}
                        className={`min-h-[76px] rounded-md border p-3 text-left transition ${
                          selected
                            ? 'border-[#d6aa62]/60 bg-[#2a271f] text-[#f0f2f5]'
                            : 'border-[#303238] bg-[#1b1c1f] text-[#aeb4bf] hover:border-[#444850] hover:bg-[#25262a] hover:text-[#d7dbe2]'
                        }`}
                      >
                        <span className="block text-xs font-medium">{option.label}</span>
                        <span className="mt-1 block text-[11px] leading-4 text-[#858b96]">{option.summary}</span>
                      </button>
                    )
                  })}
                </div>
              </div>
            </div>
            <div className="min-h-0 flex-1 p-4">
              <Textarea
                className="h-full min-h-[360px] resize-none border-[#303238] bg-[#202124] font-mono text-sm leading-7 text-[#d7dbe2] shadow-none focus-visible:ring-0"
                value={activeSlot.content}
                onChange={(event) => updateSlot({ content: event.target.value })}
                onKeyDown={(event) => {
                  if (isSaveShortcut(event)) {
                    event.preventDefault()
                    event.stopPropagation()
                    onSave()
                  }
                }}
              />
            </div>
          </section>
        ) : (
          <EmptyState title="暂无注入规则" description="为这个讲述者新增一条规则。" />
        )}
      </div>
    </div>
  )
}

function Field({ label, children }: { label: string; children: ReactNode }) {
  return (
    <label className="grid gap-1.5">
      <span className="text-[11px] text-[#858b96]">{label}</span>
      {children}
    </label>
  )
}

function EmptyState({ title, description }: { title: string; description: string }) {
  return (
    <div className="flex min-h-0 flex-1 items-center justify-center p-6">
      <div className="rounded-md border border-dashed border-[#303238] bg-[#202124] px-6 py-5 text-center">
        <div className="text-sm font-medium text-[#d7dbe2]">{title}</div>
        <div className="mt-1 text-xs text-[#858b96]">{description}</div>
      </div>
    </div>
  )
}

function ToggleSwitch({ checked, onChange }: { checked: boolean; onChange: (checked: boolean) => void }) {
  return (
    <button
      type="button"
      role="switch"
      aria-checked={checked}
      onClick={() => onChange(!checked)}
      className={`relative h-6 w-11 shrink-0 rounded-full border transition ${
        checked ? 'border-[#81b38d]/60 bg-[#31543a]' : 'border-[#3a3d44] bg-[#25262a]'
      }`}
    >
      <span
        className={`absolute top-0.5 h-5 w-5 rounded-full bg-[#f0f2f5] shadow transition ${
          checked ? 'left-[22px]' : 'left-0.5'
        }`}
      />
      <span className="sr-only">{checked ? '停用规则' : '启用规则'}</span>
    </button>
  )
}

const inputClassName = 'h-8 border-[#303238] bg-[#1b1c1f] text-xs text-[#d7dbe2] focus-visible:ring-0'
const selectClassName = 'h-8 border-[#303238] bg-[#1b1c1f] text-xs text-[#d7dbe2] focus:ring-0'

function splitTags(value: string) {
  return value
    .split(/[，,]/)
    .map((tag) => tag.trim())
    .filter(Boolean)
}

function ModeIcon({ mode }: { mode: SettingPanelMode }) {
  if (mode === 'creator') return <BookMarked className="h-3.5 w-3.5 shrink-0 text-[#9aa0aa]" />
  if (mode === 'teller') return <SlidersHorizontal className="h-3.5 w-3.5 shrink-0 text-[#9aa0aa]" />
  return <Database className="h-3.5 w-3.5 shrink-0 text-[#9aa0aa]" />
}

function sectionItems(items: LoreItem[], section: KnowledgeSection, query = '') {
  const normalizedQuery = query.trim().toLowerCase()
  return items.filter((item) => {
    if (!section.types.includes(item.type)) return false
    const tags = item.tags || []
    if (section.tag && !tags.includes(section.tag)) return false
    if (section.excludeTag && tags.includes(section.excludeTag)) return false
    if (normalizedQuery) {
      const haystack = `${item.name}\n${item.content || ''}\n${tags.join('\n')}`.toLowerCase()
      if (!haystack.includes(normalizedQuery)) return false
    }
    return true
  })
}

function targetLabel(target: TellerTarget) {
  return targetOption(target).label
}

function targetOption(target: TellerTarget) {
  return TELLER_TARGET_OPTIONS.find((option) => option.value === target) || TELLER_TARGET_OPTIONS[1]
}

function loreTypeLabel(type: LoreItem['type']) {
  return TYPE_OPTIONS.find((option) => option.value === type)?.label || '其他'
}

function panelTitle(mode: SettingPanelMode) {
  if (mode === 'creator') return '创作者'
  if (mode === 'teller') return '讲述者'
  return '资料库'
}

function editorTitle(mode: SettingPanelMode, draft: LoreItem | null, tellerDraft: Teller | null) {
  if (mode === 'creator') return CREATOR_PATH
  if (mode === 'teller') return tellerDraft?.name || '故事讲述者'
  return draft?.name || '资料库'
}

function editorSubtitle(mode: SettingPanelMode, draft: LoreItem | null, tellerDraft: Teller | null) {
  if (mode === 'creator') return '当前作品最高优先级规则'
  if (mode === 'teller') return tellerDraft?.description || '用户级 prompt slot 配置'
  if (!draft) return '角色、地点、组织、规则与素材'
  return `${loreTypeLabel(draft.type)} · ${draft.importance} · ${(draft.tags || []).join('，') || '无标签'}`
}

function newTellerDraft(): Partial<Teller> {
  const id = `custom-${Date.now()}`
  return {
    id,
    name: '自定义讲述者',
    description: '新的故事讲述规则包',
    random_event_rate: 0.15,
    tags: ['自定义'],
    context_policy: {
      creator: 'always',
      lore: 'relevant',
      runtime_state: 'always',
      recent_turns: 8,
    },
    slots: [
      {
        id: 'identity',
        name: '讲述者身份',
        target: 'system',
        enabled: true,
        content: '你是一位自定义故事讲述者。',
      },
      {
        id: 'thinking_rules',
        name: '内部思考规则',
        target: 'thinking',
        enabled: true,
        content: '先判断用户行动的目标、风险、相关角色和世界规则，再决定本回合后果；不要把分析过程写进正文。',
      },
      {
        id: 'turn_rules',
        name: '回合输出规则',
        target: 'turn',
        enabled: true,
        content: '只输出本回合故事正文，并在结尾留下自然的继续入口。',
      },
    ],
  }
}
