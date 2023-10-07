package isudb

import (
	"fmt"
	"strings"
	"testing"
)

func BenchmarkMySQLNormalizer(b *testing.B) {
	msb := mysqlSegmentBuilder{}

	queryPart := fmt.Sprintf("(%s?)", strings.Repeat("?, ", 5))
	query := fmt.Sprintf("INSERT INTO users (name, email, password, salt, created_at, updated_at) VALUES %s", strings.Repeat(queryPart+", ", 999)+queryPart)

	for i := 0; i < b.N; i++ {
		msb.normalizer(query)
	}
}

func TestMySQLNormalizer(t *testing.T) {
	msb := mysqlSegmentBuilder{}

	queryPart := fmt.Sprintf("(%s?)", strings.Repeat("?, ", 5))
	query := fmt.Sprintf("INSERT INTO users (name, email, password, salt, created_at, updated_at) VALUES %s", strings.Repeat(queryPart+", ", 999)+queryPart)

	normalizedQuery := msb.normalizer(query)

	if normalizedQuery != "INSERT INTO users (name, email, password, salt, created_at, updated_at) VALUES ..., (..., ?)" {
		t.Errorf("unexpected query: %s", normalizedQuery)
	}
}
