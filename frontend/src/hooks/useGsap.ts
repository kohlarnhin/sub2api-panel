import { useEffect, useLayoutEffect, useRef } from 'react'
import gsap from 'gsap'

type Direction = 'up' | 'down' | 'left' | 'right'

type EntranceOptions = {
  delay?: number
  duration?: number
  distance?: number
  from?: Direction
  ease?: string
}

// useEntrance — 元素挂载后从指定方向淡入。仅触发一次。
export function useEntrance<T extends HTMLElement>(opts: EntranceOptions = {}) {
  const ref = useRef<T | null>(null)

  useLayoutEffect(() => {
    const el = ref.current
    if (!el) return
    const distance = opts.distance ?? 14
    const from = opts.from ?? 'up'
    const x = from === 'left' ? -distance : from === 'right' ? distance : 0
    const y = from === 'up' ? distance : from === 'down' ? -distance : 0

    const ctx = gsap.context(() => {
      gsap.fromTo(
        el,
        { autoAlpha: 0, x, y },
        {
          autoAlpha: 1,
          x: 0,
          y: 0,
          duration: opts.duration ?? 0.55,
          ease: opts.ease ?? 'power3.out',
          delay: opts.delay ?? 0,
          clearProps: 'transform',
        },
      )
    }, el)
    return () => ctx.revert()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  return ref
}

type StaggerOptions = EntranceOptions & {
  selector: string
  stagger?: number
}

// useStagger — 容器内匹配 selector 的子元素依次淡入。
export function useStagger<T extends HTMLElement>(opts: StaggerOptions) {
  const ref = useRef<T | null>(null)

  useLayoutEffect(() => {
    const root = ref.current
    if (!root) return
    const items = root.querySelectorAll(opts.selector)
    if (items.length === 0) return
    const distance = opts.distance ?? 14
    const from = opts.from ?? 'up'
    const x = from === 'left' ? -distance : from === 'right' ? distance : 0
    const y = from === 'up' ? distance : from === 'down' ? -distance : 0

    const ctx = gsap.context(() => {
      gsap.fromTo(
        items,
        { autoAlpha: 0, x, y },
        {
          autoAlpha: 1,
          x: 0,
          y: 0,
          duration: opts.duration ?? 0.5,
          ease: opts.ease ?? 'power3.out',
          delay: opts.delay ?? 0,
          stagger: opts.stagger ?? 0.06,
          clearProps: 'transform',
        },
      )
    }, root)
    return () => ctx.revert()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  return ref
}

// useReStagger — 当 trigger 改变时再次以 stagger 方式重播容器子元素的动画。
// 用于数据更新后重排表格行等场景。
export function useReStagger<T extends HTMLElement>(
  trigger: unknown,
  opts: StaggerOptions,
) {
  const ref = useRef<T | null>(null)

  useEffect(() => {
    const root = ref.current
    if (!root) return
    const items = root.querySelectorAll(opts.selector)
    if (items.length === 0) return
    const distance = opts.distance ?? 10
    const from = opts.from ?? 'up'
    const x = from === 'left' ? -distance : from === 'right' ? distance : 0
    const y = from === 'up' ? distance : from === 'down' ? -distance : 0

    const ctx = gsap.context(() => {
      gsap.fromTo(
        items,
        { autoAlpha: 0, x, y },
        {
          autoAlpha: 1,
          x: 0,
          y: 0,
          duration: opts.duration ?? 0.45,
          ease: opts.ease ?? 'power3.out',
          delay: opts.delay ?? 0,
          stagger: opts.stagger ?? 0.035,
          clearProps: 'transform',
        },
      )
    }, root)
    return () => ctx.revert()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [trigger])

  return ref
}

type BarOptions = {
  duration?: number
  ease?: string
  delay?: number
  /** 仅在首次播放时附加的延迟（用于"生长"动画在卡片入场后再开始） */
  initialDelay?: number
}

type BouncyOptions = {
  delay?: number
  duration?: number
  ease?: string
  /** 起始 Y 偏移量（向下落入），默认 24 */
  distance?: number
  /** 起始旋转角度的绝对值上限，例如 4 表示 -4 ~ +4 度。默认 0（不旋转） */
  rotation?: number
  /** 起始缩放比例，默认 0.9 */
  scaleFrom?: number
}

// useBouncyEntrance — 元素以 back.out 反弹 ease 入场，可叠加随机微旋转与微缩放，营造"跳进来"的感觉。
export function useBouncyEntrance<T extends HTMLElement>(
  opts: BouncyOptions = {},
) {
  const ref = useRef<T | null>(null)

  useLayoutEffect(() => {
    const el = ref.current
    if (!el) return
    const distance = opts.distance ?? 24
    const rot = opts.rotation ?? 0
    const ctx = gsap.context(() => {
      gsap.from(el, {
        autoAlpha: 0,
        y: distance,
        scale: opts.scaleFrom ?? 0.92,
        rotation: rot > 0 ? gsap.utils.random(-rot, rot) : 0,
        duration: opts.duration ?? 0.85,
        ease: opts.ease ?? 'back.out(1.6)',
        delay: opts.delay ?? 0,
        clearProps: 'transform',
      })
    }, el)
    return () => ctx.revert()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  return ref
}

type BouncyStaggerOptions = BouncyOptions & {
  selector: string
  stagger?: number
  /** stagger 顺序，可用 'random' 让卡片随机弹入而不是从左到右 */
  staggerFrom?: 'start' | 'end' | 'center' | 'random'
}

// useBouncyStagger — 容器内匹配 selector 的子元素以 back.out 弹入，每个元素的旋转角度独立随机，stagger 顺序可设。
export function useBouncyStagger<T extends HTMLElement>(
  opts: BouncyStaggerOptions,
) {
  const ref = useRef<T | null>(null)

  useLayoutEffect(() => {
    const root = ref.current
    if (!root) return
    const items = root.querySelectorAll(opts.selector)
    if (items.length === 0) return
    const distance = opts.distance ?? 24
    const rot = opts.rotation ?? 0
    const ctx = gsap.context(() => {
      gsap.from(items, {
        autoAlpha: 0,
        y: distance,
        scale: opts.scaleFrom ?? 0.88,
        // 第三个参数传 true 让 gsap 每次取值都生成新的随机数，每张卡有自己的角度
        rotation: rot > 0 ? gsap.utils.random(-rot, rot, true) : 0,
        duration: opts.duration ?? 0.9,
        ease: opts.ease ?? 'back.out(1.7)',
        delay: opts.delay ?? 0,
        stagger: {
          each: opts.stagger ?? 0.09,
          from: opts.staggerFrom ?? 'random',
        },
        clearProps: 'transform',
      })
    }, root)
    return () => ctx.revert()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  return ref
}

// useAnimatedWidth — 把元素宽度从当前值补间到 `pct` 百分比。
export function useAnimatedWidth<T extends HTMLElement>(
  pct: number,
  opts: BarOptions = {},
) {
  const ref = useRef<T | null>(null)
  const prev = useRef<number | null>(null)

  useEffect(() => {
    const el = ref.current
    if (!el) return
    const target = Math.max(0, Math.min(100, pct))
    const isFirst = prev.current === null
    const fromPct = prev.current ?? 0
    const delay =
      (opts.delay ?? 0) + (isFirst ? opts.initialDelay ?? 0 : 0)
    const ctx = gsap.context(() => {
      gsap.fromTo(
        el,
        { width: `${fromPct}%` },
        {
          width: `${target}%`,
          duration: opts.duration ?? 0.8,
          ease: opts.ease ?? 'power2.out',
          delay,
        },
      )
    }, el)
    prev.current = target
    return () => ctx.revert()
  }, [pct, opts.duration, opts.ease, opts.delay, opts.initialDelay])

  return ref
}
