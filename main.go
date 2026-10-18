package main

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

func run(ctx context.Context, c apiClient, rows []Record, output string) error {
	ordered, err := digestRows(rows, time.Now().UTC())
	if err != nil {
		return err
	}
	requests := make([]any, 0, len(ordered))
	for _, r := range ordered {
		requests = append(requests, map[string]any{"custom_id": r.ID, "method": "POST", "url": "/v1/chat/completions", "body": map[string]any{"model": "auto", "messages": []any{map[string]string{"role": "user", "content": fmt.Sprintf("Summarize this property-management record in one factual sentence. Type: %s; property: %s; due: %s; detail: %s", r.Kind, r.Property, r.Due, r.Detail)}}}})
	}
	encoded, _ := json.Marshal(requests)
	hash := sha256.Sum256(encoded)
	key := hex.EncodeToString(hash[:])
	var batch struct {
		ID string `json:"batch_id"`
	}
	if err := c.call(ctx, "POST", "/ai/batch/submit", map[string]any{"requests": requests, "idempotency_key": key}, "", &batch); err != nil {
		return err
	}
	if batch.ID == "" {
		return fmt.Errorf("batch response missing id")
	}
	for {
		var status struct {
			State string `json:"state"`
		}
		if err := c.call(ctx, "GET", "/ai/batch/status/"+batch.ID, nil, "", &status); err != nil {
			return err
		}
		if status.State == "completed" || status.State == "partial" {
			break
		}
		if status.State == "failed" || status.State == "expired" || status.State == "cancelled" {
			return fmt.Errorf("batch ended: %s", status.State)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(5 * time.Second):
		}
	}
	var results struct {
		Items []struct {
			RequestIndex int  `json:"request_index"`
			OK           bool `json:"ok"`
			Result       struct {
				Content string `json:"content"`
			} `json:"result"`
			Error json.RawMessage `json:"error"`
		} `json:"items"`
	}
	if err := c.call(ctx, "GET", "/ai/batch/results/"+batch.ID, nil, "", &results); err != nil {
		return err
	}
	summaries := make(map[string]string, len(results.Items))
	for _, result := range results.Items {
		if result.RequestIndex < 0 || result.RequestIndex >= len(ordered) {
			return fmt.Errorf("batch result has invalid request index %d", result.RequestIndex)
		}
		if !result.OK {
			return fmt.Errorf("batch request %s failed: %s", ordered[result.RequestIndex].ID, result.Error)
		}
		if result.Result.Content != "" {
			summaries[ordered[result.RequestIndex].ID] = result.Result.Content
		}
	}
	for _, r := range ordered {
		if summaries[r.ID] == "" {
			return fmt.Errorf("missing summary for %s", r.ID)
		}
	}
	var pdf struct {
		URL string `json:"url"`
		PDF string `json:"pdf"`
	}
	if err := c.call(ctx, "POST", "/pdf/generate", map[string]any{"html": digestHTML(ordered, summaries), "page_size": "A4", "idempotency_key": key + "-pdf", "store": false}, "", &pdf); err != nil {
		return err
	}
	if pdf.PDF != "" {
		data, err := base64.StdEncoding.DecodeString(pdf.PDF)
		if err != nil {
			return err
		}
		return os.WriteFile(output, data, 0600)
	}
	if pdf.URL == "" {
		return fmt.Errorf("PDF response missing document")
	}
	if encoded, ok := strings.CutPrefix(pdf.URL, "data:application/pdf;base64,"); ok {
		data, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			return err
		}
		return os.WriteFile(output, data, 0600)
	}
	req, err := http.NewRequestWithContext(ctx, "GET", pdf.URL, nil)
	if err != nil {
		return err
	}
	res, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("PDF download HTTP %d", res.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(res.Body, 32<<20))
	if err != nil {
		return err
	}
	return os.WriteFile(output, data, 0600)
}

func main() {
	input := flag.String("input", "records.json", "JSON array of property records")
	output := flag.String("output", "digest.pdf", "Rendered PDF path")
	flag.Parse()
	key := os.Getenv("INFRAI_API_KEY")
	if key == "" {
		log.Fatal("set INFRAI_API_KEY")
	}
	raw, err := os.ReadFile(*input)
	if err != nil {
		log.Fatal(err)
	}
	var rows []Record
	if err := json.Unmarshal(raw, &rows); err != nil {
		log.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	if err := run(ctx, apiClient{key: key, http: &http.Client{Timeout: 30 * time.Second}, base: baseURL}, rows, *output); err != nil {
		log.Fatal(err)
	}
	fmt.Println(*output)
}
