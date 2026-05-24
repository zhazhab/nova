import { create } from 'zustand'
import type { BranchSummary, InteractiveSubmode, Snapshot, StorySummary, Teller } from '../types'

interface InteractiveStore {
  stories: StorySummary[]
  tellers: Teller[]
  branches: BranchSummary[]
  snapshot: Snapshot | null
  currentStoryId: string
  currentBranchId: string
  submode: InteractiveSubmode
  setStories: (stories: StorySummary[], currentStoryId?: string) => void
  setTellers: (tellers: Teller[]) => void
  setBranches: (branches: BranchSummary[]) => void
  setSnapshot: (snapshot: Snapshot | null) => void
  setCurrentStoryId: (storyId: string) => void
  setCurrentBranchId: (branchId: string) => void
  setSubmode: (mode: InteractiveSubmode) => void
}

export const useInteractiveStore = create<InteractiveStore>((set) => ({
  stories: [],
  tellers: [],
  branches: [],
  snapshot: null,
  currentStoryId: '',
  currentBranchId: 'main',
  submode: 'story',
  setStories: (stories, currentStoryId) => set((state) => ({
    stories,
    currentStoryId: currentStoryId || state.currentStoryId || stories[0]?.id || '',
  })),
  setTellers: (tellers) => set({ tellers }),
  setBranches: (branches) => set((state) => ({
    branches,
    currentBranchId: branches.find(branch => branch.current)?.id || state.currentBranchId || 'main',
  })),
  setSnapshot: (snapshot) => set((state) => ({
    snapshot,
    currentBranchId: snapshot?.branch_id || state.currentBranchId,
  })),
  setCurrentStoryId: (storyId) => set({ currentStoryId: storyId, currentBranchId: 'main', snapshot: null, branches: [] }),
  setCurrentBranchId: (branchId) => set({ currentBranchId: branchId }),
  setSubmode: (submode) => set({ submode }),
}))
