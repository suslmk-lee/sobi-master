// 로딩 중 빈 화면 대신 보여주는 쉬머 플레이스홀더.

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
  return <div className="sk" style={{ height: h, width: w, borderRadius: r, ...style }} />;
}

// 대시보드 모양의 스켈레톤: 요약 카드 4개 + 큰 차트 + 2열 카드
export function DashboardSkeleton() {
  return (
    <div>
      <div className="toolbar">
        <Sk w={150} h={36} r={10} />
      </div>
      <div className="stat-grid">
        {[0, 1, 2, 3].map((i) => (
          <div className="card stat" key={i}>
            <Sk w={80} h={12} />
            <Sk w={130} h={22} />
            <Sk w={100} h={12} />
          </div>
        ))}
      </div>
      <div className="card">
        <Sk w={180} h={16} style={{ marginBottom: 12 }} />
        <Sk h={200} r={12} />
      </div>
      <div className="dash-grid">
        {[0, 1, 2, 3].map((i) => (
          <div className="card" key={i}>
            <Sk w={140} h={16} style={{ marginBottom: 12 }} />
            <Sk h={150} r={12} />
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
        <tr key={i}>
          {Array.from({ length: cols }, (_, j) => (
            <td key={j}>
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
    <div className="card-grid">
      {[0, 1].map((i) => (
        <div className="card credit-card" key={i}>
          <Sk w={180} h={18} style={{ marginBottom: 10 }} />
          <Sk w={220} h={12} style={{ marginBottom: 8 }} />
          <Sk w={160} h={14} style={{ marginBottom: 10 }} />
          <Sk h={14} r={4} />
        </div>
      ))}
    </div>
  );
}
