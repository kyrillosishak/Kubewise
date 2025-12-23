export interface User {
  id: string
  email: string
  name: string
  avatar?: string
}

export interface Permission {
  namespace: string
  role: 'viewer' | 'editor' | 'admin'
}
