package main

import (
	"strings"
	"testing"
	"time"
)

func TestDigestPriority(t *testing.T) {
	cases := []struct {
		name string
		due  string
		want string
	}{
		{"inspection due today", "2026-09-19", "repair,z-inspect,a-lease"},
		{"inspection next week", "2026-09-26", "repair,a-lease,z-inspect"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			records := []Record{{ID: "a-lease", Kind: "tenant_document", Property: "Oak", Detail: "Signed lease"}, {ID: "z-inspect", Kind: "inspection", Property: "Oak", Detail: "Fire doors", Due: tc.due}, {ID: "repair", Kind: "maintenance", Property: "Oak", Detail: "Leaking tap"}}
			got, err := digestRows(records, time.Date(2026, 9, 19, 0, 0, 0, 0, time.UTC))
			if err != nil {
				t.Fatal(err)
			}
			ids := []string{got[0].ID, got[1].ID, got[2].ID}
			if strings.Join(ids, ",") != tc.want {
				t.Fatalf("order = %v, want %s", ids, tc.want)
			}
		})
	}
}

func TestDigestEscapesSource(t *testing.T) {
	document := digestHTML([]Record{{ID: "1", Kind: "inspection", Property: "A < B"}}, map[string]string{"1": "Check & file"})
	if strings.Contains(document, "A < B") || !strings.Contains(document, "Check &amp; file") {
		t.Fatal("unsafe report text")
	}
}
