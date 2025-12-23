describe('Recommendations', () => {
  beforeEach(() => {
    cy.login()
    cy.intercept('GET', '/api/recommendations*', {
      statusCode: 200,
      body: [
        {
          id: 'rec-1',
          namespace: 'production',
          deployment: 'api-server',
          container: 'main',
          confidence: 0.92,
          estimatedSavings: 150.5,
          currentCpu: '500m',
          recommendedCpu: '250m',
          currentMemory: '512Mi',
          recommendedMemory: '256Mi',
          status: 'pending',
          createdAt: '2024-12-20T10:00:00Z',
          updatedAt: '2024-12-20T10:00:00Z',
        },
        {
          id: 'rec-2',
          namespace: 'staging',
          deployment: 'worker',
          container: 'processor',
          confidence: 0.85,
          estimatedSavings: 75.25,
          currentCpu: '1000m',
          recommendedCpu: '500m',
          currentMemory: '1Gi',
          recommendedMemory: '512Mi',
          status: 'approved',
          createdAt: '2024-12-19T10:00:00Z',
          updatedAt: '2024-12-19T10:00:00Z',
        },
      ],
    }).as('getRecommendations')
  })

  it('displays recommendations list', () => {
    cy.visit('/recommendations')
    cy.wait('@getRecommendations')
    cy.contains('api-server').should('be.visible')
    cy.contains('worker').should('be.visible')
  })

  it('shows recommendation count', () => {
    cy.visit('/recommendations')
    cy.wait('@getRecommendations')
    cy.contains('2 recommendations').should('be.visible')
  })

  it('displays filter controls', () => {
    cy.visit('/recommendations')
    cy.wait('@getRecommendations')
    cy.contains('Namespace').should('be.visible')
    cy.contains('Min Confidence').should('be.visible')
    cy.contains('Status').should('be.visible')
  })

  it('filters by namespace', () => {
    cy.visit('/recommendations')
    cy.wait('@getRecommendations')
    cy.get('select').first().select('production')
    cy.contains('api-server').should('be.visible')
  })

  it('navigates to recommendation detail on row click', () => {
    cy.intercept('GET', '/api/recommendations/rec-1', {
      statusCode: 200,
      body: {
        id: 'rec-1',
        namespace: 'production',
        deployment: 'api-server',
        container: 'main',
        confidence: 0.92,
        estimatedSavings: 150.5,
        currentCpu: '500m',
        recommendedCpu: '250m',
        currentMemory: '512Mi',
        recommendedMemory: '256Mi',
        status: 'pending',
        createdAt: '2024-12-20T10:00:00Z',
        updatedAt: '2024-12-20T10:00:00Z',
        history: [],
        reasoning: 'Based on 7 days of usage data',
      },
    }).as('getRecommendationDetail')

    cy.visit('/recommendations')
    cy.wait('@getRecommendations')
    cy.contains('api-server').click()
    cy.url().should('include', '/recommendations/rec-1')
  })
})
