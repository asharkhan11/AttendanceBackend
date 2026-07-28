@echo off
echo Rebuilding and restarting backend services...
docker compose -f deployments/docker-compose.yml up --build -d
echo.
echo Resetting attendance sessions finalization and clearing whatsapp logs in DB...
docker compose -f deployments/docker-compose.yml exec postgres psql -U attendance_owner -d attendance -c "UPDATE attendance_sessions SET is_finalized = false, finalized_at = null; DELETE FROM whatsapp_logs;"
echo.
echo Done! Backend has been reloaded and reset.
pause
