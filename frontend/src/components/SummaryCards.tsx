import type { HistoricalSummary, TodaySummary } from '@/types'
import { formatCompact, formatCompactCurrency } from '@/lib/format'

type Props = {
  today: TodaySummary | null
  historical: HistoricalSummary | null
}

type StatValue = {
  value: string
  unit: string
  accent: string
  size?: 'sm' | 'md'
}

type Card = {
  label: string
  primary: StatValue
  secondary?: StatValue
  badge?: string
}

const DASH = '—'

function compact(n: number, available: boolean): string {
  if (!available) return DASH
  return formatCompact(n)
}

function compactCurrency(n: number, available: boolean): string {
  if (!available) return DASH
  return formatCompactCurrency(n)
}

function buildCards(today: TodaySummary, hist: HistoricalSummary | null): Card[] {
  const histOK = !!hist && hist.enabled

  return [
    {
      label: '今日',
      primary: {
        value: formatCompact(today.total_tokens),
        unit: 'Token',
        accent: 'text-coral-500',
      },
      secondary: {
        value: formatCompactCurrency(today.total_cost),
        unit: '消费',
        accent: 'text-moss',
      },
    },
    {
      label: '累计',
      badge: histOK ? undefined : '未启用',
      primary: {
        value: compact(hist?.total_tokens ?? 0, histOK),
        unit: 'Token',
        accent: 'text-coral-600',
      },
      secondary: {
        value: compactCurrency(hist?.total_cost ?? 0, histOK),
        unit: '消费',
        accent: 'text-moss',
      },
    },
    {
      label: '请求',
      badge: histOK ? undefined : '累计未启用',
      primary: {
        value: formatCompact(today.total_requests),
        unit: '今日',
        accent: 'text-warmgray-900',
      },
      secondary: {
        value: compact(hist?.total_requests ?? 0, histOK),
        unit: '累计',
        accent: 'text-coral-600',
      },
    },
    {
      label: '数据周期',
      badge: histOK ? undefined : '未启用',
      primary: {
        value: histOK ? String(hist!.days_covered) : DASH,
        unit: '累计天数',
        accent: 'text-warmgray-900',
      },
      secondary: {
        value: histOK && hist!.since ? hist!.since : DASH,
        unit: '起始日',
        accent: 'text-coral-600',
        size: 'sm',
      },
    },
  ]
}

function StatBlock({ s }: { s: StatValue }) {
  const sizeCls = s.size === 'sm' ? 'text-[18px]' : 'text-[28px]'
  return (
    <div className="min-w-0">
      <div
        className={`num truncate font-semibold leading-none tracking-tightish ${sizeCls} ${s.accent}`}
      >
        {s.value}
      </div>
      <div className="mt-1.5 text-[11px] text-warmgray-500">{s.unit}</div>
    </div>
  )
}

export function SummaryCards({ today, historical }: Props) {
  const s = today ?? {
    total_tokens: 0,
    total_cost: 0,
    total_requests: 0,
    active_users: 0,
  }
  const cards = buildCards(s, historical)

  return (
    <div className="grid shrink-0 grid-cols-4 gap-4">
      {cards.map((c) => (
        <article
          key={c.label}
          className="flex flex-col rounded-2xl border border-warmgray-200/70 bg-canvas px-5 py-4 shadow-card"
        >
          <header className="flex shrink-0 items-center justify-between">
            <div className="text-[12px] font-medium uppercase tracking-[0.18em] text-warmgray-500">
              {c.label}
            </div>
            {c.badge ? (
              <span className="rounded-full bg-warmgray-50 px-2 py-0.5 text-[10px] text-warmgray-400">
                {c.badge}
              </span>
            ) : null}
          </header>

          <div
            className={`mt-3 flex flex-1 items-end ${
              c.secondary ? 'gap-8' : ''
            }`}
          >
            <StatBlock s={c.primary} />
            {c.secondary ? (
              <>
                <div className="h-9 w-px shrink-0 bg-warmgray-100" />
                <StatBlock s={c.secondary} />
              </>
            ) : null}
          </div>
        </article>
      ))}
    </div>
  )
}
