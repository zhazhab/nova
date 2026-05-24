import { GitBranch } from 'lucide-react'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { ScrollArea } from '@/components/ui/scroll-area'
import type { BranchSummary, Snapshot } from '../types'

interface BranchTimelineProps {
  snapshot: Snapshot | null
  branches: BranchSummary[]
  currentBranchId: string
  onSwitchBranch: (branchId: string) => void
  onCreateBranch: (turnId: string) => void
}

export function BranchTimeline({ snapshot, branches, currentBranchId, onSwitchBranch, onCreateBranch }: BranchTimelineProps) {
  return (
    <div className="h-[86px] border-t border-[#30343b] bg-[#16181c] px-4 py-3">
      <div className="mb-2 flex items-center justify-between text-xs text-[#858b96]">
        <span className="flex items-center gap-1.5 font-medium text-[#7f8898]">
          <GitBranch className="h-3.5 w-3.5 text-[#80b7ff]" />
          剧情时间线 / 分支树
        </span>
        <div className="flex gap-2">
          {branches.map((branch) => (
            <Button key={branch.id} variant={branch.id === currentBranchId ? 'default' : 'outline'} size="xs" className={branch.id === currentBranchId ? '' : 'border-[#333842] bg-[#20242b] text-[#aab2c0] hover:bg-[#252831]'} onClick={() => onSwitchBranch(branch.id)}>
              {branch.title || branch.id}
            </Button>
          ))}
        </div>
      </div>
      <ScrollArea className="w-full">
        <div className="flex items-center gap-2 pb-1">
          {(snapshot?.turns || []).map((turn, index) => (
            <button key={turn.id} className="group flex items-center gap-2" onClick={() => onCreateBranch(turn.id)} title="从此分叉">
              {index > 0 && <span className="h-px w-12 bg-[#3a465a]" />}
              <Badge className="h-5 w-5 rounded-full bg-[#2f7dd3] p-0 text-[10px] group-hover:ring-2 group-hover:ring-[#2f7dd3]/40">{index + 1}</Badge>
            </button>
          ))}
          {!snapshot?.turns?.length && <span className="text-xs text-[#858b96]">还没有回合，输入第一句话开始。</span>}
        </div>
      </ScrollArea>
    </div>
  )
}
