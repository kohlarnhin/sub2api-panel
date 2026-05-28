import { useEffect, useLayoutEffect, useRef } from 'react'
import gsap from 'gsap'
import type { UserTokenRank } from '@/types'
import { formatCompact, formatCurrency, formatNumber } from '@/lib/format'
import { useAnimatedWidth, useBouncyEntrance } from '@/hooks/useGsap'
import { AnimatedNumber } from '@/components/AnimatedNumber'

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

type RowProps = { r: UserTokenRank; rank: number; max: number }

function TokenRow({ r, rank, max }: RowProps) {
  const pct = max > 0 ? (r.total_tokens / max) * 100 : 0
  const barRef = useAnimatedWidth<HTMLDivElement>(Math.max(3, pct), {
    duration: 1,
    ease: 'power3.out',
    initialDelay: 0.55,
  })

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
    <tr className="group transition-colors hover:bg-cream/60">
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
            <div ref={barRef} className="h-2 rounded-full bg-coral-500" />
          </div>
          <span className="num w-[68px] shrink-0 text-right text-[13px] font-medium text-warmgray-900">
            <AnimatedNumber value={r.total_tokens} format={formatCompact} />
          </span>
        </div>
      </td>

      {/* Input */}
      <td className="num border-t border-warmgray-100 py-3 text-right text-warmgray-700">
        <AnimatedNumber value={r.input_tokens} format={formatCompact} />
      </td>

      {/* Output */}
      <td className="num border-t border-warmgray-100 py-3 text-right text-warmgray-700">
        <AnimatedNumber value={r.output_tokens} format={formatCompact} />
      </td>

      {/* Cache (write + read combined) */}
      <td className="num border-t border-warmgray-100 py-3 text-right text-warmgray-700">
        <AnimatedNumber value={cacheSum} format={formatCompact} />
      </td>

      {/* Cache hit rate */}
      <td
        className={`num border-t border-warmgray-100 py-3 text-right ${hitColor}`}
      >
        {hitText}
      </td>

      {/* Cost */}
      <td className="num border-t border-warmgray-100 py-3 text-right text-[14px] font-medium text-moss">
        <AnimatedNumber value={r.total_cost} format={(v) => formatCurrency(v, 2)} />
      </td>

      {/* Requests */}
      <td className="num border-t border-warmgray-100 py-3 pr-6 text-right text-warmgray-700">
        <AnimatedNumber value={r.request_count} format={formatNumber} />
      </td>
    </tr>
  )
}

export function TokenRanking({ rows }: Props) {
  const max = rows[0]?.total_tokens ?? 0
  const sectionRef = useBouncyEntrance<HTMLElement>({
    delay: 0.22,
    distance: 30,
    rotation: 1.6,
    scaleFrom: 0.94,
    duration: 0.85,
    ease: 'back.out(1.5)',
  })
  const tbodyRef = useRef<HTMLTableSectionElement>(null)
  const initialized = useRef(false)

  useLayoutEffect(() => {
    if (initialized.current) return
    if (rows.length === 0) return
    const tbody = tbodyRef.current
    if (!tbody) return
    const trs = tbody.querySelectorAll('tr')
    if (trs.length === 0) return
    initialized.current = true
    gsap.fromTo(
      trs,
      { autoAlpha: 0, y: 10 },
      {
        autoAlpha: 1,
        y: 0,
        duration: 0.45,
        ease: 'power3.out',
        stagger: 0.035,
        clearProps: 'transform',
      },
    )
  }, [rows.length])

  // 每次有新数据时让表头副标题数字短暂闪烁，提示数据已刷新。
  const counterRef = useRef<HTMLSpanElement>(null)
  useEffect(() => {
    const el = counterRef.current
    if (!el) return
    gsap.fromTo(
      el,
      { color: '#c96442' },
      { color: '#a09c8e', duration: 1.1, ease: 'power2.out' },
    )
  }, [rows.length])

  return (
    <section
      ref={sectionRef}
      className="flex h-full flex-col rounded-2xl border border-warmgray-200/70 bg-canvas shadow-card"
    >
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
          共 <span ref={counterRef}>{rows.length}</span> 位
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
            <tbody ref={tbodyRef}>
              {rows.map((r, i) => (
                <TokenRow key={r.user_id} r={r} rank={i + 1} max={max} />
              ))}
            </tbody>
          </table>
        )}
      </div>
    </section>
  )
}
