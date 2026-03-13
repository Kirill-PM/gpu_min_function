package opencl

import (
	"fmt"
	"regexp"
	"strings"
)

// Преобразует формулу вида "x1 + cos(x2) * (x3 + 54)" в OpenCL-код
func GenerateKernel(formula string, variableCount int, mode string, target float64) (string, error) {
	// Заменяем x1, x2, ... на x[0], x[1], ...
	re := regexp.MustCompile(`x(\d+)`)
	clFormula := re.ReplaceAllStringFunc(formula, func(match string) string {
		var num int
		fmt.Sscanf(match, "x%d", &num)
		if num < 1 || num > variableCount {
			return match // оставляем как есть, если выходит за пределы
		}
		return fmt.Sprintf("x[%d]", num-1)
	})

	// Заменяем функции на OpenCL-версии
	replacements := map[string]string{
		"sin":  "sin",
		"cos":  "cos",
		"tan":  "tan",
		"sqrt": "sqrt",
		"exp":  "exp",
		"log":  "log",
		"abs":  "fabs",
		"pow":  "pow",
	}

	for mathFunc, clFunc := range replacements {
		clFormula = strings.ReplaceAll(clFormula, mathFunc+"(", clFunc+"(")
	}

	// Для режима find_target оборачиваем в fabs(val - target)
	if mode == "find_target" {
		clFormula = fmt.Sprintf("fabs((%s) - %ff)", clFormula, target)
	}

	// Генерируем массив переменных
	varDecl := ""
	for i := 0; i < variableCount; i++ {
		varDecl += fmt.Sprintf("        state = (state * 1103515245 + 12345) & 0x7FFFFFFF;\n")
		varDecl += fmt.Sprintf("        float x[%d] = ((float)state / 2147483647.0f) * (%ff - (%ff)) + (%ff);\n",
			variableCount, 200.0, -100.0, -100.0)
	}

	// Упрощённая генерация для переменных
	varGen := ""
	for i := 0; i < variableCount; i++ {
		varGen += fmt.Sprintf("        state = (state * 1103515245 + 12345) & 0x7FFFFFFF;\n")
		varGen += fmt.Sprintf("        float x%d = ((float)state / 2147483647.0f) * 200.0f - 100.0f;\n", i+1)
	}

	// Снова заменяем x1, x2 на сгенерированные переменные
	clFormula = re.ReplaceAllStringFunc(formula, func(match string) string {
		return match // оставляем x1, x2 как есть, они будут объявлены выше
	})

	// Для режима find_target
	if mode == "find_target" {
		clFormula = fmt.Sprintf("fabs((%s) - %ff)", clFormula, target)
	}

	// Заменяем функции
	for mathFunc, clFunc := range replacements {
		clFormula = strings.ReplaceAll(clFormula, mathFunc+"(", clFunc+"(")
	}

	kernel := fmt.Sprintf(`
__kernel void find_optimal(__global float* results, __global float* best_x, const int N, const int seed, const int var_count) {
    int gid = get_global_id(0);
    uint state = seed + gid;
    float local_min = 1e18f;
    float local_best_x[%d];

    for (int i = 0; i < N; i++) {
%s
        float val = %s;
        
        // Обработка NaN и inf
        if (isnan(val) || isinf(val)) {
            val = 1e18f;
        }
        
        if (val < local_min) {
            local_min = val;
%s
        }
    }
    results[gid] = local_min;
%s
}
`, variableCount, varGen, clFormula, generateXCopy(variableCount), generateXStore(variableCount))

	return kernel, nil
}

func generateXCopy(varCount int) string {
	var sb strings.Builder
	for i := 0; i < varCount; i++ {
		sb.WriteString(fmt.Sprintf("            local_best_x[%d] = x%d;\n", i, i+1))
	}
	return sb.String()
}

func generateXStore(varCount int) string {
	var sb strings.Builder
	for i := 0; i < varCount; i++ {
		sb.WriteString(fmt.Sprintf("    best_x[gid * %d + %d] = local_best_x[%d];\n", varCount, i, i))
	}
	return sb.String()
}

// Более простая версия для начала
func GenerateKernelSimple(formula string, variableCount int, mode string, target float64, rangeMin, rangeMax float64) string {
	// Заменяем x1, x2, ... на x[0], x[1], ...
	re := regexp.MustCompile(`x(\d+)`)
	clFormula := re.ReplaceAllStringFunc(formula, func(match string) string {
		var num int
		fmt.Sscanf(match, "x%d", &num)
		if num < 1 || num > variableCount {
			return match
		}
		return fmt.Sprintf("x[%d]", num-1)
	})

	// Заменяем функции
	replacements := map[string]string{
		"sin": "sin", "cos": "cos", "tan": "tan",
		"sqrt": "sqrt", "exp": "exp", "log": "log",
		"abs": "fabs", "pow": "pow",
	}
	for mathFunc, clFunc := range replacements {
		clFormula = strings.ReplaceAll(clFormula, mathFunc+"(", clFunc+"(")
	}

	// Для режима find_target
	if mode == "find_target" {
		clFormula = fmt.Sprintf("fabs((%s) - %ff)", clFormula, target)
	}

	// Генерация переменных
	varGen := ""
	rangeSpan := rangeMax - rangeMin
	for i := 0; i < variableCount; i++ {
		varGen += fmt.Sprintf("        state = (state * 1103515245 + 12345) & 0x7FFFFFFF;\n")
		varGen += fmt.Sprintf("        float x[%d] = ((float)state / 2147483647.0f) * %ff + %ff;\n",
			i, rangeSpan, rangeMin)
	}

	kernel := fmt.Sprintf(`
__kernel void find_optimal(__global float* results, __global float* best_x, const int N, const int seed, const int var_count) {
    int gid = get_global_id(0);
    uint state = seed + gid;
    float local_min = 1e18f;
    float local_best_x[10]; // максимум 10 переменных

    for (int i = 0; i < N; i++) {
%s
        float val = %s;
        
        if (isnan(val) || isinf(val)) {
            val = 1e18f;
        }
        
        if (val < local_min) {
            local_min = val;
            for (int j = 0; j < var_count; j++) {
                local_best_x[j] = x[j];
            }
        }
    }
    results[gid] = local_min;
    for (int j = 0; j < var_count; j++) {
        best_x[gid * var_count + j] = local_best_x[j];
    }
}
`, varGen, clFormula)

	return kernel
}
