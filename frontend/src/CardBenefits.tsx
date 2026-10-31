import { FormEvent, useCallback, useEffect, useState } from "react";
import {
  DeleteCardBenefit,
  GetCardBenefits,
  MoveCardBenefit,
  SaveCardBenefit,
} from "../wailsjs/go/main/App";
import { AlertTriangle, ArrowDown, ArrowUp, ExternalLink, Pencil, Trash2 } from "lucide-react";
import { store } from "../wailsjs/go/models";
import { formatAmount, parseAmount, won } from "./lib";

// 카드 혜택 자료 패널.
//
// 카드사 안내를 옮겨 적는 표다. 카드사 표기가 "최대 4.5%"처럼 실적 구간에 따라
// 달라지는 경우가 많아, 최대치 여부와 "상세 미확인" 표시를 함께 남길 수 있게 했다.
// 그래야 나중에 이 자료를 믿고 판단해도 되는지 스스로 알 수 있다.

const KIND_LABEL: Record<string, string> = {
  point: "적립",
  discount: "할인",
  service: "서비스",
};

// 450 → "4.5%" (정수면 소수점 없이)
export const bpToPct = (bp: number) =>
  `${bp % 100 === 0 ? bp / 100 : (bp / 100).toFixed(2).replace(/0$/, "")}%`;

const BLANK = {
  id: 0,
  area: "",
  kind: "point",
  ratePct: "",
  isMax: false,
  monthlyCap: "",
  note: "",
  needsCheck: false,
};
type BenefitForm = typeof BLANK;

export default function CardBenefits({
  card,
  onChanged,
}: {
  card: store.PaymentMethod;
  onChanged?: () => void;
}) {
  const [items, setItems] = useState<store.CardBenefit[]>([]);
  const [f, setF] = useState<BenefitForm>({ ...BLANK });
  const [editing, setEditing] = useState(false);
  const [err, setErr] = useState("");

  const load = useCallback(async () => {
    try {
      setItems(await GetCardBenefits(card.id));
      setErr("");
    } catch (e: any) {
      setErr(String(e));
    }
  }, [card.id]);

  useEffect(() => {
    load();
  }, [load]);

  const run = async (fn: () => Promise<unknown>) => {
    setErr("");
    try {
      await fn();
      await load();
      onChanged?.(); // 카드 타일의 대표 혜택 미리보기도 같이 갱신
    } catch (e: any) {
      setErr(String(e));
    }
  };

  const save = async (e: FormEvent) => {
    e.preventDefault();
    if (!f.area.trim()) {
      setErr("혜택 영역을 입력하세요.");
      return;
    }
    await run(async () => {
      await SaveCardBenefit(
        new store.CardBenefit({
          id: f.id,
          paymentMethodId: card.id,
          area: f.area.trim(),
          kind: f.kind,
          rateBp: Math.round((Number(f.ratePct) || 0) * 100),
          isMax: f.isMax,
          monthlyCap: f.monthlyCap ? parseAmount(f.monthlyCap) : 0,
          note: f.note.trim(),
          needsCheck: f.needsCheck,
          sortOrder: 0,
        })
      );
      setF({ ...BLANK });
    });
  };

  const edit = (b: store.CardBenefit) => {
    setEditing(true);
    setF({
      id: b.id,
      area: b.area,
      kind: b.kind,
      ratePct: b.rateBp ? String(b.rateBp / 100) : "",
      isMax: b.isMax,
      monthlyCap: b.monthlyCap ? formatAmount(String(b.monthlyCap)) : "",
      note: b.note,
      needsCheck: b.needsCheck,
    });
  };

  const unchecked = items.filter((b) => b.needsCheck).length;

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <h3 className="section-title">혜택</h3>
        <div className="flex items-center gap-2">
          {card.benefitUrl && (
            <a className="btn-ghost !px-3 !py-1.5 !text-xs" href={card.benefitUrl} target="_blank" rel="noreferrer">
              자료 출처
              <ExternalLink className="h-3.5 w-3.5" />
            </a>
          )}
          <button className="btn-ghost !px-3 !py-1.5 !text-xs" onClick={() => setEditing((v) => !v)}>
            {editing ? "편집 닫기" : "혜택 편집"}
          </button>
        </div>
      </div>

      {/* 카드 기본 조건 */}
      <div className="flex flex-wrap gap-2">
        <span className="chip">
          <span className="text-zinc-500">전월 실적</span>
          <strong className="num font-medium text-white">{card.perfTarget > 0 ? won(card.perfTarget) : "조건 없음"}</strong>
        </span>
        <span className="chip">
          <span className="text-zinc-500">연회비</span>
          <strong className="num font-medium text-white">
            {card.annualFee > 0 || card.annualFeeGlobal > 0
              ? `국내 ${won(card.annualFee)} / 해외 ${won(card.annualFeeGlobal)}`
              : "미입력"}
          </strong>
        </span>
      </div>

      {err && <p className="text-sm text-rose-300">{err}</p>}

      {items.length === 0 ? (
        <p className="text-sm text-zinc-400">
          등록된 혜택이 없습니다. <strong className="text-white">혜택 편집</strong>으로 카드사 안내 내용을 옮겨 적으세요.
        </p>
      ) : (
        <div className="overflow-x-auto">
          <table className="w-full text-sm">
            <thead>
              <tr className="table-head border-b border-white/[0.07] text-left">
                <th className="py-2 pr-3 font-normal">영역</th>
                <th className="py-2 pr-3 font-normal">구분</th>
                <th className="py-2 pr-3 text-right font-normal">적립·할인</th>
                <th className="py-2 pr-3 text-right font-normal">월 한도</th>
                <th className="py-2 pr-3 font-normal">비고</th>
                {editing && <th className="py-2 font-normal"></th>}
              </tr>
            </thead>
            <tbody>
              {items.map((b, i) => (
                <tr key={b.id} className="table-row">
                  <td className="py-2.5 pr-3 text-zinc-100">{b.area}</td>
                  <td className="py-2.5 pr-3">
                    <span className="chip">{KIND_LABEL[b.kind] ?? b.kind}</span>
                  </td>
                  <td className="num py-2.5 pr-3 text-right">
                    {/* 서비스거나, 요율이 없는 정액 혜택(리터당 N원 등)은 숫자를 비우고 비고로 설명한다 */}
                    {b.kind === "service" || b.rateBp === 0 ? (
                      <span className="text-zinc-500">—</span>
                    ) : (
                      <>
                        {b.isMax && <span className="label-mono">최대 </span>}
                        <strong className="font-semibold text-mint-300">{bpToPct(b.rateBp)}</strong>
                      </>
                    )}
                  </td>
                  <td className="num py-2.5 pr-3 text-right">
                    {b.monthlyCap > 0 ? won(b.monthlyCap) : <span className="text-zinc-500">—</span>}
                  </td>
                  <td className="py-2.5 pr-3">
                    <span className="label-mono whitespace-normal">{b.note}</span>
                    {b.needsCheck && (
                      <span className="badge ml-1.5 bg-amber-500/15 text-amber-300" title="카드사 약관에서 상세 조건 확인 필요">
                        확인 필요
                      </span>
                    )}
                  </td>
                  {editing && (
                    <td className="py-2.5">
                      <div className="flex justify-end gap-1">
                        <button className="btn-icon disabled:opacity-30" title="위로" aria-label="위로" disabled={i === 0}
                          onClick={() => run(() => MoveCardBenefit(b.id, -1))}>
                          <ArrowUp className="h-4 w-4" />
                        </button>
                        <button className="btn-icon disabled:opacity-30" title="아래로" aria-label="아래로" disabled={i === items.length - 1}
                          onClick={() => run(() => MoveCardBenefit(b.id, 1))}>
                          <ArrowDown className="h-4 w-4" />
                        </button>
                        <button className="btn-icon" title="수정" aria-label="수정" onClick={() => edit(b)}>
                          <Pencil className="h-4 w-4" />
                        </button>
                        <button className="btn-icon hover:!text-rose-300" title="삭제" aria-label="삭제"
                          onClick={() => run(() => DeleteCardBenefit(b.id))}>
                          <Trash2 className="h-4 w-4" />
                        </button>
                      </div>
                    </td>
                  )}
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {unchecked > 0 && (
        <p className="flex items-center gap-1.5 text-xs text-amber-300">
          <AlertTriangle className="h-3.5 w-3.5 shrink-0" />
          {unchecked}개 항목은 상세 조건(실적 구간별 차등·월 한도·제외 업종)을 아직 확인하지 못했습니다.
        </p>
      )}

      {card.benefitNote && <p className="label-mono whitespace-normal">{card.benefitNote}</p>}

      {editing && (
        <form className="surface-inset grid items-end gap-3 p-4 sm:grid-cols-2 lg:grid-cols-4" onSubmit={save}>
          <label className="block sm:col-span-2">
            <span className="field-label">영역</span>
            <input
              type="text"
              className="input-field"
              placeholder="예: 차량유지관리 업종"
              value={f.area}
              onChange={(e) => setF({ ...f, area: e.target.value })}
            />
          </label>
          <label className="block">
            <span className="field-label">구분</span>
            <select className="input-field" value={f.kind} onChange={(e) => setF({ ...f, kind: e.target.value })}>
              {Object.entries(KIND_LABEL).map(([k, v]) => (
                <option key={k} value={k}>{v}</option>
              ))}
            </select>
          </label>
          {f.kind !== "service" && (
            <label className="block">
              <span className="field-label">적립·할인 (%)</span>
              <input
                type="text"
                className="input-field num"
                inputMode="decimal"
                placeholder="예: 4.5"
                value={f.ratePct}
                onChange={(e) => setF({ ...f, ratePct: e.target.value })}
              />
            </label>
          )}
          <label className="block">
            <span className="field-label">월 한도 (원)</span>
            <input
              type="text"
              className="input-field num"
              inputMode="numeric"
              placeholder="없으면 비움"
              value={f.monthlyCap}
              onChange={(e) => setF({ ...f, monthlyCap: formatAmount(e.target.value) })}
            />
          </label>
          <label className="block sm:col-span-2">
            <span className="field-label">비고</span>
            <input
              type="text"
              className="input-field"
              placeholder="예: 전월 실적 구간별 차등"
              value={f.note}
              onChange={(e) => setF({ ...f, note: e.target.value })}
            />
          </label>
          <label className="flex items-center gap-2 text-sm text-zinc-300">
            <input type="checkbox" className="accent-mint-400" checked={f.isMax}
              onChange={(e) => setF({ ...f, isMax: e.target.checked })} />
            최대치
          </label>
          <label className="flex items-center gap-2 text-sm text-zinc-300">
            <input type="checkbox" className="accent-mint-400" checked={f.needsCheck}
              onChange={(e) => setF({ ...f, needsCheck: e.target.checked })} />
            확인 필요
          </label>
          <div className="flex gap-2">
            <button type="submit" className="btn-primary">{f.id ? "수정" : "추가"}</button>
            {f.id !== 0 && (
              <button type="button" className="btn-ghost" onClick={() => setF({ ...BLANK })}>취소</button>
            )}
          </div>
        </form>
      )}
    </div>
  );
}
