import { useStatsStream } from '@/hooks/useStatsStream'
import { SummaryCards } from '@/components/SummaryCards'
import { TokenRanking } from '@/components/TokenRanking'
import { TrendChart } from '@/components/TrendChart'
import { ModelBreakdown } from '@/components/ModelBreakdown'
import { AccountMonitorCard } from '@/components/AccountMonitorCard'
import { Brandmark } from '@/components/Brandmark'
import { StatusBar } from '@/components/StatusBar'

export default function App() {
  const { snapshot, state } = useStatsStream()

  return (
    <div className="flex h-screen flex-col gap-4 overflow-hidden bg-cream px-7 py-5 text-warmgray-900">
      {/* HEADER */}
      <header className="flex shrink-0 items-center justify-between">
        <Brandmark />
        <StatusBar
          state={state}
          generatedAt={snapshot?.generated_at}
          timezone={snapshot?.timezone}
        />
      </header>

      {/* KPI —— 今日 / 今日请求 / 累计 / 累计请求 */}
      <SummaryCards
        today={snapshot?.today_summary ?? null}
        historical={snapshot?.historical ?? null}
      />

      {/* MAIN ROW —— 排行榜 (左) + 账号监控 (右) */}
      <section className="grid min-h-0 flex-1 grid-cols-12 gap-4">
        <div className="col-span-8 min-h-0">
          <TokenRanking rows={snapshot?.token_ranking ?? []} />
        </div>
        <div className="col-span-4 min-h-0">
          <AccountMonitorCard data={snapshot?.account_monitor ?? null} />
        </div>
      </section>

      {/* BOTTOM ROW —— 近 7 天趋势 + 模型用量 */}
      <section
        className="grid min-h-0 shrink-0 grid-cols-12 gap-4"
        style={{ height: '32%' }}
      >
        <div className="col-span-8 min-h-0">
          <TrendChart rows={snapshot?.seven_day_trend ?? []} />
        </div>
        <div className="col-span-4 min-h-0">
          <ModelBreakdown rows={snapshot?.model_breakdown ?? []} />
        </div>
      </section>
    </div>
  )
}
