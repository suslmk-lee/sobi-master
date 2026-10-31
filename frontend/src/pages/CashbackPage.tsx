import { useCallback, useEffect, useState } from "react";
import { GetCashback, MarkPaid } from "../../wailsjs/go/main/App";
import { Check, Clock, Coins, Undo2 } from "lucide-react";
import { store } from "../../wailsjs/go/models";
import CountUp from "../components/CountUp";
import IconTile from "../components/ui/IconTile";
import { won } from "../lib";
import MonthPicker from "../MonthPicker";
import { autoColor } from "../PmChip";
import { Sk } from "../Skeleton";

// 캐시백 화면.
//
// 카드마다 결제액의 일정 비율이 기본으로 쌓이고, 카드에 따라서는 정해진 기한 안에
// 카드대금을 갚으면 추가분을 한 번 더 준다. 추가분은 기한을 넘기면 못 받으므로
// "지금 갚으면 받을 수 있는 건"을 맨 위에 남은 일수와 함께 띄우는 게 이 화면의 핵심이다.

const bpPct = (bp: number) => `${(bp / 100).toFixed(bp % 100 === 0 ? 0 : 2)}%`;

// 남은 일수를 D-표기로. 0이면 오늘이 마감.
const dday = (n: number) => (n === 0 ? "오늘 마감" : `D-${n}`);

export default function CashbackPage({
  month,
  setMonth,
}: {
  month: string;
  setMonth: (m: string) => void;
}) {
  const [rep, setRep] = useState<store.CashbackReport | null>(null);
  const [busy, setBusy] = useState(false);
  const [msg, setMsg] = useState("");
  const [err, setErr] = useState("");

  const load = useCallback(async () => {
    try {
      const [y, m] = month.split("-").map(Number);
      setRep(await GetCashback(y, m));
      setErr("");
    } catch (e: any) {
      setErr(String(e));
    }
  }, [month]);

  useEffect(() => {
    load();
  }, [load]);

  // paidAt: "" = 오늘로 기록, "-" = 기록 지우기
  const mark = async (ids: number[], paidAt: string, label: string) => {
    if (ids.length === 0) return;
    setBusy(true);
    setMsg("");
    try {
      await MarkPaid(ids, paidAt);
      setMsg(`${ids.length}건 ${label}`);
      await load();
    } catch (e: any) {
      setErr(String(e));
    } finally {
      setBusy(false);
    }
  };

  if (err)
    return (
      <div className="surface space-y-3 p-5">
        <p className="text-sm text-rose-300">캐시백 정보를 불러오지 못했습니다: {err}</p>
        <button className="btn-ghost" onClick={load}>다시 시도</button>
      </div>
    );

  if (!rep) {
    return (
      <div className="space-y-4">
        <div className="flex items-center">
          <MonthPicker value={month} onChange={setMonth} />
        </div>
        <div className="surface p-5"><Sk h={160} r={12} /></div>
      </div>
    );
  }

  if (!rep.hasCashbackCard) {
    return (
      <div className="space-y-4">
        <div className="flex items-center">
          <MonthPicker value={month} onChange={setMonth} />
        </div>
        <div className="surface space-y-2 p-5">
          <h3 className="section-title">캐시백</h3>
          <p className="text-sm text-zinc-400">
            캐시백이 설정된 카드가 없습니다. <strong className="text-white">카드 탭</strong>에서 카드를 수정해
            기본 적립률(예: 1%)과, 기한 내 납부 시 주는 추가 적립률·기한일(예: 1% / 5일)을
            입력하면 여기에 집계됩니다.
          </p>
        </div>
      </div>
    );
  }

  const pendingTotal = rep.pending.reduce((s, i) => s + i.possible, 0);

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="flex min-w-0 items-center gap-3">
          <IconTile icon={Coins} tone="mint" />
          <div className="min-w-0">
            <h2 className="text-2xl font-light tracking-tight text-white">캐시백</h2>
            <p className="label-mono num">{month}</p>
          </div>
        </div>
        <MonthPicker value={month} onChange={setMonth} />
      </div>

      {msg && <p className="label-mono">{msg}</p>}

      {/* 기한이 남은 건 — 이 화면의 핵심. 월과 무관하게 항상 보여준다. */}
      <div className="surface space-y-4 p-5">
        <div className="flex flex-wrap items-center justify-between gap-2">
          <div className="flex items-center gap-3">
            <IconTile icon={Clock} tone="amber" size="sm" />
            <h3 className="section-title">지금 갚으면 추가 적립</h3>
          </div>
          {rep.pending.length > 0 && (
            <button
              className="btn-primary"
              disabled={busy}
              onClick={() => mark(rep.pending.map((i) => i.txId), "", "납부 처리했습니다.")}
            >
              전부 오늘 납부 처리
            </button>
          )}
        </div>
        {rep.pending.length === 0 ? (
          <p className="text-sm text-zinc-400">기한이 남은 미납부 결제가 없습니다. 놓친 추가 적립이 없네요.</p>
        ) : (
          <>
            <p className="text-sm text-zinc-400">
              아래 {rep.pending.length}건을 기한 안에 갚으면{" "}
              <strong className="num text-emerald-300">{won(pendingTotal)}</strong>을
              더 받습니다. 기한이 급한 순서입니다.
            </p>
            <ul className="space-y-2">
              {rep.pending.map((it) => (
                <li
                  key={it.txId}
                  className="surface-inset flex flex-wrap items-center gap-x-4 gap-y-1.5 px-4 py-2.5 text-sm"
                >
                  <span
                    className={`badge w-20 justify-center ${
                      it.daysLeft <= 1 ? "bg-amber-500/15 text-amber-300" : "bg-white/5 text-zinc-300"
                    }`}
                  >
                    {dday(it.daysLeft)}
                  </span>
                  <span className="label-mono num">{it.date.slice(5)}</span>
                  <span className="min-w-0 flex-1 truncate text-zinc-100">{it.merchant}</span>
                  <span className="label-mono flex items-center gap-1.5">
                    <span className="h-2 w-2 rounded-full" style={{ background: autoColor(it.card) }} />
                    {it.card}
                  </span>
                  <span className="num text-zinc-300">{won(it.amount)}</span>
                  <span className="num font-semibold text-emerald-300">+{won(it.possible)}</span>
                  <button
                    className="btn-ghost !px-3 !py-1.5 !text-xs"
                    disabled={busy}
                    onClick={() => mark([it.txId], "", "납부 처리했습니다.")}
                  >
                    납부 처리
                  </button>
                </li>
              ))}
            </ul>
          </>
        )}
      </div>

      {/* 월별 카드 집계 */}
      {rep.cards.map((c) => {
        const total = c.totalBase + c.totalBonus;
        return (
          <div className="surface space-y-4 p-5" key={c.card.id}>
            <div className="flex flex-wrap items-center justify-between gap-2">
              <h3 className="section-title flex flex-wrap items-center gap-2">
                <span className="h-2.5 w-2.5 rounded-full" style={{ background: c.card.color || autoColor(c.card.name) }} />
                {c.card.name}
                <span className="label-mono font-normal">
                  기본 {bpPct(c.card.cashbackBp)}
                  {c.card.cashbackBonusBp > 0 &&
                    ` · ${c.card.cashbackBonusDays}일 내 납부 시 +${bpPct(c.card.cashbackBonusBp)}`}
                </span>
              </h3>
              <span className="text-sm text-zinc-400">
                {Number(month.slice(5))}월 적립{" "}
                <strong className="num font-semibold text-emerald-300">
                  <CountUp value={total} format={won} />
                </strong>
              </span>
            </div>

            <div className="grid grid-cols-2 gap-3 lg:grid-cols-4">
              <div className="surface-inset p-4">
                <div className="label-mono">대상 결제</div>
                <div className="num mt-1 text-lg font-semibold text-white">
                  <CountUp value={c.totalAmount} format={won} />
                </div>
              </div>
              <div className="surface-inset p-4">
                <div className="label-mono">기본 적립</div>
                <div className="num mt-1 text-lg font-semibold text-emerald-300">
                  <CountUp value={c.totalBase} format={won} />
                </div>
              </div>
              <div className="surface-inset p-4">
                <div className="label-mono">추가 적립(획득)</div>
                <div className="num mt-1 text-lg font-semibold text-emerald-300">
                  <CountUp value={c.totalBonus} format={won} />
                </div>
              </div>
              <div className="surface-inset p-4">
                <div className="label-mono">놓친 추가분</div>
                <div className={`num mt-1 text-lg font-semibold ${c.missedBonus > 0 ? "text-rose-300" : "text-white"}`}>
                  <CountUp value={c.missedBonus} format={won} />
                </div>
              </div>
            </div>

            {c.items.length === 0 ? (
              <p className="text-sm text-zinc-400">이 달 결제 내역이 없습니다.</p>
            ) : (
              <div className="overflow-x-auto">
                <table className="w-full text-sm">
                  <thead>
                    <tr className="table-head border-b border-white/[0.07] text-left">
                      <th className="py-2 pr-3 font-normal">날짜</th>
                      <th className="py-2 pr-3 font-normal">내용</th>
                      <th className="py-2 pr-3 text-right font-normal">금액</th>
                      <th className="py-2 pr-3 text-right font-normal">기본</th>
                      <th className="py-2 pr-3 text-right font-normal">추가</th>
                      <th className="py-2 pr-3 font-normal">납부</th>
                      <th className="py-2 font-normal"></th>
                    </tr>
                  </thead>
                  <tbody>
                    {c.items.map((it) => (
                      <tr key={it.txId} className="table-row">
                        <td className="num py-2.5 pr-3 text-zinc-400">{it.date.slice(5)}</td>
                        <td className="py-2.5 pr-3 text-zinc-100">{it.merchant}</td>
                        <td className="num py-2.5 pr-3 text-right">{won(it.amount)}</td>
                        <td className="num py-2.5 pr-3 text-right text-emerald-300">{won(it.base)}</td>
                        <td className="num py-2.5 pr-3 text-right">
                          {it.status === "earned" && it.bonus > 0 && (
                            <span className="text-emerald-300">{won(it.bonus)}</span>
                          )}
                          {it.status === "pending" && (
                            <span className="badge bg-amber-500/15 text-amber-300">{dday(it.daysLeft)} · +{won(it.possible)}</span>
                          )}
                          {it.status === "missed" && (
                            <span className="text-zinc-500" title={`놓친 추가분 ${won(it.possible)}`}>—</span>
                          )}
                        </td>
                        <td className="py-2.5 pr-3">
                          {it.paidAt ? (
                            <span className="label-mono num">
                              {it.paidAt.slice(5)} ({it.paidInDays}일)
                            </span>
                          ) : (
                            <span className="label-mono">미납부</span>
                          )}
                        </td>
                        <td className="py-2.5 text-right">
                          {it.paidAt ? (
                            <button
                              className="btn-icon"
                              title="납부 기록 취소"
                              aria-label="납부 기록 취소"
                              disabled={busy}
                              onClick={() => mark([it.txId], "-", "납부 기록을 지웠습니다.")}
                            ><Undo2 className="h-4 w-4" /></button>
                          ) : (
                            <button
                              className="btn-icon hover:!text-emerald-300"
                              title="오늘 납부 처리"
                              aria-label="오늘 납부 처리"
                              disabled={busy}
                              onClick={() => mark([it.txId], "", "납부 처리했습니다.")}
                            ><Check className="h-4 w-4" /></button>
                          )}
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </div>
        );
      })}
    </div>
  );
}
