import { store } from "../wailsjs/go/models";

export interface Refs {
  members: store.Member[];
  categories: store.Category[];
  paymentMethods: store.PaymentMethod[];
}

export const won = (n: number) => n.toLocaleString("ko-KR") + "원";

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
