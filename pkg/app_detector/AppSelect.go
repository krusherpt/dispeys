package appdetector

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// FocusOrRun: program - имя бинарника, args - аргументы при запуске (если потребуется запустить).
// Возвращает ошибку в случае проблем.
func FocusOrRun(program string, args ...string) error {
	// 1) Найти PID'ы процесса
	pids, err := getPIDs(program)
	if err != nil {
		return fmt.Errorf("getPIDs: %w", err)
	}

	// 2) Получить список окон и отфильтровать по PID'ам
	winIDs, err := windowsForPIDs(pids)
	if err != nil {
		return fmt.Errorf("windowsForPIDs: %w", err)
	}

	// 3) Если окон нет — запустить приложение
	if len(winIDs) == 0 {
		// Попробуем запустить программу
		if err := startProgram(program, args...); err != nil {
			return fmt.Errorf("startProgram: %w", err)
		}
		return nil
	}

	// 4) Получить активное окно
	active, err := getActiveWindow()
	if err != nil {
		return fmt.Errorf("getActiveWindow: %w", err)
	}

	// 5) Решить что активировать
	var toActivate string
	if active == "" {
		// никакое окно не определено — активируем первое
		toActivate = winIDs[0]
	} else {
		// проверить, входит ли active в список winIDs
		idx := indexOf(winIDs, active)
		if idx == -1 {
			// активное окно не из нашего списка — активируем первое
			toActivate = winIDs[0]
		} else {
			// активируем следующее (wrap)
			next := (idx + 1) % len(winIDs)
			toActivate = winIDs[next]
		}
	}

	// 6) Активируем окно
	if err := activateWindow(toActivate); err != nil {
		return fmt.Errorf("activateWindow: %w", err)
	}

	return nil
}

func getPIDs(program string) ([]string, error) {
	// Используем pgrep -x program
	cmd := exec.Command("pgrep", "-x", program)
	out, err := cmd.Output()
	if err != nil {
		// pgrep возвращает ненулевой код если ничего не найдено.
		// Тогда считаем что PID'ов нет — вернуть пустой с nil ошибкой.
		if exitErr, ok := err.(*exec.ExitError); ok {
			if len(exitErr.Stderr) == 0 {
				// отсутствие совпадений
				return nil, nil
			}
		}
		// прочая ошибка
		return nil, err
	}
	var pids []string
	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			pids = append(pids, line)
		}
	}
	return pids, nil
}

func windowsForPIDs(pids []string) ([]string, error) {
	// Если pids == nil или пусто — сразу вернуть пустой
	if len(pids) == 0 {
		return nil, nil
	}
	// wmctrl -lp выводит строки: <winid> <desktop> <pid> <host> <title...>
	cmd := exec.Command("wmctrl", "-lp")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	pidSet := make(map[string]struct{})
	for _, p := range pids {
		pidSet[p] = struct{}{}
	}

	var wins []string
	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		winid := fields[0]
		pid := fields[2]
		if _, ok := pidSet[pid]; ok {
			// keep winid as-is (wmctrl uses hex like 0x03e00007)
			wins = append(wins, winid)
		}
	}
	return wins, nil
}

func getActiveWindow() (string, error) {
	// Используем xdotool getactivewindow (возвращает decimal id обычно)
	// Но wmctrl/xdotool работают и с hex вида 0x...; xdotool возвращает DECIMAL window id.
	// Чтобы сравнить, приведём оба к hex с префиксом 0x.
	cmd := exec.Command("xdotool", "getactivewindow")
	out, err := cmd.Output()
	if err != nil {
		// если ошибка или нет активного окна — вернуть пустую строку без ошибки
		if exitErr, ok := err.(*exec.ExitError); ok {
			if len(exitErr.Stderr) == 0 {
				return "", nil
			}
		}
		return "", err
	}
	s := strings.TrimSpace(string(out))
	if s == "" {
		return "", nil
	}
	// s — десятичный id; нужно перевести в hex 0x...
	var idNum uint64
	_, err = fmt.Sscanf(s, "%d", &idNum)
	if err != nil {
		// Возможно xdotool вернул hex уже — попробуем использовать как есть
		// Вернем s напрямую (позже indexOf будет сравнивать строки; но wmctrl возвращает hex)
		return toHexWindowID(s)
	}
	return fmt.Sprintf("0x%x", idNum), nil
}

func toHexWindowID(s string) (string, error) {
	// если s уже начинается с 0x — вернуть как есть
	if strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0X") {
		return strings.ToLower(s), nil
	}
	// если десятичное в строке — попробовать парсить
	var idNum uint64
	_, err := fmt.Sscanf(s, "%d", &idNum)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("0x%x", idNum), nil
}

func activateWindow(winid string) error {
	if winid == "" {
		return errors.New("empty window id")
	}
	// wmctrl -ia <winid>
	cmd := exec.Command("wmctrl", "-ia", winid)
	if err := cmd.Run(); err != nil {
		// Попробуем xdotool windowactivate <winid> (иногда принимает decimal)
		alt := exec.Command("xdotool", "windowactivate", winid)
		if err2 := alt.Run(); err2 != nil {
			return fmt.Errorf("wmctrl failed: %v; xdotool also failed: %v", err, err2)
		}
	}
	return nil
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

func indexOf(slice []string, val string) int {
	for i, s := range slice {
		// сравниваем в нижнем регистре, т.к. winid может быть 0x... vs 0X...
		if strings.EqualFold(s, val) {
			return i
		}
	}
	return -1
}