import { FormEvent, useEffect, useState } from "react";
import { Pencil, X } from "lucide-react";
import {
  RenameCategory,
  SetCategoryDefaultMember,
  SetCategoryFixed,
  SetCategorySpendClass,
} from "../wailsjs/go/main/App";
import { store } from "../wailsjs/go/models";
import Portal from "./components/ui/Portal";
import IconTile from "./components/ui/IconTile";

const KIND_LABEL: Record<string, string> = { income: "수입", expense: "지출", transfer: "이체" };

// 지출 성격 — 낭비율(낭비 + 선택 카테고리의 예산 초과분 ÷ 총지출)이 이 값으로 거래를 나눈다.
const SPEND_CLASSES: { key: string; label: string; hint: string }[] = [
  { key: "", label: "미지정", hint: "" },
  { key: "essential", label: "필수", hint: "통신비·대출·의료비처럼 줄이기 어려운 돈. 낭비율에 넣지 않습니다." },
  { key: "discretionary", label: "선택", hint: "쓸지 말지 고를 수 있는 돈. 예산을 넘은 만큼만 낭비로 봅니다." },
  { key: "waste", label: "낭비", hint: "안 써도 됐던 돈. 전액을 낭비로 봅니다." },
];
export const SPEND_CLASS_LABEL: Record<string, string> = { essential: "필수", discretionary: "선택", waste: "낭비" };

// 카테고리 수정 팝업 — 이름·기본 귀속자·고정비 표시·지출 성격을 바꾼다. 거래·규칙·예산은 카테고리 id 로 이어져 있어 그대로 따라온다.
// 고정비로 표시하면 구성원 분석이 이 카테고리(주면 그 부까지)를 '생활 소비'에서 빼고 따로 보여 준다.
// 상위(주) 이동은 목록의 선택 상자에서, 순서는 끌어서 바꾼다.
export default function CategoryEditModal({
  cat,
  members,
  subCount,
  onClose,
  onDone,
}: {
  cat: store.Category;
  members: store.Member[];
  subCount: number;
  onClose: () => void;
  onDone: () => void;
}) {
  const [name, setName] = useState(cat.name);
  const [fixed, setFixed] = useState(cat.fixedCost);
  const [spendClass, setSpendClass] = useState(cat.spendClass);
  const initialMember = cat.defaultMemberId ? String(cat.defaultMemberId) : "";
  const [defMember, setDefMember] = useState(initialMember);
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState("");

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose();
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [onClose]);

  const trimmed = name.trim();
  const renamed = trimmed !== cat.name;
  const fixedChanged = fixed !== cat.fixedCost;
  const memberChanged = defMember !== initialMember;
  const classChanged = spendClass !== cat.spendClass;
  const changed = trimmed !== "" && (renamed || fixedChanged || memberChanged || classChanged);
  const parentMemberName = cat.parentDefaultMemberId
    ? members.find((m) => m.id === cat.parentDefaultMemberId)?.name
    : undefined;

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    if (!changed || busy) return;
    setBusy(true);
    setErr("");
    try {
      if (renamed) await RenameCategory(cat.id, trimmed);
      if (fixedChanged) await SetCategoryFixed(cat.id, fixed);
      if (memberChanged) await SetCategoryDefaultMember(cat.id, defMember ? Number(defMember) : 0);
      if (classChanged) await SetCategorySpendClass(cat.id, spendClass);
      onDone();
    } catch (ex) {
      setErr(String(ex));
      setBusy(false);
    }
  };

  const where = cat.parent
    ? `${cat.parent} 아래 부 카테고리`
    : `${KIND_LABEL[cat.kind] ?? cat.kind} 주 카테고리${subCount > 0 ? ` · 부 ${subCount}개` : ""}`;

  return (
    <Portal>
      <div
        className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-3 backdrop-blur-sm sm:p-6"
        onMouseDown={(e) => {
          if (e.target === e.currentTarget) onClose();
        }}
      >
        <form
          role="dialog"
          aria-modal="true"
          aria-label={`${cat.fullName} 수정`}
          onSubmit={submit}
          className="surface w-full max-w-md space-y-4 p-5 sm:p-6"
        >
          <div className="flex items-start gap-3">
            <IconTile icon={Pencil} tone="mint" size="md" />
            <div className="min-w-0 flex-1">
              <h3 className="section-title">카테고리 수정</h3>
              <p className="mt-1.5 break-words text-sm text-zinc-400">{where}</p>
            </div>
            <button type="button" className="btn-icon" aria-label="닫기" onClick={onClose}>
              <X className="h-4 w-4" />
            </button>
          </div>
          <label className="block">
            <span className="field-label">이름</span>
            <input
              type="text"
              className="input-field"
              value={name}
              autoFocus
              onFocus={(e) => e.target.select()}
              onChange={(e) => setName(e.target.value)}
            />
          </label>
          <p className="text-xs text-zinc-500">
            이 카테고리로 분류된 거래·규칙·예산은 새 이름으로 그대로 이어집니다.
          </p>
          <label className="block">
            <span className="field-label">기본 귀속자</span>
            <select className="input-field" value={defMember} onChange={(e) => setDefMember(e.target.value)}>
              <option value="">{parentMemberName ? `상위 '${cat.parent}' 따름 (${parentMemberName})` : "없음"}</option>
              {members.map((m) => (
                <option key={m.id} value={m.id}>
                  {m.name}
                </option>
              ))}
            </select>
            <span className="mt-1 block text-xs text-zinc-500">
              거래·정기결제·규칙에서 이 카테고리를 고르면 귀속자 칸이 이 사람으로 바뀝니다. 직접 고른 귀속자는 바꾸지
              않습니다.{subCount > 0 && " 부 카테고리도 따로 정하지 않으면 이 값을 따릅니다."}
            </span>
          </label>
          {cat.kind === "expense" && (
            <label className="flex cursor-pointer items-start gap-2.5 rounded-xl border border-white/[0.08] px-3 py-2.5">
              <input
                type="checkbox"
                className="mt-0.5"
                checked={fixed || cat.parentFixed}
                disabled={cat.parentFixed}
                onChange={(e) => setFixed(e.target.checked)}
              />
              <span className="text-sm text-zinc-200">
                고정비로 보기
                <span className="mt-0.5 block text-xs text-zinc-500">
                  {cat.parentFixed
                    ? `상위 '${cat.parent}'가 고정비라 이 부도 고정비로 봅니다.`
                    : "보험료·통신비처럼 매달 정해진 돈. 구성원 분석의 '생활 소비'에서 빼고 따로 보여 줍니다."}
                </span>
              </span>
            </label>
          )}
          {cat.kind === "expense" && (
            <div>
              <span className="field-label">지출 성격</span>
              <div className="glass-pill inline-flex gap-1 p-1" role="group" aria-label="지출 성격">
                {SPEND_CLASSES.map((o) => (
                  <button
                    key={o.key}
                    type="button"
                    aria-pressed={spendClass === o.key}
                    className={`pill-item ${spendClass === o.key ? "pill-item-active" : ""}`}
                    onClick={() => setSpendClass(o.key)}
                  >
                    {o.key === "" && cat.parentSpendClass ? `상위 따름 (${SPEND_CLASS_LABEL[cat.parentSpendClass]})` : o.label}
                  </button>
                ))}
              </div>
              <span className="mt-1 block text-xs text-zinc-500">
                {spendClass
                  ? SPEND_CLASSES.find((o) => o.key === spendClass)?.hint
                  : cat.parentSpendClass
                    ? `상위 '${cat.parent}'의 성격(${SPEND_CLASS_LABEL[cat.parentSpendClass]})을 따릅니다.`
                    : fixed || cat.parentFixed
                      ? "정하지 않으면 고정비라서 필수로 봅니다."
                      : "정하지 않으면 정기결제로 낸 것만 필수로 보고, 나머지는 '미지정'으로 따로 셉니다."}
                {subCount > 0 && " 부 카테고리도 따로 정하지 않으면 이 값을 따릅니다."}
              </span>
            </div>
          )}
          {/* 자금 계획은 이름이 "급여"인 카테고리로 월급을 찾는다(cashplan.go) */}
          {cat.name === "급여" && renamed && trimmed !== "" && (
            <p className="text-xs text-amber-300">
              자금 계획은 '급여' 카테고리로 월급을 찾습니다. 이름을 바꾸면 월급을 전체 수입 평균으로 계산합니다.
            </p>
          )}
          {err && <p className="text-sm text-rose-300">{err}</p>}
          <div className="flex items-center justify-end gap-2">
            <button type="button" className="btn-ghost" onClick={onClose}>
              취소
            </button>
            <button type="submit" className="btn-primary" disabled={!changed || busy}>
              {busy ? "저장 중…" : "저장"}
            </button>
          </div>
        </form>
      </div>
    </Portal>
  );
}
