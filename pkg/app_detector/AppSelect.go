package appdetector

import (
	"os/exec"
)

// FocusOrRun: program - имя бинарника, args - аргументы при запуске (если потребуется запустить).
// Возвращает ошибку в случае проблем.
func FocusOrRun(program string, args ...string) error {
	// Всегда запускаем новый экземпляр
	return startProgram(program, args...)
}

func startProgram(program string, args ...string) error {
	cmd := exec.Command(program, args...)
	// Отключаем наследование ввода/вывода (демонизируем)
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil
	// Запустить асинхронно
	if err := cmd.Start(); err != nil {
		return err
	}
	return nil
}
