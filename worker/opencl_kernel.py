import pyopencl as cl
import numpy as np
import time

class OpenCLWorker:
    def __init__(self):
        self.platform = None
        self.device = None
        self.ctx = None
        self.queue = None
        self._init_opencl()
    
    def _init_opencl(self):
        platforms = cl.get_platforms()
        if not platforms:
            raise RuntimeError("No OpenCL platforms found")
        
        self.platform = platforms[0]
        devices = self.platform.get_devices(device_type=cl.device_type.GPU)
        
        if not devices:
            devices = self.platform.get_devices(device_type=cl.device_type.CPU)
        
        self.device = devices[0]
        self.ctx = cl.Context([self.device])
        self.queue = cl.CommandQueue(self.ctx)
    
    def get_gpu_info(self):
        return {
            "gpu_name": self.device.name,
            "thread_count": self.device.max_compute_units * 256,  # примерная оценка
            "max_memory": self.device.global_mem_size // (1024 * 1024),  # MB
        }
    
    def generate_kernel(self, formula: str, var_count: int, mode: str, 
                       target: float, range_min: float, range_max: float) -> str:
        """Генерирует OpenCL-ядро под формулу"""
        
        # Заменяем x1, x2, ... на x[0], x[1], ...
        import re
        cl_formula = formula
        for i in range(var_count, 0, -1):
            cl_formula = cl_formula.replace(f'x{i}', f'x[{i-1}]')
        
        # Заменяем функции
        replacements = {
            'sin(': 'sin(', 'cos(': 'cos(', 'tan(': 'tan(',
            'sqrt(': 'sqrt(', 'exp(': 'exp(', 'log(': 'log(',
            'abs(': 'fabs(', 'pow(': 'pow(',
        }
        for py_func, cl_func in replacements.items():
            cl_formula = cl_formula.replace(py_func, cl_func)
        
        # Для режима find_target
        if mode == 'find_target':
            cl_formula = f'fabs(({cl_formula}) - {target}f)'
        
        # Генерация переменных
        range_span = range_max - range_min
        var_gen = ""
        for i in range(var_count):
            var_gen += f"""        state = (state * 1103515245 + 12345) & 0x7FFFFFFF;
        float x[{i}] = ((float)state / 2147483647.0f) * {range_span}f + {range_min}f;
"""
        
        kernel = f"""
__kernel void find_optimal(__global float* results, __global float* best_x, 
                           const int N, const int seed, const int var_count) {{
    int gid = get_global_id(0);
    uint state = seed + gid;
    float local_min = 1e18f;
    float local_best_x[10];

    for (int i = 0; i < N; i++) {{
{var_gen}
        float val = {cl_formula};
        
        if (isnan(val) || isinf(val)) {{
            val = 1e18f;
        }}
        
        if (val < local_min) {{
            local_min = val;
            for (int j = 0; j < var_count; j++) {{
                local_best_x[j] = x[j];
            }}
        }}
    }}
    results[gid] = local_min;
    for (int j = 0; j < var_count; j++) {{
        best_x[gid * var_count + j] = local_best_x[j];
    }}
}}
"""
        return kernel
    
    def execute_task(self, formula: str, var_count: int, mode: str,
                    target: float, range_min: float, range_max: float,
                    iterations: int, seed: int, thread_count: int) -> dict:
        """Выполняет задачу на GPU"""
        
        kernel_source = self.generate_kernel(formula, var_count, mode, 
                                            target, range_min, range_max)
        
        prg = cl.Program(self.ctx, kernel_source).build()
        
        results = np.zeros(thread_count, dtype=np.float32)
        best_x = np.zeros(thread_count * var_count, dtype=np.float32)
        
        mf = cl.mem_flags
        results_buf = cl.Buffer(self.ctx, mf.WRITE_ONLY, results.nbytes)
        best_x_buf = cl.Buffer(self.ctx, mf.WRITE_ONLY, best_x.nbytes)
        
        start = time.time()
        
        prg.find_optimal(self.queue, (thread_count,), None,
                        results_buf, best_x_buf, 
                        np.int32(iterations), np.int32(seed), np.int32(var_count))
        
        self.queue.finish()
        
        cl.enqueue_copy(self.queue, results, results_buf)
        cl.enqueue_copy(self.queue, best_x, best_x_buf)
        
        elapsed = time.time() - start
        
        # Находим лучший результат среди всех потоков
        min_idx = np.argmin(results)
        best_value = float(results[min_idx])
        best_x_values = best_x[min_idx * var_count:(min_idx + 1) * var_count].tolist()
        
        return {
            "best_value": best_value,
            "best_x": best_x_values,
            "iterations": iterations * thread_count,
            "time_spent": elapsed,
        }
    

import math
import re

def validate_formula_cpu(formula: str, var_count: int, sample_point: list = None) -> tuple[bool, str]:
    """
    Проверяет формулу на корректность синтаксиса, выполняя её на CPU с тестовыми значениями.
    Возвращает (success, error_message)
    """
    if not formula or not formula.strip():
        return False, "Формула пустая"
    
    # Проверка на недопустимые символы
    if re.search(r'[;{}$`\\]', formula):
        return False, "Обнаружены недопустимые символы"
    
    # Подготовка безопасного окружения для eval
    allowed_names = {
        k: v for k, v in math.__dict__.items() 
        if not k.startswith("__") and k != "name"
    }
    allowed_names['abs'] = abs  # built-in abs
    
    # Создаём переменные x1, x2, ...
    if sample_point is None:
        sample_point = [0.5] * var_count
    
    for i in range(var_count):
        allowed_names[f'x{i+1}'] = sample_point[i]
    
    # Заменяем функции на Python-версии (если пользователь ввёл OpenCL-стиль)
    safe_formula = formula
    # math.log в Python — это ln, как и в OpenCL
    # math.pow есть, но ** тоже работает
    
    try:
        # Пробуем вычислить с тестовыми значениями
        result = eval(safe_formula, {"__builtins__": {}}, allowed_names)
        
        if not isinstance(result, (int, float)):
            return False, f"Результат должен быть числом, получено {type(result)}"
        
        if math.isnan(result) or math.isinf(result):
            # Это не ошибка синтаксиса, но предупреждение
            return True, "Предупреждение: формула возвращает NaN/inf при тестовых значениях"
        
        return True, "OK"
        
    except SyntaxError as e:
        return False, f"Синтаксическая ошибка: {e}"
    except NameError as e:
        # Извлекаем имя неизвестной переменной
        match = re.search(r"name '(\w+)' is not defined", str(e))
        var_name = match.group(1) if match else "unknown"
        return False, f"Неизвестная переменная или функция: {var_name}. Используйте x1-x{var_count}, sin, cos, sqrt..."
    except ZeroDivisionError:
        return True, "Предупреждение: возможное деление на ноль при некоторых значениях"
    except Exception as e:
        return False, f"Ошибка вычисления: {type(e).__name__}: {e}"