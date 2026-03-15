import React, { useState } from 'react';
import { InlineMath } from 'react-katex';
import 'katex/dist/katex.min.css';
import FormulaInput from './components/FormulaInput';
import ProgressChart from './components/ProgressChart';
import Results from './components/Results';
import Controls from './components/Controls';
import { apiClient } from './api/client';
import { useWebSocket } from './hooks/useWebSocket';

interface ProgressData {
  elapsedTime: number;
  bestValue: number;
}

function App() {
  const [formula, setFormula] = useState('x1 + cos(x2) * (x3 + 54)');
  const [mode, setMode] = useState<'minimize' | 'find_target'>('minimize');
  const [target, setTarget] = useState(0);
  const [variableCount, setVariableCount] = useState(3);
  const [rangeMin, setRangeMin] = useState(-100);
  const [rangeMax, setRangeMax] = useState(100);
  const [stopValue, setStopValue] = useState(30);
  const [isRunning, setIsRunning] = useState(false);
  const [progressData, setProgressData] = useState<ProgressData[]>([]);
  const [currentProgress, setCurrentProgress] = useState<any>(null);
  const [results, setResults] = useState<any>(null);
  const [fixedChartDuration, setFixedChartDuration] = useState<number | undefined>(undefined);

  const ws = useWebSocket('ws://localhost:3000/ws', (data) => {
    try {
      setCurrentProgress(data);

      // Don't add the initial placeholder value (1e18) to the chart.
      // Only plot values received from workers (when at least one task completed).
      const bestValue = data.best_value;
      const hasWorkerValue = Number.isFinite(bestValue) && bestValue < 1e18 && data.completed_tasks > 0;

      if (data.is_running && hasWorkerValue) {
        setProgressData(prev => [
          ...prev,
          {
            elapsedTime: data.elapsed_time || 0,
            bestValue,
          },
        ]);
      }

      if (!data.is_running) {
        setIsRunning(false);
        // Не стираем прогресс, чтобы график оставался видимым после завершения.
        setResults({
          bestValue: data.best_value || 0,
          bestX: data.best_x || [],
          totalIterations: data.total_iterations || 0,
          totalTime: data.elapsed_time || 0,
        });
      }
    } catch (e) {
      console.error('Error processing WebSocket data:', e, data);
    }
  });

  const handleStart = async () => {
    try {
      await apiClient.start({
        formula,
        mode,
        target: mode === 'find_target' ? target : undefined,
        variable_count: variableCount,
        range_min: rangeMin,
        range_max: rangeMax,
        stop_condition: {
          type: 'time',
          duration: stopValue,
        },
      });
      setIsRunning(true);
      // Сбрасываем график и добавляем начальную точку в 0
      setProgressData([{ elapsedTime: 0, bestValue: 0 }]);
      setResults(null);
      setFixedChartDuration(stopValue);
    } catch (err) {
      alert('Ошибка запуска: ' + err);
    }
  };

  const handleStop = async () => {
    await apiClient.stop();
    setIsRunning(false);
  };

  // LaTeX представление формулы
  const latexFormula = formula
    .replace(/x(\d+)/g, 'x_$1')
    .replace(/sin\(/g, '\\sin(')
    .replace(/cos\(/g, '\\cos(')
    .replace(/tan\(/g, '\\tan(')
    .replace(/sqrt\(/g, '\\sqrt{')
    .replace(/exp\(/g, '\\exp(')
    .replace(/log\(/g, '\\log(')
    .replace(/abs\(/g, '|')
    .replace(/\*/g, '\\cdot ')
    .replace(/\//g, '/');

  return (
    <div className="app">
      <header>
        <h1>GPU Optimizer</h1>
        <div className="formula-display">
          <InlineMath math={latexFormula} />
        </div>
      </header>

      <main>
        <section className="controls-section">
          <FormulaInput 
            value={formula} 
            onChange={setFormula} 
            variableCount={variableCount}
          />
          
          <div className="mode-selector">
            <label>
              <input 
                type="radio" 
                checked={mode === 'minimize'} 
                onChange={() => setMode('minimize')} 
              />
              Поиск минимума
            </label>
            <label>
              <input 
                type="radio" 
                checked={mode === 'find_target'} 
                onChange={() => setMode('find_target')} 
              />
              Поиск ближайшего решения
            </label>
            {mode === 'find_target' && (
              <input 
                type="number" 
                value={target} 
                onChange={e => setTarget(parseFloat(e.target.value))}
                placeholder="Target value"
              />
            )}
          </div>

          <div className="variable-settings">
            <label>
              Количество переменных:
              <input 
                type="number" 
                min="1" 
                max="10" 
                value={variableCount} 
                onChange={e => setVariableCount(parseInt(e.target.value))} 
              />
            </label>
            <label>
              Диапазон: [{rangeMin}, {rangeMax}]
              <input 
                type="number" 
                value={rangeMin} 
                onChange={e => setRangeMin(parseFloat(e.target.value))} 
              />
              <input 
                type="number" 
                value={rangeMax} 
                onChange={e => setRangeMax(parseFloat(e.target.value))} 
              />
            </label>
          </div>

          <div className="stop-condition">
            <label>
              Количество секунд:
              <input 
                type="number" 
                value={stopValue} 
                onChange={e => setStopValue(parseInt(e.target.value))} 
              />
            </label>
          </div>

          <Controls 
            isRunning={isRunning} 
            onStart={handleStart} 
            onStop={handleStop} 
          />
        </section>

        <section className="progress-section">
          <ProgressChart data={progressData} currentProgress={currentProgress} maxElapsedTime={fixedChartDuration} />
        </section>

        <section className="results-section">
          <Results results={results} />
        </section>
      </main>
    </div>
  );
}

export default App;