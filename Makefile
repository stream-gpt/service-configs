.PHONY: help generate run test build clean install-tools docker-build docker-run lint

# Default target
help: ## Показать справку по командам
	@echo "Доступные команды:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

# Variables
SERVICE_NAME := service-configs
DOCKER_IMAGE := $(SERVICE_NAME)
DOCKER_TAG := latest
PORT := 8080

# Tools versions
OAPI_CODEGEN_VERSION := v2.1.0
GOLANGCI_LINT_VERSION := v1.55.2
SWAG_VERSION := v1.16.2

generate: ## Генерировать код из OpenAPI спецификации и Go generate
	@echo "🔧 Генерация кода..."
	@$(MAKE) generate-openapi
	@$(MAKE) generate-go
	@echo "✅ Генерация завершена"

generate-openapi: ## Генерировать код из OpenAPI спецификации
	@echo "📋 Генерация из OpenAPI спецификации..."
	@mkdir -p internal/generated/server/api internal/generated/clients internal/api

	@oapi-codegen -generate types      -package api -o internal/generated/server/api/types.go api/*.yaml
	@oapi-codegen -generate chi-server -package api -o internal/generated/server/api/handlers.go api/schema.yaml
	@oapi-codegen -generate chi-server -package api -o internal/generated/server/api/server.go -templates=api/templates api/schema.yaml
	
	@echo "🔧 Генерация клиентов..."
	@if [ -d "api/clients" ]; then \
		file_count=$$(find api/clients -name "*.yaml" -o -name "*.yml" 2>/dev/null | wc -l); \
		if [ "$$file_count" -gt 0 ]; then \
			for file in api/clients/*.yaml api/clients/*.yml; do \
				if [ -f "$$file" ]; then \
					filename=$$(basename "$$file" .yaml); \
					filename=$$(basename "$$filename" .yml); \
					package_name=$$(echo "$$filename" | tr '[:upper:]' '[:lower:]' | sed 's/[ -]/_/g'); \
					echo "📦 Генерация клиента для $$file в пакет $$package_name..."; \
					mkdir -p "internal/generated/clients/$$package_name"; \
					oapi-codegen -generate types  -package $$package_name -o "internal/generated/clients/$$package_name/types.go" "$$file"; \
					oapi-codegen -generate client -package $$package_name -o "internal/generated/clients/$$package_name/client.go" "$$file"; \
				fi; \
			done; \
		else \
			echo "⚠️  Директория api/clients пуста, пропускаем генерацию клиентов"; \
		fi; \
	else \
		echo "⚠️  Директория api/clients не найдена, пропускаем генерацию клиентов"; \
	fi
	
	@echo "✅ Генерация из OpenAPI завершена"

generate-go: ## Запустить go generate
	@echo "🔧 Запуск go generate..."
	@go generate ./...
	@echo "✅ go generate завершен"

install-tools: ## Установить необходимые инструменты
	@echo "🔧 Установка инструментов..."
	@go install github.com/deepmap/oapi-codegen/v2/cmd/oapi-codegen@$(OAPI_CODEGEN_VERSION)
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)
	@go install github.com/swaggo/swag/cmd/swag@$(SWAG_VERSION)
	@echo "✅ Инструменты установлены"

run: ## Запустить сервис
	@echo "🚀 Запуск сервиса..."
	@go run cmd/main.go

run-jq:
	@go run cmd/main.go | jq .

run-dev: ## Запустить сервис в режиме разработки с auto-reload
	@echo "🔄 Запуск в режиме разработки..."
	@if command -v air > /dev/null; then \
		air; \
	else \
		echo "⚠️  Air не установлен. Установите: go install github.com/cosmtrek/air@latest"; \
		$(MAKE) run; \
	fi

test: ## Запустить тесты
	@echo "🧪 Запуск тестов..."
	@go test -v -race -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out -o coverage.html
	@echo "✅ Тесты завершены. Отчет о покрытии: coverage.html"

test-short: ## Запустить быстрые тесты
	@echo "⚡ Запуск быстрых тестов..."
	@go test -short -v ./...

benchmark: ## Запустить бенчмарки
	@echo "📊 Запуск бенчмарков..."
	@go test -bench=. -benchmem ./...

build: ## Собрать бинарный файл
	@echo "🔨 Сборка бинарного файла..."
	@CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags '-extldflags "-static"' -o bin/$(SERVICE_NAME) ./cmd/
	@echo "✅ Сборка завершена: bin/$(SERVICE_NAME)"

build-local: ## Собрать бинарный файл для текущей ОС
	@echo "🔨 Сборка для текущей ОС..."
	@go build -o bin/$(SERVICE_NAME) ./cmd/
	@echo "✅ Сборка завершена: bin/$(SERVICE_NAME)"

fmt: ## Форматировать код
	@echo "💅 Форматирование кода..."
	@go fmt ./...
	@goimports -w .
	@echo "✅ Форматирование завершено"

mod-tidy: ## Очистить зависимости
	@echo "📦 Очистка зависимостей..."
	@go mod tidy
	@go mod verify
	@echo "✅ Зависимости обновлены"

docker-build: ## Собрать Docker образ
	@echo "🐳 Сборка Docker образа..."
	@docker build -t $(DOCKER_IMAGE):$(DOCKER_TAG) .
	@echo "✅ Docker образ собран: $(DOCKER_IMAGE):$(DOCKER_TAG)"

docker-run: ## Запустить Docker контейнер
	@echo "🐳 Запуск Docker контейнера..."
	@docker run -p $(PORT):$(PORT) --env-file .env --rm --name $(SERVICE_NAME) $(DOCKER_IMAGE):$(DOCKER_TAG)

docker-stop: ## Остановить Docker контейнер
	@echo "🛑 Остановка Docker контейнера..."
	@docker stop $(SERVICE_NAME) || true

docker-clean: ## Удалить Docker образы
	@echo "🧹 Очистка Docker образов..."
	@docker rmi $(DOCKER_IMAGE):$(DOCKER_TAG) || true
	@docker system prune -f

dev-setup: install-tools mod-tidy generate ## Полная настройка среды разработки
	@echo "🎉 Среда разработки готова!"

ci: lint test build ## CI pipeline: линтинг, тесты, сборка
	@echo "✅ CI pipeline завершен успешно"

# Проверка здоровья сервиса
health-check: ## Проверить здоровье запущенного сервиса
	@echo "🔍 Проверка здоровья сервиса..."
	@curl -f http://localhost:$(PORT)/health || echo "❌ Сервис недоступен"

# Мониторинг метрик
metrics: ## Показать метрики Prometheus
	@echo "📊 Метрики Prometheus:"
	@curl -s http://localhost:$(PORT)/metrics

# Отображение версии и информации
version: ## Показать версию Go и информацию о проекте
	@echo "📋 Информация о проекте:"
	@echo "Сервис: $(SERVICE_NAME)"
	@echo "Go версия: $$(go version)"
	@echo "Git коммит: $$(git rev-parse --short HEAD 2>/dev/null || echo 'unknown')"
	@echo "Дата сборки: $$(date)"

# Запуск всех проверок перед коммитом
pre-commit: fmt lint test ## Запустить все проверки перед коммитом
	@echo "✅ Все проверки прошли успешно. Готово к коммиту!"

# Создание release
release: clean generate test build docker-build ## Создать release
	@echo "🚀 Release готов!"
