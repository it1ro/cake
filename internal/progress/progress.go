// Package progress рисует однострочный индикатор прогресса
// в stderr. Только для TTY: в пайпах и CI молчит, чтобы не
// засорять логи и не ломать парсинг stderr.
package progress

import (
	"fmt"
	"io"
	"strings"
	"time"
)

// MinTotal — ниже этого числа файлов индикатор не показывается:
// для маленьких проектов прогресс мигает и мешает.
const MinTotal = 200

// redrawInterval — как часто перерисовывать. 100 ms — компромисс:
// быстрее мигает, медленнее «залипает» на медленных дисках.
const redrawInterval = 100 * time.Millisecond

// padWidth — ширина строки прогресса. Всегда дополняем пробелами
// до этой ширины: проще, чем ANSI \033[K (который сломался бы,
// если writer не терминал — например, в тестах с bytes.Buffer).
// Хватает на любые разумные счётчики: 8-значное число + 8-значное
// число + « файлов (100%)» ≈ 31 байт.
const padWidth = 60

// Bar — счётчик с однострочным индикатором.
// Zero value безопасен и работает как no-op (w == nil).
type Bar struct {
	total    int
	current  int
	w        io.Writer // nil → no-op
	lastDraw time.Time
	finished bool
}

// New создаёт бар. w == nil или total < MinTotal → no-op.
func New(total int, w io.Writer) *Bar {
	if w == nil || total < MinTotal {
		return &Bar{}
	}
	return &Bar{total: total, w: w}
}

// Inc продвигает счётчик. Перерисовывает не чаще, чем раз в
// redrawInterval — кроме последнего шага, там рисуем всегда,
// чтобы пользователь увидел финальные 100%.
func (b *Bar) Inc() {
	if b.w == nil {
		return
	}
	b.current++
	now := time.Now()
	if b.current < b.total && now.Sub(b.lastDraw) < redrawInterval {
		return
	}
	b.lastDraw = now
	b.draw()
}

// Finish затирает строку прогресса. Идемпотентен: повторный вызов
// — no-op. Нужно, чтобы `defer prog.Finish()` не портил вывод
// после явного Finish перед summary.
func (b *Bar) Finish() {
	if b.w == nil || b.finished {
		return
	}
	b.finished = true
	fmt.Fprintf(b.w, "\r%s\r", strings.Repeat(" ", padWidth))
}

func (b *Bar) draw() {
	pct := b.current * 100 / b.total
	line := fmt.Sprintf("  %d/%d файлов (%d%%)", b.current, b.total, pct)
	if n := padWidth - len(line); n > 0 {
		line += strings.Repeat(" ", n)
	}
	fmt.Fprintf(b.w, "\r%s", line)
}
