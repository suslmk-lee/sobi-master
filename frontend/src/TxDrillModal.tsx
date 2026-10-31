import { useMemo, useState } from "react";
import { ArrowDown, ArrowUp, BarChart3, Pencil, X } from "lucide-react";
import { store } from "../wailsjs/go/models";
import { won } from "./lib";
import { MiniTrend } from "./charts";
import Portal from "./components/ui/Portal";

// 표의 칸 하나를 열어 보는 모달.
// 위에는 그 항목의 월별 추이, 아래에는 고른 달의 거래 목록. 목록에서 바로 고칠 수 있다.
// 통계의 "월별 상세" 표와 "카테고리 × 월 히트맵"이 같이 쓴다.
// 목록은 열 제목을 눌러 정렬한다(같은 열을 다시 누르면 순서가 뒤집힌다). 기본은 날짜 최신순.

type SortKey = "date" | "merchant" | "category" | "member" | "payment" | "amount";
type SortDir = "asc" | "desc";

// 처음 눌렀을 때의 방향: 날짜·금액은 큰 쪽(최신·큰 금액)부터, 글자는 가나다순
const FIRST_DIR: Record<SortKey, SortDir> = {
  date: "desc",
  amount: "desc",
  merchant: "asc",
  category: "asc",
  member: "asc",
  payment: "asc",
};

const sortValue = (t: store.Transaction, k: SortKey): string | number => {
  switch (k) {
    case "date":
      return t.date;
    case "amount":
      return t.amount;
    case "merchant":
      return t.merchant;
    case "category":
      return t.categoryName;
    case "member":
      return t.memberName;
    case "payment":
      return t.paymentMethodName;
  }
};
export default function TxDrillModal({
  title,
  ym,
  months,
  values,
  txs,
  onPickMonth,
  onEdit,
  onClose,
  heading,
}: {
  title: string;
  ym: string; // "YYYY-MM"
  months: string[]; // 추이에 그릴 월 목록 (없으면 그래프 생략)
  values: number[]; // months 와 같은 길이
  txs: store.Transaction[];
  onPickMonth: (ym: string) => void;
  onEdit: (t: store.Transaction) => void;
  onClose: () => void;
  // heading 을 주면 "항목 · N월" 대신 이 제목을 쓴다(지출 달력의 "10월 12일 (월)" 등)
  heading?: string;
}) {
  const active = months.indexOf(ym);
  const total = values.reduce((s, v) => s + v, 0);
  const prev = active > 0 ? values[active - 1] : 0;
  const delta =
    active > 0 && prev > 0 ? Math.round(((values[active] - prev) / prev) * 100) : null;
  const sum = txs.reduce((s, t) => s + t.amount, 0);

  const [sort, setSort] = useState<{ key: SortKey; dir: SortDir }>({ key: "date", dir: "desc" });
  const sorted = useMemo(() => {
    const sign = sort.dir === "asc" ? 1 : -1;
    return [...txs].sort((a, b) => {
      const va = sortValue(a, sort.key);
      const vb = sortValue(b, sort.key);
      // 빈 값(미지정)은 방향과 상관없이 맨 뒤로
      if (va === "" && vb !== "") return 1;
      if (vb === "" && va !== "") return -1;
      const c =
        typeof va === "number" && typeof vb === "number"
          ? va - vb
          : String(va).localeCompare(String(vb), "ko");
      if (c !== 0) return c * sign;
      // 같으면 날짜 최신순 → id 로 순서를 고정한다
      return b.date.localeCompare(a.date) || b.id - a.id;
    });
  }, [txs, sort]);
  const toggle = (key: SortKey) =>
    setSort((s) => (s.key === key ? { key, dir: s.dir === "asc" ? "desc" : "asc" } : { key, dir: FIRST_DIR[key] }));

  // 정렬 가능한 열 제목: 누르면 정렬, 정렬 중이면 민트색 + 화살표
  const Th = ({ k, label, right, className = "" }: { k: SortKey; label: string; right?: boolean; className?: string }) => {
    const on = sort.key === k;
    const Arrow = sort.dir === "asc" ? ArrowUp : ArrowDown;
    return (
      <th
        className={`whitespace-nowrap py-2.5 font-normal ${right ? "text-right" : "text-left"} ${className}`}
        aria-sort={on ? (sort.dir === "asc" ? "ascending" : "descending") : "none"}
      >
        <button
          type="button"
          onClick={() => toggle(k)}
          className={`inline-flex items-center gap-1 uppercase tracking-wider transition-colors ${
            on ? "text-mint-300" : "hover:text-zinc-300"
          }`}
          title={`${label} 기준 정렬`}
        >
          {label}
          {on && <Arrow className="h-3 w-3" />}
        </button>
      </th>
    );
  };

  // 모달은 Portal 로 body 에 띄운다. 페이지 안에 두면 조상의 transform/filter 가
  // position:fixed 의 기준 상자를 바꿔 모달이 화면 밖으로 밀려날 수 있다.
  return (
    <Portal>
      <div
        className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-3 backdrop-blur-sm sm:p-6"
        onMouseDown={(e) => {
          if (e.target === e.currentTarget) onClose();
        }}
      >
        <div
          role="dialog"
          aria-modal="true"
          aria-label={heading ?? `${title} · ${Number(ym.slice(5))}월`}
          className="surface max-h-[92vh] w-full max-w-5xl overflow-y-auto p-4 sm:p-6"
        >
          <div className="mb-4 flex items-start justify-between gap-3">
            <div className="flex min-w-0 items-center gap-3">
              <span className="icon-tile icon-tile-mint !h-8 !w-8 !rounded-lg">
                <BarChart3 className="h-4 w-4" strokeWidth={1.75} />
              </span>
              <div className="min-w-0">
                <h3 className="section-title">
                  {heading ?? (
                    <>
                      {title} · {Number(ym.slice(5))}월
                    </>
                  )}
                </h3>
                {!heading && <p className="label-mono !normal-case mt-0.5">{ym}</p>}
              </div>
            </div>
            <button
              type="button"
              onClick={onClose}
              className="btn-icon shrink-0"
              aria-label="닫기"
              title="닫기"
            >
              <X className="h-4 w-4" />
            </button>
          </div>

          {/* 이 항목이 달마다 어떻게 움직였는지 먼저 보여 준다 */}
          {months.length > 0 && values.length === months.length && (
            <div className="surface-inset mb-4 p-4">
              <MiniTrend
                values={values}
                months={months}
                active={active}
                onPick={(i) => onPickMonth(months[i])}
              />
              <p className="mt-2 text-xs text-zinc-500">
                {months.length}개월 합계 <span className="num">{won(total)}</span>
                {delta !== null && (
                  <>
                    {" · 전월 대비 "}
                    <span
                      className={`num font-medium ${
                        delta > 0 ? "text-rose-300" : delta < 0 ? "text-emerald-300" : "text-zinc-400"
                      }`}
                    >
                      {delta > 0 ? "▲" : delta < 0 ? "▼" : ""} {Math.abs(delta)}%
                    </span>
                  </>
                )}
                {" · 값이 있는 달을 누르면 그 달로 이동합니다"}
              </p>
            </div>
          )}

          {txs.length === 0 ? (
            <p className="py-10 text-center text-sm text-zinc-500">
              거래를 불러오지 못했거나 내역이 없습니다.
            </p>
          ) : (
            <>
              <p className="text-xs text-zinc-500">
                <span className="num">{txs.length}</span>건 · 합계{" "}
                <strong className="num font-medium text-rose-300">{won(sum)}</strong>
              </p>
              <div className="mt-3 max-h-[50vh] overflow-auto">
                <table className="w-full text-sm">
                  <thead>
                    <tr className="table-head border-b border-white/[0.08]">
                      <Th k="date" label="날짜" className="pr-3" />
                      <Th k="merchant" label="내용" className="px-3" />
                      <Th k="category" label="카테고리" className="px-3" />
                      <Th k="member" label="귀속자" className="px-3" />
                      <Th k="payment" label="결제수단" className="px-3" />
                      <Th k="amount" label="금액" right className="px-3" />
                      <th className="py-2.5 pl-3"></th>
                    </tr>
                  </thead>
                  <tbody>
                    {sorted.map((t) => (
                      <tr key={t.id} className="table-row text-zinc-200">
                        <td className="num whitespace-nowrap py-2.5 pr-3">{t.date}</td>
                        <td className="px-3 py-2.5">
                          {t.merchant}
                          {t.memo && <span className="text-xs text-zinc-500"> · {t.memo}</span>}
                        </td>
                        <td className="whitespace-nowrap px-3 py-2.5">
                          {t.categoryName || <span className="text-zinc-500">미지정</span>}
                        </td>
                        <td className="whitespace-nowrap px-3 py-2.5">
                          {t.memberName || <span className="text-zinc-500">미지정</span>}
                        </td>
                        <td className="whitespace-nowrap px-3 py-2.5">
                          {t.paymentMethodName || <span className="text-zinc-500">미지정</span>}
                        </td>
                        <td className="num whitespace-nowrap px-3 py-2.5 text-right font-medium text-rose-300">
                          {won(t.amount)}
                        </td>
                        <td className="py-2.5 pl-3 text-right">
                          <button
                            type="button"
                            className="btn-ghost !px-3 !py-1.5 !text-xs"
                            onClick={() => onEdit(t)}
                          >
                            <Pencil className="h-3 w-3" />
                            수정
                          </button>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </>
          )}
        </div>
      </div>
    </Portal>
  );
}
