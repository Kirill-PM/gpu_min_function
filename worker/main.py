import os
import sys
import time
import uuid
import requests
from flask import Flask, request, jsonify
from opencl_kernel import OpenCLWorker, validate_formula_cpu

app = Flask(__name__)
worker = OpenCLWorker()
WORKER_ID = str(uuid.uuid4())[:8]
MASTER_URL = os.getenv("MASTER_URL", "http://master:8080")

@app.route('/health', methods=['GET'])
def health():
    return jsonify({"status": "ok", "worker_id": WORKER_ID})

@app.route('/info', methods=['GET'])
def info():
    gpu_info = worker.get_gpu_info()
    return jsonify({
        "id": WORKER_ID,
        "gpu_name": gpu_info["gpu_name"],
        "thread_count": gpu_info["thread_count"],
        "max_memory": gpu_info["max_memory"],
    })


@app.route('/task', methods=['POST'])
def receive_task():
    task = request.json
    
    # Валидация на CPU
    is_valid, message = validate_formula_cpu(
        formula=task["formula"],
        var_count=task["variable_count"],
        sample_point=[(task["range_min"] + task["range_max"]) / 2] * task["variable_count"]
    )
    
    if not is_valid:
        return jsonify({
            "success": False, 
            "error": f"Валидация формулы: {message}"
        }), 400
    
    # Если предупреждение — логируем, но продолжаем
    if "Предупреждение" in message:
        print(f"⚠️ {message}")
    
    # ... дальше выполнение задачи как раньше
    try:
        result = worker.execute_task(
            formula=task["formula"],
            var_count=task["variable_count"],
            mode=task["mode"],
            target=task.get("target", 0),
            range_min=task["range_min"],
            range_max=task["range_max"],
            iterations=task["iterations"],
            seed=task["seed"],
            thread_count=task["thread_count"],
        )
        
        # Отправляем результат мастеру
        result["task_id"] = task["id"]
        result["worker_id"] = WORKER_ID
        
        requests.post(f"{MASTER_URL}/api/task/result", json=result, timeout=5)
        
        return jsonify({"success": True, "task_id": task["id"]})
    
    except Exception as e:
        return jsonify({"success": False, "error": str(e)}), 500

def register_with_master():
    """Регистрируется у мастера при старте"""
    gpu_info = worker.get_gpu_info()
    payload = {
        "id": WORKER_ID,
        "gpu_name": gpu_info["gpu_name"],
        "thread_count": gpu_info["thread_count"],
    }
    
    max_retries = 10
    for i in range(max_retries):
        try:
            resp = requests.post(f"{MASTER_URL}/api/worker/register", 
                               json=payload, timeout=5)
            if resp.status_code == 200:
                print(f"✅ Зарегистрирован у мастера: {WORKER_ID}")
                return
        except Exception as e:
            print(f"⏳ Попытка {i+1}/{max_retries}: {e}")
            time.sleep(2)
    
    print("❌ Не удалось зарегистрироваться у мастера")

if __name__ == '__main__':
    print(f"🔧 Инициализация воркера {WORKER_ID}...")
    print(f"📊 GPU: {worker.get_gpu_info()['gpu_name']}")
    
    register_with_master()
    
    app.run(host='0.0.0.0', port=5000)