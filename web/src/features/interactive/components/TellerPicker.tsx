import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import type { StorySummary, Teller } from '../types'

interface TellerPickerProps {
  story?: StorySummary
  tellers: Teller[]
  onChange: (tellerId: string) => void
}

export function TellerPicker({ story, tellers, onChange }: TellerPickerProps) {
  return (
    <div className="flex items-center gap-1.5">
      <span className="shrink-0 text-[11px] font-medium text-[#858b96]">讲述者</span>
      <Select value={story?.story_teller_id || '__none'} disabled={!story} onValueChange={(value) => { if (value !== '__none') onChange(value) }}>
        <SelectTrigger
          size="sm"
          className="w-[170px] border-[#303238] bg-[#25262a] px-2 py-0.5 text-xs text-[#d7dbe2] hover:bg-[#303238] focus:ring-0"
          aria-label="选择讲述者"
        >
          <SelectValue placeholder="选择讲述者" />
        </SelectTrigger>
        <SelectContent className="border-[#303238] bg-[#25262a] text-[#d7dbe2]">
          <SelectItem value="__none" disabled>选择讲述者</SelectItem>
          {tellers.map((teller) => (
            <SelectItem key={teller.id} value={teller.id}>{teller.name}</SelectItem>
          ))}
        </SelectContent>
      </Select>
    </div>
  )
}
