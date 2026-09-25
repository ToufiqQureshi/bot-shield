interface EvidenceSignalsProps {
  signals?: string[];
  shadowSignals?: string[];
}

export default function EvidenceSignals({ signals = [], shadowSignals = [] }: EvidenceSignalsProps) {
  return (
    <div className="flex flex-wrap gap-1">
      {signals.slice(0, 3).map((signal) => (
        <span key={`scored-${signal}`} className="badge badge-red text-[10px]">{signal}</span>
      ))}
      {shadowSignals.slice(0, 3).map((signal) => (
        <span key={`observed-${signal}`} className="badge badge-yellow text-[10px]" title="Observed only; did not affect the decision">
          Observed: {signal}
        </span>
      ))}
    </div>
  );
}
