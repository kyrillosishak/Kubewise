const TOKEN_KEY = 'auth_token'
const EXPIRY_KEY = 'auth_token_expiry'

export const authStorage = {
  setToken(token: string, expiresIn: number): void {
    const expiryTime = Date.now() + expiresIn * 1000
    localStorage.setItem(TOKEN_KEY, token)
    localStorage.setItem(EXPIRY_KEY, expiryTime.toString())
  },

  getToken(): string | null {
    const token = localStorage.getItem(TOKEN_KEY)
    const expiry = localStorage.getItem(EXPIRY_KEY)

    if (!token || !expiry) {
      return null
    }

    if (Date.now() > parseInt(expiry, 10)) {
      this.clearToken()
      return null
    }

    return token
  },

  clearToken(): void {
    localStorage.removeItem(TOKEN_KEY)
    localStorage.removeItem(EXPIRY_KEY)
  },

  isTokenExpired(): boolean {
    const expiry = localStorage.getItem(EXPIRY_KEY)
    if (!expiry) return true
    return Date.now() > parseInt(expiry, 10)
  },

  getTimeUntilExpiry(): number | null {
    const expiry = localStorage.getItem(EXPIRY_KEY)
    if (!expiry) return null
    const remaining = parseInt(expiry, 10) - Date.now()
    return remaining > 0 ? remaining : null
  },
}
