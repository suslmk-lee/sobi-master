import { FormEvent, useCallback, useEffect, useState } from "react";
import {
  DeletePaymentMethod,
  GetAllCardBenefits,
  GetCardBreakdown,
  GetCardStatuses,
  SaveCard,
} from "../../wailsjs/go/main/App";
import { Check, CreditCard, Pencil, Trash2 } from "lucide-react";
import IconTile from "../components/ui/IconTile";
import { store } from "../../wailsjs/go/models";
import RingGauge from "../components/ui/RingGauge";
import { formatAmount, parseAmount, won } from "../lib";
import Bars from "../Bars";
import CardBenefits, { bpToPct as bpToPctLabel } from "../CardBenefits";
import PmChip, { autoColor, CHIP_PALETTE } from "../PmChip";
import { CardsSkeleton } from "../Skeleton";
import { useConfirm } from "../ConfirmModal";

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
  statementStartDay: "",
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
          statementStartDay: f.statementStartDay ? Number(f.statementStartDay) : 0,
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
    <form className="surface space-y-4 p-5" onSubmit={submit}>
      <h3 className="section-title">{f.id ? "카드 수정" : "카드 등록"}</h3>
      <div className="grid gap-3 sm:grid-cols-2">
        <select
          className="input-field"
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
          className="input-field"
          placeholder="카드 이름 (예: 딥드림 체크)"
          value={f.name}
          onChange={(e) => setF({ ...f, name: e.target.value })}
        />
      </div>
      <div className="grid items-end gap-3 sm:grid-cols-2 lg:grid-cols-4">
        <label className="block">
          <span className="field-label">결제일 (매월)</span>
          <input
            type="number"
            className="input-field num"
            min={1}
            max={31}
            placeholder="예: 25"
            value={f.billingDay}
            onChange={(e) => setF({ ...f, billingDay: e.target.value })}
          />
        </label>
        <label className="block">
          <span className="field-label">실적 시작일 (1이면 1일~말일)</span>
          <input
            type="number"
            className="input-field num"
            min={1}
            max={31}
            value={f.cycleStartDay}
            onChange={(e) => setF({ ...f, cycleStartDay: e.target.value })}
          />
        </label>
        <label className="block">
          <span className="field-label">실적한도 (원)</span>
          <input
            type="text"
            className="input-field num"
            inputMode="numeric"
            placeholder="예: 300,000"
            value={f.perfTarget}
            onChange={(e) => setF({ ...f, perfTarget: formatAmount(e.target.value) })}
          />
        </label>
        <div className="flex gap-2">
          <button type="submit" className="btn-primary flex-1">{f.id ? "저장" : "등록"}</button>
          {f.id !== 0 && onCancel && (
            <button type="button" className="btn-ghost" onClick={onCancel}>취소</button>
          )}
        </div>
      </div>
      <div className="grid items-start gap-3 sm:grid-cols-2 lg:grid-cols-4">
        <label className="block">
          <span className="field-label">청구(이용)기간 시작일</span>
          <input
            type="number"
            className="input-field num"
            min={0}
            max={31}
            placeholder="자동"
            value={f.statementStartDay}
            onChange={(e) => setF({ ...f, statementStartDay: e.target.value })}
          />
        </label>
        <p className="text-xs text-zinc-500 sm:col-span-2 lg:col-span-3 sm:pt-6">
          비우면 실적 시작일, 그것도 1일이면 결제일 기준으로 자동 (1일 결제 → 전전월 19일~전월 18일). 카드사 명세서의 이용기간과 맞춰 주세요.
        </p>
      </div>
      {/* 캐시백: 결제액의 기본 %, 그리고 기한 내 카드대금 납부 시 얹어 주는 추가 % */}
      <div className="grid items-end gap-3 sm:grid-cols-2 lg:grid-cols-4">
        <label className="block">
          <span className="field-label">캐시백 기본 (%)</span>
          <input
            type="text"
            className="input-field num"
            inputMode="decimal"
            placeholder="예: 1"
            value={f.cashbackPct}
            onChange={(e) => setF({ ...f, cashbackPct: e.target.value })}
          />
        </label>
        <label className="block">
          <span className="field-label">기한 내 납부 시 추가 (%)</span>
          <input
            type="text"
            className="input-field num"
            inputMode="decimal"
            placeholder="예: 1"
            value={f.cashbackBonusPct}
            onChange={(e) => setF({ ...f, cashbackBonusPct: e.target.value })}
          />
        </label>
        <label className="block">
          <span className="field-label">추가 적립 기한 (일)</span>
          <input
            type="number"
            className="input-field num"
            min={0}
            max={60}
            placeholder="예: 5"
            value={f.cashbackBonusDays}
            onChange={(e) => setF({ ...f, cashbackBonusDays: e.target.value })}
          />
        </label>
        <span className="label-mono whitespace-normal">
          결제일로부터 기한 안에 카드대금을 갚으면 추가분을 받습니다. 캐시백 탭에서 집계·독촉을 봅니다.
        </span>
      </div>
      {/* 혜택 자료: 카드사 안내를 그대로 적어 두는 칸. 앱 계산에는 쓰지 않는다 */}
      <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
        <label className="block">
          <span className="field-label">연회비 국내 (원)</span>
          <input
            type="text"
            className="input-field num"
            inputMode="numeric"
            placeholder="예: 70,000"
            value={f.annualFee}
            onChange={(e) => setF({ ...f, annualFee: formatAmount(e.target.value) })}
          />
        </label>
        <label className="block">
          <span className="field-label">연회비 해외 (원)</span>
          <input
            type="text"
            className="input-field num"
            inputMode="numeric"
            placeholder="예: 70,000"
            value={f.annualFeeGlobal}
            onChange={(e) => setF({ ...f, annualFeeGlobal: formatAmount(e.target.value) })}
          />
        </label>
        <label className="block sm:col-span-2">
          <span className="field-label">혜택 자료 출처 (URL)</span>
          <input
            type="text"
            className="input-field"
            placeholder="카드사 안내 페이지 주소"
            value={f.benefitUrl}
            onChange={(e) => setF({ ...f, benefitUrl: e.target.value })}
          />
        </label>
      </div>
      <label className="block">
        <span className="field-label">혜택 유의사항</span>
        <input
          type="text"
          className="input-field"
          placeholder="예: 전월 실적 50만원 이상 시 적용, 실적 산정 제외 업종 있음"
          value={f.benefitNote}
          onChange={(e) => setF({ ...f, benefitNote: e.target.value })}
        />
      </label>
      <div className="flex flex-wrap items-end gap-6">
        <div>
          <span className="field-label">칩 색상 (목록에서 이 카드를 표시할 색)</span>
          <div className="flex flex-wrap items-center gap-1.5">
            <button
              type="button"
              className={`pill-item !px-3 !py-1 !text-xs ${f.color === "" ? "pill-item-active" : ""}`}
              title="자동 (이름 기반)"
              onClick={() => setF({ ...f, color: "" })}
            >
              자동
            </button>
            {CHIP_PALETTE.map((c) => (
              <button
                key={c}
                type="button"
                aria-label={c}
                className={`h-6 w-6 rounded-full border-2 transition-transform ${
                  f.color === c ? "scale-110 border-white" : "border-transparent"
                }`}
                style={{ background: c }}
                onClick={() => setF({ ...f, color: c })}
              />
            ))}
          </div>
        </div>
        {f.name && (
          <div>
            <span className="field-label">미리보기</span>
            <PmChip name={f.name} type="card" color={f.color} />
          </div>
        )}
      </div>
      {err && <p className="text-sm text-rose-300">{err}</p>}
    </form>
  );
}

export default function CardsPage({ reloadRefs }: { reloadRefs: () => void }) {
  const [statuses, setStatuses] = useState<store.CardStatus[]>([]);
  const [loading, setLoading] = useState(true);
  const [editing, setEditing] = useState<FormState>({ ...EMPTY });
  // 선택은 id 로만 들고 있다가 목록에서 찾는다 — 카드 정보를 고쳐도 화면이 같이 갱신되도록
  const [selectedId, setSelectedId] = useState(0);
  const [confirm, confirmUI] = useConfirm();
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
      statementStartDay: st.card.statementStartDay ? String(st.card.statementStartDay) : "",
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
    const ok = await confirm({
      title: `"${st.card.name}" 카드를 삭제할까요?`,
      danger: true,
      confirmLabel: "삭제",
      message: "이 카드로 기록된 거래는 그대로 남고 결제수단만 비워집니다. 카드 혜택 자료도 함께 삭제됩니다.",
    });
    if (!ok) return;
    await DeletePaymentMethod(st.card.id);
    if (selectedId === st.card.id) {
      setSelectedId(0);
      setBreakdown(null);
    }
    load();
    reloadRefs();
  };

  return (
    <div className="space-y-6">
      {confirmUI}
      <div className="flex min-w-0 items-center gap-3">
        <IconTile icon={CreditCard} tone="mint" />
        <div className="min-w-0">
          <h2 className="text-2xl font-light tracking-tight text-white">카드</h2>
          <p className="label-mono num">{loading && statuses.length === 0 ? "불러오는 중…" : `${statuses.length} cards`}</p>
        </div>
      </div>
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
      <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-3">
        {statuses.map((st) => {
          const pct =
            st.card.perfTarget > 0
              ? Math.min(100, Math.round((st.spent / st.card.perfTarget) * 100))
              : 0;
          const color = st.card.color || autoColor(st.card.name);
          const ringColor = st.achieved
            ? "rgb(var(--c-emerald-400))"
            : pct >= 70
              ? "rgb(var(--c-amber-400))"
              : "rgb(var(--c-mint-400))";
          const cardBenefits = benefits[st.card.id] ?? [];
          return (
            <div
              key={st.card.id}
              className={`surface surface-interactive p-5 ${selectedId === st.card.id ? "!border-mint-400/40" : ""}`}
              onClick={() => select(st)}
            >
              <div className="flex items-start justify-between gap-2">
                <div className="flex min-w-0 items-center gap-2">
                  {/* 사용자가 고른 카드 색은 작은 점으로만 남긴다 */}
                  <span className="h-2.5 w-2.5 shrink-0 rounded-full" style={{ background: color }} />
                  <strong className="min-w-0 truncate text-sm font-semibold text-white">
                    {st.card.issuer ? `${st.card.issuer} ` : ""}{st.card.name}
                  </strong>
                </div>
                <div className="flex shrink-0 gap-1">
                  <button
                    className="btn-icon"
                    title="수정"
                    aria-label="수정"
                    onClick={(e) => { e.stopPropagation(); edit(st); }}
                  >
                    <Pencil className="h-4 w-4" />
                  </button>
                  <button
                    className="btn-icon hover:!text-rose-300"
                    title="삭제"
                    aria-label="삭제"
                    onClick={(e) => { e.stopPropagation(); remove(st); }}
                  >
                    <Trash2 className="h-4 w-4" />
                  </button>
                </div>
              </div>

              <div className="mt-3 flex flex-wrap gap-1.5">
                {st.card.billingDay ? <span className="chip">결제일 매월 {st.card.billingDay}일</span> : null}
                {st.card.statementStartDay ? <span className="chip">청구 {st.card.statementStartDay}일~</span> : null}
                <span className="chip">실적기간 {st.periodStart} ~ {st.periodEnd}</span>
                {st.card.annualFee > 0 && (
                  <span className="chip">연회비 <span className="num">{won(st.card.annualFee)}</span></span>
                )}
              </div>

              <div className="mt-4 flex items-center gap-4">
                {st.card.perfTarget > 0 && (
                  <RingGauge value={pct} size={72} stroke={4} color={ringColor}>
                    <span className="text-sm">{pct}%</span>
                  </RingGauge>
                )}
                <div className="min-w-0 flex-1 space-y-1.5">
                  <div className="label-mono">기간 내 사용</div>
                  <div className="num text-lg font-semibold text-white">{won(st.spent)}</div>
                  {st.card.perfTarget > 0 && (
                    <div className="label-mono">한도 <span className="num">{won(st.card.perfTarget)}</span></div>
                  )}
                  {st.card.perfTarget > 0 &&
                    (st.achieved ? (
                      <span className="badge bg-emerald-500/15 text-emerald-300">
                        <Check className="h-3 w-3" strokeWidth={2.5} />
                        실적 달성
                      </span>
                    ) : (
                      <span className="badge bg-amber-500/15 text-amber-300">
                        <span className="num">{won(st.remaining)}</span> 남음
                      </span>
                    ))}
                </div>
              </div>

              {st.excluded > 0 && (
                <p className="label-mono mt-3 whitespace-normal">
                  실적 제외 <span className="num">{won(st.excluded)}</span> (세금·공과금 등, 실적 합계에서 뺀 금액)
                </p>
              )}

              {/* 대표 혜택 두 줄만 미리보기 — 전체는 카드를 눌러서 본다 */}
              {cardBenefits.length > 0 && (
                <div className="mt-3 flex flex-wrap items-center gap-1.5">
                  {cardBenefits.slice(0, 2).map((b) => (
                    <span key={b.id} className="chip" title={b.note}>
                      {b.area}
                      {b.rateBp > 0 && (
                        <span className="num text-mint-300">
                          {b.isMax ? "최대 " : ""}
                          {bpToPctLabel(b.rateBp)}
                        </span>
                      )}
                    </span>
                  ))}
                  {cardBenefits.length > 2 && (
                    <span className="label-mono">외 {cardBenefits.length - 2}건</span>
                  )}
                </div>
              )}
            </div>
          );
        })}
        {!loading && statuses.length === 0 && (
          <p className="label-mono">등록된 카드가 없습니다. 위에서 카드를 등록하세요.</p>
        )}
      </div>

      {selected && (
        <div className="space-y-3">
          <h3 className="section-title">
            {selected.card.issuer} {selected.card.name} — 혜택과 실적
          </h3>
          <div className="surface p-5">
            <CardBenefits card={selected.card} onChanged={load} />
          </div>
        </div>
      )}

      {selected && breakdown && (
        <div className="space-y-3">
          <h3 className="section-title">
            실적기간({selected.periodStart} ~ {selected.periodEnd}) 지출 분석
          </h3>
          <div className="grid gap-4 lg:grid-cols-2">
            <Bars title="어디에서 썼는가 (가맹점별)" rows={breakdown.byMerchant} />
            <Bars title="무엇에 썼는가 (카테고리별)" rows={breakdown.byCategory} />
          </div>
        </div>
      )}
    </div>
  );
}

