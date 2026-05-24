import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { InteractiveLayout } from './InteractiveLayout'

describe('InteractiveLayout', () => {
  it('renders story stage and snapshot panels', async () => {
    const { container } = render(<InteractiveLayout />)

    expect(await screen.findByText('故事舞台 · 当前分支 main')).toBeInTheDocument()
    expect(screen.getByText('当前快照')).toBeInTheDocument()
    expect(container.querySelector('[data-slot="select-trigger"]')).toBeInTheDocument()
    expect(container.querySelector('[data-slot="button"]')).toBeInTheDocument()
    expect(container.querySelector('[data-slot="tabs-list"]')).toBeInTheDocument()
    expect(screen.getByTestId('interactive-shell')).toHaveClass('rounded-xl')
    expect(screen.getByTestId('story-stage-card')).toHaveClass('rounded-xl')
  })
})
