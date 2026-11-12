// frontend/src/components/ui/IconTile.tsx
import { LucideIcon } from 'lucide-react';
import { ACCENT_FILL, duoFor } from '../icons';

type Tone = 'warm' | 'mint' | 'amber' | 'sky';

// 면 색: 기본은 테마 강조색, 경고·안내 성격의 팝업은 그 상태색
const TONE_FILL: Record<Tone, string> = {
  warm: ACCENT_FILL,
  mint: ACCENT_FILL,
  amber: 'rgb(var(--c-amber-400))',
  sky: 'rgb(var(--c-sky-400))',
};

interface IconTileProps {
  icon: LucideIcon;
  tone?: Tone;
  size?: 'sm' | 'md';
  className?: string;
}

// 탭 · 섹션 머리 · 설정 묶음 · 팝업 제목 아이콘. 이름은 예전 타일에서 왔지만 이제 타일 없이
// 두 색 아이콘(선 + 어긋난 테마색 면, components/icons.tsx)을 그린다. 호출부는 lucide 아이콘을 그대로 넘긴다.
export default function IconTile({ icon, tone = 'warm', size = 'md', className = '' }: IconTileProps) {
  const Duo = duoFor(icon);
  const sizeClass = size === 'sm' ? 'h-7 w-7' : 'h-9 w-9';
  return <Duo className={`shrink-0 text-zinc-100 ${sizeClass} ${className}`} fill={TONE_FILL[tone]} />;
}
