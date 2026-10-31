import { store } from "../wailsjs/go/models";
import { won } from "./lib";
import { ChartEmpty } from "./charts";
import { useThemeColors } from "./appearance/AppearanceContext";

// colors 를 주면 행 이름에 해당하는 색으로 막대를 칠한다 (예: 카드 지정 색).
// 지정 색이 없으면 강조색(mint) 막대.
export default function Bars({
  title,
  rows,
  colors,
}: {
  title: string;
  rows: store.NamedAmount[];
  colors?: Record<string, string>;
}) {
  const tc = useThemeColors();
  const max = rows.reduce((m, r) => Math.max(m, r.amount), 0);
  return (
    <div className="surface p-5 sm:p-6">
      <h3 className="section-title mb-4">{title}</h3>
      {rows.length === 0 && <ChartEmpty />}
      <div className="space-y-2.5">
        {rows.map((r, i) => {
          const c = colors?.[r.name];
          return (
            <div key={i} className="grid grid-cols-[minmax(70px,120px)_1fr_auto] items-center gap-3 text-xs">
              <span className="flex items-center gap-1.5 text-zinc-300 truncate">
                {c && <i className="w-2 h-2 rounded-full shrink-0" style={{ background: c }} />}
                <span className="truncate">{r.name}</span>
              </span>
              <div className="h-2 rounded-full bg-white/[0.04] overflow-hidden">
                <div
                  className="h-full rounded-full"
                  style={{ width: max ? `${(r.amount / max) * 100}%` : 0, background: c ?? tc.accent }}
                />
              </div>
              <span className="num text-white text-right min-w-[88px]">{won(r.amount)}</span>
            </div>
          );
        })}
      </div>
    </div>
  );
}
