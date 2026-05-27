import type { ConnState } from '@/hooks/useStatsStream'
import { formatTime } from '@/lib/format'

type Props = {
  state: ConnState
  generatedAt?: string
  timezone?: string
}

const labels: Record<ConnState, { text: string; dot: string; pill: string }> = {
  connecting: {
    text: '连接中',
    dot: 'bg-amber-400',
    pill: 'bg-amber-50 text-amber-700 ring-amber-200',
  },
  open: {
    text: '实时连接',
    dot: 'bg-emerald-500',
    pill: 'bg-emerald-50 text-emerald-700 ring-emerald-200',
  },
  error: {
    text: '正在重连',
    dot: 'bg-rose-500',
    pill: 'bg-rose-50 text-rose-700 ring-rose-200',
  },
}

export function StatusBar({ state, generatedAt, timezone }: Props) {
  const { text, dot, pill } = labels[state]
  return (
    <div className="flex items-center gap-3 text-[12px] text-warmgray-500">
      <span
        className={`inline-flex items-center gap-2 rounded-full px-3 py-1 text-[11px] font-medium ring-1 ring-inset ${pill}`}
      >
        <span className={`h-1.5 w-1.5 rounded-full ${dot} animate-pulse-dot`} />
        {text}
      </span>

      <span className="hidden md:inline">·</span>

      <span className="hidden md:inline">
        <span className="text-warmgray-400">时区</span>{' '}
        <span className="num text-warmgray-700">{timezone || 'Asia/Shanghai'}</span>
      </span>

      <span className="hidden md:inline">·</span>

      <span>
        <span className="text-warmgray-400">数据更新</span>{' '}
        <span className="num text-warmgray-900">
          {generatedAt ? formatTime(generatedAt) : '—'}
        </span>
      </span>
    </div>
  )
}
