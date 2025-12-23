describe('Cost Analytics', () => {
  beforeEach(() => {
    cy.login()
    cy.intercept('GET', '/api/costs*', {
      statusCode: 200,
      body: {
        labels: ['Dec 17', 'Dec 18', 'Dec 19', 'Dec 20', 'Dec 21', 'Dec 22', 'Dec 23'],
        datasets: [
          {
            label: 'Total',
            data: [1200, 1150, 1180, 1100, 1050, 1020, 980],
            borderColor: '#3b82f6',
          },
        ],
        total: 7680,
        change: -5.2,
      },
    }).as('getCosts')

    cy.intercept('GET', '/api/savings*', {
      statusCode: 200,
      body: {
        realized: 2500,
        projected: 1800,
        trend: [
          { date: 'Dec 17', realized: 2000, projected: 1500 },
          { date: 'Dec 20', realized: 2300, projected: 1700 },
          { date: 'Dec 23', realized: 2500, projected: 1800 },
        ],
      },
    }).as('getSavings')

    cy.intercept('GET', '/api/costs/namespaces*', {
      statusCode: 200,
      body: [
        { namespace: 'production', currentCost: 500, previousCost: 550, change: -9.1, containers: 25 },
        { namespace: 'staging', currentCost: 200, previousCost: 180, change: 11.1, containers: 12 },
      ],
    }).as('getNamespaceCosts')
  })

  it('displays cost analytics page', () => {
    cy.visit('/costs')
    cy.contains('Cost Analytics').should('be.visible')
  })

  it('shows savings summary', () => {
    cy.visit('/costs')
    cy.wait('@getSavings')
    cy.contains('Savings Summary').should('be.visible')
    cy.contains('Realized Savings').should('be.visible')
    cy.contains('Projected Savings').should('be.visible')
  })

  it('displays namespace cost table', () => {
    cy.visit('/costs')
    cy.wait('@getNamespaceCosts')
    cy.contains('production').should('be.visible')
    cy.contains('staging').should('be.visible')
  })
})
