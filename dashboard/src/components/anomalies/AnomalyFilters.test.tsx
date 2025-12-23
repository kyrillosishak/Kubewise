import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { AnomalyFilters } from './AnomalyFilters'
import type { AnomalyFilters as Filters } from '@/types/anomalies'

describe('AnomalyFilters', () => {
  const mockNamespaces = ['default', 'production', 'staging']
  const defaultFilters: Filters = {}

  it('renders all filter controls', () => {
    const onFilterChange = vi.fn()
    render(
      <AnomalyFilters
        filters={defaultFilters}
        namespaces={mockNamespaces}
        onFilterChange={onFilterChange}
      />
    )

    expect(screen.getByLabelText('Type')).toBeInTheDocument()
    expect(screen.getByLabelText('Severity')).toBeInTheDocument()
    expect(screen.getByLabelText('Namespace')).toBeInTheDocument()
    expect(screen.getByLabelText('From')).toBeInTheDocument()
    expect(screen.getByLabelText('To')).toBeInTheDocument()
  })

  it('calls onFilterChange when type is selected', async () => {
    const user = userEvent.setup()
    const onFilterChange = vi.fn()
    render(
      <AnomalyFilters
        filters={defaultFilters}
        namespaces={mockNamespaces}
        onFilterChange={onFilterChange}
      />
    )

    await user.selectOptions(screen.getByLabelText('Type'), 'memory_leak')
    expect(onFilterChange).toHaveBeenCalledWith({ type: 'memory_leak' })
  })

  it('calls onFilterChange when severity is selected', async () => {
    const user = userEvent.setup()
    const onFilterChange = vi.fn()
    render(
      <AnomalyFilters
        filters={defaultFilters}
        namespaces={mockNamespaces}
        onFilterChange={onFilterChange}
      />
    )

    await user.selectOptions(screen.getByLabelText('Severity'), 'critical')
    expect(onFilterChange).toHaveBeenCalledWith({ severity: 'critical' })
  })

  it('calls onFilterChange when namespace is selected', async () => {
    const user = userEvent.setup()
    const onFilterChange = vi.fn()
    render(
      <AnomalyFilters
        filters={defaultFilters}
        namespaces={mockNamespaces}
        onFilterChange={onFilterChange}
      />
    )

    await user.selectOptions(screen.getByLabelText('Namespace'), 'production')
    expect(onFilterChange).toHaveBeenCalledWith({ namespace: 'production' })
  })

  it('shows clear filters button when filters are active', () => {
    const onFilterChange = vi.fn()
    render(
      <AnomalyFilters
        filters={{ type: 'cpu_spike' }}
        namespaces={mockNamespaces}
        onFilterChange={onFilterChange}
      />
    )

    expect(screen.getByText('Clear filters')).toBeInTheDocument()
  })

  it('hides clear filters button when no filters are active', () => {
    const onFilterChange = vi.fn()
    render(
      <AnomalyFilters
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
      <AnomalyFilters
        filters={{ type: 'cpu_spike', severity: 'warning' }}
        namespaces={mockNamespaces}
        onFilterChange={onFilterChange}
      />
    )

    await user.click(screen.getByText('Clear filters'))
    expect(onFilterChange).toHaveBeenCalledWith({})
  })
})
