import { useCallback, useEffect, useMemo, useRef, useState } from 'react'

type RegisterUser = {
  id: number
  username: string
  group_id: number
  is_duck: boolean
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

const AUTHORIZATION_STORAGE = 'sub2api-panel:phone-register-authorization'
const USERNAME_STORAGE = 'sub2api-panel:phone-register-username'
const HERO_KEY_STORAGE = 'sub2api-panel:herosms-api-key'
const DUCK_AUTH_STORAGE = 'sub2api-panel:duck-authorization'
const REGISTER_COUNT_STORAGE = 'sub2api-panel:user-register-count'
const EMAIL_COUNT_STORAGE = 'sub2api-panel:user-email-count'
const EMAIL_PAGE_SIZE = 20

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
  const [emailLoading, setEmailLoading] = useState(false)
  const [message, setMessage] = useState('请输入 username 进入注册控制台')
  const [messageType, setMessageType] = useState<'info' | 'error' | 'ok'>('info')
  const [loading, setLoading] = useState(false)
  const timer = useRef<number | null>(null)
  const emailPageRef = useRef(1)
  const autoLoginRef = useRef(false)

  const user = dashboard?.user
  const run = dashboard?.run
  const summary = dashboard?.summary
  const isRunning = runIsActive(run)
  const latestLog = run?.logs?.length ? run.logs[run.logs.length - 1] : undefined
  const latestMessage = latestLog?.message || run?.step || '暂无运行任务'
  const phoneProgress = useMemo(() => {
    if (!run?.target_count) return 0
    return Math.min(
      100,
      Math.round(((run.phone_success_count + run.phone_failure_count) / run.target_count) * 100),
    )
  }, [run])
  const loginProgress = useMemo(() => {
    if (!run?.target_count) return 0
    return Math.min(
      100,
      Math.round(((run.login_success_count + run.login_failed_count) / run.target_count) * 100),
    )
  }, [run])

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

  const refreshDashboard = useCallback(
    async (userID: number, keepTimer = true) => {
      const data = await requestJSON<UserDashboard>(
        `/api/phone-register/users/${encodeURIComponent(userID)}/dashboard`,
      )
      setDashboard(data)
      if (!runIsActive(data.run) && !keepTimer) {
        clearTimer()
      }
      return data
    },
    [clearTimer],
  )

  const refreshEmails = useCallback(async (userID: number, page = emailPageRef.current) => {
    setEmailLoading(true)
    try {
      const data = await requestJSON<UserEmailListResponse>(
        `/api/phone-register/users/${encodeURIComponent(userID)}/emails?page=${page}&page_size=${EMAIL_PAGE_SIZE}`,
      )
      setEmailList(data)
      setEmailPage(data.page)
      emailPageRef.current = data.page
      return data
    } finally {
      setEmailLoading(false)
    }
  }, [])

  const scheduleRefresh = useCallback(
    (userID: number) => {
      clearTimer()
      timer.current = window.setInterval(() => {
        void refreshDashboard(userID, false)
          .then(() => refreshEmails(userID).catch(() => undefined))
          .catch(renderError)
      }, 2500)
    },
    [clearTimer, refreshDashboard, refreshEmails, renderError],
  )

  useEffect(() => {
    return clearTimer
  }, [clearTimer])

  useEffect(() => {
    emailPageRef.current = emailPage
  }, [emailPage])

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
      setApiKey(localStorage.getItem(userStorageKey(HERO_KEY_STORAGE, data.user)) || '')
      setDuckAuth(localStorage.getItem(userStorageKey(DUCK_AUTH_STORAGE, data.user)) || '')
      setRegisterCount(
        localStorage.getItem(userStorageKey(REGISTER_COUNT_STORAGE, data.user)) || '1',
      )
      setEmailCount(localStorage.getItem(userStorageKey(EMAIL_COUNT_STORAGE, data.user)) || '1')
      setMessage(`已登录 ${data.user.username}，上传分组 #${data.user.group_id}`)
      setMessageType('ok')
      await refreshEmails(data.user.id, 1)
      if (runIsActive(data.run)) scheduleRefresh(data.user.id)
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
      await refreshDashboard(user.id)
      await refreshEmails(user.id, 1)
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
      setMessage('任务已启动')
      setMessageType('ok')
      await refreshEmails(user.id, 1)
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
    setMessage('正在停止当前用户任务...')
    setMessageType('info')
    try {
      const data = await requestJSON<UserDashboard>('/api/phone-register/user/register/stop', {
        method: 'POST',
        body: JSON.stringify({ user_id: user.id }),
      })
      setDashboard(data)
      setMessage('已发送停止请求')
      setMessageType('info')
      void refreshEmails(user.id).catch(() => undefined)
      if (runIsActive(data.run)) {
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
    setMessage('请输入 username 进入注册控制台')
    setMessageType('info')
  }

  if (!dashboard) {
    return (
      <div className="grid min-h-0 flex-1 place-items-center overflow-hidden">
        <section className="w-full max-w-[420px] rounded-2xl border border-warmgray-200/70 bg-canvas p-6 shadow-card">
          <div className="mb-5">
            <h2 className="text-[18px] font-semibold tracking-tightish text-warmgray-900">
              手机号注册登录
            </h2>
            <p className="mt-1 text-[12px] text-warmgray-500">
              输入 username 后进入对应用户的注册队列和邮箱池。
            </p>
          </div>
          <label className="grid gap-1.5 text-[12px] font-medium text-warmgray-600">
            Username
            <input
              className="h-10 rounded-md border border-warmgray-200 bg-white px-3 text-[13px] text-warmgray-900 transition-colors focus:border-coral-500"
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
            className="mt-4 h-10 w-full rounded-md bg-coral-500 px-4 text-[13px] font-semibold text-white transition-colors hover:bg-coral-600 disabled:cursor-not-allowed disabled:opacity-50"
            type="button"
            onClick={() => void login()}
            disabled={loading}
          >
            进入注册页面
          </button>
          <div
            className={`mt-3 min-h-5 text-[12px] ${
              messageType === 'error'
                ? 'text-rose-600'
                : messageType === 'ok'
                  ? 'text-emerald-700'
                  : 'text-warmgray-500'
            }`}
          >
            {message}
          </div>
        </section>
      </div>
    )
  }

  return (
    <div className="grid min-h-0 flex-1 grid-cols-12 gap-4 overflow-hidden">
      <section className="col-span-12 flex min-h-0 flex-col rounded-2xl border border-warmgray-200/70 bg-canvas shadow-card lg:col-span-4">
        <div className="border-b border-warmgray-100 px-5 py-4">
          <div className="flex items-start justify-between gap-4">
            <div className="min-w-0">
              <h2 className="truncate text-[17px] font-semibold tracking-tightish text-warmgray-900">
                {user?.username}
              </h2>
              <p className="mt-1 text-[12px] text-warmgray-500">
                分组 #{user?.group_id} · {user?.is_duck ? 'Duck 邮箱' : '自备邮箱'}
              </p>
            </div>
            <button
              className="rounded-md border border-warmgray-200 bg-white px-3 py-1.5 text-[12px] font-semibold text-warmgray-600 hover:bg-warmgray-50"
              type="button"
              onClick={logout}
              disabled={loading || isRunning}
            >
              切换
            </button>
          </div>
        </div>

        <div className="min-h-0 flex-1 overflow-auto px-5 py-4">
          <div className="grid gap-4">
            <div className="grid grid-cols-3 gap-2">
              <MiniStat
                label="邮箱"
                value={`${summary?.email_unused ?? 0}/${summary?.email_total ?? 0}`}
              />
              <MiniStat label="成功" value={String(summary?.account_success ?? 0)} />
              <MiniStat label="失败" value={String(summary?.account_failed ?? 0)} />
            </div>

            {user?.is_duck ? (
              <div className="rounded-lg border border-warmgray-200 bg-warmgray-50 p-4">
                <div className="mb-3 flex items-center justify-between gap-3">
                  <div>
                    <div className="label">Duck 邮箱</div>
                    <div className="mt-1 text-[12px] text-warmgray-500">
                      Authorization 会缓存在当前浏览器。
                    </div>
                  </div>
                  <input
                    className="h-9 w-20 rounded-md border border-warmgray-200 bg-white px-2 text-[13px] text-warmgray-900 focus:border-coral-500"
                    type="number"
                    min={1}
                    max={100}
                    value={emailCount}
                    onChange={(event) => setEmailCount(event.target.value)}
                    disabled={loading}
                  />
                </div>
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
              </div>
            ) : (
              <div className="rounded-lg border border-warmgray-200 bg-warmgray-50 p-4 text-[12px] leading-5 text-warmgray-500">
                当前用户不是 Duck 邮箱用户，注册时会从 user_email 表中取未使用邮箱。
              </div>
            )}

            <div className="rounded-lg border border-warmgray-200 bg-warmgray-50 p-4">
              <div className="mb-3 flex items-center justify-between gap-3">
                <div>
                  <div className="label">自动注册</div>
                  <div className="mt-1 text-[12px] text-warmgray-500">
                    一个用户同一时间只能运行一个任务。
                  </div>
                </div>
                <input
                  className="h-9 w-20 rounded-md border border-warmgray-200 bg-white px-2 text-[13px] text-warmgray-900 focus:border-coral-500"
                  type="number"
                  min={1}
                  max={100}
                  value={registerCount}
                  onChange={(event) => setRegisterCount(event.target.value)}
                  disabled={loading || isRunning}
                />
              </div>
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
                  disabled={loading || isRunning}
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
            </div>

            <div
              className={`min-h-6 text-[12px] ${
                messageType === 'error'
                  ? 'text-rose-600'
                  : messageType === 'ok'
                    ? 'text-emerald-700'
                    : 'text-warmgray-500'
              }`}
            >
              {message}
            </div>
          </div>
        </div>
      </section>

      <section className="col-span-12 flex min-h-0 flex-col rounded-2xl border border-warmgray-200/70 bg-canvas shadow-card lg:col-span-8">
        <div className="shrink-0 border-b border-warmgray-100 px-5 py-3">
          <div className="flex flex-wrap items-start justify-between gap-3">
            <div className="min-w-0 flex-1">
              <div className="flex items-center gap-2">
                <h2 className="text-[17px] font-semibold tracking-tightish text-warmgray-900">
                  当前进度
                </h2>
                <span
                  className={`rounded-full px-2.5 py-0.5 text-[10px] font-semibold ring-1 ring-inset ${toneForStatus(
                    run?.status || 'idle',
                  )}`}
                >
                  {labelForStatus(run?.status || 'idle')}
                </span>
              </div>
              <div className="mt-2 flex min-h-5 items-center gap-2 text-[12px] text-warmgray-600">
                {latestLog ? (
                  <>
                    <span className="num shrink-0 text-warmgray-400">
                      {formatTime(latestLog.time)}
                    </span>
                    <span className={`h-2 w-2 shrink-0 rounded-full ${dotForLevel(latestLog.level)}`} />
                  </>
                ) : null}
                <span className="min-w-0 truncate">{latestMessage}</span>
              </div>
            </div>
          </div>
          <div className="mt-3 grid gap-3 md:grid-cols-2">
            <ProgressBar
              label="手机号注册"
              value={phoneProgress}
              detail={`${run?.phone_success_count ?? 0} 成功 / ${run?.phone_failure_count ?? 0} 失败 / 当前 ${run?.current_phone || '-'}`}
            />
            <ProgressBar
              label="登录上传"
              value={loginProgress}
              detail={`${run?.login_success_count ?? 0} 成功 / ${run?.login_failed_count ?? 0} 失败 / 队列 ${run?.login_queued_count ?? 0}`}
            />
          </div>
        </div>

        <div className="flex min-h-0 flex-1 flex-col overflow-hidden">
          <div className="flex shrink-0 items-center justify-between border-b border-warmgray-100 px-5 py-2">
            <div>
              <div className="label">邮箱列表</div>
              <div className="mt-0.5 text-[11px] text-warmgray-400">
                共 {emailList.total} 个邮箱，按创建时间倒序
              </div>
            </div>
            <button
              className="h-8 rounded-md border border-warmgray-200 bg-white px-3 text-[12px] font-semibold text-warmgray-600 hover:bg-warmgray-50 disabled:cursor-not-allowed disabled:opacity-50"
              type="button"
              onClick={() => user && void refreshEmails(user.id, emailPage)}
              disabled={emailLoading}
            >
              刷新
            </button>
          </div>
          <div className="min-h-0 flex-1 overflow-auto">
            <table className="min-w-full border-separate border-spacing-0 text-left">
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
                      <tr key={item.id} className="text-[12px] text-warmgray-700">
                        <td className="max-w-[260px] px-5 py-3">
                          <div className="truncate font-semibold text-warmgray-900">{item.email}</div>
                          <div className="mt-1 truncate text-[11px] text-warmgray-400">
                            #{item.id} · {item.provider || 'manual'}
                          </div>
                        </td>
                        <td className="px-4 py-3">
                          <div className="num whitespace-nowrap text-warmgray-700">
                            {item.phone || '-'}
                          </div>
                        </td>
                        <td className="px-4 py-3">
                          <span
                            className={`inline-flex whitespace-nowrap rounded-full px-2.5 py-1 text-[10px] font-semibold ring-1 ring-inset ${toneForStatus(
                              status,
                            )}`}
                          >
                            {status === 'unused'
                              ? '未使用'
                              : status === 'used'
                                ? '已占用'
                                : labelForStatus(status)}
                          </span>
                          {item.account_error ? (
                            <div className="mt-1 max-w-[180px] truncate text-[11px] text-rose-600">
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
                      {emailLoading ? '正在加载邮箱列表...' : '暂无邮箱记录。'}
                    </td>
                  </tr>
                )}
              </tbody>
            </table>
          </div>
          <div className="flex shrink-0 items-center justify-between border-t border-warmgray-100 px-5 py-3">
            <div className="num text-[12px] text-warmgray-500">
              第 {emailList.total ? emailList.page : 0} / {emailList.total_pages || 0} 页
            </div>
            <div className="flex gap-2">
              <button
                className="h-8 rounded-md border border-warmgray-200 bg-white px-3 text-[12px] font-semibold text-warmgray-600 hover:bg-warmgray-50 disabled:cursor-not-allowed disabled:opacity-50"
                type="button"
                onClick={() => void changeEmailPage(emailPage - 1)}
                disabled={emailLoading || emailPage <= 1}
              >
                上一页
              </button>
              <button
                className="h-8 rounded-md border border-warmgray-200 bg-white px-3 text-[12px] font-semibold text-warmgray-600 hover:bg-warmgray-50 disabled:cursor-not-allowed disabled:opacity-50"
                type="button"
                onClick={() => void changeEmailPage(emailPage + 1)}
                disabled={emailLoading || emailPage >= Math.max(1, emailList.total_pages || 1)}
              >
                下一页
              </button>
            </div>
          </div>
        </div>
      </section>
    </div>
  )
}

function MiniStat({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-lg border border-warmgray-200 bg-white px-3 py-2">
      <div className="label">{label}</div>
      <div className="num mt-1 truncate text-[16px] font-semibold text-warmgray-900">{value}</div>
    </div>
  )
}

function ProgressBar({ label, value, detail }: { label: string; value: number; detail: string }) {
  return (
    <div>
      <div className="mb-1 flex items-center justify-between gap-3 text-[12px]">
        <span className="font-semibold text-warmgray-700">{label}</span>
        <span className="num text-warmgray-500">{value}%</span>
      </div>
      <div className="h-2 overflow-hidden rounded-full bg-warmgray-100">
        <div
          className="h-2 rounded-full bg-coral-500 transition-all"
          style={{ width: `${value}%` }}
        />
      </div>
      <div className="mt-1 text-[11px] text-warmgray-400">{detail}</div>
    </div>
  )
}
