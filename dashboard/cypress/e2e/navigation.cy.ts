describe('Navigation', () => {
  beforeEach(() => {
    cy.login()
  })

  it('redirects unauthenticated users to login', () => {
    cy.clearLocalStorage()
    cy.visit('/')
    cy.url().should('include', '/login')
  })

  it('displays login page with form elements', () => {
    cy.clearLocalStorage()
    cy.visit('/login')
    cy.contains('Kubewise Dashboard').should('be.visible')
    cy.get('input[type="email"]').should('be.visible')
    cy.get('input[type="password"]').should('be.visible')
    cy.get('button[type="submit"]').contains('Sign In').should('be.visible')
  })

  it('navigates to dashboard after login', () => {
    cy.visit('/')
    cy.url().should('eq', Cypress.config().baseUrl + '/')
  })

  it('navigates to recommendations page', () => {
    cy.visit('/recommendations')
    cy.contains('Recommendations').should('be.visible')
  })

  it('navigates to costs page', () => {
    cy.visit('/costs')
    cy.contains('Cost Analytics').should('be.visible')
  })

  it('navigates to anomalies page', () => {
    cy.visit('/anomalies')
    cy.contains('Anomalies').should('be.visible')
  })

  it('navigates to clusters page', () => {
    cy.visit('/clusters')
    cy.contains('Cluster').should('be.visible')
  })
})
