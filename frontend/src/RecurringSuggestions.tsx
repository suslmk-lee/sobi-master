import { useCallback, useEffect, useState } from "react";
import { Plus, ScanSearch } from "lucide-react";
import { DismissRecurringCandidate, ListRecurringCandidates } from "../wailsjs/go/main/App";
import { store } from "../wailsjs/go/models";
import { won } from "./lib";
import IconTile from "./components/ui/IconTile";

// 기록에서 찾은 정기 출금 후보 (정기결제 화면 위쪽).
// 매달 같은 날(휴일이면 앞뒤 영업일) 비슷한 금액으로 빠진 출금 중 아직 정기결제로 등록되지 않은 것.
// [등록]은 아래 등록 폼을 이 값으로 채우고, [무시]는 다시 제안하지 않는다.

const WD = ["일", "월", "화", "수", "목", "금", "토"];
const md = (d: string) => {
  const t = new Date(d + "T00:00:00");
  return `${t.getMonth() + 1}/${t.getDate()}(${WD[t.getDay()]})`;
};
export const HOLIDAY_TEXT: Record<string, string> = {
  next: "휴일이면 다음 영업일",
  prev: "휴일이면 이전 영업일",
  none: "휴일에도 그날",
};
export const candidateAmount = (c: store.RecurringCandidate) =>
  c.fixed ? won(c.amount) : `${won(c.amountMin)}~${won(c.amountMax)}`;

export default function RecurringSuggestions({
  reloadKey,
  onRegister,
}: {
  reloadKey: number;
  onRegister: (c: store.RecurringCandidate) => void;
}) {
  const [list, setList] = useState<store.RecurringCandidate[]>([]);

  const load = useCallback(() => {
    ListRecurringCandidates()
      .then((r) => setList(r ?? []))
      .catch(() => setList([]));
  }, []);
  useEffect(load, [load, reloadKey]);

  const dismiss = async (c: store.RecurringCandidate) => {
    try {
      await DismissRecurringCandidate(c.key);
    } finally {
      load();
    }
  };

  if (list.length === 0) return null;
  return (
    <div className="surface p-5">
      <div className="mb-4 flex items-center gap-3">
        <IconTile icon={ScanSearch} tone="mint" size="sm" />
        <div className="min-w-0">
          <h3 className="section-title">기록에서 찾은 정기 출금 {list.length}건</h3>
          <p className="label-mono mt-0.5 !normal-case">
            매달 같은 날 비슷한 금액으로 빠졌는데 정기결제로 등록되지 않은 출금입니다. 등록하면 자금 계획 일정에 들어갑니다.
          </p>
        </div>
      </div>
      <div className="space-y-2">
        {list.map((c) => (
          <div key={c.key} className="surface-inset flex flex-wrap items-center gap-x-4 gap-y-2 px-4 py-3">
            <div className="min-w-0 flex-1 space-y-1">
              <p className="flex flex-wrap items-baseline gap-x-2 text-sm">
                <strong className="font-medium text-white">{c.merchant}</strong>
                <span className="num text-rose-300">{candidateAmount(c)}</span>
                <span className={`badge ${c.account ? "bg-amber-500/10 text-amber-300" : "bg-white/[0.06] text-zinc-300"}`}>
                  {c.account ? `계좌 · ${c.paymentLabel}` : c.paymentLabel}
                </span>
              </p>
              <p className="num text-xs text-zinc-300">
                매월 {c.day}일 · {HOLIDAY_TEXT[c.holidayRule] ?? ""}
                {c.nextDate && <span className="text-zinc-400"> · 다음 {md(c.nextDate)}</span>}
              </p>
              <p className="num text-xs text-zinc-500">
                {[c.category, c.member].filter(Boolean).join(" · ")}
                {(c.category || c.member) && " · "}
                최근 {c.months}개월 ({c.dates.slice(0, 4).map(md).join(", ")}
                {c.dates.length > 4 ? " …" : ""})
              </p>
              {c.merchants.length > 1 && (
                <p className="text-xs text-amber-300/90">
                  기록에 {c.merchants.map((m) => `'${m}'`).join("·")} 이름이 섞여 있습니다. 앞으로는 '{c.merchant}'로 적어 주세요.
                </p>
              )}
            </div>
            <div className="flex shrink-0 gap-2">
              <button type="button" className="btn-ghost !px-3 !py-1.5 !text-xs" onClick={() => dismiss(c)} title="다시 제안하지 않기">
                무시
              </button>
              <button type="button" className="btn-primary !px-3 !py-1.5 !text-xs" onClick={() => onRegister(c)}>
                <Plus className="h-3.5 w-3.5" />
                등록
              </button>
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}
