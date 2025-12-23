import axios, { AxiosError, type AxiosInstance, type AxiosRequestConfig } from 'axios'
import { authStorage } from '@/lib/authStorage'
import type { ApiError } from '@/types'

export class ApiClient {
  private client: AxiosInstance

  constructor(baseURL = '/api') {
    this.client = axios.create({
      baseURL,
      headers: {
        'Content-Type': 'application/json',
      },
      timeout: 30000,
    })

    this.setupInterceptors()
  }

  private setupInterceptors(): void {
    this.client.interceptors.request.use(
      (config) => {
        const token = authStorage.getToken()
        if (token) {
          config.headers.Authorization = `Bearer ${token}`
        }
        return config
      },
      (error) => Promise.reject(error)
    )

    this.client.interceptors.response.use(
      (response) => response,
      (error: AxiosError<ApiError>) => {
        if (error.response?.status === 401) {
          authStorage.clearToken()
          window.location.href = '/login'
          return Promise.reject(new ApiRequestError('Session expired', 401, 'UNAUTHORIZED'))
        }

        if (error.response?.status === 403) {
          return Promise.reject(
            new ApiRequestError('Access denied', 403, 'FORBIDDEN', error.response.data)
          )
        }

        if (error.response?.status === 404) {
          return Promise.reject(
            new ApiRequestError('Resource not found', 404, 'NOT_FOUND', error.response.data)
          )
        }

        if (error.response?.status && error.response.status >= 500) {
          return Promise.reject(
            new ApiRequestError('Server error', error.response.status, 'SERVER_ERROR')
          )
        }

        if (error.code === 'ECONNABORTED') {
          return Promise.reject(new ApiRequestError('Request timeout', 408, 'TIMEOUT'))
        }

        if (!error.response) {
          return Promise.reject(new ApiRequestError('Network error', 0, 'NETWORK_ERROR'))
        }

        return Promise.reject(
          new ApiRequestError(
            error.response.data?.error || 'Request failed',
            error.response.status,
            error.response.data?.code || 'UNKNOWN',
            error.response.data
          )
        )
      }
    )
  }


  async get<T>(url: string, config?: AxiosRequestConfig): Promise<T> {
    const response = await this.client.get<T>(url, config)
    return response.data
  }

  async post<T, D = unknown>(url: string, data?: D, config?: AxiosRequestConfig): Promise<T> {
    const response = await this.client.post<T>(url, data, config)
    return response.data
  }

  async put<T, D = unknown>(url: string, data?: D, config?: AxiosRequestConfig): Promise<T> {
    const response = await this.client.put<T>(url, data, config)
    return response.data
  }

  async delete<T>(url: string, config?: AxiosRequestConfig): Promise<T> {
    const response = await this.client.delete<T>(url, config)
    return response.data
  }

  getAxiosInstance(): AxiosInstance {
    return this.client
  }
}

export class ApiRequestError extends Error {
  status: number
  code: string
  details?: ApiError

  constructor(message: string, status: number, code: string, details?: ApiError) {
    super(message)
    this.name = 'ApiRequestError'
    this.status = status
    this.code = code
    this.details = details
  }

  isUnauthorized(): boolean {
    return this.status === 401
  }

  isForbidden(): boolean {
    return this.status === 403
  }

  isNotFound(): boolean {
    return this.status === 404
  }

  isServerError(): boolean {
    return this.status >= 500
  }

  isNetworkError(): boolean {
    return this.status === 0
  }
}

const apiClient = new ApiClient()

export { apiClient }
export default apiClient
