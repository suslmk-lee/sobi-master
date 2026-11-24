// 화면 데이터 기억.
//
// DB 가 멀리 있어 화면을 열 때마다 받아 오면 잠깐씩 빈 화면(뼈대)이 보인다. 한 번 받은 내용을 기억해 두고,
// 화면을 다시 열면 그 내용을 먼저 보여 준 뒤 뒤에서 새로 받아 바꾼다(그사이 바뀐 게 있으면 곧 반영된다).
// 앱을 켜 둔 동안 메모리에만 둔다 — 디스크에 남기지 않는다.

const mem = new Map<string, unknown>();

/** key 로 기억해 둔 값(없으면 undefined). */
export function cached<T>(key: string): T | undefined {
  return mem.get(key) as T | undefined;
}

/** fetch 로 새로 받아 key 로 기억하고 돌려준다. 실패하면 기억해 둔 값은 그대로 둔다. */
export async function refresh<T>(key: string, fetch: () => Promise<T>): Promise<T> {
  const v = await fetch();
  mem.set(key, v);
  return v;
}
