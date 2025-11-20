@echo off
echo Adding all changes to Git...
git add .

set /p commitMessage="Enter commit message: "

echo Committing with message: "%commitMessage%"
git commit -m "%commitMessage%"

echo Pushing to origin...
git push

echo.
echo Push complete.
pause
