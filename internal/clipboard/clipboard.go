// Package clipboard копирует данные в системный буфер обмена
// через OSC 52 — escape-последовательность, которую понимают
// iTerm2, kitty, WezTerm, alacritty, tmux (с set-clipboard on),
// Windows Terminal. Не требует cgo и не ломает статическую сборку.
package clipboard

import (
	"encoding/base64"
	"fmt"
	"io"
	"os"
)

// Encode возвращает OSC 52-последовательность для data.
// Экспортируется для тестов и для случаев, когда вызывающая
// сторона сама управляет терминалом.
func Encode(data []byte) string {
	return "\x1b]52;c;" + base64.StdEncoding.EncodeToString(data) + "\x07"
}

// Copy пишет OSC 52 в w.
func Copy(w io.Writer, data []byte) error {
	_, err := io.WriteString(w, Encode(data))
	return err
}

// CopyToTTY открывает управляющий терминал и пишет OSC 52 туда.
//
// Именно терминал, а не stdout: stdout может быть перенаправлен
// в файл (`cake dump > out.xml`), и escape-последовательность
// попадёт в сам XML — это сломает и файл, и копирование.
func CopyToTTY(data []byte) error {
	f, err := os.OpenFile(ttyPath(), os.O_WRONLY, 0)
	if err != nil {
		return fmt.Errorf("open terminal: %w", err)
	}
	defer f.Close()
	return Copy(f, data)
}
