import { useMemo } from 'react'
import { Cell, Pie, PieChart, ResponsiveContainer, Tooltip } from 'recharts'
import type { ModelStat } from '@/types'
import { formatCurrency } from '@/lib/format'

type Props = { rows: ModelStat[] }

// 与 Claude coral 主色协调的橙暖系阶梯 + 几个对比色
const PALETTE = [
  '#c96442', // coral 500
  '#e0814d', // coral 400
  '#eda575', // coral 300
  '#506b3a', // moss
  '#7d8f5e',
  '#a09c8e', // warmgray 400
  '#73706a', // warmgray 500
  '#c08a2e',
  '#cfcbbe',
]

function CustomTooltip({ active, payload }: any) {
  if (!active || !payload?.[0]) return null
  const d = payload[0].payload
  return (
    <div
      style={{
        background: '#ffffff',
        border: '1px solid #e5e2d6',
        borderRadius: 10,
        padding: '8px 12px',
        boxShadow: '0 6px 20px -6px rgba(31,30,28,0.22)',
      }}
    >
      <div className="num text-[11px] font-medium text-warmgray-900">{d.model}</div>
      <div className="num mt-0.5 text-[12px] text-moss">{formatCurrency(d.total_cost, 4)}</div>
    </div>
  )
}

export function ModelBreakdown({ rows }: Props) {
  const data = useMemo(() => {
    const top = rows.slice(0, 6)
    const rest = rows.slice(6)
    const restSum = rest.reduce(
      (acc, r) => {
        acc.total_cost += r.total_cost
        acc.total_tokens += r.total_tokens
        acc.request_count += r.request_count
        return acc
      },
      { total_cost: 0, total_tokens: 0, request_count: 0 },
    )
    const items = top.map((r) => ({ ...r }))
    if (rest.length > 0 && restSum.total_cost > 0) {
      items.push({ model: `其他 ${rest.length}`, ...restSum })
    }
    return items
  }, [rows])

  const totalCost = data.reduce((s, r) => s + r.total_cost, 0)
  const dominant = data[0]

  return (
    <section className="flex h-full flex-col rounded-2xl border border-warmgray-200/70 bg-canvas shadow-card">
      <header className="shrink-0 px-5 pt-4 pb-2">
        <h2 className="text-[16px] font-semibold tracking-tightish text-warmgray-900">
          模型用量
        </h2>
        <p className="mt-0.5 text-[12px] text-warmgray-500">按金额占比</p>
      </header>

      <div className="grid min-h-0 flex-1 grid-rows-[auto_1fr]">
        <div className="relative h-[148px] shrink-0">
          <ResponsiveContainer width="100%" height="100%">
            <PieChart>
              <Pie
                data={data}
                dataKey="total_cost"
                nameKey="model"
                cx="50%"
                cy="50%"
                innerRadius={44}
                outerRadius={66}
                paddingAngle={2}
                stroke="#ffffff"
                strokeWidth={2}
              >
                {data.map((_, i) => (
                  <Cell key={i} fill={PALETTE[i % PALETTE.length]} />
                ))}
              </Pie>
              <Tooltip
                content={<CustomTooltip />}
                wrapperStyle={{ zIndex: 50, outline: 'none' }}
                allowEscapeViewBox={{ x: true, y: true }}
              />
            </PieChart>
          </ResponsiveContainer>
          <div className="pointer-events-none absolute inset-0 flex flex-col items-center justify-center">
            <div className="label text-[10px]">TOP</div>
            <div
              className="mt-0.5 max-w-[120px] truncate text-center text-[13px] font-semibold text-warmgray-900"
              title={dominant?.model}
            >
              {dominant?.model ?? '—'}
            </div>
          </div>
        </div>

        <ul className="min-h-0 overflow-hidden px-3 pb-2 pt-1">
          {data.length === 0 ? (
            <li className="py-6 text-center text-[12px] text-warmgray-400">
              暂无今日数据
            </li>
          ) : (
            data.map((r, i) => {
              const pct = totalCost > 0 ? (r.total_cost / totalCost) * 100 : 0
              return (
                <li
                  key={r.model + i}
                  className="grid grid-cols-[10px_minmax(0,1fr)_56px_42px] items-center gap-2 rounded-lg px-2 py-1 text-[11px] hover:bg-warmgray-50"
                >
                  <span
                    className="h-2 w-2 rounded-sm"
                    style={{ backgroundColor: PALETTE[i % PALETTE.length] }}
                  />
                  <span className="truncate text-warmgray-700" title={r.model}>
                    {r.model}
                  </span>
                  <span className="num text-right text-moss">
                    {formatCurrency(r.total_cost, 2)}
                  </span>
                  <span className="num text-right text-warmgray-400">
                    {pct.toFixed(1)}%
                  </span>
                </li>
              )
            })
          )}
        </ul>
      </div>
    </section>
  )
}
