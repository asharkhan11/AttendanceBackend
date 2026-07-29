@echo off
echo Generating Swagger/OpenAPI documentation...
docker run --rm -v "%CD%:/app" -w /app golang:1.22-alpine go run github.com/swaggo/swag/cmd/swag@latest init -g cmd/api/main.go
echo.
echo Rebuilding and restarting backend services...
docker compose -f deployments/docker-compose.yml up --build -d
echo.
echo Resetting attendance sessions finalization and clearing whatsapp logs in DB...
docker compose -f deployments/docker-compose.yml exec postgres psql -U attendance_owner -d attendance -c "UPDATE attendance_sessions SET is_finalized = false, finalized_at = null; DELETE FROM whatsapp_logs;"
echo.
echo Done! Backend has been reloaded and reset.
pause
