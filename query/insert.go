package query

import "strings"

type BulkInsert struct {
	sb         *strings.Builder
	first      bool
	valueQuery string
	args       []any
}

/*
	NewBulkInsert

	eg 1) INSERT INTO chair(id, name, description, thumbnail, price, height, width, depth, color, features, kind, popularity, stock) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?)
		table -> "chair"
		columns -> "id, name, description, thumbnail, price, height, width, depth, color, features, kind, popularity, stock"
		valueQuery -> "(?,?,?,?,?,?,?,?,?,?,?,?,?)"

	eg 2) INSERT INTO chair VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?)
		table -> "chair"
		columns -> ""
		valueQuery -> "(?,?,?,?,?,?,?,?,?,?,?,?,?)"
*/
func NewBulkInsert(table, colNames, valueQuery string) *BulkInsert {
	sb := &strings.Builder{}
	sb.WriteString("INSERT INTO ")
	sb.WriteString(table)
	sb.WriteString(" (")
	sb.WriteString(colNames)
	sb.WriteString(") VALUES ")

	return &BulkInsert{
		sb:         sb,
		first:      true,
		valueQuery: valueQuery,
	}
}

func (b *BulkInsert) Add(args ...any) {
	if b.first {
		b.first = false
	} else {
		b.sb.WriteString(", ")
	}

	b.sb.WriteString(b.valueQuery)

	b.args = append(b.args, args...)
}

func (b *BulkInsert) Query() (string, []any) {
	return b.sb.String(), b.args
}
