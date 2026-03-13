FROM python:3.11-slim

# Устанавливаем OpenCL для Windows/Linux
RUN apt-get update && apt-get install -y \
    ocl-icd-opencl-dev \
    opencl-headers \
    clinfo \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

COPY . .

EXPOSE 5000
ENV MASTER_URL=http://master:8080
CMD ["python", "main.py"]