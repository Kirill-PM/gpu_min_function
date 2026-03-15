import React from 'react';

interface Props {
  onInsert: (text: string) => void;
}

const Calculator: React.FC<Props> = ({ onInsert }) => {
  const buttons = [
    ['7', '8', '9', '/'],
    ['4', '5', '6', '*'],
    ['1', '2', '3', '-'],
    ['0', '.', '(', ')'],
    ['sin(', 'cos(', 'tan(', 'sqrt('],
    ['exp(', 'log(', 'abs(', 'pow('],
  ];

  return (
    <div className="calculator">
      <div className="calc-title">Панель ввода</div>
      <div className="calc-grid">
        {buttons.flat().map(btn => (
          <button
            key={btn}
            className="calc-btn"
            onClick={() => onInsert(btn)}
          >
            {btn}
          </button>
        ))}
        <button className="calc-btn clear" onClick={() => onInsert('')}>
          C
        </button>
        <button className="calc-btn backspace" onClick={() => onInsert('\b')}>
          ←
        </button>
      </div>
    </div>
  );
};

export default Calculator;