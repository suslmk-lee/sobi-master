import { store } from "../wailsjs/go/models";
import { won } from "./lib";

// colors 를 주면 행 이름에 해당하는 색으로 막대를 칠한다 (예: 카드 지정 색).
export default function Bars({
  title,
  rows,
  colors,
}: {
  title: string;
  rows: store.NamedAmount[];
  colors?: Record<string, string>;
}) {
  const max = rows.reduce((m, r) => Math.max(m, r.amount), 0);
  return (
    <div className="card">
      <h3>{title}</h3>
      {rows.length === 0 && <p className="muted">데이터 없음</p>}
      {rows.map((r, i) => {
        const c = colors?.[r.name];
        return (
          <div className="bar-row" key={i}>
            <span className="bar-label">
              {c && <span className="cc-dot" style={{ background: c }} />}
              {r.name}
            </span>
            <div className="bar-track">
              <div
                className="bar-fill"
                style={{
                  width: max ? `${(r.amount / max) * 100}%` : 0,
                  ...(c ? { background: c } : {}),
                }}
              />
            </div>
            <span className="bar-amount">{won(r.amount)}</span>
          </div>
        );
      })}
    </div>
  );
}
