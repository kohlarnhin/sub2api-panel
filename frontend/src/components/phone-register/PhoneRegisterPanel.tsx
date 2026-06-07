import { useCallback, useEffect, useLayoutEffect, useMemo, useRef, useState } from 'react'
import type { ReactNode } from 'react'
import gsap from 'gsap'
import { AnimatedNumber } from '@/components/AnimatedNumber'
import { useAnimatedWidth, useReStagger } from '@/hooks/useGsap'

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
  phone_code_attempt: number
  phone_code_max_attempts: number
  login_email_code_attempt: number
  login_email_code_max: number
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

type UserEmailRun = {
  user_id: number
  username: string
  target: number
  created: number
  attempts: number
  skipped: number
  failed: number
  max_attempts: number
  status: string
  step: string
  error: string
  last_email: string
  last_error: string
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
  email_run?: UserEmailRun
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
  target: number
  created: number
  attempts: number
  skipped: number
  failed: number
  max_attempts: number
  emails: string[]
  errors: string[]
  summary: UserSummary
}

type EmailRefreshSnapshot = {
  emailCreated: number
  phoneSuccess: number
  loginSuccess: number
}

type CustomSub2APISettings = {
  enabled: boolean
  baseURL: string
  apiKey: string
  groupIDs: string
  proxyID: string
}

const AUTHORIZATION_STORAGE = 'sub2api-panel:phone-register-authorization'
const USERNAME_STORAGE = 'sub2api-panel:phone-register-username'
const PASSWORD_STORAGE = 'sub2api-panel:phone-register-password'
const HERO_KEY_STORAGE = 'sub2api-panel:herosms-api-key'
const DUCK_AUTH_STORAGE = 'sub2api-panel:duck-authorization'
const REGISTER_COUNT_STORAGE = 'sub2api-panel:user-register-count'
const EMAIL_COUNT_STORAGE = 'sub2api-panel:user-email-count'
const CUSTOM_SUB2API_ENABLED_STORAGE = 'sub2api-panel:custom-sub2api-enabled'
const CUSTOM_SUB2API_BASE_URL_STORAGE = 'sub2api-panel:custom-sub2api-base-url'
const CUSTOM_SUB2API_API_KEY_STORAGE = 'sub2api-panel:custom-sub2api-api-key'
const CUSTOM_SUB2API_GROUPS_STORAGE = 'sub2api-panel:custom-sub2api-groups'
const CUSTOM_SUB2API_PROXY_ID_STORAGE = 'sub2api-panel:custom-sub2api-proxy-id'
const EMAIL_PAGE_SIZE = 10
const inputClass =
  'rounded-md border border-warmgray-200 bg-white text-warmgray-900 transition-colors placeholder:text-warmgray-400 outline-none focus:border-coral-500 focus:outline-none focus:ring-0 focus-visible:outline-none focus-visible:ring-0'

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

function emailRunIsActive(run?: UserEmailRun) {
  return run?.status === 'running'
}

function dashboardNeedsRefresh(dashboard: UserDashboard) {
  return runIsActive(dashboard.run) || emailRunIsActive(dashboard.email_run)
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

function usernameStorageKey(base: string, username: string) {
  const name = username.trim()
  return name ? `${base}:${name}` : base
}

function boolFromStorage(value: string | null) {
  return value === '1' || value === 'true'
}

function customSub2APISettingsFor(username?: string): CustomSub2APISettings {
  const userKey = (base: string) => (username ? usernameStorageKey(base, username) : base)
  return {
    enabled: boolFromStorage(
      localStorage.getItem(userKey(CUSTOM_SUB2API_ENABLED_STORAGE)) ??
        localStorage.getItem(CUSTOM_SUB2API_ENABLED_STORAGE),
    ),
    baseURL:
      localStorage.getItem(userKey(CUSTOM_SUB2API_BASE_URL_STORAGE)) ||
      localStorage.getItem(CUSTOM_SUB2API_BASE_URL_STORAGE) ||
      '',
    apiKey:
      localStorage.getItem(userKey(CUSTOM_SUB2API_API_KEY_STORAGE)) ||
      localStorage.getItem(CUSTOM_SUB2API_API_KEY_STORAGE) ||
      '',
    groupIDs:
      localStorage.getItem(userKey(CUSTOM_SUB2API_GROUPS_STORAGE)) ||
      localStorage.getItem(CUSTOM_SUB2API_GROUPS_STORAGE) ||
      '',
    proxyID:
      localStorage.getItem(userKey(CUSTOM_SUB2API_PROXY_ID_STORAGE)) ||
      localStorage.getItem(CUSTOM_SUB2API_PROXY_ID_STORAGE) ||
      '',
  }
}

function parseGroupIDs(value: string) {
  const seen = new Set<number>()
  const ids: number[] = []
  value
    .split(/[\s,，;；]+/)
    .map((item) => Number.parseInt(item, 10))
    .filter((item) => Number.isFinite(item) && item > 0)
    .forEach((item) => {
      if (seen.has(item)) return
      seen.add(item)
      ids.push(item)
    })
  return ids
}

function saveCustomSub2APISettings(settings: CustomSub2APISettings, user?: RegisterUser) {
  const enabled = settings.enabled ? '1' : '0'
  const normalizedGroups = parseGroupIDs(settings.groupIDs).join(',')
  const pairs = [
    [CUSTOM_SUB2API_ENABLED_STORAGE, enabled],
    [CUSTOM_SUB2API_BASE_URL_STORAGE, settings.baseURL.trim()],
    [CUSTOM_SUB2API_API_KEY_STORAGE, settings.apiKey.trim()],
    [CUSTOM_SUB2API_GROUPS_STORAGE, normalizedGroups],
    [CUSTOM_SUB2API_PROXY_ID_STORAGE, settings.proxyID.trim()],
  ] as const
  pairs.forEach(([key, value]) => {
    localStorage.setItem(key, value)
    if (user) {
      localStorage.setItem(userStorageKey(key, user), value)
    }
  })
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
    emailCreated: dashboard.email_run?.created ?? 0,
    phoneSuccess: dashboard.run?.phone_success_count ?? 0,
    loginSuccess: Math.max(
      dashboard.run?.login_success_count ?? 0,
      dashboard.login_summary?.success ?? 0,
    ),
  }
}

function emailRefreshNeeded(previous: EmailRefreshSnapshot, next: EmailRefreshSnapshot) {
  return (
    next.emailCreated > previous.emailCreated ||
    next.phoneSuccess > previous.phoneSuccess ||
    next.loginSuccess > previous.loginSuccess
  )
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
  const [loginPassword, setLoginPassword] = useState(
    () => localStorage.getItem(PASSWORD_STORAGE) || '',
  )
  const [apiKey, setApiKey] = useState(() => localStorage.getItem(HERO_KEY_STORAGE) || '')
  const [duckAuth, setDuckAuth] = useState(() => localStorage.getItem(DUCK_AUTH_STORAGE) || '')
  const [registerCount, setRegisterCount] = useState(
    () => localStorage.getItem(REGISTER_COUNT_STORAGE) || '1',
  )
  const [emailCount, setEmailCount] = useState(() => localStorage.getItem(EMAIL_COUNT_STORAGE) || '1')
  const [customSub2APIEnabled, setCustomSub2APIEnabled] = useState(
    () => customSub2APISettingsFor().enabled,
  )
  const [customSub2APICollapsed, setCustomSub2APICollapsed] = useState(
    () => !customSub2APISettingsFor().enabled,
  )
  const [customSub2APIBaseURL, setCustomSub2APIBaseURL] = useState(
    () => customSub2APISettingsFor().baseURL,
  )
  const [customSub2APIKey, setCustomSub2APIKey] = useState(
    () => customSub2APISettingsFor().apiKey,
  )
  const [customSub2APIGroups, setCustomSub2APIGroups] = useState(
    () => customSub2APISettingsFor().groupIDs,
  )
  const [customSub2APIProxyID, setCustomSub2APIProxyID] = useState(
    () => customSub2APISettingsFor().proxyID,
  )
  const [dashboard, setDashboard] = useState<UserDashboard | null>(null)
  // booting：有缓存会话时，先显示加载视图自动恢复，避免切回本页时闪现登录表单。
  const [booting, setBooting] = useState(
    () => !!(localStorage.getItem(AUTHORIZATION_STORAGE) || '').trim(),
  )
  const [emailList, setEmailList] = useState<UserEmailListResponse>(emptyEmailList)
  const [emailPage, setEmailPage] = useState(1)
  const [emailSearch, setEmailSearch] = useState('')
  const [emailQuery, setEmailQuery] = useState('')
  const [otpEmail, setOtpEmail] = useState('')
  const [newPassword, setNewPassword] = useState('')
  const [emailLoading, setEmailLoading] = useState(false)
  const [message, setMessage] = useState('请输入 username 进入注册控制台')
  const [messageType, setMessageType] = useState<'info' | 'error' | 'ok'>('info')
  const [loading, setLoading] = useState(false)
  const timer = useRef<number | null>(null)
  const emailPageRef = useRef(1)
  const emailQueryRef = useRef('')
  const emailRefreshSnapshotRef = useRef<EmailRefreshSnapshot>({
    emailCreated: 0,
    phoneSuccess: 0,
    loginSuccess: 0,
  })
  const autoLoginRef = useRef(false)
  const dashRef = useRef<HTMLElement | null>(null)

  const user = dashboard?.user
  const run = dashboard?.run
  const emailRun = dashboard?.email_run
  const summary = dashboard?.summary
  const isRunning = runIsActive(run)
  const isEmailGenerating = emailRunIsActive(emailRun)
  const passwordDirty = newPassword.trim().length > 0
  const userInfoDirty = otpEmail.trim() !== (user?.otp_email || '').trim() || passwordDirty
  const customGroupIDs = useMemo(() => parseGroupIDs(customSub2APIGroups), [customSub2APIGroups])
  const phoneProcessed = (run?.phone_success_count ?? 0) + (run?.phone_failure_count ?? 0)
  const phoneProgress = percent(phoneProcessed, run?.target_count ?? 0)
  const phoneCodeBadge =
    (run?.phone_code_attempt ?? 0) > 0 && (run?.phone_code_max_attempts ?? 0) > 0
      ? `获取 ${run?.phone_code_attempt}/${run?.phone_code_max_attempts}`
      : ''
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
  const loginEmailCodeBadge =
    (run?.login_email_code_attempt ?? 0) > 0 && (run?.login_email_code_max ?? 0) > 0
      ? `获取 ${run?.login_email_code_attempt}/${run?.login_email_code_max}`
      : ''
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

  useEffect(() => {
    if (dashboard) return
    const name = username.trim()
    if (!name) return
    const cachedPassword =
      localStorage.getItem(usernameStorageKey(PASSWORD_STORAGE, name)) ||
      localStorage.getItem(PASSWORD_STORAGE)
    if (cachedPassword) {
      setLoginPassword(cachedPassword)
    }
  }, [dashboard, username])

  // 看板入场：登录→看板时用 GSAP timeline 编排侧栏与右侧区域滑入，替代硬切换。
  // 依赖布尔 showingDashboard（而非 dashboard 本身），确保 2.5s 轮询刷新不会重放动画。
  // 登录 / 加载视图各自在挂载时自带入场动画（见 EntryForm / EntryLoading）。
  const showingDashboard = dashboard !== null
  useLayoutEffect(() => {
    if (!showingDashboard) return
    const root = dashRef.current
    if (!root) return
    const aside = root.querySelector('[data-enter-aside]')
    const regions = root.querySelectorAll('[data-enter]')
    const ctx = gsap.context(() => {
      const tl = gsap.timeline({ defaults: { ease: 'power3.out' } })
      if (aside) {
        tl.fromTo(
          aside,
          { autoAlpha: 0, x: -28 },
          { autoAlpha: 1, x: 0, duration: 0.6, clearProps: 'transform' },
          0,
        )
      }
      if (regions.length) {
        tl.fromTo(
          regions,
          { autoAlpha: 0, y: 22, scale: 0.985 },
          {
            autoAlpha: 1,
            y: 0,
            scale: 1,
            duration: 0.6,
            stagger: 0.09,
            clearProps: 'transform',
          },
          0.08,
        )
      }
    }, root)
    return () => ctx.revert()
  }, [showingDashboard])

  const login = async (nextUsername = username, nextPassword = loginPassword) => {
    const name = nextUsername.trim()
    if (!name) {
      setMessage('请输入 username')
      setMessageType('error')
      return
    }
    const password = (
      nextPassword.trim() ||
      localStorage.getItem(usernameStorageKey(PASSWORD_STORAGE, name)) ||
      localStorage.getItem(PASSWORD_STORAGE) ||
      ''
    ).trim()
    if (!password) {
      setMessage('请输入密码')
      setMessageType('error')
      return
    }
    setLoading(true)
    try {
      const data = await requestJSON<UserDashboard>('/api/phone-register/user/login', {
        method: 'POST',
        body: JSON.stringify({ username: name, password }),
      })
      localStorage.setItem(AUTHORIZATION_STORAGE, name)
      localStorage.setItem(USERNAME_STORAGE, name)
      localStorage.setItem(PASSWORD_STORAGE, password)
      localStorage.setItem(usernameStorageKey(PASSWORD_STORAGE, name), password)
      setDashboard(data)
      emailRefreshSnapshotRef.current = emailRefreshSnapshot(data)
      setApiKey(localStorage.getItem(userStorageKey(HERO_KEY_STORAGE, data.user)) || '')
      setDuckAuth(localStorage.getItem(userStorageKey(DUCK_AUTH_STORAGE, data.user)) || '')
      setRegisterCount(
        localStorage.getItem(userStorageKey(REGISTER_COUNT_STORAGE, data.user)) || '1',
      )
      setEmailCount(localStorage.getItem(userStorageKey(EMAIL_COUNT_STORAGE, data.user)) || '1')
      const customSettings = customSub2APISettingsFor(data.user.username)
      setCustomSub2APIEnabled(customSettings.enabled)
      setCustomSub2APICollapsed(!customSettings.enabled)
      setCustomSub2APIBaseURL(customSettings.baseURL)
      setCustomSub2APIKey(customSettings.apiKey)
      setCustomSub2APIGroups(customSettings.groupIDs)
      setCustomSub2APIProxyID(customSettings.proxyID)
      setLoginPassword(password)
      setOtpEmail(data.user.otp_email || '')
      setNewPassword('')
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
    if (!cached.trim()) {
      setBooting(false)
      return
    }
    autoLoginRef.current = true
    const cachedPassword =
      localStorage.getItem(usernameStorageKey(PASSWORD_STORAGE, cached)) ||
      localStorage.getItem(PASSWORD_STORAGE) ||
      ''
    if (!cachedPassword.trim()) {
      setBooting(false)
      return
    }
    setUsername(cached)
    setLoginPassword(cachedPassword)
    void login(cached, cachedPassword).finally(() => setBooting(false))
  }, [dashboard, loading])

  const generateEmails = async () => {
    if (!user) return
    const count = clampCount(emailCount, 50)
    if (user.is_duck && !duckAuth.trim()) {
      setMessage('请输入 Duck Authorization')
      setMessageType('error')
      return
    }
    setLoading(true)
    setMessage(`正在创建 ${count} 个 Duck 邮箱...`)
    setMessageType('info')
    try {
      scheduleRefresh(user.id)
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
      const latestDashboard = await refreshDashboard(user.id, false, true)
      setDashboard((current) =>
        current ? { ...current, ...latestDashboard, summary: result.summary } : current,
      )
      setEmailSearch('')
      setEmailQuery('')
      emailQueryRef.current = ''
      await refreshEmails(user.id, 1, '')
      const failed = result.failed
      const target = result.target || count
      const incomplete = result.created < target
      if (incomplete) {
        const firstError = result.errors[0] ? `，首个失败原因：${result.errors[0]}` : ''
        setMessage(`请求创建 ${target} 个新邮箱，已创建 ${result.created} 个，跳过 ${result.skipped} 个，失败 ${failed} 个${firstError}`)
        setMessageType('error')
      } else {
        const failedText = failed > 0 ? `，失败尝试 ${failed} 次` : ''
        setMessage(`已创建 ${result.created} 个新邮箱，跳过 ${result.skipped} 个已存在邮箱${failedText}`)
        setMessageType(result.created > 0 ? 'ok' : 'info')
      }
    } catch (err) {
      renderError(err)
      void refreshDashboard(user.id, false, true).catch(() => undefined)
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
    if (customSub2APIEnabled) {
      if (!customSub2APIBaseURL.trim()) {
        setMessage('请输入自定义 Sub2API 地址')
        setMessageType('error')
        return
      }
      if (!customSub2APIKey.trim()) {
        setMessage('请输入自定义 Sub2API 密钥')
        setMessageType('error')
        return
      }
      if (customGroupIDs.length === 0) {
        setMessage('请输入自定义上传分组')
        setMessageType('error')
        return
      }
    }
    setLoading(true)
    setMessage(`正在启动 ${count} 个账号的注册任务...`)
    setMessageType('info')
    try {
      localStorage.setItem(HERO_KEY_STORAGE, apiKey.trim())
      localStorage.setItem(REGISTER_COUNT_STORAGE, String(count))
      localStorage.setItem(userStorageKey(HERO_KEY_STORAGE, user), apiKey.trim())
      localStorage.setItem(userStorageKey(REGISTER_COUNT_STORAGE, user), String(count))
      const customSettings = {
        enabled: customSub2APIEnabled,
        baseURL: customSub2APIBaseURL,
        apiKey: customSub2APIKey,
        groupIDs: customSub2APIGroups,
        proxyID: customSub2APIProxyID,
      }
      saveCustomSub2APISettings(customSettings, user)
      const data = await requestJSON<UserDashboard>('/api/phone-register/user/register/start', {
        method: 'POST',
        body: JSON.stringify({
          user_id: user.id,
          api_key: apiKey.trim(),
          count,
          custom_sub2api: customSub2APIEnabled
            ? {
                enabled: true,
                base_url: customSub2APIBaseURL.trim(),
                api_key: customSub2APIKey.trim(),
                group_ids: customGroupIDs,
                proxy_id: customSub2APIProxyID.trim(),
              }
            : { enabled: false },
        }),
      })
      setDashboard(data)
      emailRefreshSnapshotRef.current = emailRefreshSnapshot(data)
      setCustomSub2APIGroups(customGroupIDs.join(','))
      setMessage(customSub2APIEnabled ? '任务已启动，将上传到自定义 Sub2API' : '任务已启动')
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
    const nextPassword = newPassword.trim()
    if (nextPassword && nextPassword.length < 6) {
      setMessage('新密码至少 6 位')
      setMessageType('error')
      return
    }
    setLoading(true)
    setMessage('正在保存用户信息...')
    setMessageType('info')
    try {
      const data = await requestJSON<UserDashboard>('/api/phone-register/user/update', {
        method: 'POST',
        body: JSON.stringify({
          user_id: user.id,
          otp_email: nextOtpEmail,
          password: nextPassword,
          current_password: nextPassword ? loginPassword.trim() : '',
        }),
      })
      setDashboard(data)
      setOtpEmail(data.user.otp_email || '')
      if (nextPassword) {
        localStorage.setItem(PASSWORD_STORAGE, nextPassword)
        localStorage.setItem(userStorageKey(PASSWORD_STORAGE, data.user), nextPassword)
        setLoginPassword(nextPassword)
        setNewPassword('')
      }
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
    const doReset = () => {
      clearTimer()
      localStorage.removeItem(AUTHORIZATION_STORAGE)
      setDashboard(null)
      setEmailList(emptyEmailList)
      setEmailPage(1)
      setEmailSearch('')
      setEmailQuery('')
      setOtpEmail('')
      setNewPassword('')
      setLoginPassword(localStorage.getItem(PASSWORD_STORAGE) || '')
      emailRefreshSnapshotRef.current = { emailCreated: 0, phoneSuccess: 0, loginSuccess: 0 }
      setMessage('请输入 username 进入注册控制台')
      setMessageType('info')
    }
    // 先播放看板淡出，再切回登录视图（登录卡由 EntryForm 挂载时自带入场动画）。
    const root = dashRef.current
    if (!root) {
      doReset()
      return
    }
    gsap.to(root, {
      autoAlpha: 0,
      y: 10,
      scale: 0.985,
      duration: 0.28,
      ease: 'power2.in',
      onComplete: doReset,
    })
  }

  if (!dashboard) {
    return (
      <main className="relative grid min-h-0 flex-1 place-items-center overflow-hidden">
        <EntryBackdrop />
        {booting ? (
          <EntryLoading username={username} />
        ) : (
          <EntryForm
            username={username}
            password={loginPassword}
            onUsernameChange={setUsername}
            onPasswordChange={setLoginPassword}
            onSubmit={() => void login()}
            loading={loading}
            message={message}
            messageType={messageType}
          />
        )}
      </main>
    )
  }

  return (
    <main ref={dashRef} className="grid min-h-0 flex-1 grid-cols-12 gap-4 overflow-hidden">
      <aside
        data-enter-aside
        className="col-span-12 flex min-h-0 flex-col rounded-2xl border border-warmgray-200/70 bg-canvas shadow-card lg:col-span-4 xl:col-span-3"
      >
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
              className="group flex h-9 items-center gap-1.5 rounded-md border border-warmgray-200 bg-white px-3 text-[12px] font-semibold text-warmgray-600 transition-all hover:border-warmgray-300 hover:bg-warmgray-50 hover:text-warmgray-800 active:scale-95 disabled:cursor-not-allowed disabled:opacity-50 disabled:hover:bg-white"
              type="button"
              onClick={logout}
              disabled={loading || isRunning || isEmailGenerating}
              title={isRunning || isEmailGenerating ? '任务运行中，无法切换用户' : '切换用户'}
            >
              <svg
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                strokeWidth={2}
                strokeLinecap="round"
                strokeLinejoin="round"
                className="h-3.5 w-3.5 transition-transform duration-300 group-hover:rotate-180"
              >
                <path d="M7 16V4m0 0L3.5 7.5M7 4l3.5 3.5" />
                <path d="M17 8v12m0 0l3.5-3.5M17 20l-3.5-3.5" />
              </svg>
              切换
            </button>
          </div>
        </div>

        <div className="min-h-0 flex-1 overflow-auto px-5 py-4">
          <div className="grid gap-5">
            <div className="grid grid-cols-3 gap-2">
              <MiniStat label="可用邮箱" value={summary?.email_unused ?? 0} />
              <MiniStat label="成功账号" value={summary?.account_success ?? 0} />
              <MiniStat label="失败账号" value={summary?.account_failed ?? 0} />
            </div>

            <ControlGroup
              title="用户信息"
              description="接收邮箱用于当前用户注册和登录阶段自动读取验证码。"
            >
              <label className="grid gap-1.5 text-[12px] font-medium text-warmgray-600">
                接收验证码邮箱
                <input
                  className={`${inputClass} h-10 px-3 text-[13px]`}
                  type="email"
                  value={otpEmail}
                  onChange={(event) => setOtpEmail(event.target.value)}
                  disabled={loading}
                  autoComplete="off"
                  placeholder="例如 user-otp@example.com"
                />
              </label>
              <label className="mt-3 grid gap-1.5 text-[12px] font-medium text-warmgray-600">
                新登录密码
                <input
                  className={`${inputClass} h-10 px-3 text-[13px]`}
                  type="password"
                  value={newPassword}
                  onChange={(event) => setNewPassword(event.target.value)}
                  disabled={loading}
                  autoComplete="new-password"
                  placeholder="留空则不修改"
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
                    className={`${inputClass} h-9 w-20 px-2 text-[13px]`}
                    type="number"
                    min={1}
                    max={50}
                    value={emailCount}
                    onChange={(event) => setEmailCount(event.target.value)}
                    disabled={loading || isEmailGenerating}
                  />
                }
              >
                <label className="grid gap-1.5 text-[12px] font-medium text-warmgray-600">
                  Duck Authorization
                  <input
                    className={`${inputClass} h-10 px-3 text-[13px]`}
                    type="password"
                    value={duckAuth}
                    onChange={(event) => setDuckAuth(event.target.value)}
                    disabled={loading || isEmailGenerating}
                    autoComplete="off"
                    placeholder="不需要输入 Bearer"
                  />
                </label>
                <button
                  className="mt-3 h-10 w-full rounded-md border border-coral-200 bg-coral-50 px-4 text-[13px] font-semibold text-coral-700 transition-colors hover:bg-coral-100 disabled:cursor-not-allowed disabled:opacity-50"
                  type="button"
                  onClick={() => void generateEmails()}
                  disabled={loading || isEmailGenerating}
                >
                  创建邮箱
                </button>
              </ControlGroup>
            ) : (
              <div className="border-t border-warmgray-100 pt-4 text-[12px] leading-5 text-warmgray-500">
                当前用户不是 Duck 邮箱用户，注册时会从 user_email 表中取未使用邮箱。
              </div>
            )}

            {user?.is_duck ? <EmailCreateProgressPanel run={emailRun} /> : null}

            <ControlGroup
              title="手机号注册"
              description="一个用户同一时间只运行一个手机号注册任务。"
              side={
                <input
                  className={`${inputClass} h-9 w-20 px-2 text-[13px]`}
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
                  className={`${inputClass} h-10 px-3 text-[13px]`}
                  type="password"
                  value={apiKey}
                  onChange={(event) => setApiKey(event.target.value)}
                  disabled={loading || isRunning}
                  autoComplete="off"
                  placeholder="不同使用者可各自缓存"
                />
              </label>
              <div className="mt-3 rounded-xl border border-warmgray-200 bg-white px-3 py-3">
                <div className="flex items-start justify-between gap-3">
                  <label className="flex min-w-0 items-start gap-3">
                    <input
                      className="mt-0.5 h-4 w-4 rounded border-warmgray-300 text-coral-500 focus:ring-coral-500"
                      type="checkbox"
                      checked={customSub2APIEnabled}
                      onChange={(event) => {
                        const checked = event.target.checked
                        setCustomSub2APIEnabled(checked)
                        setCustomSub2APICollapsed(!checked)
                      }}
                      disabled={loading || isRunning}
                    />
                    <span className="min-w-0">
                      <span className="block text-[12px] font-semibold text-warmgray-800">
                        使用自定义 Sub2API
                      </span>
                      <span className="mt-0.5 block text-[11px] leading-4 text-warmgray-500">
                        开启后使用下方地址、密钥、分组和代理 ID 上传。
                      </span>
                    </span>
                  </label>
                  <button
                    className="mt-0.5 shrink-0 rounded-full bg-warmgray-50 px-2.5 py-1 text-[10px] font-semibold text-warmgray-500 ring-1 ring-inset ring-warmgray-200 transition-colors hover:bg-warmgray-100 disabled:cursor-not-allowed disabled:opacity-50"
                    type="button"
                    onClick={() => setCustomSub2APICollapsed((value) => !value)}
                    disabled={!customSub2APIEnabled || loading || isRunning}
                  >
                    {customSub2APIEnabled && !customSub2APICollapsed ? '收起' : '展开'}
                  </button>
                </div>
                {customSub2APIEnabled && !customSub2APICollapsed ? (
                  <div className="mt-3 grid gap-3">
                    <label className="grid gap-1.5 text-[12px] font-medium text-warmgray-600">
                      Sub2API 地址
                      <input
                        className={`${inputClass} h-10 px-3 text-[13px]`}
                        type="url"
                        value={customSub2APIBaseURL}
                        onChange={(event) => setCustomSub2APIBaseURL(event.target.value)}
                        disabled={loading || isRunning}
                        autoComplete="off"
                        placeholder="https://sub2api.example.com"
                      />
                    </label>
                    <label className="grid gap-1.5 text-[12px] font-medium text-warmgray-600">
                      Sub2API 密钥
                      <input
                        className={`${inputClass} h-10 px-3 text-[13px]`}
                        type="password"
                        value={customSub2APIKey}
                        onChange={(event) => setCustomSub2APIKey(event.target.value)}
                        disabled={loading || isRunning}
                        autoComplete="off"
                        placeholder="自定义 x-api-key"
                      />
                    </label>
                    <label className="grid gap-1.5 text-[12px] font-medium text-warmgray-600">
                      上传分组
                      <input
                        className={`${inputClass} h-10 px-3 text-[13px]`}
                        value={customSub2APIGroups}
                        onChange={(event) => setCustomSub2APIGroups(event.target.value)}
                        disabled={loading || isRunning}
                        autoComplete="off"
                        placeholder="例如 5,8,12"
                      />
                    </label>
                    <label className="grid gap-1.5 text-[12px] font-medium text-warmgray-600">
                      代理 ID
                      <input
                        className={`${inputClass} h-10 px-3 text-[13px]`}
                        value={customSub2APIProxyID}
                        onChange={(event) => setCustomSub2APIProxyID(event.target.value)}
                        disabled={loading || isRunning}
                        autoComplete="off"
                        placeholder="可选，留空则不上传 proxy_id"
                      />
                    </label>
                  </div>
                ) : null}
              </div>
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
            done={phoneProcessed}
            total={run?.target_count ?? 0}
            detail={`${run?.phone_success_count ?? 0} 成功 / ${run?.phone_failure_count ?? 0} 失败 / 当前 ${run?.current_phone || '-'}`}
            badge={phoneCodeBadge}
            emptyText={run?.phone_done ? '手机号注册阶段已完成' : '暂无手机号注册任务'}
            logs={phoneLogs}
          />
          <StageProgressPanel
            title="登录进度"
            status={currentLoginStatus}
            tone={currentLoginTone}
            value={currentLoginProgress}
            done={currentLoginDone}
            total={currentLoginTotal}
            detail={`${currentLoginQueued} 排队 / ${currentLoginRunning} 处理中 / ${run?.login_success_count ?? 0} 成功 / ${run?.login_failed_count ?? 0} 失败`}
            badge={loginEmailCodeBadge}
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

function MiniStat({ label, value }: { label: string; value: number }) {
  return (
    <div className="rounded-xl border border-warmgray-200 bg-white px-3 py-3">
      <div className="label">{label}</div>
      <div className="num mt-1 truncate text-[18px] font-semibold text-warmgray-900">
        <AnimatedNumber value={value} format={(v) => String(Math.round(v))} />
      </div>
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
    <article
      data-enter
      className="rounded-2xl border border-warmgray-200/70 bg-canvas px-5 py-4 shadow-card"
    >
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
                <AnimatedNumber value={value} format={(v) => String(Math.round(v))} />
              </div>
            </div>
          ))}
        </div>
      </div>
    </article>
  )
}

function EmailCreateProgressPanel({ run }: { run?: UserEmailRun }) {
  const done = run?.created ?? 0
  const total = run?.target ?? 0
  const skipped = run?.skipped ?? 0
  const failed = run?.failed ?? 0
  const value = percent(done, total)
  const status = labelForStatus(run?.status || 'idle')
  const tone = toneForStatus(run?.status || 'idle')

  return (
    <div className="border-t border-warmgray-100 pt-4">
      <div className="mb-3 flex items-start justify-between gap-3">
        <div>
          <div className="text-[13px] font-semibold text-warmgray-900">邮箱创建进度</div>
          <div className="mt-1 text-[12px] leading-5 text-warmgray-500">
            重复 Duck 邮箱会跳过，不计入目标数量。
          </div>
        </div>
        <span className={`shrink-0 rounded-full px-2.5 py-1 text-[10px] font-semibold ring-1 ring-inset ${tone}`}>
          {status}
        </span>
      </div>
      <div className="rounded-xl border border-warmgray-200 bg-white px-3 py-3">
        <div className="mb-2 flex items-center justify-between gap-3 text-[12px]">
          <div className="num font-semibold text-warmgray-800">
            <AnimatedNumber value={done} format={(v) => String(Math.round(v))} />
            <span className="text-warmgray-400">/{total}</span>
          </div>
          <div className="truncate text-warmgray-500">
            跳过 {skipped} 个 · 失败 {failed} 次
          </div>
        </div>
        <ProgressMeter value={value} />
        <div className="mt-2 truncate text-[12px] leading-5 text-warmgray-600">
          {run?.step || '暂无邮箱创建任务'}
        </div>
        {run?.last_error ? (
          <div className="mt-1 truncate text-[11px] text-rose-600">{run.last_error}</div>
        ) : null}
      </div>
    </div>
  )
}

function StageProgressPanel({
  title,
  status,
  tone,
  value,
  done,
  total,
  detail,
  badge,
  logs,
  emptyText,
}: {
  title: string
  status: string
  tone: string
  value: number
  done: number
  total: number
  detail: string
  badge?: string
  logs: UserRunLog[]
  emptyText: string
}) {
  const visibleLogs = logs.slice(-6)
  return (
    <article
      data-enter
      className="rounded-2xl border border-warmgray-200/70 bg-canvas px-5 py-4 shadow-card"
    >
      <div className="flex items-start justify-between gap-3">
        <div>
          <div className="text-[13px] font-semibold text-warmgray-900">{title}</div>
          <div className="mt-1 text-[12px] text-warmgray-500">{detail}</div>
        </div>
        <div className="flex shrink-0 items-center gap-2">
          {badge ? (
            <span className="num rounded-full bg-white px-2.5 py-1 text-[10px] font-semibold text-warmgray-600 ring-1 ring-inset ring-warmgray-200">
              {badge}
            </span>
          ) : null}
          <span className={`rounded-full px-2.5 py-1 text-[10px] font-semibold ring-1 ring-inset ${tone}`}>
            {status}
          </span>
        </div>
      </div>
      <div className="num mt-5 text-[30px] font-semibold leading-none tracking-tightish text-warmgray-900">
        <AnimatedNumber value={done} format={(v) => String(Math.round(v))} />
        <span className="text-warmgray-400">/{total}</span>
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
  const barRef = useAnimatedWidth<HTMLDivElement>(value, { duration: 0.8, ease: 'power3.out' })
  return (
    <div className={className}>
      <div className="mb-1.5 flex items-center justify-between text-[11px] text-warmgray-500">
        <span>进度</span>
        <span className="num">
          <AnimatedNumber value={value} format={(v) => `${Math.round(v)}%`} duration={0.8} />
        </span>
      </div>
      <div className="h-2.5 overflow-hidden rounded-full bg-warmgray-100">
        <div ref={barRef} className="h-full rounded-full bg-coral-500" />
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
  // 仅在翻页/搜索时重放行错峰；轮询刷新同页（trigger 不变）不会抖动。
  const rowsRef = useReStagger<HTMLTableSectionElement>(`${emailPage}|${emailQuery}`, {
    selector: '[data-email-row]',
    distance: 8,
    duration: 0.4,
    stagger: 0.03,
    delay: 0.1,
  })

  return (
    <section
      data-enter
      className="flex min-h-0 flex-1 flex-col overflow-hidden rounded-2xl border border-warmgray-200/70 bg-canvas shadow-card"
    >
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
              className={`${inputClass} h-9 w-[240px] px-3 text-[13px]`}
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
          <tbody ref={rowsRef} className="divide-y divide-warmgray-100">
            {emailList.items.length ? (
              emailList.items.map((item) => {
                const status = item.account_status || (item.used_at ? 'used' : 'unused')
                return (
                  <tr
                    key={item.id}
                    data-email-row
                    className="text-[12px] text-warmgray-700 transition-colors hover:bg-cream/60"
                  >
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

function PhoneMark({ className = 'h-[18px] w-[18px]' }: { className?: string }) {
  return (
    <svg
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth={2}
      strokeLinecap="round"
      strokeLinejoin="round"
      className={className}
    >
      <rect x="7" y="3" width="10" height="18" rx="2.5" />
      <path d="M11 18h2" />
    </svg>
  )
}

function Spinner({ className = 'h-4 w-4' }: { className?: string }) {
  return (
    <svg className={`${className} animate-spin`} viewBox="0 0 24 24" fill="none">
      <circle className="opacity-30" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth={3} />
      <path
        className="opacity-90"
        d="M12 2a10 10 0 0 1 10 10"
        stroke="currentColor"
        strokeWidth={3}
        strokeLinecap="round"
      />
    </svg>
  )
}

// EntryBackdrop — 入口视图的环境光晕背景：几枚模糊暖色光斑缓慢漂浮（repeatRefresh 让每轮重取随机目标）。
function EntryBackdrop() {
  const ref = useRef<HTMLDivElement>(null)
  useLayoutEffect(() => {
    const el = ref.current
    if (!el) return
    const ctx = gsap.context(() => {
      gsap.from(el, { autoAlpha: 0, duration: 1.2, ease: 'power2.out' })
      gsap.to('[data-orb]', {
        xPercent: 'random(-15, 15)',
        yPercent: 'random(-15, 15)',
        scale: 'random(0.9, 1.15)',
        duration: 9,
        ease: 'sine.inOut',
        repeat: -1,
        yoyo: true,
        repeatRefresh: true,
        stagger: { each: 0.6, from: 'random' },
      })
    }, el)
    return () => ctx.revert()
  }, [])
  return (
    <div ref={ref} aria-hidden className="pointer-events-none absolute inset-0 overflow-hidden">
      <div data-orb className="absolute -left-20 top-2 h-72 w-72 rounded-full bg-coral-200/45 blur-3xl" />
      <div data-orb className="absolute -right-12 top-1/4 h-80 w-80 rounded-full bg-amber-100/55 blur-3xl" />
      <div data-orb className="absolute -bottom-12 left-1/3 h-64 w-64 rounded-full bg-coral-100/50 blur-3xl" />
      <div data-orb className="absolute bottom-8 right-1/4 h-44 w-44 rounded-full bg-moss/10 blur-3xl" />
    </div>
  )
}

// EntryForm — 玻璃拟态登录卡，挂载时用 timeline 编排「模糊聚焦 + 子元素错峰」入场，图标光环呼吸。
function EntryForm({
  username,
  password,
  onUsernameChange,
  onPasswordChange,
  onSubmit,
  loading,
  message,
  messageType,
}: {
  username: string
  password: string
  onUsernameChange: (value: string) => void
  onPasswordChange: (value: string) => void
  onSubmit: () => void
  loading: boolean
  message: string
  messageType: 'info' | 'error' | 'ok'
}) {
  const ref = useRef<HTMLElement>(null)
  useLayoutEffect(() => {
    const el = ref.current
    if (!el) return
    const ctx = gsap.context(() => {
      const tl = gsap.timeline({ defaults: { ease: 'power3.out' } })
      tl.fromTo(
        el,
        { autoAlpha: 0, y: 30, scale: 0.94, filter: 'blur(12px)' },
        { autoAlpha: 1, y: 0, scale: 1, filter: 'blur(0px)', duration: 0.85 },
      ).fromTo(
        '[data-entry-item]',
        { autoAlpha: 0, y: 16 },
        { autoAlpha: 1, y: 0, duration: 0.5, stagger: 0.08 },
        '-=0.45',
      )
      gsap.to('[data-mark-ring]', {
        scale: 1.18,
        autoAlpha: 0.35,
        duration: 1.6,
        ease: 'sine.inOut',
        repeat: -1,
        yoyo: true,
      })
    }, el)
    return () => ctx.revert()
  }, [])
  return (
    <section
      ref={ref}
      className="relative z-10 w-full max-w-[440px] overflow-hidden rounded-[26px] border border-white/60 bg-canvas/85 p-8 shadow-[0_24px_70px_-28px_rgba(31,30,28,0.35)] backdrop-blur-xl"
    >
      <div
        aria-hidden
        className="pointer-events-none absolute -top-24 left-1/2 h-48 w-56 -translate-x-1/2 rounded-full bg-coral-200/40 blur-3xl"
      />
      <div data-entry-item className="relative mb-7 flex items-center gap-3">
        <div className="relative grid h-12 w-12 place-items-center">
          <span data-mark-ring className="absolute inset-0 rounded-2xl bg-coral-300/40 blur-[2px]" />
          <span className="relative grid h-12 w-12 place-items-center rounded-2xl bg-gradient-to-br from-coral-400 to-coral-600 text-white shadow-lg shadow-coral-500/30">
            <PhoneMark className="h-5 w-5" />
          </span>
        </div>
        <div>
          <h2 className="text-[19px] font-semibold tracking-tightish text-warmgray-900">
            手机号注册控制台
          </h2>
          <p className="mt-0.5 text-[12px] text-warmgray-500">输入 username 进入专属看板</p>
        </div>
      </div>
      <label data-entry-item className="grid gap-1.5 text-[12px] font-medium text-warmgray-600">
        Username
        <input
          className={`${inputClass} h-11 px-3.5 text-[14px]`}
          value={username}
          onChange={(event) => onUsernameChange(event.target.value)}
          onKeyDown={(event) => {
            if (event.key === 'Enter') onSubmit()
          }}
          autoComplete="off"
          placeholder="输入 username"
          autoFocus
        />
      </label>
      <label data-entry-item className="mt-3 grid gap-1.5 text-[12px] font-medium text-warmgray-600">
        Password
        <input
          className={`${inputClass} h-11 px-3.5 text-[14px]`}
          type="password"
          value={password}
          onChange={(event) => onPasswordChange(event.target.value)}
          onKeyDown={(event) => {
            if (event.key === 'Enter') onSubmit()
          }}
          autoComplete="current-password"
          placeholder="输入密码"
        />
      </label>
      <button
        data-entry-item
        className="group mt-4 flex h-11 w-full items-center justify-center gap-2 rounded-xl bg-coral-500 px-4 text-[13px] font-semibold text-white shadow-lg shadow-coral-500/25 transition-all hover:bg-coral-600 hover:shadow-coral-500/35 active:scale-[0.98] disabled:cursor-not-allowed disabled:opacity-60"
        type="button"
        onClick={onSubmit}
        disabled={loading}
      >
        {loading ? (
          <>
            <Spinner /> 进入中…
          </>
        ) : (
          <>
            进入
            <svg
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              strokeWidth={2}
              strokeLinecap="round"
              strokeLinejoin="round"
              className="h-4 w-4 transition-transform group-hover:translate-x-0.5"
            >
              <path d="M5 12h14M13 6l6 6-6 6" />
            </svg>
          </>
        )}
      </button>
      <div data-entry-item>
        <MessageLine message={message} type={messageType} className="mt-3" />
      </div>
      <p
        data-entry-item
        className="mt-5 border-t border-warmgray-100 pt-3 text-[11px] leading-4 text-warmgray-400"
      >
        会话信息缓存在本浏览器，下次进入将自动恢复。
      </p>
    </section>
  )
}

// EntryLoading — 自动恢复会话时的加载视图：旋转光环 + 呼吸光晕 + 渐变图标，避免切回本页时闪现登录表单。
function EntryLoading({ username }: { username: string }) {
  const ref = useRef<HTMLDivElement>(null)
  useLayoutEffect(() => {
    const el = ref.current
    if (!el) return
    const ctx = gsap.context(() => {
      gsap.from(el, { autoAlpha: 0, scale: 0.9, duration: 0.55, ease: 'power3.out' })
      gsap.to('[data-load-ring]', { rotation: 360, duration: 1.1, ease: 'none', repeat: -1 })
      gsap.to('[data-load-pulse]', {
        scale: 1.25,
        autoAlpha: 0.25,
        duration: 1,
        ease: 'sine.inOut',
        repeat: -1,
        yoyo: true,
      })
    }, el)
    return () => ctx.revert()
  }, [])
  return (
    <div ref={ref} className="relative z-10 flex flex-col items-center gap-5">
      <div className="relative grid h-16 w-16 place-items-center">
        <span data-load-pulse className="absolute inset-0 rounded-full bg-coral-200/55 blur-md" />
        <span data-load-ring className="absolute inset-0 rounded-full border-[2.5px] border-coral-200 border-t-coral-500" />
        <span className="relative grid h-11 w-11 place-items-center rounded-xl bg-gradient-to-br from-coral-400 to-coral-600 text-white shadow-lg shadow-coral-500/30">
          <PhoneMark className="h-5 w-5" />
        </span>
      </div>
      <div className="text-center">
        <div className="text-[14px] font-semibold text-warmgray-900">正在进入控制台</div>
        <div className="mt-1 text-[12px] text-warmgray-500">
          {username ? `恢复 ${username} 的会话…` : '加载中…'}
        </div>
      </div>
    </div>
  )
}
