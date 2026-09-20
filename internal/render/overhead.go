package render

import "github.com/it1ro/cake/pkg/types"

// countWriter считает байты, ничего не сохраняя.
type countWriter struct{ n int }

func (c *countWriter) Write(p []byte) (int, error) {
	c.n += len(p)
	return len(p), nil
}

// perFileSlack — запас на переменную длину чисел в атрибутах
// (lines/bytes), которые при пустом содержимом равны "0".
// Реальная длина обычно 1–4 символа на атрибут, 2 атрибута,
// с запасом — 8.
const perFileSlack = 8

// Overhead возвращает размер обвязки формата в байтах для набора
// файлов: теги/заголовки, дерево, CDATA-маркеры, заголовки
// markdown — всё, кроме самого содержимого.
//
// Считается прогоном настоящего рендера с пустым содержимым,
// поэтому не расходится с шаблонами: правка рендера автоматически
// попадает в оценку. perFileSlack компенсирует, что при пустом
// содержимом lines="0" и bytes="0" короче реальных значений.
func Overhead(format Format, project string, files []types.FileEntry) int {
	pf := make([]types.ProcessedFile, len(files))
	for i, e := range files {
		pf[i] = types.ProcessedFile{Entry: e}
	}
	var cw countWriter
	_ = Render(types.Context{Project: project, Root: project, Files: pf}, format, &cw)
	return cw.n + perFileSlack*len(files)
}
