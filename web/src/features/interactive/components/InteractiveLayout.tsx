import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { BookMarked, Database, GripHorizontal, GripVertical, MessageSquareText, PanelLeft, PanelRight, SlidersHorizontal } from 'lucide-react'
import { Group, Panel, Separator } from 'react-resizable-panels'
import type { Layout, PanelImperativeHandle, PanelSize } from 'react-resizable-panels'
import { createInteractiveBranch, createInteractiveStory, deleteInteractiveBranch, deleteInteractiveStory, getInteractiveBranches, getInteractiveSnapshot, getInteractiveStories, getInteractiveTellers, switchInteractiveBranch, updateInteractiveStory } from '../api'
import { useInteractiveStore } from '../stores/interactive-store'
import type { BranchSummary, InteractiveSubmode, Snapshot } from '../types'
import { BranchTimeline } from './BranchTimeline'
import { SettingPanel, type SettingPanelMode } from './SettingPanel'
import { SnapshotPanel } from './SnapshotPanel'
import { StoryPicker } from './StoryPicker'
import { StoryStage } from './StoryStage'
import { TellerPicker } from './TellerPicker'

const MAIN_VERTICAL_LAYOUT_KEY = 'nova-interactive-main-vertical'
const TIMELINE_EXPANDED_KEY = 'nova-interactive-branch-timeline-expanded'

interface InteractiveLayoutProps {
  workspace?: string
  leftPanelVisible?: boolean
  rightPanelVisible?: boolean
  onToggleLeftPanel?: () => void
  onToggleRightPanel?: () => void
}

export function InteractiveLayout({
  workspace,
  leftPanelVisible = true,
  rightPanelVisible = true,
  onToggleLeftPanel,
  onToggleRightPanel,
}: InteractiveLayoutProps) {
  const {
    stories, tellers, branches, snapshot, currentStoryId, currentBranchId, submode,
    setStories, setTellers, setBranches, setSnapshot, setCurrentStoryId, setCurrentBranchId, setSubmode, resetWorkspaceState,
  } = useInteractiveStore()
  const currentStory = stories.find((story) => story.id === currentStoryId)
  const currentBranchSnapshot = snapshot?.story_id === currentStoryId && snapshot.branch_id === currentBranchId ? snapshot : null
  const snapshotStoryIdRef = useRef('')
  const snapshotRequestSeqRef = useRef(0)
  const initialMainLayout = useMemo(() => readStoredLayout(MAIN_VERTICAL_LAYOUT_KEY), [])
  const [timelineExpanded, setTimelineExpanded] = useState(readStoredTimelineExpanded)
  const timelinePanelRef = useRef<PanelImperativeHandle | null>(null)
  const timelineExpandedHeightRef = useRef(220)
  const timelinePanelSyncedRef = useRef(false)
  const preferredTimelineHeight = useMemo(() => estimateTimelineHeight(snapshot, branches), [branches, snapshot])

  useEffect(() => {
    snapshotStoryIdRef.current = snapshot?.story_id || ''
  }, [snapshot?.story_id])

  const reloadStories = useCallback(async () => {
    const index = await getInteractiveStories()
    setStories(index.stories || [], index.current_story_id)
  }, [setStories])

  const reloadSnapshot = useCallback(async (branchOverride?: string, storyOverride?: string) => {
    const requestSeq = snapshotRequestSeqRef.current + 1
    snapshotRequestSeqRef.current = requestSeq
    const storyId = storyOverride || currentStoryId
    if (!storyId) {
      setSnapshot(null)
      return
    }
    const branchId = branchOverride ?? (snapshotStoryIdRef.current === storyId ? currentBranchId : '')
    const [nextSnapshot, nextBranches] = await Promise.all([
      getInteractiveSnapshot(storyId, branchId),
      getInteractiveBranches(storyId),
    ])
    if (requestSeq !== snapshotRequestSeqRef.current) return
    setSnapshot(nextSnapshot)
    setBranches(nextBranches)
  }, [currentBranchId, currentStoryId, setBranches, setSnapshot])

  useEffect(() => {
    snapshotRequestSeqRef.current += 1
    snapshotStoryIdRef.current = ''
    if (workspace !== undefined) {
      resetWorkspaceState()
      if (!workspace) return
    }
    void Promise.all([reloadStories(), getInteractiveTellers().then(setTellers)])
  }, [reloadStories, resetWorkspaceState, setTellers, workspace])

  useEffect(() => {
    void reloadSnapshot()
  }, [currentStoryId])

  useEffect(() => {
    if (snapshot?.current_turn?.state_status !== 'pending') return
    const timer = window.setInterval(() => {
      void reloadSnapshot(snapshot.branch_id)
    }, 1000)
    return () => window.clearInterval(timer)
  }, [reloadSnapshot, snapshot?.branch_id, snapshot?.current_turn?.id, snapshot?.current_turn?.state_status])

  const handleCreateStory = async (input: { title: string; origin: string; story_teller_id: string }) => {
    const story = await createInteractiveStory(input)
    await reloadStories()
    setCurrentStoryId(story.id)
  }

  const handleDeleteStory = async (storyId: string) => {
    await deleteInteractiveStory(storyId)
    await reloadStories()
  }

  const handleTellerChange = async (tellerId: string) => {
    if (!currentStoryId) return
    await updateInteractiveStory(currentStoryId, { story_teller_id: tellerId })
    await reloadStories()
  }

  const handleSwitchBranch = async (branchId: string) => {
    const storyId = currentStoryId || useInteractiveStore.getState().currentStoryId || snapshot?.story_id
    if (!storyId) return
    await switchInteractiveBranch(storyId, branchId)
    setCurrentBranchId(branchId)
    await reloadSnapshot(branchId, storyId)
  }

  const handleCreateBranch = async (turnId: string, title: string) => {
    if (!currentStoryId) return
    const branch = await createInteractiveBranch(currentStoryId, { parent_event_id: turnId, title })
    setCurrentBranchId(branch.id)
    await reloadSnapshot(branch.id)
  }

  const handleDeleteBranch = async (branchId: string) => {
    if (!currentStoryId) return
    await deleteInteractiveBranch(currentStoryId, branchId)
    if (branchId === currentBranchId) {
      setCurrentBranchId('main')
    }
    await reloadSnapshot(branchId === currentBranchId ? 'main' : undefined)
    await reloadStories()
  }

  const handleTimelineExpandedChange = (nextExpanded: boolean) => {
    setTimelineExpanded(nextExpanded)
    storeTimelineExpanded(nextExpanded)
    window.requestAnimationFrame(() => {
      if (nextExpanded) {
        const nextHeight = Math.max(180, timelineExpandedHeightRef.current, preferredTimelineHeight)
        timelineExpandedHeightRef.current = nextHeight
        timelinePanelRef.current?.resize(`${nextHeight}px`)
      } else {
        timelinePanelRef.current?.resize('48px')
      }
    })
  }

  const handleMainLayoutChanged = useCallback((layout: Layout) => {
    storeLayout(MAIN_VERTICAL_LAYOUT_KEY, layout)
    if (typeof layout?.['branch-timeline'] !== 'number') return
    const expandedByLayout = isTimelineExpandedLayout(layout)
    setTimelineExpanded((current) => {
      if (current === expandedByLayout) return current
      storeTimelineExpanded(expandedByLayout)
      return expandedByLayout
    })
  }, [])

  const handleTimelineResize = (size: PanelSize) => {
    if (timelineExpanded) timelineExpandedHeightRef.current = Math.max(180, size.inPixels)
  }

  useEffect(() => {
    if (!timelineExpanded) {
      timelinePanelSyncedRef.current = false
      return
    }
    if (timelinePanelSyncedRef.current && preferredTimelineHeight <= timelineExpandedHeightRef.current + 24) return
    const nextHeight = Math.max(180, timelineExpandedHeightRef.current, preferredTimelineHeight)
    timelineExpandedHeightRef.current = nextHeight
    timelinePanelSyncedRef.current = true
    window.requestAnimationFrame(() => {
      timelinePanelRef.current?.resize(`${nextHeight}px`)
    })
  }, [preferredTimelineHeight, timelineExpanded])

  const mainTabs: Array<{ value: InteractiveSubmode; label: string; icon: typeof MessageSquareText }> = [
    { value: 'story', label: '剧情', icon: MessageSquareText },
    { value: 'lore', label: '资料库', icon: Database },
    { value: 'creator', label: '创作者', icon: BookMarked },
    { value: 'teller', label: '讲述者', icon: SlidersHorizontal },
  ]
  const settingMode: SettingPanelMode = submode === 'story' ? 'lore' : submode
  const settingsWorkspaceVisible = submode !== 'story'
  return (
    <div className="flex h-full min-h-0 flex-col bg-[var(--nova-bg)] text-[var(--nova-text)]">
      <div data-testid="interactive-shell" className="flex min-h-0 flex-1 flex-col overflow-hidden bg-[var(--nova-bg)]">
        <div className="flex min-h-0 flex-1">
          <aside className={`nova-sidebar flex shrink-0 flex-col gap-1 border-r p-3 transition-[width] duration-500 ease-[var(--nova-ease)] ${leftPanelVisible ? 'w-64' : 'w-16'}`} aria-label="互动页面切换">
            {leftPanelVisible && (
              <div className="mb-2 flex flex-col gap-3 border-b border-[var(--nova-border)] pb-4">
                <StoryPicker layout="sidebar" stories={stories} currentStoryId={currentStoryId} tellers={tellers} onSelect={setCurrentStoryId} onCreate={handleCreateStory} onDelete={handleDeleteStory} />
                <TellerPicker layout="sidebar" story={currentStory} tellers={tellers} onChange={handleTellerChange} />
              </div>
            )}
            <div className="flex min-h-0 flex-1 flex-col gap-1">
              {mainTabs.map((item) => {
                const Icon = item.icon
                const active = submode === item.value
                return (
                  <button
                    key={item.value}
                    type="button"
                    onClick={() => setSubmode(item.value)}
                    className={`nova-nav-item flex h-10 items-center text-left text-xs ${
                      leftPanelVisible ? 'gap-3 px-4' : 'justify-center px-0'
                    } ${active ? 'is-active' : ''}`}
                    aria-current={active ? 'page' : undefined}
                    aria-label={item.label}
                    title={item.label}
                  >
                    <Icon className="h-4 w-4 shrink-0" />
                    {leftPanelVisible && <span className="truncate font-medium">{item.label}</span>}
                  </button>
                )
              })}
            </div>
            <div className="mt-2 flex flex-col gap-1 border-t border-[var(--nova-border)] pt-3">
              <button
                type="button"
                className={`nova-nav-item flex h-10 items-center text-xs ${
                  leftPanelVisible ? 'gap-3 px-4' : 'justify-center px-0'
                } ${rightPanelVisible ? 'is-active' : ''}`}
                onClick={onToggleRightPanel}
                title={rightPanelVisible ? '隐藏场景记忆' : '显示场景记忆'}
                aria-label={rightPanelVisible ? '隐藏场景记忆' : '显示场景记忆'}
              >
                <PanelRight className="h-4 w-4 shrink-0" />
                {leftPanelVisible && <span className="truncate font-medium">场景记忆</span>}
              </button>
              <button
                type="button"
                className={`nova-nav-item flex h-10 items-center text-xs disabled:cursor-not-allowed disabled:opacity-50 ${
                  leftPanelVisible ? 'gap-3 px-4' : 'justify-center px-0'
                }`}
                onClick={onToggleLeftPanel}
                disabled={!onToggleLeftPanel}
                title={leftPanelVisible ? '收起左侧导航' : '展开左侧导航'}
                aria-label={leftPanelVisible ? '收起左侧导航' : '展开左侧导航'}
              >
                <PanelLeft className={`h-4 w-4 shrink-0 transition-transform ${leftPanelVisible ? '' : 'rotate-180'}`} />
                {leftPanelVisible && <span className="truncate font-medium">收起导航</span>}
              </button>
              {leftPanelVisible && (
                <div className="mt-2 flex items-center gap-2 px-4 py-1 text-[11px] text-[var(--nova-text-faint)]">
                  <span className={`h-2 w-2 rounded-full ${snapshot?.current_turn?.state_status === 'pending' ? 'bg-[#d6aa62]' : 'bg-[#81b38d]'}`} />
                  <span className="truncate">{snapshot?.current_turn?.state_status === 'pending' ? '场景同步中' : '已同步'}</span>
                </div>
              )}
            </div>
          </aside>
          <div className="flex min-w-0 flex-1 flex-col">
            {settingsWorkspaceVisible ? (
              <SettingPanel
                mode={settingMode}
                tellers={tellers}
                onTellersChange={setTellers}
              />
            ) : (
              <Group
                id={MAIN_VERTICAL_LAYOUT_KEY}
                defaultLayout={initialMainLayout}
                onLayoutChanged={handleMainLayoutChanged}
                orientation="vertical"
                className="min-h-0 flex-1"
              >
                <Panel id="stage-area" minSize="240px" className="min-h-0">
                  <Group
                    id="nova-interactive-horizontal"
                    defaultLayout={readStoredLayout('nova-interactive-horizontal')}
                    onLayoutChanged={(layout) => storeLayout('nova-interactive-horizontal', layout)}
                    orientation="horizontal"
                    className="min-h-0"
                  >
                    <Panel id="story-stage" minSize="240px" className="min-w-0">
                      <StoryStage workspace={workspace} storyId={currentStoryId} branchId={currentBranchId} snapshot={currentBranchSnapshot} onDone={reloadSnapshot} />
                    </Panel>
                    {rightPanelVisible && (
                      <>
                        <InteractiveResizeHandle direction="vertical" label="调整场景记忆宽度" />
                        <Panel id="snapshot" defaultSize="320px" minSize="180px" maxSize="45%" className="min-w-0">
                          <SnapshotPanel snapshot={currentBranchSnapshot} />
                        </Panel>
                      </>
                    )}
                  </Group>
                </Panel>
                <InteractiveResizeHandle direction="horizontal" label="调整剧情路线图高度" prominent />
                <Panel
                  id="branch-timeline"
                  defaultSize={timelineExpanded ? '220px' : '48px'}
                  minSize={timelineExpanded ? '160px' : '48px'}
                  maxSize="38%"
                  className="min-h-0"
                  onResize={handleTimelineResize}
                  panelRef={timelinePanelRef}
                >
                  <BranchTimeline
                    snapshot={snapshot}
                    branches={branches}
                    currentBranchId={currentBranchId}
                    onSwitchBranch={handleSwitchBranch}
                    onCreateBranch={handleCreateBranch}
                    onDeleteBranch={handleDeleteBranch}
                    expanded={timelineExpanded}
                    fill
                    onExpandedChange={handleTimelineExpandedChange}
                  />
                </Panel>
              </Group>
            )}
          </div>
        </div>
      </div>
    </div>
  )
}

function InteractiveResizeHandle({ direction, label, prominent = false }: { direction: 'horizontal' | 'vertical'; label: string; prominent?: boolean }) {
  const Icon = direction === 'vertical' ? GripVertical : GripHorizontal
  const className = direction === 'vertical'
    ? 'nova-resize-handle group -mx-1 flex w-3 cursor-col-resize items-center justify-center bg-transparent transition-colors'
    : `nova-resize-handle group ${prominent ? '-my-0.5 h-4' : '-my-1 h-3'} flex cursor-row-resize items-center justify-center bg-transparent transition-colors`

  return (
    <Separator aria-label={label} className={className}>
      <span className={`flex items-center justify-center rounded-full border border-[var(--nova-border)] bg-[var(--nova-surface)] text-[var(--nova-text-faint)] shadow-[0_4px_14px_rgba(0,0,0,0.22)] transition-colors group-hover:border-[#3a3a3a] group-data-[resize-handle-active]:border-[#4a4a4a] group-data-[resize-handle-active]:text-[var(--nova-text)] ${direction === 'vertical' ? 'h-9 w-2.5' : 'h-2.5 w-16'}`}>
        <Icon className={direction === 'vertical' ? 'h-3.5 w-3.5' : 'h-3 w-3'} aria-hidden="true" />
      </span>
    </Separator>
  )
}

function estimateTimelineHeight(snapshot: Snapshot | null, branches: BranchSummary[]) {
  const graphNodes = snapshot?.graph?.nodes || []
  const graphBranches = snapshot?.graph?.branches?.length ? snapshot.graph.branches : branches
  const branchIds = new Set(graphBranches.map((branch) => branch.id))
  for (const node of graphNodes) branchIds.add(node.branch_id)
  const rowCount = Math.max(1, branchIds.size)
  const contentHeight = 104 + rowCount * 56
  const viewportHeight = typeof window === 'undefined' ? 720 : window.innerHeight
  const maxHeight = Math.max(220, Math.floor(viewportHeight * 0.36))
  return Math.min(maxHeight, Math.max(190, contentHeight))
}

function readStoredLayout(key: string): Layout | undefined {
  if (typeof window === 'undefined') return undefined
  const value = window.localStorage.getItem(key)
  if (!value) return undefined
  try {
    return JSON.parse(value) as Layout
  } catch {
    return undefined
  }
}

function readStoredTimelineExpanded() {
  if (typeof window !== 'undefined') {
    const value = window.localStorage.getItem(TIMELINE_EXPANDED_KEY)
    if (value === 'true') return true
    if (value === 'false') return false
  }
  return true
}

function storeTimelineExpanded(expanded: boolean) {
  if (typeof window === 'undefined') return
  window.localStorage.setItem(TIMELINE_EXPANDED_KEY, expanded ? 'true' : 'false')
}

function isTimelineExpandedLayout(layout?: Layout) {
  const timelineSize = layout?.['branch-timeline']
  return typeof timelineSize === 'number' && timelineSize > 12
}

function storeLayout(key: string, layout: Layout) {
  if (typeof window === 'undefined') return
  window.localStorage.setItem(key, JSON.stringify(layout))
}
