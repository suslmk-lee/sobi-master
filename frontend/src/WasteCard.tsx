import { Flame } from "lucide-react";
import { store } from "../wailsjs/go/models";
import { won } from "./lib";
import { SectionHead } from "./components/ui/StatsUi";

// 대시보드의 '낭비율' 카드. 숫자는 DB 의 dash 뷰(spending_health.go)에서 오며 개인 대시보드와 같다.
//   낭비율 = (낭비 카테고리 지출 + 선택 카테고리의 예산 초과분) ÷ 그달 총지출
// 지출 구성 막대는 필수·선택·낭비·미지정·바구니로 총지출을 나눈다(다 더하면 총지출).

const pct = (r: number) => `${(r * 100).toFixed(1)}%`;

const PARTS: { key: "essential" | "discretionary" | "waste" | "unassigned" | "basket"; label: string; dot: string }[] = [
  { key: "essential", label: "필수", dot: "bg-emerald-400" },
  { key: "discretionary", label: "선택", dot: "bg-amber-400" },
  { key: "waste", label: "낭비", dot: "bg-rose-400" },
  { key: "unassigned", label: "미지정", dot: "bg-zinc-500" },
  { key: "basket", label: "바구니", dot: "bg-sky-400" },
];

// 미지정 비율이 이보다 크면 낭비율이 실제보다 낮게 나올 수 있다고 알린다.
const UNASSIGNED_WARN = 0.3;

// 최근 6개월 낭비율 — 막대 높이는 그 기간 최댓값 기준, 고른 달만 강조한다.
function RateBars({ trend, active }: { trend: store.WastePoint[]; active: string }) {
  const max = Math.max(...trend.map((p) => p.wasteRate), 0);
  return (
    <div className="flex items-end gap-1.5" aria-label="최근 6개월 낭비율">
      {trend.map((p) => {
        const h = max > 0 ? Math.max(4, (p.wasteRate / max) * 48) : 4;
        const on = p.ym === active;
        return (
          <div key={p.ym} className="flex flex-1 flex-col items-center gap-1" title={`${p.ym} ${pct(p.wasteRate)}`}>
            <span className={`num text-[10px] ${on ? "text-rose-300" : "text-zinc-500"}`}>
              {p.wasteRate > 0 ? `${Math.round(p.wasteRate * 100)}%` : "–"}
            </span>
            <div
              className={`w-full max-w-[22px] rounded-sm ${on ? "bg-rose-400" : "bg-white/[0.12]"}`}
              style={{ height: h }}
            />
            <span className={`num text-[10px] ${on ? "text-zinc-200" : "text-zinc-500"}`}>
              {Number(p.ym.slice(5))}월
            </span>
          </div>
        );
      })}
    </div>
  );
}

export default function WasteCard({ h }: { h: store.SpendingHealth }) {
  const mo = Number(h.ym.slice(5));
  const diff = h.wasteRate - h.prevWasteRate; // 비율 차이(%p)
  const cats = h.topWaste.filter((t) => t.kind === "category");
  const merchants = h.topWaste.filter((t) => t.kind === "merchant").slice(0, 3);
  const total = h.totalExpense;

  return (
    <div className="surface p-5 sm:p-6">
      <SectionHead icon={Flame} title={`낭비율 (${mo}월)`} sub="낭비 + 선택 카테고리의 예산 초과분 ÷ 총지출" />
      {total <= 0 ? (
        <p className="text-sm text-zinc-500">이 달에는 지출이 없습니다.</p>
      ) : (
        <div className="grid gap-6 lg:grid-cols-[minmax(0,2fr)_minmax(0,3fr)]">
          {/* 왼쪽: 낭비율 숫자와 추이 */}
          <div className="flex flex-col gap-4">
            <div>
              <div className="flex flex-wrap items-baseline gap-x-3 gap-y-1">
                <span className="num text-4xl font-light tracking-tight text-white">{pct(h.wasteRate)}</span>
                {(h.prevWasteRate > 0 || h.wasteRate > 0) && (
                  <span
                    className={`num text-xs ${diff > 0 ? "text-rose-300" : diff < 0 ? "text-emerald-300" : "text-zinc-400"}`}
                  >
                    {diff > 0 ? "▲" : diff < 0 ? "▼" : ""} {Math.abs(diff * 100).toFixed(1)}%p
                    <span className="text-zinc-500"> 전월 {pct(h.prevWasteRate)}</span>
                  </span>
                )}
              </div>
              <p className="num mt-2 text-sm text-zinc-400">
                낭비 <span className="text-rose-300">{won(h.wasteAmount)}</span> / 지출 {won(total)}
              </p>
              {h.wasteAmount > 0 && (
                <p className="num mt-0.5 text-xs text-zinc-500">
                  낭비 카테고리 {won(h.waste)} · 선택 예산 초과 {won(h.discretionaryOverBudget)}
                </p>
              )}
            </div>
            <RateBars trend={h.trend} active={h.ym} />
          </div>

          {/* 오른쪽: 지출 구성과 낭비 상위 */}
          <div className="flex min-w-0 flex-col gap-5">
            <div>
              <p className="label-mono mb-2">지출 구성</p>
              <div className="flex h-2.5 overflow-hidden rounded-full bg-white/[0.06]">
                {PARTS.map((p) =>
                  h[p.key] > 0 ? (
                    <div
                      key={p.key}
                      className={p.dot}
                      style={{ width: `${(h[p.key] / total) * 100}%` }}
                      title={`${p.label} ${won(h[p.key])}`}
                    />
                  ) : null,
                )}
              </div>
              <div className="mt-2.5 flex flex-wrap gap-x-4 gap-y-1.5 text-xs">
                {PARTS.filter((p) => h[p.key] > 0).map((p) => (
                  <span key={p.key} className="flex items-center gap-1.5 text-zinc-400">
                    <i className={`h-2 w-2 rounded-full ${p.dot}`} />
                    {p.label}
                    <span className="num text-zinc-200">{won(h[p.key])}</span>
                    <span className="num text-zinc-500">{pct(h[p.key] / total)}</span>
                  </span>
                ))}
              </div>
            </div>

            <div>
              <p className="label-mono mb-2">낭비 상위</p>
              {cats.length === 0 ? (
                <p className="text-sm text-zinc-500">
                  낭비로 잡힌 지출이 없습니다. 설정 → 카테고리에서 '낭비'·'선택'을 정하고, 선택 카테고리에 예산을 걸면
                  넘은 만큼이 여기에 나옵니다.
                </p>
              ) : (
                <ul className="space-y-1.5">
                  {cats.map((c) => (
                    <li key={c.name} className="flex items-baseline justify-between gap-3 text-sm">
                      <span className="truncate text-zinc-300">{c.name}</span>
                      <span className="num shrink-0 text-rose-300">{won(c.amount)}</span>
                    </li>
                  ))}
                </ul>
              )}
              {merchants.length > 0 && (
                <p className="num mt-2 truncate text-xs text-zinc-500">
                  가맹점: {merchants.map((m) => `${m.name} ${won(m.amount)}`).join(" · ")}
                </p>
              )}
            </div>

            {h.unassignedRate >= UNASSIGNED_WARN && (
              <p className="text-xs text-amber-300">
                지출의 {pct(h.unassignedRate)}가 성격 미지정이라 낭비율이 실제보다 낮게 나올 수 있습니다. 설정 →
                카테고리에서 필수·선택·낭비를 정해 주세요.
              </p>
            )}
          </div>
        </div>
      )}
    </div>
  );
}
