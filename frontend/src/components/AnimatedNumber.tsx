import { useEffect, useRef } from 'react'
import gsap from 'gsap'

type Props = {
  value: number
  format: (v: number) => string
  duration?: number
  ease?: string
  className?: string
  title?: string
}

// AnimatedNumber — 数字从上一个值平滑补间到新值，再用 format 渲染。
// 通过 textContent 由 GSAP 驱动，避免每帧触发 React 渲染。
export function AnimatedNumber({
  value,
  format,
  duration = 0.9,
  ease = 'power2.out',
  className,
  title,
}: Props) {
  const ref = useRef<HTMLSpanElement>(null)
  const current = useRef<number>(0)
  const formatRef = useRef(format)
  formatRef.current = format

  useEffect(() => {
    const el = ref.current
    if (!el) return
    if (current.current === value) {
      el.textContent = formatRef.current(value)
      return
    }
    const obj = { v: current.current }
    const tween = gsap.to(obj, {
      v: value,
      duration,
      ease,
      onUpdate: () => {
        if (ref.current) ref.current.textContent = formatRef.current(obj.v)
      },
      onComplete: () => {
        current.current = value
        if (ref.current) ref.current.textContent = formatRef.current(value)
      },
    })
    return () => {
      tween.kill()
    }
  }, [value, duration, ease])

  return (
    <span ref={ref} className={className} title={title}>
      {format(value)}
    </span>
  )
}
