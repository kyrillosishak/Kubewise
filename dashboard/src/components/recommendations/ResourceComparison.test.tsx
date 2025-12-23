import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import { ResourceComparison } from './ResourceComparison'

describe('ResourceComparison', () => {
  it('renders resource comparison with CPU and memory values', () => {
    render(
      <ResourceComparison
        currentCpu="500m"
        recommendedCpu="250m"
        currentMemory="512Mi"
        recommendedMemory="256Mi"
      />
    )

    expect(screen.getByText('Resource Comparison')).toBeInTheDocument()
    expect(screen.getByText('CPU Request')).toBeInTheDocument()
    expect(screen.getByText('Memory Request')).toBeInTheDocument()
  })

  it('displays current and recommended CPU values', () => {
    render(
      <ResourceComparison
        currentCpu="500m"
        recommendedCpu="250m"
        currentMemory="512Mi"
        recommendedMemory="256Mi"
      />
    )

    expect(screen.getByText('500m')).toBeInTheDocument()
    expect(screen.getByText('250m')).toBeInTheDocument()
  })

  it('displays current and recommended memory values', () => {
    render(
      <ResourceComparison
        currentCpu="500m"
        recommendedCpu="250m"
        currentMemory="512Mi"
        recommendedMemory="256Mi"
      />
    )

    expect(screen.getByText('512Mi')).toBeInTheDocument()
    expect(screen.getByText('256Mi')).toBeInTheDocument()
  })

  it('shows negative percentage for resource reduction', () => {
    render(
      <ResourceComparison
        currentCpu="1000m"
        recommendedCpu="500m"
        currentMemory="1Gi"
        recommendedMemory="512Mi"
      />
    )

    const percentages = screen.getAllByText('-50%')
    expect(percentages.length).toBeGreaterThanOrEqual(1)
  })

  it('shows summary section with CPU and memory changes', () => {
    render(
      <ResourceComparison
        currentCpu="500m"
        recommendedCpu="250m"
        currentMemory="512Mi"
        recommendedMemory="256Mi"
      />
    )

    expect(screen.getByText('Summary')).toBeInTheDocument()
    expect(screen.getByText('CPU Change')).toBeInTheDocument()
    expect(screen.getByText('Memory Change')).toBeInTheDocument()
  })

  it('renders legend with current and recommended labels', () => {
    render(
      <ResourceComparison
        currentCpu="500m"
        recommendedCpu="250m"
        currentMemory="512Mi"
        recommendedMemory="256Mi"
      />
    )

    const currentLabels = screen.getAllByText('Current')
    const recommendedLabels = screen.getAllByText('Recommended')
    expect(currentLabels.length).toBeGreaterThanOrEqual(1)
    expect(recommendedLabels.length).toBeGreaterThanOrEqual(1)
  })
})
