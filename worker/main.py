import os
import sys
import time
import uuid
import requests
import threading
from flask import Flask, request, jsonify
from opencl_kernel import OpenCLWorker, validate_formula_cpu
from queue import Queue, Full, Empty

app = Flask(__name__)
worker = OpenCLWorker()
WORKER_ID = str(uuid.uuid4())[:8]
MASTER_URL = os.getenv("MASTER_URL", "http://localhost:8080")

# Текущий batch_id (в рамках одного запуска). Если приходит задача с другим batch_id —
# сбрасываем очередь, чтобы не выполнять "старые" задания.
current_batch_id = None

# Очередь задач (максимум 20) для последовательной обработки на GPU
task_queue = Queue(maxsize=20)


def clear_task_queue():
    while True:
        try:
            task_queue.get_nowait()
            task_queue.task_done()
        except Empty:
            break

def execute_with_timeout(func, timeout_sec):
    """Выполняет функцию с таймаутом"""
    result = [None]
    exception = [None]
    
    def worker():
        try:
            result[0] = func()
        except Exception as e:
            exception[0] = e
    
    thread = threading.Thread(target=worker)
    thread.start()
    thread.join(timeout_sec)
    
    if thread.is_alive():
        print(f"⏰ Таймаут выполнения задачи ({timeout_sec}s)")
        return None
    
    if exception[0]:
        print(f"❌ Исключение в задаче: {exception[0]}")
        return None
    
    return result[0]

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
    packet_id = task.get('packet_id')
    print(f"📨 Получена задача: {task['id']} (packet_id={packet_id})")

    # Если пришла задача из другой сессии (batch_id), сбрасываем очередь, чтобы не выполнять старые задачи.
    batch_id = task.get("batch_id")
    global current_batch_id
    if current_batch_id is None:
        current_batch_id = batch_id
    elif batch_id != current_batch_id:
        print(f"🔄 Новая сессия batch_id={batch_id}, сбрасываю очередь")
        current_batch_id = batch_id
        clear_task_queue()

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
        print(f"{message}")

    # Пытаемся поставить задачу в очередь (не более 20 задач).
    try:
        task_queue.put_nowait(task)
    except Full:
        return jsonify({"success": False, "error": "task queue full"}), 429

    return jsonify({"success": True, "task_id": task["id"]})


@app.route('/stop', methods=['POST'])
def stop():
    """Очищаем очередь задач (после завершения/сброса вычислений)."""
    global current_batch_id
    current_batch_id = None
    clear_task_queue()
    return jsonify({"success": True})


def worker_loop():
    while True:
        task = task_queue.get()
        if task is None:
            break

        result = execute_with_timeout(lambda: worker.execute_task(
            formula=task["formula"],
            var_count=task["variable_count"],
            mode=task["mode"],
            target=task.get("target", 0),
            range_min=task["range_min"],
            range_max=task["range_max"],
            iterations=task["iterations"],
            seed=task["seed"],
            thread_count=task["thread_count"],
        ), 5)

        if result is None:
            print("❌ Задача не выполнена (таймаут или ошибка)")
            task_queue.task_done()
            continue

        result["task_id"] = task["id"]
        result["packet_id"] = task.get("packet_id")
        result["worker_id"] = WORKER_ID

        try:
            resp = requests.post(f"{MASTER_URL}/api/task/result", json=result, timeout=5)
            if resp.status_code != 200:
                print(f"❌ Ошибка отправки результата: {resp.status_code}")
        except Exception as e:
            print(f"❌ Ошибка отправки результата мастеру: {e}")

        task_queue.task_done()

def register_with_master():
    """Регистрируется у мастера при старте"""
    gpu_info = worker.get_gpu_info()
    payload = {
        "id": WORKER_ID,
        "gpu_name": gpu_info["gpu_name"],
        "thread_count": gpu_info["thread_count"],
        # Мастер будет вызывать /task по этому адресу
        "address": os.getenv("WORKER_ADDRESS", "host.docker.internal:5000"),
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
    
    # Запускаем фоновый поток, который последовательно обрабатывает задачи из очереди
    threading.Thread(target=worker_loop, daemon=True).start()

    app.run(host='0.0.0.0', port=5000, threaded=True)