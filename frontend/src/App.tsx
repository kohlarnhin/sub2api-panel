import { useCallback, useEffect, useState } from 'react'
import { useStatsStream } from '@/hooks/useStatsStream'
import { SummaryCards } from '@/components/SummaryCards'
import { TokenRanking } from '@/components/TokenRanking'
import { TrendChart } from '@/components/TrendChart'
import { ModelBreakdown } from '@/components/ModelBreakdown'
import { AccountMonitorCard } from '@/components/AccountMonitorCard'
import { Brandmark } from '@/components/Brandmark'
import { StatusBar } from '@/components/StatusBar'
import { PhoneRegisterPanel } from '@/components/phone-register/PhoneRegisterPanel'

type Panel = 'stats' | 'phone-register'

function panelFromPath(pathname: string): Panel {
  return pathname.startsWith('/phone-register') ? 'phone-register' : 'stats'
}

export default function App() {
  const [panel, setPanel] = useState<Panel>(() => panelFromPath(window.location.pathname))
  const { snapshot, state } = useStatsStream('/api/stats/stream', panel === 'stats')

  useEffect(() => {
    const handlePopState = () => setPanel(panelFromPath(window.location.pathname))
    window.addEventListener('popstate', handlePopState)
    return () => window.removeEventListener('popstate', handlePopState)
  }, [])

  const navigate = useCallback((next: Panel) => {
    const nextPath = next === 'phone-register' ? '/phone-register' : '/stats'
    if (window.location.pathname !== nextPath) {
      window.history.pushState({}, '', nextPath)
    }
    setPanel(next)
  }, [])

  return (
    <div className="flex h-screen flex-col gap-4 overflow-hidden bg-cream px-7 py-5 text-warmgray-900">
      {/* HEADER */}
      <header className="flex shrink-0 flex-wrap items-center justify-between gap-3">
        <Brandmark />
        <div className="flex items-center gap-3">
          <nav className="flex rounded-lg border border-warmgray-200 bg-white p-1 shadow-soft">
            <button
              type="button"
              className={`h-8 rounded-md px-3 text-[12px] font-semibold transition-colors ${
                panel === 'stats'
                  ? 'bg-warmgray-900 text-white'
                  : 'text-warmgray-600 hover:bg-warmgray-50'
              }`}
              onClick={() => navigate('stats')}
            >
              统计
            </button>
            <button
              type="button"
              className={`h-8 rounded-md px-3 text-[12px] font-semibold transition-colors ${
                panel === 'phone-register'
                  ? 'bg-warmgray-900 text-white'
                  : 'text-warmgray-600 hover:bg-warmgray-50'
              }`}
              onClick={() => navigate('phone-register')}
            >
              手机注册
            </button>
          </nav>
          {panel === 'stats' ? (
            <StatusBar
              state={state}
              generatedAt={snapshot?.generated_at}
              timezone={snapshot?.timezone}
            />
          ) : null}
        </div>
      </header>

      {panel === 'stats' ? (
        <>
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
        </>
      ) : (
        <PhoneRegisterPanel />
      )}
    </div>
  )
}
