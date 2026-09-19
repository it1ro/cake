// Package render форматирует Context в выходной формат.
package render

import (
	"fmt"
	"io"
	"strings"

	"github.com/it1ro/cake/pkg/types"
)

// Format — формат вывода. Строковый тип, чтобы легко парсить
// из CLI-флага и печатать в help.
type Format string

const (
	FormatXML      Format = "xml"
	FormatMarkdown Format = "markdown"
	FormatPlain    Format = "plain"
)

// Parse разбирает пользовательскую строку формата.
func Parse(s string) (Format, error) {
	switch Format(strings.ToLower(strings.TrimSpace(s))) {
	case FormatXML:
		return FormatXML, nil
	case FormatMarkdown:
		return FormatMarkdown, nil
	case FormatPlain:
		return FormatPlain, nil
	}
	return "", fmt.Errorf("unknown format %q (want: xml, markdown, plain)", s)
}

// Render — диспетчер. Пустой format трактуется как XML,
// чтобы zero value Options давал осмысленный результат.
func Render(ctx types.Context, format Format, w io.Writer) error {
	if format == "" {
		format = FormatXML
	}
	switch format {
	case FormatXML:
		return XML(ctx, w)
	case FormatMarkdown:
		return Markdown(ctx, w)
	case FormatPlain:
		return Plain(ctx, w)
	}
	return fmt.Errorf("unknown format %q", format)
}
