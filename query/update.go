package query

import (
	"errors"
	"log/slog"
	"strings"
)

type BulkUpdate struct {
	sb        *strings.Builder
	keyCol    string
	keys      []any
	valueCols []string
	values    [][]any
}

func NewBulkUpdate(table, keyCol string, valueCols []string) *BulkUpdate {
	sb := &strings.Builder{}
	sb.WriteString("UPDATE ")
	sb.WriteString(table)
	sb.WriteString(" SET ")

	return &BulkUpdate{
		sb:        sb,
		keyCol:    keyCol,
		valueCols: valueCols,
		values:    make([][]any, len(valueCols)),
	}
}

func (b *BulkUpdate) Add(key any, values ...any) error {
	b.keys = append(b.keys, key)

	for i := range b.values {
		if i >= len(values) {
			return errors.New("not enough values")
		}

		b.values[i] = append(b.values[i], values[i])
	}
	if len(values) > len(b.values) {
		slog.Warn("too many values",
			slog.Int("expected", len(b.values)),
			slog.Int("actual", len(values)),
		)
	}

	return nil
}

func (b *BulkUpdate) Query(whereQuery string, whereArgs ...any) (string, []any) {
	args := make([]any, 0)

	for i, valueCol := range b.valueCols {
		b.sb.WriteString(valueCol)
		b.sb.WriteString(" = ELT(FIELD(")
		b.sb.WriteString(b.keyCol)
		b.sb.WriteString(", ")

		b.sb.WriteString(strings.Repeat("?, ", len(b.keys)-1))
		b.sb.WriteString("?), ")
		args = append(args, b.keys...)

		b.sb.WriteString(strings.Repeat("?, ", len(b.values[i])-1))
		b.sb.WriteString("?)")
		args = append(args, b.values[i]...)

		if i < len(b.valueCols)-1 {
			b.sb.WriteString(", ")
		}
	}

	b.sb.WriteString(" WHERE ")
	b.sb.WriteString(b.keyCol)
	b.sb.WriteString(" IN ")
	b.sb.WriteString(strings.Repeat("?, ", len(b.keys)-1))
	args = append(args, b.keys...)
	b.sb.WriteString("?)")

	if len(whereQuery) != 0 {
		b.sb.WriteString(" AND (")
		b.sb.WriteString(whereQuery)
		b.sb.WriteString(")")
		args = append(args, whereArgs...)
	}

	return b.sb.String(), args
}
