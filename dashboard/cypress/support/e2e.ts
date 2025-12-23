// Cypress E2E support file

// Custom commands can be added here
declare global {
  namespace Cypress {
    interface Chainable {
      login(): Chainable<void>
    }
  }
}

// Mock login command for testing authenticated routes
Cypress.Commands.add('login', () => {
  window.localStorage.setItem(
    'kubewise_auth',
    JSON.stringify({
      token: 'test-token',
      user: { id: '1', email: 'test@example.com', name: 'Test User' },
      expiresAt: Date.now() + 3600000,
    })
  )
})

// Prevent uncaught exceptions from failing tests
Cypress.on('uncaught:exception', () => {
  return false
})
