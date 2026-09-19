// Package tokens оценивает количество токенов в тексте.
//
// v0.1 — быстрая эвристика len/4: подходит для латиницы, кода и
// markdown. Для CJK и эмодзи систематически занижает — приемлемо
// на этом этапе. v0.2 подключит tiktoken (WASM, без CGo).
package tokens

// Estimate возвращает приблизительное число токенов в src.
func Estimate(src []byte) int {
	return EstimateSize(int64(len(src)))
}

// EstimateSize возвращает приблизительное число токенов
// для содержимого размером n байт.
//
// Используется в TUI: до фактического чтения файлов у нас есть
// только FileEntry.Size. Для clean-режима это оценка сверху —
// комментарии будут удалены, содержимое уменьшится.
func EstimateSize(n int64) int {
	if n <= 0 {
		return 0
	}
	return (int(n) + 3) / 4
}
