.PHONY: setup-backend setup-frontend setup-env run-app test-backend test-integration test-all

setup-backend:
	@command -v uv >/dev/null 2>&1 || (curl -LsSf https://astral.sh/uv/install.sh | sh)
	cd backend && uv venv --python 3.11
	cd backend && uv pip install --python .venv -e ".[dev]"

setup-frontend:
	cd frontend && npm install

setup-env: setup-backend setup-frontend
	@test -f config.env || cp config.env.template config.env

run-app:
	docker compose up

test-backend:
	cd backend && PYTHONPATH=. uv run pytest src/tests/

test-integration:
	@echo "Testing backend API..."
	@curl -sf http://localhost/api/v1/hello | grep -q "hello" && echo "✓ Backend API working" || (echo "✗ Backend API failed" && exit 1)
	@echo "Testing inference API..."
	@curl -sf http://localhost/api/v1/inference/current | grep -q "epoch_id" && echo "✓ Inference API working" || (echo "✗ Inference API failed" && exit 1)
	@echo "Testing frontend..."
	@curl -sf http://localhost/ | grep -q "<!doctype html>" && echo "✓ Frontend working" || (echo "✗ Frontend failed" && exit 1)
	@echo "\nAll integration tests passed"

test-all: test-backend test-integration

