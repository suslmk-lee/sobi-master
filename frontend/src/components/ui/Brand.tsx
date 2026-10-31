// frontend/src/components/ui/Brand.tsx
import { useId } from 'react';

// 상단 브랜드: ₩ 를 형상화한 마크(지그재그 W + 두 줄 가로획)를 어두운 배지에 올리고 워드마크를 붙인다.
// 앱 아이콘(build/appicon.png)과 같은 도형. 배지 바탕은 아이콘처럼 어둡게 고정하고(라이트 테마에서도 가로획이 보이게)
// W 의 그라데이션만 테마 강조색을 따른다.
export default function Brand() {
  // useId 의 ':' 는 일부 WebKit 에서 url(#…) 참조를 깨뜨리므로 뺀다
  const gid = `brand-${useId().replace(/:/g, '')}`;
  return (
    <div className="flex select-none items-center gap-3">
      <svg
        width="34"
        height="34"
        viewBox="0 0 34 34"
        aria-hidden
        className="shrink-0 drop-shadow-[0_4px_14px_rgb(var(--c-mint-400)/0.3)]"
      >
        <defs>
          <linearGradient id={`${gid}-bg`} x1="0" y1="0" x2="0" y2="1">
            <stop offset="0" stopColor="#1b1d1f" />
            <stop offset="1" stopColor="#08090a" />
          </linearGradient>
          <radialGradient id={`${gid}-glow`} cx="0.5" cy="1.05" r="0.75">
            <stop offset="0" style={{ stopColor: 'rgb(var(--c-mint-400))', stopOpacity: 0.35 }} />
            <stop offset="1" style={{ stopColor: 'rgb(var(--c-mint-400))', stopOpacity: 0 }} />
          </radialGradient>
          <linearGradient id={`${gid}-mk`} x1="0" y1="1" x2="1" y2="0">
            <stop offset="0" style={{ stopColor: 'rgb(var(--c-mint-400))' }} />
            <stop offset="1" style={{ stopColor: 'rgb(var(--c-sky-300))' }} />
          </linearGradient>
        </defs>
        <rect width="34" height="34" rx="7.6" fill={`url(#${gid}-bg)`} />
        <rect width="34" height="34" rx="7.6" fill={`url(#${gid}-glow)`} />
        <rect
          x="0.35"
          y="0.35"
          width="33.3"
          height="33.3"
          rx="7.25"
          fill="none"
          strokeWidth="0.7"
          stroke="#ffffff"
          strokeOpacity="0.14"
        />
        <g fill="none" strokeLinecap="round" strokeLinejoin="round">
          <path d="M8.5 10 L12.6 24 L17 14.2 L21.4 24 L25.5 10" stroke={`url(#${gid}-mk)`} strokeWidth="2.9" />
          {/* 가로획은 배지 위에 얹히므로 테마와 무관하게 흰색(paper) */}
          <path d="M7 15.6 H27" strokeWidth="1.6" style={{ stroke: 'rgb(var(--c-paper))' }} />
          <path d="M7 19.4 H27" strokeWidth="1.6" style={{ stroke: 'rgb(var(--c-paper))' }} />
        </g>
      </svg>
      <div className="hidden leading-none min-[440px]:block">
        <div className="whitespace-nowrap text-[17px] tracking-tight text-white">
          <span className="font-semibold">소비마스터</span>
        </div>
        <div className="label-mono mt-1.5 !text-[10px] uppercase tracking-[0.2em]">우리 집 가계부</div>
      </div>
    </div>
  );
}
