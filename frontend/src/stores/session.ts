import { reactive } from 'vue'
import { api, getToken, setToken } from '@/api'
import type { AppConfig, IsmptSession, Organization, User } from '@/api'
import { getSigner, utf8ToBase64 } from '@/services/signer'

interface SessionState {
  config: AppConfig | null
  user: User | null
  organization: Organization | null
  ismpt: IsmptSession | null
  loaded: boolean
  connecting: boolean
}

export const session = reactive<SessionState>({
  config: null,
  user: null,
  organization: null,
  ismpt: null,
  loaded: false,
  connecting: false,
})

export function isAuthenticated(): boolean {
  return !!getToken()
}

export async function loadConfig(): Promise<AppConfig | null> {
  if (session.config) return session.config
  try {
    session.config = await api.config()
  } catch {
    session.config = null
  }
  return session.config
}

export async function loadSession(force = false): Promise<void> {
  if (session.loaded && !force) return
  const [me] = await Promise.all([api.me(), loadConfig()])
  session.user = me.user
  session.organization = me.organization
  session.loaded = true
  try {
    session.ismpt = await api.ismptSession()
  } catch {
    session.ismpt = null
  }
}

export async function login(email: string, password: string): Promise<void> {
  const res = await api.login(email, password)
  setToken(res.token)
  session.user = res.user
  session.loaded = false
  await loadSession(true)
}

export function clearSession(): void {
  setToken(null)
  session.user = null
  session.organization = null
  session.ismpt = null
  session.loaded = false
}

export async function logout(): Promise<void> {
  try {
    await api.logout()
  } catch {
    void 0
  }
  clearSession()
}

export function setIsmpt(value: IsmptSession | null): void {
  session.ismpt = value
}

export async function connectIsmpt(): Promise<IsmptSession> {
  const config = await loadConfig()
  const signer = getSigner(config?.sign_mode ?? 'ncalayer')
  session.connecting = true
  try {
    const key = await api.ismptAuthKey()
    const signature = await signer.signCms(utf8ToBase64(key.data), { attached: true })
    const result = await api.ismptSignIn(key.uuid, signature)
    session.ismpt = result
    return result
  } finally {
    session.connecting = false
  }
}

export function isIsmptConnected(): boolean {
  const s = session.ismpt
  if (!s?.connected) return false
  if (!s.expires_at) return true
  return new Date(s.expires_at).getTime() > Date.now()
}
