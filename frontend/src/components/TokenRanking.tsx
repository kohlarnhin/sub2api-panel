import type { UserTokenRank } from '@/types'
import { formatCompact, formatCurrency, formatNumber } from '@/lib/format'

type Props = { rows: UserTokenRank[] }

function initials(name: string) {
  const s = (name || '?').trim()
  if (!s) return '?'
  const first = Array.from(s)[0]
  return first.toUpperCase()
}

// 头部前 3 名用 coral 系把头像染色，呈现轻微视觉层次；不再单独画 #1/#2/#3 badge。
function avatarBg(rank: number) {
  if (rank === 1) return 'bg-coral-500 text-white'
  if (rank === 2) return 'bg-coral-200 text-coral-700'
  if (rank === 3) return 'bg-coral-100 text-coral-700'
  return 'bg-warmgray-100 text-warmgray-700'
}

export function TokenRanking({ rows }: Props) {
  const max = rows[0]?.total_tokens ?? 0

  return (
    <section className="flex h-full flex-col rounded-2xl border border-warmgray-200/70 bg-canvas shadow-card">
      <header className="flex shrink-0 items-end justify-between px-6 pt-4 pb-3">
        <div>
          <h2 className="text-[16px] font-semibold tracking-tightish text-warmgray-900">
            今日用户用量排行
          </h2>
          <p className="mt-0.5 text-[12px] text-warmgray-500">
            按 Token 总量倒序 · 命中率 = 缓存读取 / 真实输入
          </p>
        </div>
        <div className="num text-[11px] text-warmgray-400">
          共 {rows.length} 位
        </div>
      </header>

      <div className="min-h-0 flex-1 overflow-y-auto overflow-x-hidden border-t border-warmgray-100">
        {rows.length === 0 ? (
          <div className="flex h-full items-center justify-center text-[13px] text-warmgray-400">
            暂无今日数据
          </div>
        ) : (
          <table className="w-full table-fixed border-separate border-spacing-0 text-[13px]">
            <colgroup>
              <col className="w-[21%]" />
              <col className="w-[20%]" />
              <col className="w-[9%]" />
              <col className="w-[9%]" />
              <col className="w-[10%]" />
              <col className="w-[9%]" />
              <col className="w-[11%]" />
              <col className="w-[11%]" />
            </colgroup>
            <thead>
              <tr className="text-left">
                <th className="label sticky top-0 z-10 bg-canvas px-6 py-2 font-medium">
                  用户
                </th>
                <th className="label sticky top-0 z-10 bg-canvas py-2 font-medium">
                  Token 用量
                </th>
                <th className="label sticky top-0 z-10 bg-canvas py-2 text-right font-medium">
                  输入
                </th>
                <th className="label sticky top-0 z-10 bg-canvas py-2 text-right font-medium">
                  输出
                </th>
                <th className="label sticky top-0 z-10 bg-canvas py-2 text-right font-medium">
                  缓存
                </th>
                <th className="label sticky top-0 z-10 bg-canvas py-2 text-right font-medium">
                  命中率
                </th>
                <th className="label sticky top-0 z-10 bg-canvas py-2 text-right font-medium">
                  金额
                </th>
                <th className="label sticky top-0 z-10 bg-canvas py-2 pr-6 text-right font-medium">
                  请求
                </th>
              </tr>
            </thead>
            <tbody>
              {rows.map((r, i) => {
                const rank = i + 1
                const pct = max > 0 ? (r.total_tokens / max) * 100 : 0
                const cacheSum = r.cache_creation_tokens + r.cache_read_tokens
                const realInput =
                  r.input_tokens + r.cache_creation_tokens + r.cache_read_tokens
                const hitRate =
                  realInput > 0 ? (r.cache_read_tokens / realInput) * 100 : -1
                const hitText = hitRate < 0 ? '—' : `${hitRate.toFixed(1)}%`
                const hitColor =
                  hitRate < 0
                    ? 'text-warmgray-400'
                    : hitRate >= 80
                    ? 'text-moss'
                    : hitRate >= 50
                    ? 'text-warmgray-700'
                    : 'text-coral-500'
                const hasUsername = !!r.username
                const hasEmail = !!r.email
                const fallback = hasUsername
                  ? r.username
                  : hasEmail
                  ? r.email
                  : `user#${r.user_id}`
                return (
                  <tr
                    key={r.user_id}
                    className="group transition-colors hover:bg-cream/60"
                  >
                    {/* User */}
                    <td className="border-t border-warmgray-100 py-3 pl-6 pr-3">
                      <div className="flex items-center gap-3">
                        <span
                          className={`grid h-8 w-8 shrink-0 place-items-center rounded-full text-[12px] font-semibold ${avatarBg(
                            rank,
                          )}`}
                        >
                          {initials(r.username || r.email)}
                        </span>
                        <div className="min-w-0">
                          {hasUsername && hasEmail ? (
                            <>
                              <div className="truncate text-[13px] font-medium text-warmgray-900">
                                {r.username}
                              </div>
                              <div className="num truncate text-[11px] text-warmgray-400">
                                {r.email}
                              </div>
                            </>
                          ) : (
                            <div className="num truncate text-[13px] font-medium text-warmgray-900">
                              {fallback}
                            </div>
                          )}
                        </div>
                      </div>
                    </td>

                    {/* Token bar */}
                    <td className="border-t border-warmgray-100 py-3 pr-4">
                      <div className="flex items-center gap-2">
                        <div className="h-2 flex-1 overflow-hidden rounded-full bg-warmgray-100">
                          <div
                            className="h-2 rounded-full bg-coral-500 transition-all"
                            style={{ width: `${Math.max(3, pct)}%` }}
                          />
                        </div>
                        <span className="num w-[68px] shrink-0 text-right text-[13px] font-medium text-warmgray-900">
                          {formatCompact(r.total_tokens)}
                        </span>
                      </div>
                    </td>

                    {/* Input */}
                    <td className="num border-t border-warmgray-100 py-3 text-right text-warmgray-700">
                      {formatCompact(r.input_tokens)}
                    </td>

                    {/* Output */}
                    <td className="num border-t border-warmgray-100 py-3 text-right text-warmgray-700">
                      {formatCompact(r.output_tokens)}
                    </td>

                    {/* Cache (write + read combined) */}
                    <td className="num border-t border-warmgray-100 py-3 text-right text-warmgray-700">
                      {formatCompact(cacheSum)}
                    </td>

                    {/* Cache hit rate */}
                    <td
                      className={`num border-t border-warmgray-100 py-3 text-right ${hitColor}`}
                    >
                      {hitText}
                    </td>

                    {/* Cost */}
                    <td className="num border-t border-warmgray-100 py-3 text-right text-[14px] font-medium text-moss">
                      {formatCurrency(r.total_cost, 2)}
                    </td>

                    {/* Requests */}
                    <td className="num border-t border-warmgray-100 py-3 pr-6 text-right text-warmgray-700">
                      {formatNumber(r.request_count)}
                    </td>
                  </tr>
                )
              })}
            </tbody>
          </table>
        )}
      </div>
    </section>
  )
}
