import pyopencl as cl
import numpy as np
import time

class OpenCLWorker:
    def __init__(self):
        self.platform = None
        self.device = None
        self.ctx = None
        self.queue = None
        self.has_gpu = False
        self._init_opencl()
    
    def _init_opencl(self):
        try:
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
            self.has_gpu = True
            print(f"✅ OpenCL initialized: {self.device.name}")
        except Exception as e:
            print(f"⚠️ OpenCL not available ({e}), falling back to CPU simulation")
            self.has_gpu = False
            self.device = None
            self.ctx = None
            self.queue = None

    
    def get_gpu_info(self):
        if self.has_gpu:
            return {
                "gpu_name": self.device.name,
                "thread_count": self.device.max_compute_units * 256,  # примерная оценка
                "max_memory": self.device.global_mem_size // (1024 * 1024),  # MB
            }
        else:
            return {
                "gpu_name": "CPU Simulation",
                "thread_count": 16,  # CPU cores
                "max_memory": 8192,  # MB
            }
    
    def generate_kernel(self, formula: str, var_count: int, mode: str,
                        target: float, range_min: float, range_max: float) -> str:
        """Генерирует OpenCL-ядро с генератором Xorshift128+"""
        
        # === Код генератора Xorshift128+ ===
        rng_code = """
        // Состояние генератора Xorshift128+ (2 x uint)
        typedef struct {
            ulong s0;
            ulong s1;
        } rng_state_t;

        // Инициализация состояния из seed и gid (используем SplitMix64)
        rng_state_t rng_init(uint seed, uint gid) {
            ulong x = ((ulong)seed << 32) | gid;  // простая комбинация для разнообразия
            // SplitMix64 для разогрева
            x = (x + 0x9e3779b97f4a7c15UL);
            ulong z = x;
            z = (z ^ (z >> 30)) * 0xbf58476d1ce4e5b9UL;
            z = (z ^ (z >> 27)) * 0x94d049bb133111ebUL;
            ulong s0 = z ^ (z >> 31);

            x = (x + 0x9e3779b97f4a7c15UL);
            z = x;
            z = (z ^ (z >> 30)) * 0xbf58476d1ce4e5b9UL;
            z = (z ^ (z >> 27)) * 0x94d049bb133111ebUL;
            ulong s1 = z ^ (z >> 31);

            rng_state_t state;
            state.s0 = s0;
            state.s1 = s1;
            return state;
        }

        // Xorshift128+ : возвращает случайное 64-битное число и обновляет состояние
        ulong rng_next(rng_state_t* state) {
            ulong s1 = state->s0;
            ulong s0 = state->s1;
            state->s0 = s0;
            s1 ^= s1 << 23;
            s1 ^= s1 >> 17;
            s1 ^= s0 ^ (s0 >> 26);
            state->s1 = s1;
            return s1 + s0;  // 64-битный результат
        }

        // Преобразование 64-битного целого в float в диапазоне [min, max]
        float rand_float(ulong rnd, float min_val, float max_val) {
            // Используем старшие 32 бита для равномерности
            uint rnd32 = (uint)(rnd >> 32);
            return fma((float)rnd32 / 4294967295.0f, (max_val - min_val), min_val);
        }
        """

        # Преобразование формулы (как и раньше)
        import re
        cl_formula = formula
        for i in range(var_count, 0, -1):
            cl_formula = re.sub(rf"\bx{i}\b", f"x[{i-1}]", cl_formula)
        replacements = {'abs(': 'fabs('}
        for py_func, cl_func in replacements.items():
            cl_formula = cl_formula.replace(py_func, cl_func)
        if mode == 'find_target':
            cl_formula = f'fabs(({cl_formula}) - ({target}))'

        # Генерация кода для переменных
        # В цикле по итерациям получаем новое состояние и генерируем все переменные
        var_loop = f"""
            for (int iter = 0; iter < N; iter++) {{
                float x[{max(var_count, 1)}];
                // Генерируем значения для всех переменных, используя одно состояние на итерацию
                // (для каждой переменной вызываем rng_next)
        """
        for i in range(var_count):
            var_loop += f"""
                ulong rnd_{i} = rng_next(&state);
                x[{i}] = rand_float(rnd_{i}, {range_min}.0f, {range_max}.0f);"""
        var_loop += f"""
                float val = {cl_formula};
                if (isnan(val) || isinf(val)) val = 1e18f;
                if (val < local_min) {{
                    local_min = val;
                    for (int j = 0; j < var_count; j++) local_best_x[j] = x[j];
                }}
            }}
        """

        # Полное ядро
        kernel = f"""
        {rng_code}
        #define MAX_VARS 20

        __kernel void find_optimal(__global float* results, __global float* best_x,
                                const int N, const int seed, const int var_count) {{
            if (var_count > MAX_VARS) return;
            
            int gid = get_global_id(0);
            // Инициализируем состояние генератора для этого потока
            rng_state_t state = rng_init(seed, gid);
            
            float local_min = 1e18f;
            float local_best_x[MAX_VARS];
            
            {var_loop}
            
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
        """Выполняет задачу на GPU или CPU"""
        
        try:
            if self.has_gpu:
                return self._execute_gpu(formula, var_count, mode, target, range_min, range_max, iterations, seed, thread_count)
            else:
                return self._execute_cpu(formula, var_count, mode, target, range_min, range_max, iterations, seed)
        except Exception as e:
            print(f"⚠️ Ошибка GPU, переключаюсь на CPU: {e}")
            return self._execute_cpu(formula, var_count, mode, target, range_min, range_max, iterations, seed)
    
    def _execute_gpu(self, formula: str, var_count: int, mode: str,
                    target: float, range_min: float, range_max: float,
                    iterations: int, seed: int, thread_count: int) -> dict:
        """GPU версия"""
        
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
    
    def _execute_cpu(self, formula: str, var_count: int, mode: str,
                    target: float, range_min: float, range_max: float,
                    iterations: int, seed: int) -> dict:
        """CPU симуляция Monte Carlo"""
        try:
            import random
            import math
            
            # Создаём переменные x1, x2, ...
            var_names = [f'x{i+1}' for i in range(var_count)]
            # Для простого x
            var_names.append('x')
            
            # Компиляция формулы в функцию
            safe_formula = formula.replace('^', '**')  # Python syntax
            
            def eval_formula(**kwargs):
                try:
                    return eval(safe_formula, {"__builtins__": {}}, {**math.__dict__, **kwargs})
                except Exception as e:
                    print(f"❌ Ошибка eval: {e}, formula: {safe_formula}, kwargs: {kwargs}")
                    return float('inf')
            
            random.seed(seed)
            start = time.time()
            print(f"🔄 Выполняю CPU задачу: {iterations} итераций")
            
            best_value = float('inf') if mode == 'minimize' else float('inf')
            best_x = [0.0] * var_count
            
            range_span = range_max - range_min
            
            for _ in range(iterations):
                # Генерируем случайную точку
                x_vals = [(random.random() * range_span + range_min) for _ in range(var_count)]
                
                # Создаём словарь переменных
                vars_dict = {f'x{i+1}': x_vals[i] for i in range(var_count)}
                vars_dict['x'] = x_vals[0]  # простое x = x1
                
                val = eval_formula(**vars_dict)
                
                if math.isnan(val) or math.isinf(val):
                    continue
                
                if mode == 'minimize':
                    if val < best_value:
                        best_value = val
                        best_x = x_vals[:]
                elif mode == 'find_target':
                    diff = abs(val - target)
                    if diff < best_value:
                        best_value = diff
                        best_x = x_vals[:]
            
            elapsed = time.time() - start
            print(f"✅ CPU задача выполнена: best_value={best_value}, iterations={iterations}, time={elapsed:.2f}s")
            
            return {
                "best_value": best_value,
                "best_x": best_x,
                "iterations": iterations,
                "time_spent": elapsed,
            }
        except Exception as e:
            print(f"❌ Ошибка в _execute_cpu: {e}")
            return None
    

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
    # Простое x также поддерживается для первой переменной
    if var_count > 0:
        allowed_names['x'] = sample_point[0]
    
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
        return False, f"Неизвестная переменная или функция: {var_name}. Используйте x (или x1-x{var_count}), sin, cos, sqrt..."
    except ZeroDivisionError:
        return True, "Предупреждение: возможное деление на ноль при некоторых значениях"
    except Exception as e:
        return False, f"Ошибка вычисления: {type(e).__name__}: {e}"