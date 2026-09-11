import { FormEvent, useCallback, useEffect, useState } from "react";
import {
  DeleteCardBenefit,
  GetCardBenefits,
  MoveCardBenefit,
  SaveCardBenefit,
} from "../wailsjs/go/main/App";
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
    <div className="cbn">
      <div className="stats-head">
        <h3>혜택</h3>
        <div className="seg-row">
          {card.benefitUrl && (
            <a className="ghost-btn cbn-src" href={card.benefitUrl} target="_blank" rel="noreferrer">
              자료 출처 ↗
            </a>
          )}
          <button className="ghost-btn" onClick={() => setEditing((v) => !v)}>
            {editing ? "편집 닫기" : "혜택 편집"}
          </button>
        </div>
      </div>

      {/* 카드 기본 조건 */}
      <div className="cbn-facts">
        <span>
          <span className="muted small">전월 실적</span>{" "}
          <strong>{card.perfTarget > 0 ? won(card.perfTarget) : "조건 없음"}</strong>
        </span>
        <span>
          <span className="muted small">연회비</span>{" "}
          <strong>
            {card.annualFee > 0 || card.annualFeeGlobal > 0
              ? `국내 ${won(card.annualFee)} / 해외 ${won(card.annualFeeGlobal)}`
              : "미입력"}
          </strong>
        </span>
      </div>

      {err && <p className="error">{err}</p>}

      {items.length === 0 ? (
        <p className="muted">
          등록된 혜택이 없습니다. <strong>혜택 편집</strong>으로 카드사 안내 내용을 옮겨 적으세요.
        </p>
      ) : (
        <table className="tx-table cbn-table">
          <thead>
            <tr>
              <th>영역</th>
              <th>구분</th>
              <th className="num">적립·할인</th>
              <th className="num">월 한도</th>
              <th>비고</th>
              {editing && <th></th>}
            </tr>
          </thead>
          <tbody>
            {items.map((b, i) => (
              <tr key={b.id} className={b.needsCheck ? "row-unclassified" : ""}>
                <td>{b.area}</td>
                <td><span className="badge">{KIND_LABEL[b.kind] ?? b.kind}</span></td>
                <td className="num">
                  {/* 서비스거나, 요율이 없는 정액 혜택(리터당 N원 등)은 숫자를 비우고 비고로 설명한다 */}
                  {b.kind === "service" || b.rateBp === 0 ? (
                    <span className="muted">—</span>
                  ) : (
                    <>
                      {b.isMax && <span className="muted small">최대 </span>}
                      <strong className="income">{bpToPct(b.rateBp)}</strong>
                    </>
                  )}
                </td>
                <td className="num">
                  {b.monthlyCap > 0 ? won(b.monthlyCap) : <span className="muted">—</span>}
                </td>
                <td className="small">
                  {b.note}
                  {b.needsCheck && (
                    <span className="badge shortfall" title="카드사 약관에서 상세 조건 확인 필요">
                      확인 필요
                    </span>
                  )}
                </td>
                {editing && (
                  <td className="actions">
                    <button className="ghost" title="위로" disabled={i === 0}
                      onClick={() => run(() => MoveCardBenefit(b.id, -1))}>↑</button>
                    <button className="ghost" title="아래로" disabled={i === items.length - 1}
                      onClick={() => run(() => MoveCardBenefit(b.id, 1))}>↓</button>
                    <button className="ghost" title="수정" onClick={() => edit(b)}>✎</button>
                    <button className="ghost" title="삭제"
                      onClick={() => run(() => DeleteCardBenefit(b.id))}>✕</button>
                  </td>
                )}
              </tr>
            ))}
          </tbody>
        </table>
      )}

      {unchecked > 0 && (
        <p className="muted small">
          ⚠ {unchecked}개 항목은 상세 조건(실적 구간별 차등·월 한도·제외 업종)을 아직 확인하지 못했습니다.
        </p>
      )}

      {card.benefitNote && <p className="muted small cbn-note">{card.benefitNote}</p>}

      {editing && (
        <form className="form-row cbn-form" onSubmit={save}>
          <label className="field cbn-area">
            영역
            <input
              type="text"
              placeholder="예: 차량유지관리 업종"
              value={f.area}
              onChange={(e) => setF({ ...f, area: e.target.value })}
            />
          </label>
          <label className="field">
            구분
            <select value={f.kind} onChange={(e) => setF({ ...f, kind: e.target.value })}>
              {Object.entries(KIND_LABEL).map(([k, v]) => (
                <option key={k} value={k}>{v}</option>
              ))}
            </select>
          </label>
          {f.kind !== "service" && (
            <label className="field">
              적립·할인 (%)
              <input
                type="text"
                inputMode="decimal"
                placeholder="예: 4.5"
                value={f.ratePct}
                onChange={(e) => setF({ ...f, ratePct: e.target.value })}
              />
            </label>
          )}
          <label className="field">
            월 한도 (원)
            <input
              type="text"
              inputMode="numeric"
              placeholder="없으면 비움"
              value={f.monthlyCap}
              onChange={(e) => setF({ ...f, monthlyCap: formatAmount(e.target.value) })}
            />
          </label>
          <label className="field cbn-note-field">
            비고
            <input
              type="text"
              placeholder="예: 전월 실적 구간별 차등"
              value={f.note}
              onChange={(e) => setF({ ...f, note: e.target.value })}
            />
          </label>
          <label className="check small">
            <input type="checkbox" checked={f.isMax}
              onChange={(e) => setF({ ...f, isMax: e.target.checked })} />
            최대치
          </label>
          <label className="check small">
            <input type="checkbox" checked={f.needsCheck}
              onChange={(e) => setF({ ...f, needsCheck: e.target.checked })} />
            확인 필요
          </label>
          <button type="submit">{f.id ? "수정" : "추가"}</button>
          {f.id !== 0 && (
            <button type="button" className="ghost" onClick={() => setF({ ...BLANK })}>취소</button>
          )}
        </form>
      )}
    </div>
  );
}
