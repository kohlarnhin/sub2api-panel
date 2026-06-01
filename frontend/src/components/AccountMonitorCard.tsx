import { useEffect, useLayoutEffect, useMemo, useRef, useState } from 'react'
import gsap from 'gsap'
import type { AccountMonitor, AccountMonitorItem } from '@/types'
import { formatNumber } from '@/lib/format'
import { useBouncyEntrance, useAnimatedWidth } from '@/hooks/useGsap'
import { AnimatedNumber } from '@/components/AnimatedNumber'

type Props = { data: AccountMonitor | null }

const SHARE_LABELS: Record<string, string> = {
  'plus-share': 'Plus',
  'free-share': 'Free',
}

type Cell = {
  label: string
  value: number
  color: string
  bg: string
}

function buildCells(d: AccountMonitorItem): Cell[] {
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

function Bar({ pct, color, initialDelay }: { pct: number; color: string; initialDelay?: number }) {
  const ref = useAnimatedWidth<HTMLDivElement>(pct, {
    duration: 1,
    ease: 'power3.out',
    initialDelay,
  })
  return (
    <div className="h-1.5 w-full overflow-hidden rounded-full bg-warmgray-100">
      <div ref={ref} className={`h-1.5 rounded-full ${color}`} />
    </div>
  )
}

function ShareSwitch({
  items,
  selectedShare,
  onSwitch,
}: {
  items: AccountMonitorItem[]
  selectedShare: string
  onSwitch: (share: string) => void
}) {
  const options = useMemo(() => {
    const byShare = new Map(items.map((item) => [item.share, item.group_id]))
    return ['plus-share', 'free-share'].map((share) => ({
      share,
      groupID: byShare.get(share) || 0,
    }))
  }, [items])

  return (
    <div className="flex flex-col items-end gap-1">
      <div className="flex rounded-full border border-warmgray-200 bg-warmgray-50 p-0.5">
        {options.map((opt) => {
          const selected = selectedShare === opt.share
          return (
            <button
              key={opt.share}
              type="button"
              disabled={selected || !opt.groupID}
              onClick={() => onSwitch(opt.share)}
              className={[
                'flex h-7 w-[76px] items-center justify-center gap-1 rounded-full px-2 text-[11px] font-semibold transition-colors',
                selected
                  ? 'bg-canvas text-warmgray-900 shadow-soft'
                  : 'text-warmgray-500 hover:text-warmgray-800',
                !opt.groupID ? 'cursor-not-allowed opacity-45' : '',
              ].join(' ')}
              title={`${opt.share}${opt.groupID ? ` · GID ${opt.groupID}` : ' 未配置'}`}
            >
              <span>{SHARE_LABELS[opt.share]}</span>
              {opt.groupID > 0 && <span className="num text-[10px] opacity-60">#{opt.groupID}</span>}
            </button>
          )
        })}
      </div>
    </div>
  )
}

export function AccountMonitorCard({ data }: Props) {
  // hooks 必须无条件调用，所以放在分支之前。
  const sectionRef = useBouncyEntrance<HTMLElement>({
    delay: 0.22,
    distance: 30,
    rotation: 2,
    scaleFrom: 0.92,
    duration: 0.95,
    ease: 'back.out(1.7)',
  })
  const cellsRef = useRef<HTMLDivElement>(null)
  const cellsAnimated = useRef(false)
  const [selectedShare, setSelectedShare] = useState('plus-share')

  const items = useMemo(() => data?.items ?? [], [data?.items])
  const activeItem = useMemo(() => {
    if (items.length === 0) return null
    return items.find((item) => item.share === selectedShare) ?? items[0]
  }, [items, selectedShare])
  const enabled = !!data && data.enabled && !!activeItem

  useEffect(() => {
    if (items.length === 0) return
    if (!items.some((item) => item.share === selectedShare)) {
      setSelectedShare(items[0].share)
    }
  }, [items, selectedShare])

  useLayoutEffect(() => {
    if (cellsAnimated.current) return
    if (!enabled) return
    const root = cellsRef.current
    if (!root) return
    const items = root.querySelectorAll('[data-mon-cell]')
    if (items.length === 0) return
    cellsAnimated.current = true
    gsap.fromTo(
      items,
      { autoAlpha: 0, y: 10 },
      {
        autoAlpha: 1,
        y: 0,
        duration: 0.5,
        ease: 'power3.out',
        stagger: 0.06,
        delay: 0.32,
        clearProps: 'transform',
      },
    )
  }, [enabled])

  // 未配置 / 未启用
  if (!enabled || !activeItem) {
    return (
      <section
        ref={sectionRef}
        className="flex h-full flex-col rounded-2xl border border-warmgray-200/70 bg-canvas shadow-card"
      >
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
              server.account_monitor_groups
            </span>
            <br />
            配置 plus-share / free-share 的 group_id 即可启用
          </p>
        </div>
      </section>
    )
  }

  const cells = buildCells(activeItem)
  const abnormal = cells[3].value
  const availPct = activeItem.total > 0 ? (activeItem.available / activeItem.total) * 100 : 0
  const limitPct = activeItem.total > 0 ? (activeItem.rate_limited / activeItem.total) * 100 : 0
  const abnPct = activeItem.total > 0 ? (abnormal / activeItem.total) * 100 : 0

  return (
    <section
      ref={sectionRef}
      className="flex h-full flex-col rounded-2xl border border-warmgray-200/70 bg-canvas shadow-card"
    >
      <header className="flex shrink-0 items-start justify-between gap-3 px-5 pt-4 pb-2">
        <div className="min-w-0">
          <h2 className="text-[16px] font-semibold tracking-tightish text-warmgray-900">
            账号监控
          </h2>
          <p className="mt-0.5 text-[12px] text-warmgray-500">
            分组 <span className="num text-warmgray-700">{activeItem.group_name || activeItem.share || `#${activeItem.group_id}`}</span>
          </p>
        </div>
        <ShareSwitch
          items={items}
          selectedShare={activeItem.share}
          onSwitch={setSelectedShare}
        />
      </header>

      {/* 4 个数字 */}
      <div
        ref={cellsRef}
        className="grid shrink-0 grid-cols-2 gap-2 px-5 pt-2"
      >
        {cells.map((c) => (
          <div
            data-mon-cell
            key={c.label}
            className={`rounded-xl ${c.bg} px-3 py-2.5`}
          >
            <div className="text-[11px] text-warmgray-500">{c.label}</div>
            <div
              className={`mt-0.5 text-[24px] font-semibold leading-none tracking-tightish ${c.color}`}
            >
              <AnimatedNumber value={c.value} format={formatNumber} />
            </div>
          </div>
        ))}
      </div>

      {/* 组成进度条 */}
      <div className="min-h-0 flex-1 space-y-2.5 px-5 pt-4">
        <div>
          <div className="mb-1 flex items-baseline justify-between text-[11px]">
            <span className="text-warmgray-500">可用率</span>
            <span className="num text-warmgray-700">
              <AnimatedNumber value={availPct} format={(v) => `${v.toFixed(1)}%`} />
            </span>
          </div>
          <Bar pct={availPct} color="bg-moss" initialDelay={0.7} />
        </div>
        <div>
          <div className="mb-1 flex items-baseline justify-between text-[11px]">
            <span className="text-warmgray-500">限流率</span>
            <span className="num text-warmgray-700">
              <AnimatedNumber value={limitPct} format={(v) => `${v.toFixed(1)}%`} />
            </span>
          </div>
          <Bar pct={limitPct} color="bg-amber-500" initialDelay={0.8} />
        </div>
        <div>
          <div className="mb-1 flex items-baseline justify-between text-[11px]">
            <span className="text-warmgray-500">异常率</span>
            <span className="num text-warmgray-700">
              <AnimatedNumber value={abnPct} format={(v) => `${v.toFixed(1)}%`} />
            </span>
          </div>
          <Bar pct={abnPct} color="bg-coral-500" initialDelay={0.9} />
        </div>
      </div>

      <footer className="shrink-0 border-t border-warmgray-100 px-5 py-2 text-[11px] text-warmgray-400">
        异常 = 总量 − 限流 − 可用
      </footer>
    </section>
  )
}
