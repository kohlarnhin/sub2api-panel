import {
  Area,
  Bar,
  CartesianGrid,
  ComposedChart,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from 'recharts'
import type { DailyTrend } from '@/types'
import { formatCompact, formatCurrency, formatDateShort } from '@/lib/format'

type Props = { rows: DailyTrend[] }

function CustomTooltip({ active, payload, label }: any) {
  if (!active || !payload || payload.length === 0) return null
  const tokens = payload.find((p: any) => p.dataKey === 'total_tokens')?.value as number
  const cost = payload.find((p: any) => p.dataKey === 'total_cost')?.value as number
  return (
    <div className="rounded-xl border border-warmgray-200 bg-canvas px-3 py-2 shadow-card">
      <div className="num text-[11px] font-medium text-warmgray-900">{label}</div>
      <div className="mt-1 space-y-0.5">
        <div className="flex items-baseline justify-between gap-4">
          <span className="text-[11px] text-warmgray-500">Tokens</span>
          <span className="num text-[12px] text-coral-500">{formatCompact(tokens ?? 0)}</span>
        </div>
        <div className="flex items-baseline justify-between gap-4">
          <span className="text-[11px] text-warmgray-500">金额</span>
          <span className="num text-[12px] text-moss">{formatCurrency(cost ?? 0, 4)}</span>
        </div>
      </div>
    </div>
  )
}

export function TrendChart({ rows }: Props) {
  const data = rows.map((r) => ({ ...r, label: formatDateShort(r.date) }))
  const totalTokens = data.reduce((s, r) => s + r.total_tokens, 0)
  const totalCost = data.reduce((s, r) => s + r.total_cost, 0)

  return (
    <section className="flex h-full flex-col rounded-2xl border border-warmgray-200/70 bg-canvas shadow-card">
      <header className="flex shrink-0 items-start justify-between px-6 pt-4 pb-2">
        <div>
          <h2 className="text-[16px] font-semibold tracking-tightish text-warmgray-900">
            近 7 天趋势
          </h2>
          <p className="mt-0.5 text-[12px] text-warmgray-500">
            柱状 Token · 区域 金额
          </p>
        </div>
        <div className="flex items-baseline gap-5">
          <div className="text-right">
            <div className="label text-[10px]">7 日 Token</div>
            <div className="num mt-0.5 text-[14px] font-medium text-coral-500">
              {formatCompact(totalTokens)}
            </div>
          </div>
          <div className="text-right">
            <div className="label text-[10px]">7 日金额</div>
            <div className="num mt-0.5 text-[14px] font-medium text-moss">
              {formatCurrency(totalCost, 2)}
            </div>
          </div>
        </div>
      </header>

      <div className="min-h-0 flex-1 px-2 pb-2">
        <ResponsiveContainer width="100%" height="100%">
          <ComposedChart data={data} margin={{ top: 8, right: 16, bottom: 4, left: 12 }}>
            <defs>
              <linearGradient id="costFill" x1="0" y1="0" x2="0" y2="1">
                <stop offset="0%" stopColor="#506b3a" stopOpacity={0.22} />
                <stop offset="100%" stopColor="#506b3a" stopOpacity={0} />
              </linearGradient>
              <linearGradient id="barFill" x1="0" y1="0" x2="0" y2="1">
                <stop offset="0%" stopColor="#c96442" stopOpacity={0.95} />
                <stop offset="100%" stopColor="#c96442" stopOpacity={0.55} />
              </linearGradient>
            </defs>
            <CartesianGrid vertical={false} />
            <XAxis dataKey="label" tickLine={false} dy={4} />
            <YAxis
              yAxisId="tokens"
              tickLine={false}
              axisLine={false}
              tickFormatter={(v) => formatCompact(v as number)}
              width={60}
            />
            <YAxis
              yAxisId="cost"
              orientation="right"
              tickLine={false}
              axisLine={false}
              tickFormatter={(v) => '$' + (v as number).toFixed(0)}
              width={36}
            />
            <Tooltip content={<CustomTooltip />} cursor={{ fill: 'rgba(201,100,66,0.06)' }} />
            <Bar
              yAxisId="tokens"
              dataKey="total_tokens"
              fill="url(#barFill)"
              barSize={26}
              radius={[6, 6, 0, 0]}
            />
            <Area
              yAxisId="cost"
              type="monotone"
              dataKey="total_cost"
              stroke="#506b3a"
              strokeWidth={1.75}
              fill="url(#costFill)"
              dot={{ r: 2.8, fill: '#506b3a', stroke: '#ffffff', strokeWidth: 1.5 }}
              activeDot={{ r: 4.5, fill: '#506b3a', stroke: '#ffffff', strokeWidth: 2 }}
            />
          </ComposedChart>
        </ResponsiveContainer>
      </div>
    </section>
  )
}
