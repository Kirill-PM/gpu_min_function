import React from 'react';
import { LineChart, Line, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer, ReferenceLine } from 'recharts';

interface Props {
  data: Array<{ timestamp: string; bestValue: number; elapsedTime: number }>;
  currentProgress: any;
}

const ProgressChart: React.FC<Props> = ({ data, currentProgress }) => {
  return (
    <div className="progress-chart">
      <h3>Прогресс поиска</h3>
      
      {currentProgress && (
        <div className="progress-stats">
          <span>⏱ {currentProgress.elapsed_time?.toFixed(2)} сек</span>
          <span>📊 Задач: {currentProgress.completed_tasks}/{currentProgress.total_tasks}</span>
          <span>🎯 Лучшее: {currentProgress.best_value?.toExponential(6)}</span>
          <span>🔄 Итераций: {currentProgress.total_iterations?.toLocaleString()}</span>
        </div>
      )}

      <ResponsiveContainer width="100%" height={300}>
        <LineChart data={data}>
          <CartesianGrid strokeDasharray="3 3" opacity={0.3} />
          <XAxis 
            dataKey="timestamp" 
            tick={{ fontSize: 10 }}
            interval="preserveStartEnd"
          />
          <YAxis 
            tickFormatter={(v) => v.toExponential(1)}
            tick={{ fontSize: 10 }}
          />
          <Tooltip 
            formatter={(v: number) => [v.toExponential(6), 'Значение']}
            labelFormatter={(l) => `Время: ${l}`}
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
    </div>
  );
};

export default ProgressChart;