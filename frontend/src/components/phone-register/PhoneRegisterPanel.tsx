import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import type { ReactNode } from 'react'

type RegisterUser = {
  id: number
  username: string
  group_id: number
  is_duck: boolean
  otp_email: string
}

type UserSummary = {
  email_total: number
  email_unused: number
  email_used: number
  account_total: number
  account_queued: number
  account_running: number
  account_success: number
  account_failed: number
  account_registered: number
}

type LoginSummary = {
  queued: number
  running: number
  success: number
  failed: number
  total: number
}

type UserRunLog = {
  time: string
  level: string
  message: string
}

type UserRun = {
  run_id: string
  user_id: number
  username: string
  target_count: number
  status: string
  step: string
  error: string
  phone_success_count: number
  phone_failure_count: number
  login_queued_count: number
  login_started_count: number
  login_success_count: number
  login_failed_count: number
  current_session_id: string
  current_phone: string
  current_account_id: number
  current_login_account_id: number
  phone_done: boolean
  stop_requested: boolean
  logs: UserRunLog[]
  created_at: string
  updated_at: string
}

type UserAccount = {
  id: number
  user_id: number
  phone: string
  email: string
  password: string
  status: string
  error: string
  created_at: string
  updated_at: string
}

type UserDashboard = {
  user: RegisterUser
  summary: UserSummary
  login_summary?: LoginSummary
  run?: UserRun
  latest_accounts: UserAccount[]
}

type UserEmailListItem = {
  id: number
  user_id: number
  email: string
  provider: string
  used_at?: string
  account_id?: number
  phone: string
  account_status: string
  account_error?: string
  sub2api_uploaded: boolean
  created_at: string
  updated_at: string
}

type UserEmailListResponse = {
  items: UserEmailListItem[]
  page: number
  page_size: number
  total: number
  total_pages: number
}

type EmailGenerateResult = {
  created: number
  emails: string[]
  errors: string[]
  summary: UserSummary
}

type EmailRefreshSnapshot = {
  phoneSuccess: number
  loginSuccess: number
}

const AUTHORIZATION_STORAGE = 'sub2api-panel:phone-register-authorization'
const USERNAME_STORAGE = 'sub2api-panel:phone-register-username'
const HERO_KEY_STORAGE = 'sub2api-panel:herosms-api-key'
const DUCK_AUTH_STORAGE = 'sub2api-panel:duck-authorization'
const REGISTER_COUNT_STORAGE = 'sub2api-panel:user-register-count'
const EMAIL_COUNT_STORAGE = 'sub2api-panel:user-email-count'
const EMAIL_PAGE_SIZE = 10

const emptyEmailList: UserEmailListResponse = {
  items: [],
  page: 1,
  page_size: EMAIL_PAGE_SIZE,
  total: 0,
  total_pages: 0,
}

const statusTone: Record<string, string> = {
  idle: 'bg-warmgray-50 text-warmgray-600 ring-warmgray-200',
  success: 'bg-emerald-50 text-emerald-700 ring-emerald-200',
  failed: 'bg-rose-50 text-rose-700 ring-rose-200',
  stopped: 'bg-warmgray-100 text-warmgray-600 ring-warmgray-200',
  running: 'bg-amber-50 text-amber-700 ring-amber-200',
  phone_code_sent: 'bg-amber-50 text-amber-700 ring-amber-200',
  codex_email_required: 'bg-blue-50 text-blue-700 ring-blue-200',
  email_code_sent: 'bg-blue-50 text-blue-700 ring-blue-200',
  queued_login: 'bg-blue-50 text-blue-700 ring-blue-200',
  login_running: 'bg-coral-50 text-coral-700 ring-coral-200',
  uploading: 'bg-coral-50 text-coral-700 ring-coral-200',
  registered: 'bg-warmgray-50 text-warmgray-700 ring-warmgray-200',
  unused: 'bg-warmgray-50 text-warmgray-500 ring-warmgray-200',
  used: 'bg-warmgray-100 text-warmgray-700 ring-warmgray-200',
}

async function requestJSON<T>(url: string, options: RequestInit = {}): Promise<T> {
  const headers = new Headers(options.headers)
  headers.set('Content-Type', 'application/json')
  const resp = await fetch(url, { ...options, headers })
  const text = await resp.text()
  let body: unknown = {}
  if (text) {
    try {
      body = JSON.parse(text)
    } catch {
      body = { error: text }
    }
  }
  if (!resp.ok) {
    const detail =
      typeof body === 'object' && body && 'error' in body
        ? String((body as { error?: unknown }).error)
        : `请求失败: ${resp.status}`
    throw new Error(detail)
  }
  return body as T
}

function toneForStatus(status: string) {
  return statusTone[status] || 'bg-warmgray-50 text-warmgray-700 ring-warmgray-200'
}

function labelForStatus(status: string) {
  const labels: Record<string, string> = {
    idle: '空闲',
    registered: '待登录',
    queued_login: '登录排队',
    login_running: '登录中',
    uploading: '上传中',
    success: '已完成',
    failed: '失败',
    stopped: '已停止',
    running: '运行中',
    phone_code_sent: '等短信',
    codex_email_required: '等邮箱',
    email_code_sent: '等邮箱验证码',
    unused: '未使用',
    used: '已占用',
  }
  return labels[status] || status || '-'
}

function dotForLevel(level: string) {
  if (level === 'error') return 'bg-rose-500'
  if (level === 'ok') return 'bg-emerald-500'
  if (level === 'warn') return 'bg-amber-500'
  return 'bg-coral-400'
}

function runIsActive(run?: UserRun) {
  return (
    run?.status === 'running' ||
    run?.status === 'phone_code_sent' ||
    run?.status === 'codex_email_required' ||
    run?.status === 'email_code_sent'
  )
}

function dashboardNeedsRefresh(dashboard: UserDashboard) {
  return runIsActive(dashboard.run)
}

function formatTime(value: string) {
  if (!value) return '-'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return '-'
  return date.toLocaleTimeString('zh-CN', { hour12: false })
}

function formatDateTime(value?: string) {
  if (!value) return '-'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return '-'
  return date.toLocaleString('zh-CN', {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    hour12: false,
  })
}

function clampCount(value: string, max = 100) {
  return Math.max(1, Math.min(Number.parseInt(value, 10) || 1, max))
}

function userStorageKey(base: string, user?: RegisterUser) {
  return user ? `${base}:${user.username}` : base
}

function percent(done: number, total: number) {
  if (!total) return 0
  return Math.min(100, Math.round((done / total) * 100))
}

function isPhoneStageText(value: string) {
  return /手机号|HeroSMS|短信|取号|激活|更换新手机号|当前模板|自动注册|注册任务/.test(value)
}

function isLoginStageText(value: string) {
  return /账号\s*#|登录|上传|登录邮箱验证码/.test(value)
}

function emailRefreshSnapshot(dashboard: UserDashboard): EmailRefreshSnapshot {
  return {
    phoneSuccess: dashboard.run?.phone_success_count ?? 0,
    loginSuccess: Math.max(
      dashboard.run?.login_success_count ?? 0,
      dashboard.login_summary?.success ?? 0,
    ),
  }
}

function emailRefreshNeeded(previous: EmailRefreshSnapshot, next: EmailRefreshSnapshot) {
  return next.phoneSuccess > previous.phoneSuccess || next.loginSuccess > previous.loginSuccess
}

function buildEmailURL(userID: number, page: number, query: string) {
  const params = new URLSearchParams({
    page: String(page),
    page_size: String(EMAIL_PAGE_SIZE),
  })
  const q = query.trim()
  if (q) {
    params.set('q', q)
  }
  return `/api/phone-register/users/${encodeURIComponent(userID)}/emails?${params.toString()}`
}

export function PhoneRegisterPanel() {
  const [username, setUsername] = useState(
    () => localStorage.getItem(AUTHORIZATION_STORAGE) || localStorage.getItem(USERNAME_STORAGE) || '',
  )
  const [apiKey, setApiKey] = useState(() => localStorage.getItem(HERO_KEY_STORAGE) || '')
  const [duckAuth, setDuckAuth] = useState(() => localStorage.getItem(DUCK_AUTH_STORAGE) || '')
  const [registerCount, setRegisterCount] = useState(
    () => localStorage.getItem(REGISTER_COUNT_STORAGE) || '1',
  )
  const [emailCount, setEmailCount] = useState(() => localStorage.getItem(EMAIL_COUNT_STORAGE) || '1')
  const [dashboard, setDashboard] = useState<UserDashboard | null>(null)
  const [emailList, setEmailList] = useState<UserEmailListResponse>(emptyEmailList)
  const [emailPage, setEmailPage] = useState(1)
  const [emailSearch, setEmailSearch] = useState('')
  const [emailQuery, setEmailQuery] = useState('')
  const [otpEmail, setOtpEmail] = useState('')
  const [emailLoading, setEmailLoading] = useState(false)
  const [message, setMessage] = useState('请输入 username 进入注册控制台')
  const [messageType, setMessageType] = useState<'info' | 'error' | 'ok'>('info')
  const [loading, setLoading] = useState(false)
  const timer = useRef<number | null>(null)
  const emailPageRef = useRef(1)
  const emailQueryRef = useRef('')
  const emailRefreshSnapshotRef = useRef<EmailRefreshSnapshot>({ phoneSuccess: 0, loginSuccess: 0 })
  const autoLoginRef = useRef(false)

  const user = dashboard?.user
  const run = dashboard?.run
  const summary = dashboard?.summary
  const isRunning = runIsActive(run)
  const userInfoDirty = otpEmail.trim() !== (user?.otp_email || '').trim()
  const phoneProcessed = (run?.phone_success_count ?? 0) + (run?.phone_failure_count ?? 0)
  const phoneProgress = percent(phoneProcessed, run?.target_count ?? 0)
  const currentLoginDone = (run?.login_success_count ?? 0) + (run?.login_failed_count ?? 0)
  const currentLoginTotal = Math.max(run?.phone_success_count ?? 0, currentLoginDone)
  const currentLoginRunning = run?.current_login_account_id ? 1 : 0
  const currentLoginQueued = Math.max(0, currentLoginTotal - currentLoginDone - currentLoginRunning)
  const currentLoginProgress = percent(currentLoginDone, currentLoginTotal)
  const currentLoginStatus =
    currentLoginRunning > 0
      ? '处理中'
      : currentLoginQueued > 0
        ? '排队中'
        : currentLoginTotal > 0 && currentLoginDone >= currentLoginTotal
          ? '已完成'
          : '空闲'
  const currentLoginTone =
    currentLoginRunning > 0
      ? 'bg-coral-50 text-coral-700 ring-coral-200'
      : currentLoginQueued > 0
        ? 'bg-blue-50 text-blue-700 ring-blue-200'
        : currentLoginTotal > 0 && currentLoginDone >= currentLoginTotal
          ? 'bg-emerald-50 text-emerald-700 ring-emerald-200'
        : 'bg-warmgray-50 text-warmgray-600 ring-warmgray-200'
  const phoneLogs = useMemo(
    () => (run?.logs ?? []).filter((log) => isPhoneStageText(log.message)),
    [run?.logs],
  )
  const loginLogs = useMemo(
    () => (run?.logs ?? []).filter((log) => isLoginStageText(log.message)),
    [run?.logs],
  )

  const clearTimer = useCallback(() => {
    if (timer.current) {
      window.clearInterval(timer.current)
      timer.current = null
    }
  }, [])

  const renderError = useCallback((err: unknown) => {
    setMessage(err instanceof Error ? err.message : String(err))
    setMessageType('error')
  }, [])

  const refreshEmails = useCallback(async (userID: number, page = emailPageRef.current, query = emailQueryRef.current) => {
    setEmailLoading(true)
    try {
      const data = await requestJSON<UserEmailListResponse>(buildEmailURL(userID, page, query))
      setEmailList(data)
      setEmailPage(data.page)
      emailPageRef.current = data.page
      return data
    } finally {
      setEmailLoading(false)
    }
  }, [])

  const refreshDashboard = useCallback(
    async (userID: number, keepTimer = true, refreshEmailOnSuccess = false) => {
      const data = await requestJSON<UserDashboard>(
        `/api/phone-register/users/${encodeURIComponent(userID)}/dashboard`,
      )
      setDashboard(data)
      const nextSnapshot = emailRefreshSnapshot(data)
      if (refreshEmailOnSuccess && emailRefreshNeeded(emailRefreshSnapshotRef.current, nextSnapshot)) {
        emailRefreshSnapshotRef.current = nextSnapshot
        void refreshEmails(userID).catch(() => undefined)
      } else {
        emailRefreshSnapshotRef.current = nextSnapshot
      }
      if (!dashboardNeedsRefresh(data) && !keepTimer) {
        clearTimer()
      }
      return data
    },
    [clearTimer, refreshEmails],
  )

  const scheduleRefresh = useCallback(
    (userID: number) => {
      clearTimer()
      timer.current = window.setInterval(() => {
        void refreshDashboard(userID, false, true).catch(renderError)
      }, 2500)
    },
    [clearTimer, refreshDashboard, renderError],
  )

  useEffect(() => {
    return clearTimer
  }, [clearTimer])

  useEffect(() => {
    emailPageRef.current = emailPage
  }, [emailPage])

  useEffect(() => {
    emailQueryRef.current = emailQuery
  }, [emailQuery])

  useEffect(() => {
    setOtpEmail(dashboard?.user.otp_email ?? '')
  }, [dashboard?.user.otp_email])

  const login = async (nextUsername = username) => {
    const name = nextUsername.trim()
    if (!name) {
      setMessage('请输入 username')
      setMessageType('error')
      return
    }
    setLoading(true)
    try {
      const data = await requestJSON<UserDashboard>('/api/phone-register/user/login', {
        method: 'POST',
        body: JSON.stringify({ username: name }),
      })
      localStorage.setItem(AUTHORIZATION_STORAGE, name)
      localStorage.setItem(USERNAME_STORAGE, name)
      setDashboard(data)
      emailRefreshSnapshotRef.current = emailRefreshSnapshot(data)
      setApiKey(localStorage.getItem(userStorageKey(HERO_KEY_STORAGE, data.user)) || '')
      setDuckAuth(localStorage.getItem(userStorageKey(DUCK_AUTH_STORAGE, data.user)) || '')
      setRegisterCount(
        localStorage.getItem(userStorageKey(REGISTER_COUNT_STORAGE, data.user)) || '1',
      )
      setEmailCount(localStorage.getItem(userStorageKey(EMAIL_COUNT_STORAGE, data.user)) || '1')
      setOtpEmail(data.user.otp_email || '')
      setEmailSearch('')
      setEmailQuery('')
      emailQueryRef.current = ''
      setMessage(`已登录 ${data.user.username}，上传分组 #${data.user.group_id}`)
      setMessageType('ok')
      await refreshEmails(data.user.id, 1, '')
      if (dashboardNeedsRefresh(data)) scheduleRefresh(data.user.id)
    } catch (err) {
      renderError(err)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    if (autoLoginRef.current || dashboard || loading) return
    const cached = localStorage.getItem(AUTHORIZATION_STORAGE) || ''
    if (!cached.trim()) return
    autoLoginRef.current = true
    setUsername(cached)
    void login(cached)
  }, [dashboard, loading])

  const generateEmails = async () => {
    if (!user) return
    const count = clampCount(emailCount, 100)
    if (user.is_duck && !duckAuth.trim()) {
      setMessage('请输入 Duck Authorization')
      setMessageType('error')
      return
    }
    setLoading(true)
    setMessage(`正在创建 ${count} 个 Duck 邮箱...`)
    setMessageType('info')
    try {
      localStorage.setItem(EMAIL_COUNT_STORAGE, String(count))
      localStorage.setItem(userStorageKey(EMAIL_COUNT_STORAGE, user), String(count))
      if (user.is_duck) {
        localStorage.setItem(DUCK_AUTH_STORAGE, duckAuth.trim())
        localStorage.setItem(userStorageKey(DUCK_AUTH_STORAGE, user), duckAuth.trim())
      }
      const result = await requestJSON<EmailGenerateResult>(
        '/api/phone-register/user/emails/generate',
        {
          method: 'POST',
          body: JSON.stringify({
            user_id: user.id,
            count,
            duck_authorization: duckAuth.trim(),
          }),
        },
      )
      setDashboard((current) => (current ? { ...current, summary: result.summary } : current))
      setEmailSearch('')
      setEmailQuery('')
      emailQueryRef.current = ''
      await refreshDashboard(user.id)
      await refreshEmails(user.id, 1, '')
      setMessage(`已创建 ${result.created} 个邮箱`)
      setMessageType(result.created > 0 ? 'ok' : 'info')
    } catch (err) {
      renderError(err)
      void refreshDashboard(user.id).catch(() => undefined)
    } finally {
      setLoading(false)
    }
  }

  const startRegister = async () => {
    if (!user) return
    const count = clampCount(registerCount, 100)
    if (!apiKey.trim()) {
      setMessage('请输入 HeroSMS API Key')
      setMessageType('error')
      return
    }
    if (isRunning) {
      setMessage('当前用户已有运行中的任务')
      setMessageType('error')
      return
    }
    setLoading(true)
    setMessage(`正在启动 ${count} 个账号的注册任务...`)
    setMessageType('info')
    try {
      localStorage.setItem(HERO_KEY_STORAGE, apiKey.trim())
      localStorage.setItem(REGISTER_COUNT_STORAGE, String(count))
      localStorage.setItem(userStorageKey(HERO_KEY_STORAGE, user), apiKey.trim())
      localStorage.setItem(userStorageKey(REGISTER_COUNT_STORAGE, user), String(count))
      const data = await requestJSON<UserDashboard>('/api/phone-register/user/register/start', {
        method: 'POST',
        body: JSON.stringify({ user_id: user.id, api_key: apiKey.trim(), count }),
      })
      setDashboard(data)
      emailRefreshSnapshotRef.current = emailRefreshSnapshot(data)
      setMessage('任务已启动')
      setMessageType('ok')
      scheduleRefresh(user.id)
    } catch (err) {
      renderError(err)
    } finally {
      setLoading(false)
    }
  }

  const stopRegister = async () => {
    if (!user) return
    setLoading(true)
    setMessage('正在停止当前用户手机号注册任务...')
    setMessageType('info')
    try {
      const data = await requestJSON<UserDashboard>('/api/phone-register/user/register/stop', {
        method: 'POST',
        body: JSON.stringify({ user_id: user.id }),
      })
      setDashboard(data)
      emailRefreshSnapshotRef.current = emailRefreshSnapshot(data)
      setMessage('已发送停止请求')
      setMessageType('info')
      if (dashboardNeedsRefresh(data)) {
        scheduleRefresh(user.id)
      } else {
        clearTimer()
      }
    } catch (err) {
      renderError(err)
    } finally {
      setLoading(false)
    }
  }

  const applyEmailSearch = async () => {
    if (!user) return
    const q = emailSearch.trim()
    setEmailQuery(q)
    emailQueryRef.current = q
    try {
      await refreshEmails(user.id, 1, q)
    } catch (err) {
      renderError(err)
    }
  }

  const clearEmailSearch = async () => {
    if (!user) return
    setEmailSearch('')
    setEmailQuery('')
    emailQueryRef.current = ''
    try {
      await refreshEmails(user.id, 1, '')
    } catch (err) {
      renderError(err)
    }
  }

  const saveUserInfo = async () => {
    if (!user) return
    const nextOtpEmail = otpEmail.trim()
    setLoading(true)
    setMessage('正在保存用户信息...')
    setMessageType('info')
    try {
      const data = await requestJSON<UserDashboard>('/api/phone-register/user/update', {
        method: 'POST',
        body: JSON.stringify({
          user_id: user.id,
          otp_email: nextOtpEmail,
        }),
      })
      setDashboard(data)
      setOtpEmail(data.user.otp_email || '')
      setMessage('用户信息已保存')
      setMessageType('ok')
    } catch (err) {
      renderError(err)
    } finally {
      setLoading(false)
    }
  }

  const changeEmailPage = async (nextPage: number) => {
    if (!user) return
    const maxPage = Math.max(1, emailList.total_pages || 1)
    const page = Math.max(1, Math.min(nextPage, maxPage))
    try {
      await refreshEmails(user.id, page)
    } catch (err) {
      renderError(err)
    }
  }

  const logout = () => {
    clearTimer()
    localStorage.removeItem(AUTHORIZATION_STORAGE)
    setDashboard(null)
    setEmailList(emptyEmailList)
    setEmailPage(1)
    setEmailSearch('')
    setEmailQuery('')
    setOtpEmail('')
    emailRefreshSnapshotRef.current = { phoneSuccess: 0, loginSuccess: 0 }
    setMessage('请输入 username 进入注册控制台')
    setMessageType('info')
  }

  if (!dashboard) {
    return (
      <main className="grid min-h-0 flex-1 place-items-center overflow-hidden">
        <section className="w-full max-w-[460px] rounded-xl border border-warmgray-200/70 bg-canvas p-7 shadow-card">
          <div className="mb-6">
            <h2 className="text-[20px] font-semibold tracking-tightish text-warmgray-900">
              手机号注册控制台
            </h2>
            <p className="mt-1 text-[13px] text-warmgray-500">
              输入 username 后进入对应用户的手机号注册、邮箱池和登录上传看板。
            </p>
          </div>
          <label className="grid gap-1.5 text-[12px] font-medium text-warmgray-600">
            Username
            <input
              className="h-11 rounded-md border border-warmgray-200 bg-white px-3 text-[14px] text-warmgray-900 transition-colors focus:border-coral-500"
              value={username}
              onChange={(event) => setUsername(event.target.value)}
              onKeyDown={(event) => {
                if (event.key === 'Enter') void login()
              }}
              autoComplete="off"
              placeholder="输入 username"
            />
          </label>
          <button
            className="mt-4 h-11 w-full rounded-md bg-coral-500 px-4 text-[13px] font-semibold text-white transition-colors hover:bg-coral-600 disabled:cursor-not-allowed disabled:opacity-50"
            type="button"
            onClick={() => void login()}
            disabled={loading}
          >
            进入
          </button>
          <MessageLine message={message} type={messageType} className="mt-3" />
        </section>
      </main>
    )
  }

  return (
    <main className="grid min-h-0 flex-1 grid-cols-12 gap-4 overflow-hidden">
      <aside className="col-span-12 flex min-h-0 flex-col rounded-xl border border-warmgray-200/70 bg-canvas shadow-card lg:col-span-4 xl:col-span-3">
        <div className="shrink-0 border-b border-warmgray-100 px-5 py-4">
          <div className="flex items-start justify-between gap-4">
            <div className="min-w-0">
              <div className="text-[11px] font-medium uppercase tracking-[0.18em] text-warmgray-400">
                当前用户
              </div>
              <h2 className="mt-1 truncate text-[20px] font-semibold tracking-tightish text-warmgray-900">
                {user?.username}
              </h2>
              <p className="mt-1 text-[12px] text-warmgray-500">
                分组 #{user?.group_id} · {user?.is_duck ? 'Duck 邮箱' : '自备邮箱'}
              </p>
            </div>
            <button
              className="h-9 rounded-md border border-warmgray-200 bg-white px-3 text-[12px] font-semibold text-warmgray-600 transition-colors hover:bg-warmgray-50 disabled:cursor-not-allowed disabled:opacity-50"
              type="button"
              onClick={logout}
              disabled={loading || isRunning}
            >
              切换
            </button>
          </div>
        </div>

        <div className="min-h-0 flex-1 overflow-auto px-5 py-4">
          <div className="grid gap-5">
            <div className="grid grid-cols-3 gap-2">
              <MiniStat label="可用邮箱" value={String(summary?.email_unused ?? 0)} />
              <MiniStat label="成功账号" value={String(summary?.account_success ?? 0)} />
              <MiniStat label="失败账号" value={String(summary?.account_failed ?? 0)} />
            </div>

            <ControlGroup
              title="用户信息"
              description="接收邮箱用于当前用户注册和登录阶段自动读取验证码。"
            >
              <label className="grid gap-1.5 text-[12px] font-medium text-warmgray-600">
                接收验证码邮箱
                <input
                  className="h-10 rounded-md border border-warmgray-200 bg-white px-3 text-[13px] text-warmgray-900 transition-colors focus:border-coral-500"
                  type="email"
                  value={otpEmail}
                  onChange={(event) => setOtpEmail(event.target.value)}
                  disabled={loading}
                  autoComplete="off"
                  placeholder="例如 user-otp@example.com"
                />
              </label>
              <button
                className="mt-3 h-10 w-full rounded-md border border-warmgray-200 bg-white px-4 text-[13px] font-semibold text-warmgray-700 transition-colors hover:bg-warmgray-50 disabled:cursor-not-allowed disabled:opacity-50"
                type="button"
                onClick={() => void saveUserInfo()}
                disabled={loading || !userInfoDirty}
              >
                保存用户信息
              </button>
              {userInfoDirty ? (
                <div className="mt-2 text-[11px] leading-4 text-amber-700">
                  接收邮箱已修改，保存后才能开始新的注册任务。
                </div>
              ) : null}
            </ControlGroup>

            {user?.is_duck ? (
              <ControlGroup
                title="Duck 邮箱"
                description="Authorization 会缓存在当前浏览器。"
                side={
                  <input
                    className="h-9 w-20 rounded-md border border-warmgray-200 bg-white px-2 text-[13px] text-warmgray-900 focus:border-coral-500"
                    type="number"
                    min={1}
                    max={100}
                    value={emailCount}
                    onChange={(event) => setEmailCount(event.target.value)}
                    disabled={loading}
                  />
                }
              >
                <label className="grid gap-1.5 text-[12px] font-medium text-warmgray-600">
                  Duck Authorization
                  <input
                    className="h-10 rounded-md border border-warmgray-200 bg-white px-3 text-[13px] text-warmgray-900 transition-colors focus:border-coral-500"
                    type="password"
                    value={duckAuth}
                    onChange={(event) => setDuckAuth(event.target.value)}
                    disabled={loading}
                    autoComplete="off"
                    placeholder="不需要输入 Bearer"
                  />
                </label>
                <button
                  className="mt-3 h-10 w-full rounded-md border border-coral-200 bg-coral-50 px-4 text-[13px] font-semibold text-coral-700 transition-colors hover:bg-coral-100 disabled:cursor-not-allowed disabled:opacity-50"
                  type="button"
                  onClick={() => void generateEmails()}
                  disabled={loading}
                >
                  创建邮箱
                </button>
              </ControlGroup>
            ) : (
              <div className="border-t border-warmgray-100 pt-4 text-[12px] leading-5 text-warmgray-500">
                当前用户不是 Duck 邮箱用户，注册时会从 user_email 表中取未使用邮箱。
              </div>
            )}

            <ControlGroup
              title="手机号注册"
              description="一个用户同一时间只运行一个手机号注册任务。"
              side={
                <input
                  className="h-9 w-20 rounded-md border border-warmgray-200 bg-white px-2 text-[13px] text-warmgray-900 focus:border-coral-500"
                  type="number"
                  min={1}
                  max={100}
                  value={registerCount}
                  onChange={(event) => setRegisterCount(event.target.value)}
                  disabled={loading || isRunning}
                />
              }
            >
              <label className="grid gap-1.5 text-[12px] font-medium text-warmgray-600">
                HeroSMS API Key
                <input
                  className="h-10 rounded-md border border-warmgray-200 bg-white px-3 text-[13px] text-warmgray-900 transition-colors focus:border-coral-500"
                  type="password"
                  value={apiKey}
                  onChange={(event) => setApiKey(event.target.value)}
                  disabled={loading || isRunning}
                  autoComplete="off"
                  placeholder="不同使用者可各自缓存"
                />
              </label>
              <div className="mt-3 flex gap-2">
                <button
                  className="h-10 flex-1 rounded-md bg-coral-500 px-4 text-[13px] font-semibold text-white transition-colors hover:bg-coral-600 disabled:cursor-not-allowed disabled:opacity-50"
                  type="button"
                  onClick={() => void startRegister()}
                  disabled={loading || isRunning || !otpEmail.trim() || userInfoDirty}
                >
                  开始注册
                </button>
                <button
                  className="h-10 rounded-md border border-warmgray-200 bg-white px-4 text-[13px] font-semibold text-warmgray-700 transition-colors hover:bg-warmgray-50 disabled:cursor-not-allowed disabled:opacity-50"
                  type="button"
                  onClick={() => void stopRegister()}
                  disabled={loading || !isRunning}
                >
                  停止
                </button>
              </div>
            </ControlGroup>

            <MessageLine message={message} type={messageType} />
          </div>
        </div>
      </aside>

      <section className="col-span-12 flex min-h-0 flex-col gap-4 overflow-hidden lg:col-span-8 xl:col-span-9">
        <AccountSummaryCard summary={summary} />

        <div className="grid shrink-0 gap-4 xl:grid-cols-2">
          <StageProgressPanel
            title="注册进度"
            status={labelForStatus(run?.phone_done ? 'success' : run?.status || 'idle')}
            tone={toneForStatus(run?.phone_done ? 'success' : run?.status || 'idle')}
            value={phoneProgress}
            primary={`${phoneProcessed}/${run?.target_count ?? 0}`}
            detail={`${run?.phone_success_count ?? 0} 成功 / ${run?.phone_failure_count ?? 0} 失败 / 当前 ${run?.current_phone || '-'}`}
            emptyText={run?.phone_done ? '手机号注册阶段已完成' : '暂无手机号注册任务'}
            logs={phoneLogs}
          />
          <StageProgressPanel
            title="登录进度"
            status={currentLoginStatus}
            tone={currentLoginTone}
            value={currentLoginProgress}
            primary={`${currentLoginDone}/${currentLoginTotal}`}
            detail={`${currentLoginQueued} 排队 / ${currentLoginRunning} 处理中 / ${run?.login_success_count ?? 0} 成功 / ${run?.login_failed_count ?? 0} 失败`}
            emptyText="暂无登录上传任务"
            logs={loginLogs}
          />
        </div>

        <EmailTable
          emailList={emailList}
          emailPage={emailPage}
          emailLoading={emailLoading}
          emailSearch={emailSearch}
          emailQuery={emailQuery}
          onSearchChange={setEmailSearch}
          onSearch={() => void applyEmailSearch()}
          onClear={() => void clearEmailSearch()}
          onRefresh={() => user && void refreshEmails(user.id, emailPage)}
          onPageChange={(page) => void changeEmailPage(page)}
        />
      </section>
    </main>
  )
}

function MiniStat({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-lg border border-warmgray-200 bg-white px-3 py-3">
      <div className="label">{label}</div>
      <div className="num mt-1 truncate text-[18px] font-semibold text-warmgray-900">{value}</div>
    </div>
  )
}

function ControlGroup({
  title,
  description,
  side,
  children,
}: {
  title: string
  description: string
  side?: ReactNode
  children: ReactNode
}) {
  return (
    <div className="border-t border-warmgray-100 pt-4">
      <div className="mb-3 flex items-start justify-between gap-3">
        <div>
          <div className="text-[13px] font-semibold text-warmgray-900">{title}</div>
          <div className="mt-1 text-[12px] leading-5 text-warmgray-500">{description}</div>
        </div>
        {side}
      </div>
      {children}
    </div>
  )
}

function AccountSummaryCard({ summary }: { summary?: UserSummary }) {
  const metrics = [
    ['账号总数', summary?.account_total ?? 0],
    ['已注册', summary?.account_registered ?? 0],
    ['待登录', summary?.account_queued ?? 0],
    ['处理中', summary?.account_running ?? 0],
    ['成功账号', summary?.account_success ?? 0],
    ['失败账号', summary?.account_failed ?? 0],
  ] as const
  return (
    <article className="rounded-xl border border-warmgray-200/70 bg-canvas px-5 py-4 shadow-card">
      <div className="flex flex-wrap items-center justify-between gap-4">
        <div className="shrink-0">
          <div className="text-[13px] font-semibold text-warmgray-900">当前用户账号统计</div>
          <div className="mt-1 text-[12px] text-warmgray-500">注册、排队、登录上传后的统一账号口径</div>
        </div>
        <div className="grid flex-1 grid-cols-2 gap-x-5 gap-y-3 sm:grid-cols-3 xl:grid-cols-6 xl:justify-items-end">
          {metrics.map(([label, value]) => (
            <div key={label} className="min-w-[76px]">
              <div className="label">{label}</div>
              <div className="num mt-1 text-[20px] font-semibold leading-none text-warmgray-900">
                {value}
              </div>
            </div>
          ))}
        </div>
      </div>
    </article>
  )
}

function StageProgressPanel({
  title,
  status,
  tone,
  value,
  primary,
  detail,
  logs,
  emptyText,
}: {
  title: string
  status: string
  tone: string
  value: number
  primary: string
  detail: string
  logs: UserRunLog[]
  emptyText: string
}) {
  const visibleLogs = logs.slice(-6)
  return (
    <article className="rounded-xl border border-warmgray-200/70 bg-canvas px-5 py-4 shadow-card">
      <div className="flex items-start justify-between gap-3">
        <div>
          <div className="text-[13px] font-semibold text-warmgray-900">{title}</div>
          <div className="mt-1 text-[12px] text-warmgray-500">{detail}</div>
        </div>
        <span className={`rounded-full px-2.5 py-1 text-[10px] font-semibold ring-1 ring-inset ${tone}`}>
          {status}
        </span>
      </div>
      <div className="num mt-5 text-[30px] font-semibold leading-none tracking-tightish text-warmgray-900">
        {primary}
      </div>
      <ProgressMeter value={value} className="mt-4" />
      <div className="mt-4 h-[168px] overflow-hidden rounded-lg bg-warmgray-50 px-3 py-2">
        {visibleLogs.length ? (
          <div className="grid gap-2">
            {visibleLogs.map((log, index) => (
              <div key={`${log.time}-${index}`} className="flex h-5 gap-2 overflow-hidden text-[12px] leading-5">
                <span className="num w-[62px] shrink-0 text-warmgray-400">{formatTime(log.time)}</span>
                <span className={`mt-1.5 h-2 w-2 shrink-0 rounded-full ${dotForLevel(log.level)}`} />
                <span className="min-w-0 flex-1 truncate text-warmgray-700">{log.message}</span>
              </div>
            ))}
          </div>
        ) : (
          <div className="grid h-full place-items-center text-[13px] text-warmgray-400">
            {emptyText}
          </div>
        )}
      </div>
    </article>
  )
}

function ProgressMeter({ value, className = '' }: { value: number; className?: string }) {
  return (
    <div className={className}>
      <div className="mb-1.5 flex items-center justify-between text-[11px] text-warmgray-500">
        <span>进度</span>
        <span className="num">{value}%</span>
      </div>
      <div className="h-2.5 overflow-hidden rounded-full bg-warmgray-100">
        <div
          className="h-full rounded-full bg-coral-500 transition-all"
          style={{ width: `${value}%` }}
        />
      </div>
    </div>
  )
}

function EmailTable({
  emailList,
  emailPage,
  emailLoading,
  emailSearch,
  emailQuery,
  onSearchChange,
  onSearch,
  onClear,
  onRefresh,
  onPageChange,
}: {
  emailList: UserEmailListResponse
  emailPage: number
  emailLoading: boolean
  emailSearch: string
  emailQuery: string
  onSearchChange: (value: string) => void
  onSearch: () => void
  onClear: () => void
  onRefresh: () => void
  onPageChange: (page: number) => void
}) {
  const maxPage = Math.max(1, emailList.total_pages || 1)
  const start = emailList.total ? (emailList.page - 1) * emailList.page_size + 1 : 0
  const end = emailList.total ? Math.min(emailList.page * emailList.page_size, emailList.total) : 0

  return (
    <section className="flex min-h-0 flex-1 flex-col overflow-hidden rounded-xl border border-warmgray-200/70 bg-canvas shadow-card">
      <div className="shrink-0 border-b border-warmgray-100 px-5 py-4">
        <div className="flex flex-wrap items-end justify-between gap-3">
          <div>
            <h2 className="text-[17px] font-semibold tracking-tightish text-warmgray-900">
              邮箱列表
            </h2>
            <p className="mt-1 text-[12px] text-warmgray-500">
              每页 10 条 · 共 {emailList.total} 条{emailQuery ? ` · 搜索：${emailQuery}` : ''}
            </p>
          </div>
          <div className="flex flex-wrap items-center gap-2">
            <input
              className="h-9 w-[240px] rounded-md border border-warmgray-200 bg-white px-3 text-[13px] text-warmgray-900 transition-colors placeholder:text-warmgray-400 focus:border-coral-500"
              value={emailSearch}
              onChange={(event) => onSearchChange(event.target.value)}
              onKeyDown={(event) => {
                if (event.key === 'Enter') onSearch()
              }}
              placeholder="搜索邮箱或手机号"
            />
            <button
              className="h-9 rounded-md bg-warmgray-900 px-3 text-[12px] font-semibold text-white transition-colors hover:bg-warmgray-700 disabled:cursor-not-allowed disabled:opacity-50"
              type="button"
              onClick={onSearch}
              disabled={emailLoading}
            >
              搜索
            </button>
            <button
              className="h-9 rounded-md border border-warmgray-200 bg-white px-3 text-[12px] font-semibold text-warmgray-600 transition-colors hover:bg-warmgray-50 disabled:cursor-not-allowed disabled:opacity-50"
              type="button"
              onClick={onClear}
              disabled={emailLoading || (!emailQuery && !emailSearch)}
            >
              清空
            </button>
            <button
              className="h-9 rounded-md border border-warmgray-200 bg-white px-3 text-[12px] font-semibold text-warmgray-600 transition-colors hover:bg-warmgray-50 disabled:cursor-not-allowed disabled:opacity-50"
              type="button"
              onClick={onRefresh}
              disabled={emailLoading}
            >
              刷新
            </button>
          </div>
        </div>
      </div>

      <div className="min-h-0 flex-1 overflow-auto">
        <table className="min-w-full table-fixed border-separate border-spacing-0 text-left">
          <colgroup>
            <col className="w-[34%]" />
            <col className="w-[18%]" />
            <col className="w-[18%]" />
            <col className="w-[14%]" />
            <col className="w-[16%]" />
          </colgroup>
          <thead className="sticky top-0 z-10 bg-canvas">
            <tr className="text-[11px] uppercase text-warmgray-400">
              <th className="border-b border-warmgray-100 px-5 py-2 font-semibold">邮箱</th>
              <th className="border-b border-warmgray-100 px-4 py-2 font-semibold">手机号</th>
              <th className="border-b border-warmgray-100 px-4 py-2 font-semibold">状态</th>
              <th className="border-b border-warmgray-100 px-4 py-2 font-semibold">Sub2API</th>
              <th className="border-b border-warmgray-100 px-5 py-2 font-semibold">使用时间</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-warmgray-100">
            {emailList.items.length ? (
              emailList.items.map((item) => {
                const status = item.account_status || (item.used_at ? 'used' : 'unused')
                return (
                  <tr key={item.id} className="text-[12px] text-warmgray-700 transition-colors hover:bg-cream/60">
                    <td className="px-5 py-3">
                      <div className="truncate font-semibold text-warmgray-900">{item.email}</div>
                      <div className="mt-1 truncate text-[11px] text-warmgray-400">
                        #{item.id} · {item.provider || 'manual'}
                      </div>
                    </td>
                    <td className="px-4 py-3">
                      <div className="num whitespace-nowrap text-warmgray-700">{item.phone || '-'}</div>
                    </td>
                    <td className="px-4 py-3">
                      <span
                        className={`inline-flex whitespace-nowrap rounded-full px-2.5 py-1 text-[10px] font-semibold ring-1 ring-inset ${toneForStatus(
                          status,
                        )}`}
                      >
                        {labelForStatus(status)}
                      </span>
                      {item.account_error ? (
                        <div className="mt-1 truncate text-[11px] text-rose-600">
                          {item.account_error}
                        </div>
                      ) : null}
                    </td>
                    <td className="px-4 py-3">
                      <span
                        className={`inline-flex whitespace-nowrap rounded-full px-2.5 py-1 text-[10px] font-semibold ring-1 ring-inset ${
                          item.sub2api_uploaded
                            ? 'bg-emerald-50 text-emerald-700 ring-emerald-200'
                            : item.account_status
                              ? 'bg-warmgray-50 text-warmgray-600 ring-warmgray-200'
                              : 'bg-warmgray-50 text-warmgray-400 ring-warmgray-200'
                        }`}
                      >
                        {item.sub2api_uploaded ? '已上传' : item.account_status ? '未上传' : '-'}
                      </span>
                    </td>
                    <td className="px-5 py-3">
                      <span className="num whitespace-nowrap text-warmgray-500">
                        {formatDateTime(item.used_at)}
                      </span>
                    </td>
                  </tr>
                )
              })
            ) : (
              <tr>
                <td colSpan={5} className="px-5 py-16 text-center text-[13px] text-warmgray-400">
                  {emailLoading ? '正在加载邮箱列表...' : emailQuery ? '没有匹配的邮箱或手机号。' : '暂无邮箱记录。'}
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </div>

      <div className="flex shrink-0 items-center justify-between border-t border-warmgray-100 px-5 py-3">
        <div className="num text-[12px] text-warmgray-500">
          {start}-{end} / {emailList.total} · 第 {emailList.total ? emailList.page : 0} / {emailList.total_pages || 0} 页
        </div>
        <div className="flex gap-2">
          <button
            className="h-8 rounded-md border border-warmgray-200 bg-white px-3 text-[12px] font-semibold text-warmgray-600 transition-colors hover:bg-warmgray-50 disabled:cursor-not-allowed disabled:opacity-50"
            type="button"
            onClick={() => onPageChange(emailPage - 1)}
            disabled={emailLoading || emailPage <= 1}
          >
            上一页
          </button>
          <button
            className="h-8 rounded-md border border-warmgray-200 bg-white px-3 text-[12px] font-semibold text-warmgray-600 transition-colors hover:bg-warmgray-50 disabled:cursor-not-allowed disabled:opacity-50"
            type="button"
            onClick={() => onPageChange(emailPage + 1)}
            disabled={emailLoading || emailPage >= maxPage}
          >
            下一页
          </button>
        </div>
      </div>
    </section>
  )
}

function MessageLine({
  message,
  type,
  className = '',
}: {
  message: string
  type: 'info' | 'error' | 'ok'
  className?: string
}) {
  return (
    <div
      className={`min-h-5 text-[12px] ${
        type === 'error'
          ? 'text-rose-600'
          : type === 'ok'
            ? 'text-emerald-700'
            : 'text-warmgray-500'
      } ${className}`}
    >
      {message}
    </div>
  )
}
