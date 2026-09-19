package types

// FileEntry — файл, найденный walker'ом.
type FileEntry struct {
	Path     string // относительный путь от корня
	AbsPath  string
	Size     int64
	Language string
}

// ProcessedFile — файл после processor'а, готов к рендеру.
type ProcessedFile struct {
	Entry   FileEntry
	Content []byte
	Lines   int
}

// Context — всё, что уходит в render.
type Context struct {
	Project string
	Root    string
	Files   []ProcessedFile
	Tokens  int
}
