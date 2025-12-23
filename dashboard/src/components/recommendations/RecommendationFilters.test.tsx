import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { RecommendationFilters } from './RecommendationFilters'
import type { RecommendationFilters as Filters } from '@/types/recommendations'

describe('RecommendationFilters', () => {
  const mockNamespaces = ['default', 'kube-system', 'monitoring']
  const defaultFilters: Filters = {}

  it('renders all filter controls', () => {
    const onFilterChange = vi.fn()
    render(
      <RecommendationFilters
        filters={defaultFilters}
        namespaces={mockNamespaces}
        onFilterChange={onFilterChange}
      />
    )

    expect(screen.getByLabelText('Namespace')).toBeInTheDocument()
    expect(screen.getByLabelText('Min Confidence')).toBeInTheDocument()
    expect(screen.getByLabelText('Status')).toBeInTheDocument()
  })

  it('renders namespace options', () => {
    const onFilterChange = vi.fn()
    render(
      <RecommendationFilters
        filters={defaultFilters}
        namespaces={mockNamespaces}
        onFilterChange={onFilterChange}
      />
    )

    const namespaceSelect = screen.getByLabelText('Namespace')
    expect(namespaceSelect).toContainHTML('All namespaces')
    mockNamespaces.forEach((ns) => {
      expect(namespaceSelect).toContainHTML(ns)
    })
  })

  it('calls onFilterChange when namespace is selected', async () => {
    const user = userEvent.setup()
    const onFilterChange = vi.fn()
    render(
      <RecommendationFilters
        filters={defaultFilters}
        namespaces={mockNamespaces}
        onFilterChange={onFilterChange}
      />
    )

    await user.selectOptions(screen.getByLabelText('Namespace'), 'monitoring')
    expect(onFilterChange).toHaveBeenCalledWith({ namespace: 'monitoring' })
  })

  it('calls onFilterChange when confidence is selected', async () => {
    const user = userEvent.setup()
    const onFilterChange = vi.fn()
    render(
      <RecommendationFilters
        filters={defaultFilters}
        namespaces={mockNamespaces}
        onFilterChange={onFilterChange}
      />
    )

    await user.selectOptions(screen.getByLabelText('Min Confidence'), '80')
    expect(onFilterChange).toHaveBeenCalledWith({ confidence: 80 })
  })

  it('calls onFilterChange when status is selected', async () => {
    const user = userEvent.setup()
    const onFilterChange = vi.fn()
    render(
      <RecommendationFilters
        filters={defaultFilters}
        namespaces={mockNamespaces}
        onFilterChange={onFilterChange}
      />
    )

    await user.selectOptions(screen.getByLabelText('Status'), 'pending')
    expect(onFilterChange).toHaveBeenCalledWith({ status: 'pending' })
  })

  it('shows clear filters button when filters are active', () => {
    const onFilterChange = vi.fn()
    render(
      <RecommendationFilters
        filters={{ namespace: 'default' }}
        namespaces={mockNamespaces}
        onFilterChange={onFilterChange}
      />
    )

    expect(screen.getByText('Clear filters')).toBeInTheDocument()
  })

  it('hides clear filters button when no filters are active', () => {
    const onFilterChange = vi.fn()
    render(
      <RecommendationFilters
        filters={defaultFilters}
        namespaces={mockNamespaces}
        onFilterChange={onFilterChange}
      />
    )

    expect(screen.queryByText('Clear filters')).not.toBeInTheDocument()
  })

  it('clears all filters when clear button is clicked', async () => {
    const user = userEvent.setup()
    const onFilterChange = vi.fn()
    render(
      <RecommendationFilters
        filters={{ namespace: 'default', confidence: 80 }}
        namespaces={mockNamespaces}
        onFilterChange={onFilterChange}
      />
    )

    await user.click(screen.getByText('Clear filters'))
    expect(onFilterChange).toHaveBeenCalledWith({})
  })
})
