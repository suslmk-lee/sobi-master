import { Banknote, CreditCard, Landmark, LucideIcon } from "lucide-react";
import { isClosed } from "./lib";

// 결제수단을 GitHub 라벨 스타일의 색상 칩으로 표시한다.
// 카드 설정에서 지정한 색이 있으면 그 색을, 없으면 이름에서 결정적으로 계산한
// 자동 색을 쓴다 (같은 카드는 항상 같은 색).
// 팔레트는 사용자가 고르는 데이터 값이라 hex 로 둔다. 화면에는 color-mix 로 투명도·밝기를 입혀 쓴다.

export const CHIP_PALETTE = [
  "#3b5fd9",
  "#0ea5e9", // 하늘색
  "#d62a4d",
  "#f97316", // 밝은 주황
  "#eab308", // 노란색
  "#84cc16", // 연한 연두
  "#15803d",
  "#a16207",
  "#7e22ce",
  "#0e7490",
  "#c2410c",
  "#be185d",
  "#0f766e",
  "#475569",
];

const TYPE_ICON: Record<string, LucideIcon> = {
  card: CreditCard,
  cash: Banknote,
  bank: Landmark,
};

export function autoColor(name: string) {
  let h = 0;
  for (let i = 0; i < name.length; i++) {
    h = (h * 31 + name.charCodeAt(i)) >>> 0;
  }
  return CHIP_PALETTE[h % CHIP_PALETTE.length];
}

export default function PmChip({
  name,
  type,
  color,
  closedAt,
}: {
  name: string;
  type?: string;
  color?: string;
  closedAt?: string;
}) {
  if (!name) return <span className="text-zinc-500">—</span>;
  const fg = color && color.trim() ? color : autoColor(name);
  const closed = isClosed({ closedAt });
  const Icon = TYPE_ICON[type ?? "card"] ?? CreditCard;
  return (
    <span
      className={`inline-flex items-center gap-1.5 whitespace-nowrap rounded-full border py-0.5 pl-2 pr-2.5 text-xs font-semibold${
        closed ? " opacity-60" : ""
      }`}
      title={closed ? `해지된 카드 (${closedAt})` : undefined}
      style={{
        // 어두운 배경에서도 읽히도록 글자는 흰색 쪽으로 섞고, 배경·테두리는 사용자 색을 옅게 깐다
        backgroundColor: `color-mix(in srgb, ${fg} 22%, transparent)`,
        borderColor: `color-mix(in srgb, ${fg} 40%, transparent)`,
        color: `color-mix(in srgb, ${fg} 55%, white)`,
      }}
    >
      <Icon className="h-3 w-3 shrink-0" strokeWidth={2} aria-hidden="true" />
      {name}
      {closed && <span className="text-[10px] font-medium text-amber-300">해지</span>}
    </span>
  );
}
