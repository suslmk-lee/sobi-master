// 로딩 중 빈 화면 대신 보여주는 플레이스홀더 (§6.15). 어두운 테마에서 펄스만으로는 잘 안 보여
// 빛이 지나가는 효과(sk-shimmer, styles/loading.css)를 쓴다.
import "./styles/loading.css";

export function Sk({
  h = 14,
  w = "100%",
  r = 8,
  style,
}: {
  h?: number | string;
  w?: number | string;
  r?: number;
  style?: React.CSSProperties;
}) {
  return (
    <div
      className="sk-shimmer"
      style={{ height: h, width: w, borderRadius: r, ...style }}
    />
  );
}

// 대시보드 모양의 스켈레톤: 요약 카드 4개 + 큰 차트 + 2열 카드
export function DashboardSkeleton() {
  return (
    <div className="space-y-4">
      <Sk w={150} h={36} r={10} />
      <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-4">
        {[0, 1, 2, 3].map((i) => (
          <div className="surface flex flex-col gap-3 p-5" key={i}>
            <Sk w={80} h={12} />
            <Sk w={130} h={22} />
            <Sk w={100} h={12} />
          </div>
        ))}
      </div>
      <div className="surface p-5 sm:p-6">
        <Sk w={180} h={16} style={{ marginBottom: 12 }} />
        <Sk h={200} r={16} />
      </div>
      <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
        {[0, 1, 2, 3].map((i) => (
          <div className="surface p-5 sm:p-6" key={i}>
            <Sk w={140} h={16} style={{ marginBottom: 12 }} />
            <Sk h={150} r={16} />
          </div>
        ))}
      </div>
    </div>
  );
}

// 표 본문 스켈레톤 행
export function TableSkeleton({ cols, rows = 6 }: { cols: number; rows?: number }) {
  return (
    <>
      {Array.from({ length: rows }, (_, i) => (
        <tr key={i} className="table-row">
          {Array.from({ length: cols }, (_, j) => (
            <td key={j} className="py-2.5 pr-3">
              <Sk h={14} w={j === 1 ? "80%" : "60%"} />
            </td>
          ))}
        </tr>
      ))}
    </>
  );
}

// 카드 탭 스켈레톤
export function CardsSkeleton() {
  return (
    <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
      {[0, 1].map((i) => (
        <div className="surface p-5" key={i}>
          <Sk w={180} h={18} style={{ marginBottom: 10 }} />
          <Sk w={220} h={12} style={{ marginBottom: 8 }} />
          <Sk w={160} h={14} style={{ marginBottom: 10 }} />
          <Sk h={14} r={4} />
        </div>
      ))}
    </div>
  );
}
