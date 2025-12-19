package utils

import (
	"fmt"
	"strings"
	"time"
)

func NormalizeToCron(input string) (string, error) {
	input = strings.TrimSpace(input)

	// Формат hh:mm
	if t, err := time.Parse("15:04", input); err == nil {
		return fmt.Sprintf("%d %d * * *", t.Minute(), t.Hour()), nil
	}

	// Простейшая проверка cron: 5 полей
	parts := strings.Fields(input)
	if len(parts) != 5 {
		return "", fmt.Errorf("Неверный формат cron")
	}
	return input, nil
}
