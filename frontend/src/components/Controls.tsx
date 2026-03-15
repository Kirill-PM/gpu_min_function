import React from 'react';

interface Props {
  isRunning: boolean;
  onStart: () => void;
  onStop: () => void;
}

const Controls: React.FC<Props> = ({ isRunning, onStart, onStop }) => {
  return (
    <div className="controls">
      <button 
        className={`btn btn-start ${isRunning ? 'disabled' : ''}`}
        onClick={onStart}
        disabled={isRunning}
      >
        Запустить
      </button>
      
      <button 
        className={`btn btn-stop ${!isRunning ? 'disabled' : ''}`}
        onClick={onStop}
        disabled={!isRunning}
      >
        Остановить
      </button>
      
      {isRunning && (
        <div className="running-indicator">
          <span className="pulse"></span>
          Вычисления выполняются...
        </div>
      )}
    </div>
  );
};

export default Controls;