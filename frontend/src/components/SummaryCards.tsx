import type { HistoricalSummary, TodaySummary } from '@/types'
import { formatCompact, formatCompactCurrency } from '@/lib/format'
import { useBouncyStagger } from '@/hooks/useGsap'
import { AnimatedNumber } from '@/components/AnimatedNumber'

type Props = {
  today: TodaySummary | null
  historical: HistoricalSummary | null
}

type StatValue = {
  num?: number
  format?: (v: number) => string
  text?: string
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

function numericStat(
  available: boolean,
  value: number,
  format: (v: number) => string,
  unit: string,
  accent: string,
  size?: 'sm' | 'md',
): StatValue {
  return available
    ? { num: value, format, unit, accent, size }
    : { text: DASH, unit, accent, size }
}

function buildCards(today: TodaySummary, hist: HistoricalSummary | null): Card[] {
  const histOK = !!hist && hist.enabled

  return [
    {
      label: '今日',
      primary: {
        num: today.total_tokens,
        format: formatCompact,
        unit: 'Token',
        accent: 'text-coral-500',
      },
      secondary: {
        num: today.total_cost,
        format: formatCompactCurrency,
        unit: '消费',
        accent: 'text-moss',
      },
    },
    {
      label: '累计',
      badge: histOK ? undefined : '未启用',
      primary: numericStat(
        histOK,
        hist?.total_tokens ?? 0,
        formatCompact,
        'Token',
        'text-coral-600',
      ),
      secondary: numericStat(
        histOK,
        hist?.total_cost ?? 0,
        formatCompactCurrency,
        '消费',
        'text-moss',
      ),
    },
    {
      label: '请求',
      badge: histOK ? undefined : '累计未启用',
      primary: {
        num: today.total_requests,
        format: formatCompact,
        unit: '今日',
        accent: 'text-warmgray-900',
      },
      secondary: numericStat(
        histOK,
        hist?.total_requests ?? 0,
        formatCompact,
        '累计',
        'text-coral-600',
      ),
    },
    {
      label: '数据周期',
      badge: histOK ? undefined : '未启用',
      primary: histOK
        ? {
            num: hist!.days_covered,
            format: (v) => String(Math.round(v)),
            unit: '累计天数',
            accent: 'text-warmgray-900',
          }
        : { text: DASH, unit: '累计天数', accent: 'text-warmgray-900' },
      secondary: {
        text: histOK && hist!.since ? hist!.since : DASH,
        unit: '起始日',
        accent: 'text-coral-600',
        size: 'sm',
      },
    },
  ]
}

function StatBlock({ s }: { s: StatValue }) {
  const sizeCls = s.size === 'sm' ? 'text-[18px]' : 'text-[28px]'
  const cls = `num truncate font-semibold leading-none tracking-tightish ${sizeCls} ${s.accent}`
  return (
    <div className="min-w-0">
      <div className={cls}>
        {typeof s.num === 'number' && s.format ? (
          <AnimatedNumber value={s.num} format={s.format} />
        ) : (
          s.text ?? DASH
        )}
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
  const ref = useBouncyStagger<HTMLDivElement>({
    selector: '[data-summary-card]',
    delay: 0.12,
    stagger: 0.1,
    staggerFrom: 'random',
    distance: 28,
    rotation: 5,
    scaleFrom: 0.82,
    duration: 1,
    ease: 'back.out(1.9)',
  })

  return (
    <div ref={ref} className="grid shrink-0 grid-cols-4 gap-4">
      {cards.map((c) => (
        <article
          data-summary-card
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
