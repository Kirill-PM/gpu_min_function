import React from 'react';
import { LineChart, Line, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer, ReferenceLine } from 'recharts';

interface Props {
  data: Array<{ elapsedTime: number; bestValue: number }>;
  currentProgress: any;
  maxElapsedTime?: number;
}

const ProgressChart: React.FC<Props> = ({ data, currentProgress, maxElapsedTime }) => {
  const hasData = Array.isArray(data) && data.length > 0;

  const formatNumber = (v: number, decimals = 6) => {
    if (!Number.isFinite(v)) return '0';
    return v.toLocaleString(undefined, { maximumFractionDigits: decimals });
  };

  return (
    <div className="progress-chart">
      <h3>Прогресс поиска</h3>
      
      {currentProgress && (
        <div className="progress-stats">
          <span>⏱ {currentProgress.elapsed_time?.toFixed(2)} сек</span>
          <span>📊 Задач: {currentProgress.completed_tasks}/{currentProgress.total_tasks}</span>
          <span>🎯 Лучшее: {formatNumber(currentProgress.best_value, 6)}</span>
          <span>🔄 Итераций: {currentProgress.total_iterations?.toLocaleString()}</span>
        </div>
      )}

      {!hasData ? (
        <div className="progress-chart__empty">Ожидаем результатов от воркеров...</div>
      ) : (
        <ResponsiveContainer width="100%" height={300}>
          <LineChart data={data}>
            <CartesianGrid strokeDasharray="3 3" opacity={0.3} />
            <XAxis 
              dataKey="elapsedTime" 
              tick={{ fontSize: 10 }}
              interval="preserveStartEnd"
              tickFormatter={(v) => typeof v === 'number' ? `${v.toFixed(1)}s` : '0.0s'}
              domain={[
              0,
              typeof maxElapsedTime === 'number'
                ? maxElapsedTime
                : currentProgress?.elapsed_time ?? 'dataMax',
            ]}
            />
            <YAxis 
              tickFormatter={(v) => typeof v === 'number' ? formatNumber(v, 3) : '0'}
              tick={{ fontSize: 10 }}
            />
            <Tooltip 
              formatter={(v: number) => [typeof v === 'number' ? formatNumber(v, 6) : '0', 'Значение']}
              labelFormatter={(l) => `Время: ${typeof l === 'number' ? l.toFixed(2) : '0.00'} с`}
            />
            <Line 
              type="monotone" 
              dataKey="bestValue" 
              stroke="#2563eb" 
              dot={false}
              strokeWidth={2}
              isAnimationActive={false}
            />
          </LineChart>
        </ResponsiveContainer>
      )}
    </div>
  );
};

export default ProgressChart;