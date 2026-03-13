.PHONY: build up down logs dev

build:
	docker-compose build

up:
	docker-compose up -d

down:
	docker-compose down

logs:
	docker-compose logs -f

dev:
	# Запуск без Docker для разработки
	cd master && go run main.go &
	cd worker && python main.py &
	cd frontend && npm run dev