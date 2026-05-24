import { Badge } from '@/components/ui/badge'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Separator } from '@/components/ui/separator'
import type { Snapshot } from '../types'

function asArray(value: unknown): unknown[] {
  return Array.isArray(value) ? value : []
}

export function SnapshotPanel({ snapshot }: { snapshot: Snapshot | null }) {
  const onStage = asArray(snapshot?.state?.on_stage)
  const events = asArray(snapshot?.state?.events)
  const characters = snapshot?.state?.characters && typeof snapshot.state.characters === 'object'
    ? Object.entries(snapshot.state.characters as Record<string, unknown>)
    : []

  return (
    <aside className="flex h-full w-[300px] shrink-0 flex-col border-l border-[#30343b] bg-[#1c1e22] p-3">
      <div className="mb-3 flex h-7 items-center justify-between">
        <h2 className="text-sm font-semibold text-[#d7dbe2]">当前快照</h2>
        <Badge variant="outline" className="border-[#333842] bg-[#252831] text-[#858b96]">{snapshot?.branch_id || 'main'}</Badge>
      </div>
      <ScrollArea className="min-h-0 flex-1">
        <section className="mb-3 rounded-lg border border-[#333842] bg-[#141519] p-3">
          <div className="mb-2 text-xs font-medium text-[#80b7ff]">在场角色</div>
          <div className="flex flex-wrap gap-1.5 text-sm text-[#a8adb7]">
            {onStage.length ? onStage.map((name) => <Badge key={String(name)} variant="secondary">{String(name)}</Badge>) : '暂无在场角色'}
          </div>
        </section>
        <Separator className="mb-3 bg-[#30343b]" />
        <section className="mb-3 rounded-lg border border-[#333842] bg-[#141519] p-3">
          <div className="mb-2 text-xs font-medium text-[#80b7ff]">角色状态</div>
          <div className="space-y-2 text-xs text-[#a8adb7]">
          {characters.length ? characters.map(([name, state]) => (
            <pre key={name} className="whitespace-pre-wrap rounded-md border border-[#333842] bg-[#1c1e22] p-2">{name}: {JSON.stringify(state, null, 2)}</pre>
          )) : '暂无角色状态'}
          </div>
        </section>
        <Separator className="mb-3 bg-[#30343b]" />
        <section className="rounded-lg border border-[#333842] bg-[#141519] p-3">
          <div className="mb-2 text-xs font-medium text-[#80b7ff]">关键事件</div>
          <div className="space-y-2 text-xs text-[#a8adb7]">
            {events.length ? events.map((event, index) => <pre key={index} className="whitespace-pre-wrap rounded-md border border-[#333842] bg-[#1c1e22] p-2">{JSON.stringify(event, null, 2)}</pre>) : '暂无关键事件'}
          </div>
        </section>
      </ScrollArea>
    </aside>
  )
}
