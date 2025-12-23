export interface OIDCConfig {
  authority: string
  clientId: string
  redirectUri: string
  postLogoutRedirectUri: string
  scope: string
  responseType: string
}

const defaultConfig: OIDCConfig = {
  authority: import.meta.env.VITE_OIDC_AUTHORITY || '',
  clientId: import.meta.env.VITE_OIDC_CLIENT_ID || '',
  redirectUri: `${window.location.origin}/auth/callback`,
  postLogoutRedirectUri: `${window.location.origin}/login`,
  scope: 'openid profile email',
  responseType: 'code',
}

function generateCodeVerifier(): string {
  const array = new Uint8Array(32)
  crypto.getRandomValues(array)
  return base64UrlEncode(array)
}

async function generateCodeChallenge(verifier: string): Promise<string> {
  const encoder = new TextEncoder()
  const data = encoder.encode(verifier)
  const hash = await crypto.subtle.digest('SHA-256', data)
  return base64UrlEncode(new Uint8Array(hash))
}

function base64UrlEncode(buffer: Uint8Array): string {
  const base64 = btoa(String.fromCharCode(...buffer))
  return base64.replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '')
}

function generateState(): string {
  const array = new Uint8Array(16)
  crypto.getRandomValues(array)
  return base64UrlEncode(array)
}

function generateNonce(): string {
  const array = new Uint8Array(16)
  crypto.getRandomValues(array)
  return base64UrlEncode(array)
}

export const oidcClient = {
  config: defaultConfig,

  setConfig(config: Partial<OIDCConfig>): void {
    Object.assign(this.config, config)
  },

  async startLogin(returnTo?: string): Promise<void> {
    const state = generateState()
    const nonce = generateNonce()
    const codeVerifier = generateCodeVerifier()
    const codeChallenge = await generateCodeChallenge(codeVerifier)

    sessionStorage.setItem('oidc_state', state)
    sessionStorage.setItem('oidc_nonce', nonce)
    sessionStorage.setItem('oidc_code_verifier', codeVerifier)
    if (returnTo) {
      sessionStorage.setItem('oidc_return_to', returnTo)
    }

    const params = new URLSearchParams({
      client_id: this.config.clientId,
      redirect_uri: this.config.redirectUri,
      response_type: this.config.responseType,
      scope: this.config.scope,
      state,
      nonce,
      code_challenge: codeChallenge,
      code_challenge_method: 'S256',
    })

    window.location.href = `${this.config.authority}/authorize?${params.toString()}`
  },

  async handleCallback(searchParams: URLSearchParams): Promise<{ code: string; codeVerifier: string; nonce: string; returnTo: string | null }> {
    const code = searchParams.get('code')
    const state = searchParams.get('state')
    const error = searchParams.get('error')
    const errorDescription = searchParams.get('error_description')

    if (error) {
      throw new Error(errorDescription || error)
    }

    if (!code) {
      throw new Error('No authorization code received')
    }

    const storedState = sessionStorage.getItem('oidc_state')
    if (state !== storedState) {
      throw new Error('Invalid state parameter')
    }

    const codeVerifier = sessionStorage.getItem('oidc_code_verifier')
    if (!codeVerifier) {
      throw new Error('No code verifier found')
    }

    const nonce = sessionStorage.getItem('oidc_nonce')
    if (!nonce) {
      throw new Error('No nonce found')
    }

    const returnTo = sessionStorage.getItem('oidc_return_to')

    sessionStorage.removeItem('oidc_state')
    sessionStorage.removeItem('oidc_nonce')
    sessionStorage.removeItem('oidc_code_verifier')
    sessionStorage.removeItem('oidc_return_to')

    return { code, codeVerifier, nonce, returnTo }
  },

  startLogout(idTokenHint?: string): void {
    const params = new URLSearchParams({
      client_id: this.config.clientId,
      post_logout_redirect_uri: this.config.postLogoutRedirectUri,
    })

    if (idTokenHint) {
      params.set('id_token_hint', idTokenHint)
    }

    window.location.href = `${this.config.authority}/logout?${params.toString()}`
  },

  isConfigured(): boolean {
    return !!this.config.authority && !!this.config.clientId
  },
}
