import type { AccountMonitor } from '@/types'
import { formatNumber } from '@/lib/format'

type Props = { data: AccountMonitor | null }

type Cell = {
  label: string
  value: number
  color: string
  bg: string
}

function buildCells(d: AccountMonitor): Cell[] {
  // 异常 = 总量 - 限流 - 可用（按用户最新口径）
  const abnormal = Math.max(0, d.total - d.rate_limited - d.available)
  return [
    {
      label: '总量',
      value: d.total,
      color: 'text-warmgray-900',
      bg: 'bg-warmgray-100',
    },
    {
      label: '可用',
      value: d.available,
      color: 'text-moss',
      bg: 'bg-moss/15',
    },
    {
      label: '限流',
      value: d.rate_limited,
      color: 'text-amber-700',
      bg: 'bg-amber-100',
    },
    {
      label: '异常',
      value: abnormal,
      color: 'text-coral-600',
      bg: 'bg-coral-100',
    },
  ]
}

function Bar({ pct, color }: { pct: number; color: string }) {
  return (
    <div className="h-1.5 w-full overflow-hidden rounded-full bg-warmgray-100">
      <div
        className={`h-1.5 rounded-full ${color} transition-[width] duration-500`}
        style={{ width: `${Math.max(0, Math.min(100, pct))}%` }}
      />
    </div>
  )
}

export function AccountMonitorCard({ data }: Props) {
  // 未配置 / 未启用
  if (!data || !data.enabled) {
    return (
      <section className="flex h-full flex-col rounded-2xl border border-warmgray-200/70 bg-canvas shadow-card">
        <header className="shrink-0 px-5 pt-4 pb-2">
          <h2 className="text-[16px] font-semibold tracking-tightish text-warmgray-900">
            账号监控
          </h2>
          <p className="mt-0.5 text-[12px] text-warmgray-500">
            指定分组下账号的健康度
          </p>
        </header>
        <div className="flex min-h-0 flex-1 flex-col items-center justify-center px-6 text-center">
          <div className="rounded-full bg-warmgray-50 px-3 py-1 text-[11px] text-warmgray-500">
            未配置
          </div>
          <p className="mt-3 text-[12px] leading-relaxed text-warmgray-400">
            在 <span className="num">config.yaml</span> 中设置
            <br />
            <span className="num text-warmgray-600">
              server.account_monitor_group_id
            </span>
            <br />
            指向 plus 分组的 group_id 即可启用
          </p>
        </div>
      </section>
    )
  }

  const cells = buildCells(data)
  const abnormal = cells[3].value
  const availPct = data.total > 0 ? (data.available / data.total) * 100 : 0
  const limitPct = data.total > 0 ? (data.rate_limited / data.total) * 100 : 0
  const abnPct = data.total > 0 ? (abnormal / data.total) * 100 : 0

  return (
    <section className="flex h-full flex-col rounded-2xl border border-warmgray-200/70 bg-canvas shadow-card">
      <header className="flex shrink-0 items-end justify-between px-5 pt-4 pb-2">
        <div>
          <h2 className="text-[16px] font-semibold tracking-tightish text-warmgray-900">
            账号监控
          </h2>
          <p className="mt-0.5 text-[12px] text-warmgray-500">
            分组 <span className="num text-warmgray-700">{data.group_name || `#${data.group_id}`}</span>
          </p>
        </div>
        <span className="num text-[11px] text-warmgray-400">
          GID {data.group_id}
        </span>
      </header>

      {/* 4 个数字 */}
      <div className="grid shrink-0 grid-cols-2 gap-2 px-5 pt-2">
        {cells.map((c) => (
          <div
            key={c.label}
            className={`rounded-xl ${c.bg} px-3 py-2.5`}
          >
            <div className="text-[11px] text-warmgray-500">{c.label}</div>
            <div className={`mt-0.5 text-[24px] font-semibold leading-none tracking-tightish ${c.color}`}>
              {formatNumber(c.value)}
            </div>
          </div>
        ))}
      </div>

      {/* 组成进度条 */}
      <div className="min-h-0 flex-1 space-y-2.5 px-5 pt-4">
        <div>
          <div className="mb-1 flex items-baseline justify-between text-[11px]">
            <span className="text-warmgray-500">可用率</span>
            <span className="num text-warmgray-700">{availPct.toFixed(1)}%</span>
          </div>
          <Bar pct={availPct} color="bg-moss" />
        </div>
        <div>
          <div className="mb-1 flex items-baseline justify-between text-[11px]">
            <span className="text-warmgray-500">限流率</span>
            <span className="num text-warmgray-700">{limitPct.toFixed(1)}%</span>
          </div>
          <Bar pct={limitPct} color="bg-amber-500" />
        </div>
        <div>
          <div className="mb-1 flex items-baseline justify-between text-[11px]">
            <span className="text-warmgray-500">异常率</span>
            <span className="num text-warmgray-700">{abnPct.toFixed(1)}%</span>
          </div>
          <Bar pct={abnPct} color="bg-coral-500" />
        </div>
      </div>

      <footer className="shrink-0 border-t border-warmgray-100 px-5 py-2 text-[11px] text-warmgray-400">
        异常 = 总量 − 限流 − 可用
      </footer>
    </section>
  )
}
