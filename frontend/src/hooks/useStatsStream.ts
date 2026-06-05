import { useEffect, useRef, useState } from 'react'
import type { Snapshot } from '@/types'

export type ConnState = 'connecting' | 'open' | 'error'

// useStatsStream 订阅后端 SSE /api/stats/stream，返回最新 snapshot 与连接状态。
// EventSource 在断线时浏览器会自动重连，所以我们不再额外做重试。
export function useStatsStream(url = '/api/stats/stream', enabled = true) {
  const [snapshot, setSnapshot] = useState<Snapshot | null>(null)
  const [state, setState] = useState<ConnState>('connecting')
  const [lastPing, setLastPing] = useState<number>(0)
  const esRef = useRef<EventSource | null>(null)

  useEffect(() => {
    if (!enabled) {
      esRef.current?.close()
      esRef.current = null
      setSnapshot(null)
      setState('connecting')
      return
    }
    const es = new EventSource(url)
    esRef.current = es

    es.addEventListener('open', () => setState('open'))

    es.addEventListener('snapshot', (ev) => {
      try {
        const data = JSON.parse((ev as MessageEvent).data) as Snapshot
        setSnapshot(data)
        setState('open')
      } catch (e) {
        console.error('parse snapshot failed', e)
      }
    })

    es.addEventListener('ping', (ev) => {
      const v = Number((ev as MessageEvent).data)
      if (!Number.isNaN(v)) setLastPing(v)
    })

    es.addEventListener('error', () => {
      setState('error')
    })

    return () => {
      es.close()
      esRef.current = null
    }
  }, [enabled, url])

  return { snapshot, state, lastPing }
}
