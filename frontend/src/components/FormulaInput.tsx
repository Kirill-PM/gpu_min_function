import React from 'react';

interface Props {
  value: string;
  onChange: (val: string) => void;
  variableCount: number;
}

const FormulaInput: React.FC<Props> = ({ value, onChange, variableCount }) => {
  const variables = Array.from({ length: variableCount }, (_, i) => `x${i + 1}`);

  return (
    <div className="formula-input">
      <label>Формула:</label>
      <input
        type="text"
        value={value}
        onChange={e => onChange(e.target.value)}
        placeholder="Например: x1 + cos(x2) * (x3 + 54)"
        className="formula-field"
      />
      <div className="variables-hint">
        Доступные переменные: {variables.join(', ')}
      </div>
      <div className="supported-funcs">
        Функции: sin, cos, tan, sqrt, exp, log, abs, pow(x,y)
      </div>
    </div>
  );
};

export default FormulaInput;