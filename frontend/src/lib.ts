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
