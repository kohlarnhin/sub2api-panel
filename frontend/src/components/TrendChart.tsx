import { useEffect, useRef, useState } from 'react'
import gsap from 'gsap'
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
import { useBouncyEntrance } from '@/hooks/useGsap'
import { AnimatedNumber } from '@/components/AnimatedNumber'

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
  const sectionRef = useBouncyEntrance<HTMLElement>({
    delay: 0.3,
    distance: 30,
    rotation: 1.8,
    scaleFrom: 0.93,
    duration: 0.9,
    ease: 'back.out(1.6)',
  })

  // GSAP 编排：禁用 Recharts 自身的 Bar/Area 动画，自己把柱子逐根生长 +
  // 折线 dashoffset 描线同步收尾。chartReady 控制 wrapper 的可见性，避免
  // GSAP 还没把初始 scaleY 设到 0 之前先闪一帧最终高度。
  const chartWrapperRef = useRef<HTMLDivElement>(null)
  const animatedRef = useRef(false)
  const [chartReady, setChartReady] = useState(false)

  useEffect(() => {
    if (animatedRef.current) return
    if (data.length === 0) return
    const wrapper = chartWrapperRef.current
    if (!wrapper) return

    let raf = 0
    let attempts = 0
    const tweens: gsap.core.Tween[] = []

    const start = () => {
      const bars = wrapper.querySelectorAll<SVGGElement>(
        '.recharts-bar-rectangle',
      )
      const stroke = wrapper.querySelector<SVGPathElement>(
        '.recharts-area-curve',
      )
      const area = wrapper.querySelector<SVGPathElement>('.recharts-area-area')
      const dots = wrapper.querySelectorAll<SVGCircleElement>(
        '.recharts-area-dots circle',
      )

      // Recharts 异步测量后才挂载这些节点，先轮询直到出现
      const ready = bars.length > 0 && stroke && area
      if (!ready) {
        attempts++
        if (attempts < 60) {
          raf = requestAnimationFrame(start)
        } else {
          setChartReady(true)
        }
        return
      }

      animatedRef.current = true

      const barCount = bars.length
      const barStagger = 0.1
      const barDuration = 0.55
      // 让 stroke / area / dots 与最后一根柱子同时完成，营造同步收尾
      const totalBarSpan = barStagger * (barCount - 1) + barDuration
      const startDelay = 0.05

      gsap.set(bars, { scaleY: 0, transformOrigin: 'center bottom' })
      const strokeLen = stroke.getTotalLength() || 1
      gsap.set(stroke, {
        strokeDasharray: strokeLen,
        strokeDashoffset: strokeLen,
      })
      gsap.set(area, { opacity: 0 })
      if (dots.length > 0) {
        gsap.set(dots, {
          scale: 0,
          opacity: 0,
          transformOrigin: 'center center',
        })
      }

      setChartReady(true)

      // 柱子：一根接一根从底部长起来
      tweens.push(
        gsap.to(bars, {
          scaleY: 1,
          duration: barDuration,
          ease: 'power3.out',
          stagger: barStagger,
          delay: startDelay,
        }),
      )

      // 折线：dashoffset 从左到右描出来，正好和最后一根柱子同时完成
      tweens.push(
        gsap.to(stroke, {
          strokeDashoffset: 0,
          duration: totalBarSpan,
          ease: 'power2.out',
          delay: startDelay,
        }),
      )

      // 折线下方填充：跟随 stroke 一起淡入
      tweens.push(
        gsap.to(area, {
          opacity: 1,
          duration: totalBarSpan * 0.75,
          ease: 'power2.out',
          delay: startDelay + 0.1,
        }),
      )

      // 数据点：和柱子同步 stagger pop 出来，背后小弹跳一下
      if (dots.length > 0) {
        tweens.push(
          gsap.to(dots, {
            scale: 1,
            opacity: 1,
            duration: 0.35,
            ease: 'back.out(2.2)',
            stagger: barStagger,
            delay: startDelay + 0.2,
          }),
        )
      }
    }

    raf = requestAnimationFrame(start)
    return () => {
      if (raf) cancelAnimationFrame(raf)
      tweens.forEach((t) => t.kill())
    }
  }, [data.length])

  return (
    <section
      ref={sectionRef}
      className="flex h-full flex-col rounded-2xl border border-warmgray-200/70 bg-canvas shadow-card"
    >
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
              <AnimatedNumber value={totalTokens} format={formatCompact} />
            </div>
          </div>
          <div className="text-right">
            <div className="label text-[10px]">7 日金额</div>
            <div className="num mt-0.5 text-[14px] font-medium text-moss">
              <AnimatedNumber value={totalCost} format={(v) => formatCurrency(v, 2)} />
            </div>
          </div>
        </div>
      </header>

      <div
        ref={chartWrapperRef}
        className="min-h-0 flex-1 px-2 pb-2"
        style={{ visibility: chartReady || data.length === 0 ? 'visible' : 'hidden' }}
      >
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
              isAnimationActive={false}
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
              isAnimationActive={false}
            />
          </ComposedChart>
        </ResponsiveContainer>
      </div>
    </section>
  )
}
