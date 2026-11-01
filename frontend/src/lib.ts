import { store } from "../wailsjs/go/models";

export interface Refs {
  members: store.Member[];
  categories: store.Category[];
  paymentMethods: store.PaymentMethod[];
}

// 정수로 반올림해 찍는다 — CountUp 애니메이션 중간값(4,063,679.873)이 소수 자리까지 나오지 않게
export const won = (n: number) => Math.round(n).toLocaleString("ko-KR") + "원";

// "1234567" → "1,234,567" : 숫자만 남기고 천 단위 쉼표를 붙인다.
export const formatAmount = (s: string) => {
  const digits = s.replace(/[^\d]/g, "");
  return digits ? Number(digits).toLocaleString("ko-KR") : "";
};
export const parseAmount = (s: string) => Number(s.replace(/[^\d]/g, ""));

export const thisMonth = () => {
  const d = new Date();
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}`;
};

export const today = () => {
  const d = new Date();
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}-${String(
    d.getDate()
  ).padStart(2, "0")}`;
};

// 카테고리 계층(주/부) 옵션 목록. 백엔드가 주 그룹 → 그 안의 부 순서로 정렬해 주므로
// 순서대로 뿌리면서 부만 들여쓰기하면 계층이 그대로 보인다.
// kind 를 주면 그 종류(income/expense/transfer)만 남긴다.
export function categoryOptions(
  categories: store.Category[],
  kind?: string
): { id: number; label: string; isSub: boolean }[] {
  return categories
    .filter((c) => !kind || c.kind === kind)
    .map((c) => ({
      id: c.id,
      label: c.parentId ? `　└ ${c.name}` : c.name,
      isSub: !!c.parentId,
    }));
}

// 주(대분류) 카테고리만. 상위 선택 드롭다운에 쓴다.
export const mainCategories = (categories: store.Category[], kind?: string) =>
  categories.filter((c) => !c.parentId && (!kind || c.kind === kind));

export const DIRECTION_LABEL: Record<string, string> = {
  income: "수입",
  expense: "지출",
  transfer: "이체",
};

export const KIND_OF_DIRECTION: Record<string, string> = {
  income: "income",
  expense: "expense",
  transfer: "transfer",
};

// ── 결제수단 해지(보관) ──
// closedAt 이 오늘 이전(또는 오늘)이면 해지된 결제수단이다. 빈 문자열은 사용 중.
export const isClosed = (
  pm: { closedAt?: string } | undefined | null,
  now: string = today()
) => !!pm && !!pm.closedAt && pm.closedAt <= now;

export const openMethods = (list: store.PaymentMethod[]) =>
  list.filter((p) => !isClosed(p));

// 새 항목용 선택지: 사용 중인 것만. currentId 가 해지된 수단이면 그것만 예외로 남긴다.
export const selectableMethods = (
  list: store.PaymentMethod[],
  currentId?: number | string | null
) =>
  list.filter(
    (p) => !isClosed(p) || (currentId != null && String(p.id) === String(currentId))
  );

export const methodLabel = (pm: { name: string; closedAt?: string }) =>
  isClosed(pm) ? `${pm.name} (해지)` : pm.name;

export const CLOSED_HINT = "해지된 카드입니다 — 다른 결제수단으로 바꾸세요";
