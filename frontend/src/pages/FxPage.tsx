import { FormEvent, ReactNode, useCallback, useEffect, useMemo, useState } from "react";
import {
  DeleteFxPlan,
  DeleteFxTrade,
  GetFxOverview,
  RefreshFxRates,
  SaveFxPlan,
  SaveFxTrade,
} from "../../wailsjs/go/main/App";
import { store } from "../../wailsjs/go/models";
import {
  AlertCircle,
  ArrowDownLeft,
  ArrowUpRight,
  CalendarClock,
  Globe,
  Pencil,
  Plus,
  Power,
  RefreshCw,
  Target,
  Trash2,
} from "lucide-react";
import IconTile from "../components/ui/IconTile";
import Modal, { Field, Segmented } from "../components/ui/Modal";
import { Sk } from "../Skeleton";
import { useConfirm } from "../ConfirmModal";
import { formatAmount, parseAmount, today, won } from "../lib";

// ---------- 외화 모으기 ----------
// 토스뱅크 외화통장을 장부로 따라간다. 앱이 실제로 사고팔지는 않는다 — 직접 적은 사기·팔기와
// 매일 모으기·목표 환율 매수 계획을 앱이 그날 환율로 자동 기록한 매수(추정)로 보유·손익을 낸다.
// 외화 금액은 통화의 최소 단위 정수(센트·엔), 환율은 외화 1단위당 원화(화면은 엔·동·루피아를 100단위로).

type Cur = store.FxCurrency;

const pow10 = (n: number) => 10 ** n;
const major = (c: Cur, minor: number) => minor / pow10(c.decimals);

// 1234.5 USD → "1,234.50 USD"
function fxText(c: Cur | undefined, minor: number, withCode = true): string {
  if (!c) return String(minor);
  const s = major(c, minor).toLocaleString("ko-KR", {
    minimumFractionDigits: c.decimals,
    maximumFractionDigits: c.decimals,
  });
  return withCode ? `${s} ${c.code}` : s;
}

// 환율(1단위당 원화) → 화면 단위("100 JPY")의 원화 문자열
const rateNum = (c: Cur, rate: number) =>
  (rate * c.unit).toLocaleString("ko-KR", { minimumFractionDigits: 2, maximumFractionDigits: 2 });
const rateText = (c: Cur, rate: number) => (rate > 0 ? `${rateNum(c, rate)}원` : "-");
const unitLabel = (c: Cur) => `${c.unit === 1 ? "1" : c.unit} ${c.code}`;

const signedWon = (n: number) => (n > 0 ? "+" : n < 0 ? "−" : "") + won(Math.abs(n));
const pct = (n: number, base: number) => (base > 0 ? `${n >= 0 ? "+" : "−"}${Math.abs((n / base) * 100).toFixed(1)}%` : "");
const tone = (n: number) => (n > 0 ? "text-emerald-300" : n < 0 ? "text-rose-300" : "text-zinc-300");
const shortDate = (d: string) => (d ? `${Number(d.slice(5, 7))}/${Number(d.slice(8, 10))}` : "");

// 외화 금액 입력: 숫자와 소수점(통화 소수 자리까지)만, 천 단위 쉼표
function formatFxInput(c: Cur | undefined, s: string): string {
  const dec = c?.decimals ?? 2;
  let [int, frac] = s.replace(/[^\d.]/g, "").split(".");
  int = int ? Number(int).toLocaleString("ko-KR") : frac !== undefined ? "0" : "";
  if (dec === 0 || frac === undefined) return int;
  return `${int}.${frac.slice(0, dec)}`;
}
const parseFxInput = (c: Cur | undefined, s: string) => {
  const v = Number(s.replace(/,/g, ""));
  return Number.isFinite(v) && c ? Math.round(v * pow10(c.decimals)) : 0;
};
const fxInputOf = (c: Cur, minor: number) => formatFxInput(c, String(major(c, minor)));

// 환율 입력(화면 단위, 소수 둘째 자리까지)
const formatRateInput = (s: string) => {
  let [int, frac] = s.replace(/[^\d.]/g, "").split(".");
  int = int ? Number(int).toLocaleString("ko-KR") : frac !== undefined ? "0" : "";
  return frac === undefined ? int : `${int}.${frac.slice(0, 2)}`;
};
const parseRateInput = (c: Cur | undefined, s: string) => {
  const v = Number(s.replace(/,/g, ""));
  return Number.isFinite(v) && c ? v / c.unit : 0;
};
const rateInputOf = (c: Cur, rate: number) => (rate > 0 ? formatRateInput((rate * c.unit).toFixed(2)) : "");

const SOURCE_LABEL: Record<string, string> = {
  manual: "직접",
  opening: "처음 잔고",
  daily: "매일 모으기",
  target: "목표 환율",
};

function CurrencySelect({ value, onChange, list }: { value: string; onChange: (v: string) => void; list: Cur[] }) {
  return (
    <select className="input-field" value={value} onChange={(e) => onChange(e.target.value)}>
      {list.map((c) => (
        <option key={c.code} value={c.code}>
          {c.name} ({c.code})
        </option>
      ))}
    </select>
  );
}

// ---------- 사기·팔기·처음 잔고 ----------
type TradeMode = "buy" | "sell" | "opening";

function TradeModal({
  ov,
  trade,
  initial,
  onClose,
  onSaved,
}: {
  ov: store.FxOverview;
  trade: store.FxTrade | null;
  initial: { currency: string; mode: TradeMode };
  onClose: () => void;
  onSaved: () => void;
}) {
  const curs = ov.currencies;
  const rateOf = (code: string) => ov.rates.find((r) => r.currency === code)?.rate ?? 0;
  const [mode, setMode] = useState<TradeMode>(
    trade ? (trade.source === "opening" ? "opening" : (trade.side as TradeMode)) : initial.mode
  );
  const [code, setCode] = useState(trade?.currency ?? initial.currency);
  const c = curs.find((x) => x.code === code);
  const [date, setDate] = useState(trade?.date ?? today());
  const [amount, setAmount] = useState(trade && c ? fxInputOf(c, trade.amount) : "");
  const [rate, setRate] = useState(() => (c ? rateInputOf(c, trade?.rate ?? rateOf(code)) : ""));
  const [krw, setKrw] = useState(trade ? formatAmount(String(trade.krw)) : "");
  const [krwTyped, setKrwTyped] = useState(!!trade);
  const [memo, setMemo] = useState(trade?.memo ?? "");
  const [err, setErr] = useState("");
  const [busy, setBusy] = useState(false);

  const minor = parseFxInput(c, amount);
  const r = parseRateInput(c, rate);
  const computed = c && minor > 0 && r > 0 ? Math.round(major(c, minor) * r) : 0;
  const krwVal = krwTyped ? parseAmount(krw) : computed;
  const held = ov.positions.find((p) => p.currency === code)?.amount ?? 0;

  const changeCode = (v: string) => {
    setCode(v);
    const nc = curs.find((x) => x.code === v);
    if (nc) {
      setRate(rateInputOf(nc, rateOf(v)));
      setAmount((a) => formatFxInput(nc, a));
    }
  };

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    setErr("");
    if (!c || minor <= 0) return setErr("외화 금액을 입력하세요.");
    if (r <= 0 && !(krwTyped && parseAmount(krw) > 0)) return setErr("환율이나 원화 금액을 입력하세요.");
    setBusy(true);
    try {
      await SaveFxTrade(
        new store.FxTrade({
          ...(trade ?? {}),
          id: trade?.id ?? 0,
          date,
          currency: code,
          side: mode === "sell" ? "sell" : "buy",
          amount: minor,
          // 원화를 직접 넣었으면 환율은 서버가 원화 ÷ 외화로 계산한다
          krw: krwTyped ? parseAmount(krw) : 0,
          rate: krwTyped ? 0 : r,
          source: trade?.source ?? (mode === "opening" ? "opening" : "manual"),
          planId: trade?.planId,
          estimated: trade?.estimated ?? false,
          memo,
        })
      );
      onSaved();
      onClose();
    } catch (e: any) {
      setErr(String(e));
    } finally {
      setBusy(false);
    }
  };

  const title = trade ? "외화 기록 수정" : mode === "opening" ? "가진 외화 넣기" : mode === "sell" ? "외화 팔기" : "외화 사기";
  return (
    <Modal title={title} icon={mode === "sell" ? ArrowUpRight : ArrowDownLeft} onClose={onClose} onSubmit={submit}>
      {!trade && (
        <div className="mb-4">
          <Segmented
            value={mode}
            onChange={setMode}
            options={[
              { v: "buy", label: "사기" },
              { v: "sell", label: "팔기" },
              { v: "opening", label: "처음 잔고" },
            ]}
          />
          {mode === "opening" && (
            <p className="label-mono mt-2 whitespace-normal">
              앱을 쓰기 전부터 토스뱅크에 있던 외화입니다. 보유 금액과 평균 환율을 넣으세요. 통장에서 빠진 돈으로 보지 않아 자금 계획에는 영향이 없습니다.
            </p>
          )}
        </div>
      )}
      {trade?.estimated && (
        <p className="mb-3 text-xs leading-relaxed text-amber-300">
          앱이 그날 환율로 추정해 기록한 매수입니다. 토스뱅크 거래 내역의 실제 체결 금액으로 고치면 추정 표시가 사라집니다.
        </p>
      )}
      <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
        <Field label="통화">
          <CurrencySelect value={code} onChange={changeCode} list={curs} />
        </Field>
        <Field label={mode === "opening" ? "기준일" : "날짜"}>
          <input type="date" className="input-field" value={date} onChange={(e) => setDate(e.target.value)} />
        </Field>
        <Field label={mode === "opening" ? "가진 외화" : mode === "sell" ? "판 외화" : "산 외화"}>
          <div className="flex items-center gap-2">
            <input
              className="input-field num"
              inputMode="decimal"
              autoFocus
              placeholder="0"
              value={amount}
              onChange={(e) => setAmount(formatFxInput(c, e.target.value))}
            />
            <span className="text-sm text-zinc-400">{code}</span>
          </div>
          {mode === "sell" && c && (
            <button type="button" className="label-mono mt-1 hover:text-zinc-200" onClick={() => setAmount(fxInputOf(c, held))}>
              보유 {fxText(c, held)} · 전부
            </button>
          )}
        </Field>
        <Field label={`${mode === "opening" ? "평균 환율" : "환율"} (${c ? unitLabel(c) : ""}당)`}>
          <div className="flex items-center gap-2">
            <input
              className="input-field num"
              inputMode="decimal"
              placeholder="0"
              value={rate}
              onChange={(e) => {
                setRate(formatRateInput(e.target.value));
                setKrwTyped(false);
              }}
            />
            <span className="text-sm text-zinc-400">원</span>
          </div>
        </Field>
        <Field label={mode === "sell" ? "받은 원화" : "든 원화"} className="sm:col-span-2">
          <input
            className="input-field num"
            inputMode="numeric"
            placeholder={computed ? computed.toLocaleString("ko-KR") : "0"}
            value={krwTyped ? krw : computed ? computed.toLocaleString("ko-KR") : ""}
            onChange={(e) => {
              setKrw(formatAmount(e.target.value));
              setKrwTyped(e.target.value !== "");
            }}
          />
          <span className="label-mono mt-1 block whitespace-normal">
            {krwTyped && c && minor > 0 && krwVal > 0
              ? `넣은 원화 기준 환율 ${rateText(c, krwVal / major(c, minor))}`
              : "외화 × 환율로 계산합니다. 토스뱅크에 찍힌 원화가 다르면 직접 고치세요"}
          </span>
        </Field>
        <Field label="메모" className="sm:col-span-2">
          <input className="input-field" value={memo} onChange={(e) => setMemo(e.target.value)} />
        </Field>
      </div>
      {err && <p className="mt-3 text-sm text-rose-300">{err}</p>}
      <div className="mt-4 flex justify-end gap-2">
        <button type="button" className="btn-ghost" onClick={onClose}>취소</button>
        <button type="submit" className="btn-primary" disabled={busy}>{trade ? "저장" : "기록"}</button>
      </div>
    </Modal>
  );
}

// ---------- 모으기 계획 ----------
function PlanModal({
  ov,
  plan,
  onClose,
  onSaved,
}: {
  ov: store.FxOverview;
  plan: store.FxPlan | null;
  onClose: () => void;
  onSaved: () => void;
}) {
  const curs = ov.currencies;
  const rateOf = (code: string) => ov.rates.find((r) => r.currency === code)?.rate ?? 0;
  const [kind, setKind] = useState<"daily" | "target">((plan?.kind as "daily" | "target") ?? "daily");
  const [code, setCode] = useState(plan?.currency ?? "USD");
  const c = curs.find((x) => x.code === code);
  const [unit, setUnit] = useState<"krw" | "fx">((plan?.unit as "krw" | "fx") ?? "krw");
  const [amount, setAmount] = useState(() =>
    plan ? (plan.unit === "fx" && c ? fxInputOf(c, plan.amount) : formatAmount(String(plan.amount))) : ""
  );
  const [target, setTarget] = useState(() => (plan && c ? rateInputOf(c, plan.targetRate) : ""));
  const [repeating, setRepeating] = useState(plan?.repeating ?? false);
  const [weekdaysOnly, setWeekdaysOnly] = useState(plan?.weekdaysOnly ?? false);
  const [start, setStart] = useState(plan?.startDate ?? today());
  const [endOn, setEndOn] = useState(!!plan?.endDate);
  const [end, setEnd] = useState(plan?.endDate || today());
  const [memo, setMemo] = useState(plan?.memo ?? "");
  const [active, setActive] = useState(plan?.active ?? true);
  const [err, setErr] = useState("");
  const [busy, setBusy] = useState(false);

  const now = rateOf(code);
  const amt = unit === "fx" ? parseFxInput(c, amount) : parseAmount(amount);
  const approx =
    c && amt > 0 && now > 0
      ? unit === "fx"
        ? `지금 환율로 약 ${won(Math.round(major(c, amt) * now))}`
        : `지금 환율로 약 ${fxText(c, Math.floor((amt / now) * pow10(c.decimals)))}`
      : "";

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    setErr("");
    if (amt <= 0) return setErr("금액을 입력하세요.");
    const tr = parseRateInput(c, target);
    if (kind === "target" && tr <= 0) return setErr("목표 환율을 입력하세요.");
    setBusy(true);
    try {
      await SaveFxPlan(
        new store.FxPlan({
          ...(plan ?? {}),
          id: plan?.id ?? 0,
          kind,
          currency: code,
          unit,
          amount: amt,
          targetRate: kind === "target" ? tr : 0,
          repeating: kind === "target" && repeating,
          weekdaysOnly: kind === "daily" && weekdaysOnly,
          startDate: start,
          endDate: endOn ? end : "",
          active,
          memo,
        })
      );
      onSaved();
      onClose();
    } catch (e: any) {
      setErr(String(e));
    } finally {
      setBusy(false);
    }
  };

  return (
    <Modal
      title={plan ? "모으기 계획 수정" : "모으기 계획 추가"}
      icon={kind === "daily" ? CalendarClock : Target}
      onClose={onClose}
      onSubmit={submit}
    >
      {!plan && (
        <div className="mb-4">
          <Segmented
            value={kind}
            onChange={setKind}
            options={[
              { v: "daily", label: "매일 모으기" },
              { v: "target", label: "목표 환율 매수" },
            ]}
          />
        </div>
      )}
      <p className="mb-4 text-xs leading-relaxed text-zinc-500">
        {kind === "daily"
          ? "토스뱅크에 설정한 매일 모으기와 같게 맞추세요. 앱이 매일 그날 환율로 매수를 기록하고, 자금 계획에도 앞으로 빠질 원화로 넣습니다."
          : "그날 환율이 목표 이하이면 앱이 그 환율로 매수를 기록합니다. 하루 한 번 받는 환율로 판단하므로, 하루 중 잠깐 내려가 토스뱅크에서 사진 것은 [사기]로 직접 기록하세요."}
      </p>
      <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
        <Field label="통화">
          <CurrencySelect
            value={code}
            onChange={(v) => {
              setCode(v);
              const nc = curs.find((x) => x.code === v);
              if (nc && unit === "fx") setAmount((a) => formatFxInput(nc, a));
            }}
            list={curs}
          />
        </Field>
        <div>
          <span className="field-label">금액 기준</span>
          <Segmented
            value={unit}
            onChange={(v) => {
              setUnit(v);
              setAmount("");
            }}
            options={[
              { v: "krw", label: "원화로" },
              { v: "fx", label: `${code}로` },
            ]}
          />
        </div>
        <Field label={kind === "daily" ? "하루에" : "한 번에"} className="sm:col-span-2">
          <div className="flex items-center gap-2">
            <input
              className="input-field num"
              inputMode={unit === "fx" ? "decimal" : "numeric"}
              placeholder="0"
              value={amount}
              onChange={(e) => setAmount(unit === "fx" ? formatFxInput(c, e.target.value) : formatAmount(e.target.value))}
            />
            <span className="shrink-0 text-sm text-zinc-400">{unit === "fx" ? code : "원어치"}</span>
          </div>
          {approx && <span className="label-mono mt-1 block">{approx}</span>}
        </Field>
        {kind === "target" && (
          <Field label={`목표 환율 (${c ? unitLabel(c) : ""}당)`} className="sm:col-span-2">
            <div className="flex items-center gap-2">
              <input className="input-field num" inputMode="decimal" placeholder="0" value={target} onChange={(e) => setTarget(formatRateInput(e.target.value))} />
              <span className="shrink-0 text-sm text-zinc-400">원 이하이면 사기</span>
            </div>
            {c && now > 0 && <span className="label-mono mt-1 block">지금 {rateText(c, now)}</span>}
          </Field>
        )}
        <Field label="시작일">
          <input type="date" className="input-field" value={start} min={plan ? undefined : today()} onChange={(e) => setStart(e.target.value)} />
        </Field>
        <div>
          <span className="field-label">끝나는 날</span>
          <div className="flex items-center gap-2">
            <label className="flex shrink-0 items-center gap-2 text-sm text-zinc-300">
              <input type="checkbox" className="accent-mint-500" checked={endOn} onChange={(e) => setEndOn(e.target.checked)} />
              정함
            </label>
            {endOn ? (
              <input type="date" className="input-field" value={end} onChange={(e) => setEnd(e.target.value)} />
            ) : (
              <span className="text-xs text-zinc-500">끌 때까지</span>
            )}
          </div>
        </div>
        {kind === "daily" ? (
          <label className="flex items-center gap-2 text-sm text-zinc-300 sm:col-span-2">
            <input type="checkbox" className="accent-mint-500" checked={weekdaysOnly} onChange={(e) => setWeekdaysOnly(e.target.checked)} />
            평일에만 (주말은 건너뜀)
          </label>
        ) : (
          <label className="flex items-center gap-2 text-sm text-zinc-300 sm:col-span-2">
            <input type="checkbox" className="accent-mint-500" checked={repeating} onChange={(e) => setRepeating(e.target.checked)} />
            조건이 맞는 날마다 사기 (끄면 한 번 사고 끝)
          </label>
        )}
        <Field label="메모" className="sm:col-span-2">
          <input className="input-field" value={memo} onChange={(e) => setMemo(e.target.value)} />
        </Field>
        {plan && (
          <label className="flex items-center gap-2 text-sm text-zinc-300 sm:col-span-2">
            <input type="checkbox" checked={active} onChange={(e) => setActive(e.target.checked)} />
            켜 둠 (끄면 기록하지 않고 자금 계획에서도 빠집니다)
          </label>
        )}
      </div>
      {err && <p className="mt-3 text-sm text-rose-300">{err}</p>}
      <div className="mt-4 flex justify-end gap-2">
        <button type="button" className="btn-ghost" onClick={onClose}>취소</button>
        <button type="submit" className="btn-primary" disabled={busy}>{plan ? "저장" : "추가"}</button>
      </div>
    </Modal>
  );
}

// 계획 한 줄 설명
function planText(c: Cur, p: store.FxPlan, st: store.FxPlanStatus): { head: string; sub: string; state?: ReactNode } {
  const amt = p.unit === "fx" ? fxText(c, p.amount) : `${won(p.amount)}어치`;
  const period = `${shortDate(p.startDate)}부터${p.endDate ? ` ${shortDate(p.endDate)}까지` : ""}`;
  const done =
    st.count > 0 ? `지금까지 ${st.count}회 · ${won(st.spent)} → ${fxText(c, st.bought)}` : "아직 기록 없음";
  if (p.kind === "daily") {
    return {
      head: `${p.weekdaysOnly ? "평일마다" : "매일"} ${amt}`,
      sub: `${period} · ${done}`,
    };
  }
  const head = `${unitLabel(c)} ${rateNum(c, p.targetRate)}원 이하이면 ${amt}`;
  let state: ReactNode = null;
  if (p.doneAt) state = <span className="text-zinc-400">{shortDate(p.doneAt)}에 샀음 · 끝남</span>;
  else if (st.rate > 0)
    state = st.hit ? (
      <span className="text-mint-300">지금 {rateText(c, st.rate)} · 조건 맞음</span>
    ) : (
      <span className="text-zinc-400">
        지금 {rateText(c, st.rate)} · 목표까지 {((st.rate - p.targetRate) * c.unit).toLocaleString("ko-KR", { maximumFractionDigits: 2 })}원
      </span>
    );
  return { head, sub: `${p.repeating ? "조건 맞는 날마다" : "한 번만"} · ${period} · ${done}`, state };
}

// ---------- 페이지 ----------
type ModalState =
  | { t: "trade"; trade: store.FxTrade | null; currency: string; mode: TradeMode }
  | { t: "plan"; plan: store.FxPlan | null }
  | null;

const TRADES_PAGE = 30;

export default function FxPage() {
  const [ov, setOv] = useState<store.FxOverview | null>(null);
  const [loading, setLoading] = useState(true);
  const [refreshing, setRefreshing] = useState(false);
  const [err, setErr] = useState("");
  const [modal, setModal] = useState<ModalState>(null);
  const [showAll, setShowAll] = useState(false);
  const [confirm, confirmUI] = useConfirm();

  const load = useCallback(async () => {
    try {
      setOv(await GetFxOverview());
      setErr("");
    } catch (e: any) {
      setErr(String(e));
    } finally {
      setLoading(false);
    }
  }, []);
  useEffect(() => {
    load();
  }, [load]);

  const refresh = async () => {
    setRefreshing(true);
    try {
      setOv(await RefreshFxRates());
      setErr("");
    } catch (e: any) {
      setErr(String(e));
    } finally {
      setRefreshing(false);
    }
  };

  const curOf = useMemo(() => {
    const m = new Map<string, Cur>();
    (ov?.currencies ?? []).forEach((c) => m.set(c.code, c));
    return (code: string) => m.get(code);
  }, [ov]);
  const close = useCallback(() => setModal(null), []);
  const openTrade = (currency = "USD", mode: TradeMode = "buy", trade: store.FxTrade | null = null) =>
    setModal({ t: "trade", trade, currency, mode });

  const removeTrade = async (t: store.FxTrade) => {
    const c = curOf(t.currency);
    if (
      !(await confirm({
        title: "이 외화 기록을 지울까요?",
        danger: true,
        confirmLabel: "삭제",
        message: t.planId ? "자동 기록을 지워도 그날 몫은 다시 기록되지 않습니다." : undefined,
        detail: `${t.date} ${t.side === "sell" ? "팔기" : "사기"} ${fxText(c, t.amount)} · ${won(t.krw)}`,
      }))
    )
      return;
    try {
      await DeleteFxTrade(t.id);
      load();
    } catch (e: any) {
      setErr(String(e));
    }
  };
  const removePlan = async (p: store.FxPlan) => {
    if (
      !(await confirm({
        title: "이 모으기 계획을 지울까요?",
        danger: true,
        confirmLabel: "삭제",
        message: "이미 기록된 매수는 그대로 남습니다.",
      }))
    )
      return;
    try {
      await DeleteFxPlan(p.id);
      load();
    } catch (e: any) {
      setErr(String(e));
    }
  };
  const togglePlan = async (p: store.FxPlan) => {
    try {
      await SaveFxPlan(new store.FxPlan({ ...p, active: !p.active }));
      load();
    } catch (e: any) {
      setErr(String(e));
    }
  };

  const positions = (ov?.positions ?? []).filter((p) => p.amount > 0 || p.realized !== 0);
  const trades = ov?.trades ?? [];
  const shownTrades = showAll ? trades : trades.slice(0, TRADES_PAGE);
  const dailyKRW = (ov?.plans ?? [])
    .filter((s) => s.plan.active && s.plan.kind === "daily" && !s.plan.doneAt)
    .reduce((a, s) => a + s.dailyKrw, 0);

  return (
    <div className="space-y-5">
      {confirmUI}
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="flex min-w-0 items-center gap-3">
          <IconTile icon={Globe} tone="mint" />
          <div className="min-w-0">
            <h2 className="text-2xl font-light tracking-tight text-white">외화</h2>
            <p className="label-mono num">
              {loading ? "불러오는 중…" : ov?.rateDate ? `${ov.rateDate} 환율 · ${ov.rateSource}` : "환율을 아직 받지 못했습니다"}
            </p>
          </div>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <button className="btn-ghost" onClick={refresh} disabled={refreshing} title="환율을 지금 다시 받아 옵니다">
            <RefreshCw className={`h-4 w-4 ${refreshing ? "animate-spin" : ""}`} />
            환율 새로고침
          </button>
          <button className="btn-primary" onClick={() => openTrade()} disabled={!ov}>
            <Plus className="h-4 w-4" />
            사기·팔기
          </button>
        </div>
      </div>

      {err && (
        <div role="alert" className="flex items-start gap-3 rounded-xl border border-rose-400/25 bg-rose-500/10 p-3.5 text-sm text-rose-200">
          <AlertCircle className="mt-0.5 h-4 w-4 shrink-0" />
          <p className="min-w-0 flex-1">{err}</p>
        </div>
      )}
      {ov?.fetchError && (
        <p className="text-xs text-amber-300">
          환율을 새로 받지 못해 {ov.rateDate ? `${ov.rateDate} 값으로` : "있던 값으로"} 보여 줍니다 ({ov.fetchError})
        </p>
      )}

      {loading || !ov ? (
        <Sk h={92} r={16} />
      ) : (
        <div className="grid grid-cols-2 gap-3 lg:grid-cols-4">
          <div className="surface flex flex-col gap-1.5 p-4">
            <span className="label-mono">평가액</span>
            <strong className="num text-xl font-semibold text-white">{won(ov.value)}</strong>
            <span className="label-mono num">{positions.filter((p) => p.amount > 0).length}개 통화</span>
          </div>
          <div className="surface flex flex-col gap-1.5 p-4">
            <span className="label-mono">산 원화 (남은 원가)</span>
            <strong className="num text-xl font-semibold text-white">{won(ov.cost)}</strong>
            <span className="label-mono num">{dailyKRW > 0 ? `매일 약 ${won(dailyKRW)} 모으는 중` : "모으기 계획 없음"}</span>
          </div>
          <div className="surface flex flex-col gap-1.5 p-4">
            <span className="label-mono">평가손익</span>
            <strong className={`num text-xl font-semibold ${tone(ov.unrealized)}`}>{signedWon(ov.unrealized)}</strong>
            <span className={`label-mono num ${tone(ov.unrealized)}`}>{pct(ov.unrealized, ov.cost)}</span>
          </div>
          <div className="surface flex flex-col gap-1.5 p-4">
            <span className="label-mono">판 외화 손익</span>
            <strong className={`num text-xl font-semibold ${tone(ov.realized)}`}>{signedWon(ov.realized)}</strong>
            <span className="label-mono">평균 매입 환율 기준</span>
          </div>
        </div>
      )}

      {/* 보유 외화 */}
      {ov && (
        <div className="surface p-5">
          <div className="mb-4 flex items-center gap-3">
            <IconTile icon={Globe} tone="sky" size="sm" />
            <h3 className="section-title">보유 외화</h3>
          </div>
          {positions.length === 0 ? (
            <p className="text-sm leading-relaxed text-zinc-500">
              아직 기록한 외화가 없습니다. 토스뱅크 외화통장에 이미 외화가 있으면 [사기·팔기] → '처음 잔고'로 보유 금액과 평균 환율을 넣으세요.
            </p>
          ) : (
            <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-3">
              {positions.map((p) => {
                const c = curOf(p.currency)!;
                return (
                  <div key={p.currency} className={`surface-inset rounded-2xl p-4 ${p.amount > 0 ? "" : "opacity-60"}`}>
                    <div className="flex items-start justify-between gap-2">
                      <div className="min-w-0">
                        <strong className="block text-sm font-semibold text-white">{c.name}</strong>
                        <span className="label-mono">{c.code}</span>
                      </div>
                      <div className="flex shrink-0 gap-1">
                        <button className="btn-ghost !px-2.5 !py-1 text-xs" onClick={() => openTrade(p.currency, "buy")}>사기</button>
                        <button className="btn-ghost !px-2.5 !py-1 text-xs" onClick={() => openTrade(p.currency, "sell")} disabled={p.amount <= 0}>팔기</button>
                      </div>
                    </div>
                    <div className="num mt-3 text-2xl font-semibold text-white">{fxText(c, p.amount)}</div>
                    <p className="num mt-1 text-xs text-zinc-400">
                      평균 {rateText(c, p.avgRate)} → 지금 {rateText(c, p.rate)}
                      <span className="text-zinc-500"> ({unitLabel(c)})</span>
                    </p>
                    <dl className="mt-3 grid grid-cols-2 gap-x-3 gap-y-1 text-xs">
                      <div>
                        <dt className="label-mono">평가</dt>
                        <dd className="num text-zinc-200">{p.rate > 0 ? won(p.value) : "-"}</dd>
                      </div>
                      <div>
                        <dt className="label-mono">손익</dt>
                        <dd className={`num ${tone(p.unrealized)}`}>
                          {p.rate > 0 ? `${signedWon(p.unrealized)} ${pct(p.unrealized, p.cost)}` : "-"}
                        </dd>
                      </div>
                      {p.realized !== 0 && (
                        <div className="col-span-2">
                          <dt className="label-mono">판 외화 손익</dt>
                          <dd className={`num ${tone(p.realized)}`}>{signedWon(p.realized)}</dd>
                        </div>
                      )}
                    </dl>
                  </div>
                );
              })}
            </div>
          )}
        </div>
      )}

      {/* 모으기 계획 */}
      {ov && (
        <div className="surface p-5">
          <div className="mb-4 flex flex-wrap items-center justify-between gap-3">
            <div className="flex items-center gap-3">
              <IconTile icon={CalendarClock} tone="amber" size="sm" />
              <h3 className="section-title">모으기 계획</h3>
            </div>
            <button className="btn-ghost" onClick={() => setModal({ t: "plan", plan: null })}>
              <Plus className="h-4 w-4" />
              계획 추가
            </button>
          </div>
          {ov.plans.length === 0 ? (
            <p className="text-sm leading-relaxed text-zinc-500">
              토스뱅크에 설정한 매일 모으기나 목표 환율 매수를 똑같이 넣어 두면, 앱이 그날 환율로 매수를 기록합니다.
            </p>
          ) : (
            <ul className="space-y-2">
              {ov.plans.map((st) => {
                const p = st.plan;
                const c = curOf(p.currency)!;
                const t = planText(c, p, st);
                const off = !p.active || !!p.doneAt;
                return (
                  <li key={p.id} className={`surface-inset flex flex-wrap items-center gap-x-4 gap-y-2 p-3.5 ${off ? "opacity-60" : ""}`}>
                    <div className="flex w-40 shrink-0 items-center gap-2">
                      {p.kind === "daily" ? (
                        <CalendarClock className="h-4 w-4 text-amber-300" />
                      ) : (
                        <Target className="h-4 w-4 text-sky-300" />
                      )}
                      <span className="text-sm text-zinc-200">{p.kind === "daily" ? "매일 모으기" : "목표 환율"}</span>
                      <span className="chip">{p.currency}</span>
                    </div>
                    <div className="min-w-[12rem] flex-1">
                      <p className="num text-sm text-white">{t.head}</p>
                      <p className="num mt-0.5 text-xs text-zinc-500">
                        {t.sub}
                        {!p.active && " · 꺼 둠"}
                        {p.memo && ` · ${p.memo}`}
                      </p>
                      {t.state && <p className="num mt-0.5 text-xs">{t.state}</p>}
                    </div>
                    <div className="flex items-center gap-1">
                      <button className="btn-icon" title="수정" aria-label="수정" onClick={() => setModal({ t: "plan", plan: p })}>
                        <Pencil className="h-4 w-4" />
                      </button>
                      <button className="btn-icon" title={p.active ? "끄기" : "켜기"} aria-label={p.active ? "끄기" : "켜기"} onClick={() => togglePlan(p)}>
                        <Power className="h-4 w-4" />
                      </button>
                      <button className="btn-icon hover:!text-rose-300" title="삭제" aria-label="삭제" onClick={() => removePlan(p)}>
                        <Trash2 className="h-4 w-4" />
                      </button>
                    </div>
                  </li>
                );
              })}
            </ul>
          )}
          <p className="mt-3 text-xs leading-relaxed text-zinc-500">
            자동 기록은 하루 한 번 받는 환율로 계산한 추정입니다. 토스뱅크의 실제 체결 금액과 다르면 아래 거래 기록에서 고치세요.
          </p>
        </div>
      )}

      <div className="grid gap-5 xl:grid-cols-[minmax(0,1fr)_minmax(0,1.4fr)]">
        {/* 환율 */}
        {ov && (
          <div className="surface p-5">
            <div className="mb-3 flex items-center gap-3">
              <IconTile icon={RefreshCw} tone="sky" size="sm" />
              <div>
                <h3 className="section-title">오늘 환율</h3>
                <p className="label-mono">눌러서 사기 · 매매기준율과 몇 원 다를 수 있음</p>
              </div>
            </div>
            <table className="w-full text-sm">
              <thead>
                <tr className="label-mono text-left">
                  <th className="py-1.5 font-normal">통화</th>
                  <th className="py-1.5 text-right font-normal">환율</th>
                  <th className="py-1.5 text-right font-normal">전날 대비</th>
                </tr>
              </thead>
              <tbody>
                {ov.rates.map((r) => {
                  const c = curOf(r.currency)!;
                  const diff = r.prev > 0 && r.rate > 0 ? (r.rate - r.prev) * c.unit : 0;
                  // 등락률은 표시 단위(1 · 100)와 상관없다
                  const pct = r.prev > 0 && r.rate > 0 ? ((r.rate - r.prev) / r.prev) * 100 : 0;
                  return (
                    <tr
                      key={r.currency}
                      className="cursor-pointer border-t border-white/[0.05] hover:bg-white/[0.03]"
                      onClick={() => openTrade(r.currency, "buy")}
                    >
                      <td className="py-1.5">
                        <span className="text-zinc-200">{c.name}</span>{" "}
                        <span className="label-mono">{unitLabel(c)}</span>
                      </td>
                      <td className="num py-1.5 text-right text-white">{rateText(c, r.rate)}</td>
                      <td className={`num py-1.5 text-right ${diff > 0 ? "text-rose-300" : diff < 0 ? "text-sky-300" : "text-zinc-500"}`}>
                        {r.prev > 0 ? (
                          <>
                            {`${diff > 0 ? "▲" : diff < 0 ? "▼" : ""}${Math.abs(diff).toFixed(2)}`}
                            <span className="ml-1.5 inline-block min-w-[3.75rem] text-xs">
                              {`${pct > 0 ? "+" : pct < 0 ? "-" : ""}${Math.abs(pct).toFixed(2)}%`}
                            </span>
                          </>
                        ) : (
                          "-"
                        )}
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        )}

        {/* 거래 기록 */}
        {ov && (
          <div className="surface p-5">
            <div className="mb-3 flex items-center gap-3">
              <IconTile icon={ArrowDownLeft} tone="mint" size="sm" />
              <h3 className="section-title">거래 기록</h3>
              <span className="label-mono num">{trades.length}건</span>
            </div>
            {trades.length === 0 ? (
              <p className="text-sm text-zinc-500">아직 기록이 없습니다.</p>
            ) : (
              <ul className="space-y-1.5">
                {shownTrades.map((t) => {
                  const c = curOf(t.currency);
                  const planName = t.planId ? ov.planNames?.[t.planId] : "";
                  return (
                    <li key={t.id} className="surface-inset flex items-center justify-between gap-3 rounded-xl px-3 py-2">
                      <div className="min-w-0">
                        <div className="flex flex-wrap items-center gap-1.5">
                          <span className="num text-sm text-zinc-300">{t.date}</span>
                          <span
                            className={`badge ${
                              t.source === "opening"
                                ? "bg-white/[0.06] text-zinc-300"
                                : t.side === "sell"
                                ? "bg-sky-500/15 text-sky-300"
                                : "bg-rose-500/10 text-rose-300"
                            }`}
                          >
                            {t.source === "opening" ? "처음 잔고" : t.side === "sell" ? "팔기" : "사기"}
                          </span>
                          {(t.source === "daily" || t.source === "target") && (
                            <span className="badge bg-amber-500/10 text-amber-300" title={planName || SOURCE_LABEL[t.source]}>
                              {SOURCE_LABEL[t.source]}
                              {t.estimated ? " · 추정" : ""}
                            </span>
                          )}
                          <span className="num text-sm font-medium text-white">{fxText(c, t.amount)}</span>
                        </div>
                        <div className="label-mono num mt-1 truncate">
                          {c ? `${rateText(c, t.rate)} / ${unitLabel(c)}` : ""}
                          {t.memo ? ` · ${t.memo}` : ""}
                        </div>
                      </div>
                      <div className="flex shrink-0 items-center gap-1">
                        <span className={`num mr-2 text-sm ${t.side === "sell" ? "text-emerald-300" : "text-zinc-200"}`}>
                          {t.side === "sell" ? "+" : t.source === "opening" ? "" : "−"}
                          {won(t.krw)}
                        </span>
                        <button className="btn-icon" title="수정" aria-label="수정" onClick={() => openTrade(t.currency, t.side as TradeMode, t)}>
                          <Pencil className="h-4 w-4" />
                        </button>
                        <button className="btn-icon hover:!text-rose-300" title="삭제" aria-label="삭제" onClick={() => removeTrade(t)}>
                          <Trash2 className="h-4 w-4" />
                        </button>
                      </div>
                    </li>
                  );
                })}
              </ul>
            )}
            {trades.length > TRADES_PAGE && (
              <button className="btn-ghost mt-3 w-full justify-center" onClick={() => setShowAll((v) => !v)}>
                {showAll ? "접기" : `전체 ${trades.length}건 보기`}
              </button>
            )}
          </div>
        )}
      </div>

      {modal?.t === "trade" && ov && (
        <TradeModal ov={ov} trade={modal.trade} initial={{ currency: modal.currency, mode: modal.mode }} onClose={close} onSaved={load} />
      )}
      {modal?.t === "plan" && ov && <PlanModal ov={ov} plan={modal.plan} onClose={close} onSaved={load} />}
    </div>
  );
}
