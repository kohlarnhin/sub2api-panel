import { useEntrance } from '@/hooks/useGsap'

export function Brandmark() {
  const ref = useEntrance<HTMLDivElement>({ from: 'left', delay: 0.05 })

  return (
    <div ref={ref} className="flex items-baseline gap-3">
      <h1 className="text-[22px] font-semibold tracking-tightish text-warmgray-900">
        sub2api
        <span className="text-warmgray-300"> · </span>
        <span className="font-normal text-warmgray-500">usage</span>
      </h1>
      <span className="hidden h-1 w-1 rounded-full bg-coral-500 md:inline-block" />
      <p className="hidden text-[12px] text-warmgray-500 md:inline-block">
        实时消耗与用户用量看板
      </p>
    </div>
  )
}
