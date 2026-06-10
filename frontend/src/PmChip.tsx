// 결제수단을 GitHub 라벨 스타일의 색상 칩으로 표시한다.
// 카드 설정에서 지정한 색이 있으면 그 색을, 없으면 이름에서 결정적으로 계산한
// 자동 색을 쓴다 (같은 카드는 항상 같은 색).

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

const TYPE_ICON: Record<string, string> = {
  card: "💳",
  cash: "💵",
  bank: "🏦",
};

// "#3b5fd9" → "rgba(59, 95, 217, a)"
function withAlpha(hex: string, a: number) {
  const m = /^#?([0-9a-f]{6})$/i.exec(hex.trim());
  if (!m) return hex;
  const v = parseInt(m[1], 16);
  return `rgba(${(v >> 16) & 255}, ${(v >> 8) & 255}, ${v & 255}, ${a})`;
}

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
}: {
  name: string;
  type?: string;
  color?: string;
}) {
  if (!name) return <span className="muted">—</span>;
  const fg = color && color.trim() ? color : autoColor(name);
  return (
    <span className="pm-chip" style={{ background: withAlpha(fg, 0.14), color: fg }}>
      <span className="pm-icon">{TYPE_ICON[type ?? "card"] ?? "💳"}</span>
      {name}
    </span>
  );
}
