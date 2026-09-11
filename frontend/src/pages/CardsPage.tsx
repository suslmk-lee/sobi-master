import { FormEvent, useCallback, useEffect, useState } from "react";
import {
  DeletePaymentMethod,
  GetAllCardBenefits,
  GetCardBreakdown,
  GetCardStatuses,
  SaveCard,
} from "../../wailsjs/go/main/App";
import { store } from "../../wailsjs/go/models";
import { formatAmount, parseAmount, won } from "../lib";
import Bars from "../Bars";
import CardBenefits, { bpToPct as bpToPctLabel } from "../CardBenefits";
import PmChip, { autoColor, CHIP_PALETTE } from "../PmChip";
import { CardsSkeleton } from "../Skeleton";

const ISSUERS = [
  "신한카드",
  "삼성카드",
  "현대카드",
  "KB국민카드",
  "롯데카드",
  "우리카드",
  "하나카드",
  "NH농협카드",
  "BC카드",
  "IBK기업은행",
  "카카오뱅크",
  "케이뱅크",
  "토스뱅크",
  "씨티카드",
  "우체국",
  "새마을금고",
  "신협",
  "기타",
];

const EMPTY = {
  id: 0,
  issuer: "",
  name: "",
  billingDay: "",
  cycleStartDay: "1",
  perfTarget: "",
  color: "",
  // 캐시백: 화면에는 %로 입력받고 저장할 때 만분율(bp)로 바꾼다 (1% → 100)
  cashbackPct: "",
  cashbackBonusPct: "",
  cashbackBonusDays: "",
  // 혜택 자료: 카드사 안내에서 옮겨 적는 값들
  annualFee: "",
  annualFeeGlobal: "",
  benefitUrl: "",
  benefitNote: "",
};

type FormState = typeof EMPTY;

// "1.5" → 150 (만분율). 빈 값이나 숫자가 아니면 0.
const pctToBp = (s: string) => Math.round((Number(s) || 0) * 100);
// 150 → "1.5" (0 이면 빈 칸으로 둬서 "설정 안 함"이 드러나게)
const bpToPct = (bp: number) => (bp ? String(bp / 100) : "");

// 카드 등록/수정 폼: 카드사, 카드명, 결제일, 실적 산정 시작일, 실적한도.
function CardForm({
  initial,
  onSaved,
  onCancel,
}: {
  initial: FormState;
  onSaved: () => void;
  onCancel?: () => void;
}) {
  const [f, setF] = useState(initial);
  const [err, setErr] = useState("");
  useEffect(() => setF(initial), [initial]);

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    setErr("");
    if (!f.name.trim()) {
      setErr("카드 이름을 입력하세요.");
      return;
    }
    try {
      await SaveCard(
        new store.PaymentMethod({
          id: f.id,
          name: f.name,
          type: "card",
          issuer: f.issuer,
          billingDay: f.billingDay ? Number(f.billingDay) : 0,
          cycleStartDay: f.cycleStartDay ? Number(f.cycleStartDay) : 1,
          perfTarget: parseAmount(f.perfTarget) || 0,
          color: f.color,
          cashbackBp: pctToBp(f.cashbackPct),
          cashbackBonusBp: pctToBp(f.cashbackBonusPct),
          cashbackBonusDays: f.cashbackBonusDays ? Number(f.cashbackBonusDays) : 0,
          annualFee: parseAmount(f.annualFee) || 0,
          annualFeeGlobal: parseAmount(f.annualFeeGlobal) || 0,
          benefitUrl: f.benefitUrl.trim(),
          benefitNote: f.benefitNote.trim(),
        })
      );
      setF({ ...EMPTY });
      onSaved();
    } catch (e: any) {
      setErr(String(e));
    }
  };

  return (
    <form className="card" onSubmit={submit}>
      <h3>{f.id ? "카드 수정" : "카드 등록"}</h3>
      <div className="form-row">
        <select
          value={f.issuer}
          onChange={(e) => setF({ ...f, issuer: e.target.value })}
        >
          <option value="">카드사 선택</option>
          {/* 기존 데이터의 카드사가 목록에 없어도 선택 상태가 유지되도록 추가 */}
          {f.issuer && !ISSUERS.includes(f.issuer) && (
            <option value={f.issuer}>{f.issuer}</option>
          )}
          {ISSUERS.map((name) => (
            <option key={name} value={name}>{name}</option>
          ))}
        </select>
        <input
          type="text"
          placeholder="카드 이름 (예: 딥드림 체크)"
          value={f.name}
          onChange={(e) => setF({ ...f, name: e.target.value })}
        />
      </div>
      <div className="form-row">
        <label className="field">
          결제일 (매월)
          <input
            type="number"
            min={1}
            max={31}
            placeholder="예: 25"
            value={f.billingDay}
            onChange={(e) => setF({ ...f, billingDay: e.target.value })}
          />
        </label>
        <label className="field">
          실적 시작일 (1이면 1일~말일)
          <input
            type="number"
            min={1}
            max={31}
            value={f.cycleStartDay}
            onChange={(e) => setF({ ...f, cycleStartDay: e.target.value })}
          />
        </label>
        <label className="field">
          실적한도 (원)
          <input
            type="text"
            inputMode="numeric"
            placeholder="예: 300,000"
            value={f.perfTarget}
            onChange={(e) => setF({ ...f, perfTarget: formatAmount(e.target.value) })}
          />
        </label>
        <button type="submit">{f.id ? "저장" : "등록"}</button>
        {f.id !== 0 && onCancel && (
          <button type="button" className="ghost" onClick={onCancel}>취소</button>
        )}
      </div>
      {/* 캐시백: 결제액의 기본 %, 그리고 기한 내 카드대금 납부 시 얹어 주는 추가 % */}
      <div className="form-row">
        <label className="field">
          캐시백 기본 (%)
          <input
            type="text"
            inputMode="decimal"
            placeholder="예: 1"
            value={f.cashbackPct}
            onChange={(e) => setF({ ...f, cashbackPct: e.target.value })}
          />
        </label>
        <label className="field">
          기한 내 납부 시 추가 (%)
          <input
            type="text"
            inputMode="decimal"
            placeholder="예: 1"
            value={f.cashbackBonusPct}
            onChange={(e) => setF({ ...f, cashbackBonusPct: e.target.value })}
          />
        </label>
        <label className="field">
          추가 적립 기한 (일)
          <input
            type="number"
            min={0}
            max={60}
            placeholder="예: 5"
            value={f.cashbackBonusDays}
            onChange={(e) => setF({ ...f, cashbackBonusDays: e.target.value })}
          />
        </label>
        <span className="muted small cb-hint">
          결제일로부터 기한 안에 카드대금을 갚으면 추가분을 받습니다. 캐시백 탭에서 집계·독촉을 봅니다.
        </span>
      </div>
      {/* 혜택 자료: 카드사 안내를 그대로 적어 두는 칸. 앱 계산에는 쓰지 않는다 */}
      <div className="form-row">
        <label className="field">
          연회비 국내 (원)
          <input
            type="text"
            inputMode="numeric"
            placeholder="예: 70,000"
            value={f.annualFee}
            onChange={(e) => setF({ ...f, annualFee: formatAmount(e.target.value) })}
          />
        </label>
        <label className="field">
          연회비 해외 (원)
          <input
            type="text"
            inputMode="numeric"
            placeholder="예: 70,000"
            value={f.annualFeeGlobal}
            onChange={(e) => setF({ ...f, annualFeeGlobal: formatAmount(e.target.value) })}
          />
        </label>
        <label className="field grow">
          혜택 자료 출처 (URL)
          <input
            type="text"
            placeholder="카드사 안내 페이지 주소"
            value={f.benefitUrl}
            onChange={(e) => setF({ ...f, benefitUrl: e.target.value })}
          />
        </label>
      </div>
      <div className="form-row">
        <label className="field grow">
          혜택 유의사항
          <input
            type="text"
            placeholder="예: 전월 실적 50만원 이상 시 적용, 실적 산정 제외 업종 있음"
            value={f.benefitNote}
            onChange={(e) => setF({ ...f, benefitNote: e.target.value })}
          />
        </label>
      </div>
      <div className="form-row">
        <label className="field">
          칩 색상 (목록에서 이 카드를 표시할 색)
          <div className="swatches">
            <button
              type="button"
              className={`swatch auto ${f.color === "" ? "sel" : ""}`}
              title="자동 (이름 기반)"
              onClick={() => setF({ ...f, color: "" })}
            >
              자동
            </button>
            {CHIP_PALETTE.map((c) => (
              <button
                key={c}
                type="button"
                className={`swatch ${f.color === c ? "sel" : ""}`}
                style={{ background: c }}
                onClick={() => setF({ ...f, color: c })}
              />
            ))}
          </div>
        </label>
        {f.name && (
          <label className="field">
            미리보기
            <PmChip name={f.name} type="card" color={f.color} />
          </label>
        )}
      </div>
      {err && <p className="error">{err}</p>}
    </form>
  );
}

export default function CardsPage({ reloadRefs }: { reloadRefs: () => void }) {
  const [statuses, setStatuses] = useState<store.CardStatus[]>([]);
  const [loading, setLoading] = useState(true);
  const [editing, setEditing] = useState<FormState>({ ...EMPTY });
  // 선택은 id 로만 들고 있다가 목록에서 찾는다 — 카드 정보를 고쳐도 화면이 같이 갱신되도록
  const [selectedId, setSelectedId] = useState(0);
  const [breakdown, setBreakdown] = useState<store.CardBreakdown | null>(null);
  // 카드 타일에 대표 혜택을 미리 보여 주려고 전체를 한 번에 받아 둔다
  const [benefits, setBenefits] = useState<Record<number, store.CardBenefit[]>>({});

  const load = useCallback(async () => {
    try {
      const [sts, bns] = await Promise.all([GetCardStatuses(), GetAllCardBenefits()]);
      setStatuses(sts);
      setBenefits(bns ?? {});
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    load();
  }, [load]);

  const selected = statuses.find((st) => st.card.id === selectedId) ?? null;

  const select = async (st: store.CardStatus) => {
    setSelectedId(st.card.id);
    setBreakdown(await GetCardBreakdown(st.card.id, st.periodStart, st.periodEnd));
  };

  const edit = (st: store.CardStatus) =>
    setEditing({
      id: st.card.id,
      issuer: st.card.issuer,
      name: st.card.name,
      billingDay: st.card.billingDay ? String(st.card.billingDay) : "",
      cycleStartDay: String(st.card.cycleStartDay || 1),
      perfTarget: st.card.perfTarget ? formatAmount(String(st.card.perfTarget)) : "",
      color: st.card.color || "",
      cashbackPct: bpToPct(st.card.cashbackBp),
      cashbackBonusPct: bpToPct(st.card.cashbackBonusBp),
      cashbackBonusDays: st.card.cashbackBonusDays ? String(st.card.cashbackBonusDays) : "",
      annualFee: st.card.annualFee ? formatAmount(String(st.card.annualFee)) : "",
      annualFeeGlobal: st.card.annualFeeGlobal ? formatAmount(String(st.card.annualFeeGlobal)) : "",
      benefitUrl: st.card.benefitUrl || "",
      benefitNote: st.card.benefitNote || "",
    });

  const remove = async (st: store.CardStatus) => {
    if (!window.confirm(`"${st.card.name}" 카드를 삭제할까요? 거래 기록은 남습니다.`)) return;
    await DeletePaymentMethod(st.card.id);
    if (selectedId === st.card.id) {
      setSelectedId(0);
      setBreakdown(null);
    }
    load();
    reloadRefs();
  };

  return (
    <div>
      <CardForm
        initial={editing}
        onSaved={() => {
          setEditing({ ...EMPTY });
          load();
          reloadRefs();
        }}
        onCancel={() => setEditing({ ...EMPTY })}
      />

      {loading && statuses.length === 0 && <CardsSkeleton />}
      <div className="card-grid">
        {statuses.map((st) => {
          const pct =
            st.card.perfTarget > 0
              ? Math.min(100, Math.round((st.spent / st.card.perfTarget) * 100))
              : 0;
          const color = st.card.color || autoColor(st.card.name);
          return (
            <div
              key={st.card.id}
              className={`card credit-card ${selectedId === st.card.id ? "selected" : ""}`}
              onClick={() => select(st)}
            >
              <div className="cc-head">
                <strong>
                  <span className="cc-dot" style={{ background: color }} />
                  {st.card.issuer ? `${st.card.issuer} ` : ""}{st.card.name}
                </strong>
                <span>
                  <button className="ghost" onClick={(e) => { e.stopPropagation(); edit(st); }}>수정</button>
                  <button className="ghost" onClick={(e) => { e.stopPropagation(); remove(st); }}>✕</button>
                </span>
              </div>
              <p className="muted small">
                {st.card.billingDay ? `결제일 매월 ${st.card.billingDay}일 · ` : ""}
                실적기간 {st.periodStart} ~ {st.periodEnd}
              </p>
              <p>
                기간 내 사용 <strong>{won(st.spent)}</strong>
                {st.card.perfTarget > 0 && <> / 한도 {won(st.card.perfTarget)}</>}
              </p>
              {st.excluded > 0 && (
                <p className="muted small">
                  실적 제외 {won(st.excluded)} (세금·공과금 등, 실적 합계에서 뺀 금액)
                </p>
              )}
              {st.card.perfTarget > 0 && (
                <>
                  <div className="bar-track">
                    <div className="bar-fill" style={{ width: `${pct}%`, background: color }} />
                  </div>
                  {st.achieved ? (
                    <span className="badge achieved">실적 달성 ✓</span>
                  ) : (
                    <span className="badge shortfall">{won(st.remaining)} 남음 ({pct}%)</span>
                  )}
                </>
              )}
              {/* 대표 혜택 두 줄만 미리보기 — 전체는 카드를 눌러서 본다 */}
              {(benefits[st.card.id]?.length ?? 0) > 0 && (
                <div className="cc-benefits">
                  {benefits[st.card.id].slice(0, 2).map((b) => (
                    <span key={b.id} className="badge benefit" title={b.note}>
                      {b.area}
                      {b.rateBp > 0 && (
                        <>
                          {" "}
                          {b.isMax ? "최대 " : ""}
                          {bpToPctLabel(b.rateBp)}
                        </>
                      )}
                    </span>
                  ))}
                  {benefits[st.card.id].length > 2 && (
                    <span className="muted small">외 {benefits[st.card.id].length - 2}건</span>
                  )}
                </div>
              )}
            </div>
          );
        })}
        {!loading && statuses.length === 0 && (
          <p className="muted">등록된 카드가 없습니다. 위에서 카드를 등록하세요.</p>
        )}
      </div>

      {selected && (
        <div>
          <h3 className="section-title">
            {selected.card.issuer} {selected.card.name} — 혜택과 실적
          </h3>
          <div className="card">
            <CardBenefits card={selected.card} onChanged={load} />
          </div>
        </div>
      )}

      {selected && breakdown && (
        <div>
          <h3 className="section-title">
            실적기간({selected.periodStart} ~ {selected.periodEnd}) 지출 분석
          </h3>
          <div className="dash-grid">
            <Bars title="어디에서 썼는가 (가맹점별)" rows={breakdown.byMerchant} />
            <Bars title="무엇에 썼는가 (카테고리별)" rows={breakdown.byCategory} />
          </div>
        </div>
      )}
    </div>
  );
}
