import React from 'react';
import { LineChart, Line, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer, ReferenceLine } from 'recharts';

interface Props {
  data: Array<{ elapsedTime: number; bestValue: number }>;
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
            dataKey="elapsedTime" 
            tick={{ fontSize: 10 }}
            interval="preserveStartEnd"
            tickFormatter={(v) => typeof v === 'number' ? `${v.toFixed(1)}s` : '0.0s'}
            domain={[0, 'dataMax']}
          />
          <YAxis 
            tickFormatter={(v) => typeof v === 'number' ? v.toExponential(1) : '0'}
            tick={{ fontSize: 10 }}
          />
          <Tooltip 
            formatter={(v: number) => [typeof v === 'number' ? v.toExponential(6) : '0', 'Значение']}
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
    </div>
  );
};

export default ProgressChart;