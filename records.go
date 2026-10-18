package main

import (
	"fmt"
	"html"
	"sort"
	"strings"
	"time"
)

type Record struct {
	ID       string `json:"id"`
	Kind     string `json:"kind"`
	Property string `json:"property"`
	Detail   string `json:"detail"`
	Due      string `json:"due"`
}

func digestRows(records []Record, day time.Time) ([]Record, error) {
	rows := append([]Record(nil), records...)
	for _, r := range rows {
		if r.ID == "" || r.Property == "" || r.Detail == "" {
			return nil, fmt.Errorf("record needs id, property and detail")
		}
		switch r.Kind {
		case "maintenance", "tenant_document", "inspection":
		default:
			return nil, fmt.Errorf("unknown record kind %q", r.Kind)
		}
		if r.Due != "" {
			if _, err := time.Parse("2006-01-02", r.Due); err != nil {
				return nil, err
			}
		}
	}
	priority := func(r Record) int {
		if r.Kind == "maintenance" {
			return 0
		}
		if r.Kind == "inspection" && r.Due != "" && r.Due <= day.Format("2006-01-02") {
			return 1
		}
		return 2
	}
	sort.Slice(rows, func(i, j int) bool {
		a, b := priority(rows[i]), priority(rows[j])
		if a != b {
			return a < b
		}
		return rows[i].ID < rows[j].ID
	})
	return rows, nil
}

func digestHTML(rows []Record, summaries map[string]string) string {
	var b strings.Builder
	b.WriteString("<!doctype html><html><head><meta charset=\"utf-8\"></head><body><h1>Overnight property digest</h1><table><tr><th>Property</th><th>Record</th><th>Summary</th></tr>")
	for _, r := range rows {
		fmt.Fprintf(&b, "<tr><td>%s</td><td>%s (%s)</td><td>%s</td></tr>", html.EscapeString(r.Property), html.EscapeString(r.Kind), html.EscapeString(r.ID), html.EscapeString(summaries[r.ID]))
	}
	b.WriteString("</table></body></html>")
	return b.String()
}
