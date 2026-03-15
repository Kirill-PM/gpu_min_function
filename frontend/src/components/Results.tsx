import React from 'react';
import { InlineMath, BlockMath } from 'react-katex';

interface Props {
  results: {
    bestValue: number;
    bestX: number[];
    totalIterations: number;
    totalTime: number;
  } | null;
}

const Results: React.FC<Props> = ({ results }) => {
  if (!results) {
    return (
      <div className="results placeholder">
        <p>Запусти вычисления, чтобы увидеть результаты</p>
      </div>
    );
  }

  const formatNumber = (v: number, digits = 4) => {
    if (!Number.isFinite(v)) return '—';
    return v.toLocaleString(undefined, { maximumFractionDigits: digits });
  };

  const latexX = results.bestX
    .map((v, i) => `x_${i + 1} = ${formatNumber(v, 4)}`)
    .join(', \\quad ');

  return (
    <div className="results">
      <h3>✅ Результаты</h3>
      
      <div className="result-card">
        <div className="result-row">
          <span className="label">Минимальное значение:</span>
          <span className="value highlight">{formatNumber(results.bestValue, 8)}</span>
        </div>
        
        <div className="result-row">
          <span className="label">Точка минимума:</span>
          <span className="value">
            <InlineMath math={latexX} />
          </span>
        </div>

        <div className="result-row">
          <span className="label">Всего итераций:</span>
          <span className="value">{results.totalIterations.toLocaleString()}</span>
        </div>

        <div className="result-row">
          <span className="label">Время расчёта:</span>
          <span className="value">
            {results.totalTime > 0
              ? `${results.totalTime.toFixed(3)} сек`
              : '—'}
          </span>
        </div>

        <div className="result-row">
          <span className="label">Производительность:</span>
          <span className="value">
            {results.totalTime > 0
              ? `${(results.totalIterations / results.totalTime / 1e6).toFixed(2)} млн итер/сек`
              : '—'}
          </span>
        </div>
      </div>
    </div>
  );
};

export default Results;