import { store } from "../wailsjs/go/models";
import { won } from "./lib";
import { ChartEmpty } from "./charts";
import { useThemeColors } from "./appearance/AppearanceContext";

// colors 를 주면 행 이름에 해당하는 색으로 막대를 칠한다 (예: 카드 지정 색).
// 지정 색이 없으면 강조색(mint) 막대.
// onPick 을 주면 각 줄이 버튼이 되어, 누르면 그 항목 이름으로 호출한다(내역 팝업 등).
export default function Bars({
  title,
  rows,
  colors,
  onPick,
}: {
  title: string;
  rows: store.NamedAmount[];
  colors?: Record<string, string>;
  onPick?: (name: string) => void;
}) {
  const tc = useThemeColors();
  const max = rows.reduce((m, r) => Math.max(m, r.amount), 0);
  const grid = "grid grid-cols-[minmax(70px,120px)_1fr_auto] items-center gap-3 text-xs";
  return (
    <div className="surface p-5 sm:p-6">
      <h3 className="section-title mb-4">{title}</h3>
      {rows.length === 0 && <ChartEmpty />}
      {/* 누를 수 있으면 줄마다 위아래 여백(py-1)을 넣는 대신 줄 간격을 줄여 같은 높이를 유지한다 */}
      <div className={onPick ? "space-y-0.5" : "space-y-2.5"}>
        {rows.map((r, i) => {
          const c = colors?.[r.name];
          const cells = (
            <>
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
            </>
          );
          return onPick ? (
            <button
              key={i}
              type="button"
              onClick={() => onPick(r.name)}
              title={`${r.name} 내역 보기`}
              className={`${grid} -mx-2 w-[calc(100%+1rem)] rounded-lg px-2 py-1 text-left transition-colors hover:bg-white/[0.05]`}
            >
              {cells}
            </button>
          ) : (
            <div key={i} className={grid}>
              {cells}
            </div>
          );
        })}
      </div>
    </div>
  );
}
