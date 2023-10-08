package isudb

import (
	"fmt"
	"strings"
	"testing"
)

func BenchmarkPostgresNormalizer(b *testing.B) {
	psb := postgresSegmentBuilder{}

	queryPart := fmt.Sprintf("(%s$2)", strings.Repeat("$1, ", 5))
	query := fmt.Sprintf("INSERT INTO users (name, email, password, salt, created_at, updated_at) VALUES %s", strings.Repeat(queryPart+", ", 999)+queryPart)

	for i := 0; i < b.N; i++ {
		psb.normalizer(query)
	}
}

func TestPostgresNormalizer(t *testing.T) {
	psb := postgresSegmentBuilder{}

	tests := []string{
		"$1",
	}

	for _, test := range tests {
		test := test
		t.Run(test, func(t *testing.T) {
			t.Parallel()

			queryPart := fmt.Sprintf("(%s%s)", strings.Repeat(fmt.Sprintf("%s, ", test), 5), test)
			query := fmt.Sprintf("INSERT INTO users (name, email, password, salt, created_at, updated_at) VALUES %s", strings.Repeat(queryPart+", ", 999)+queryPart)

			normalizedQuery := psb.normalizer(query)

			if normalizedQuery != "INSERT INTO users (name, email, password, salt, created_at, updated_at) VALUES ..., (..., ?)" {
				t.Errorf("unexpected query: %s", normalizedQuery)
			}
		})
	}
}
