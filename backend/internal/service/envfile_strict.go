package service

import (
	"bufio"
	"strconv"
	"strings"

	"github.com/santapong/KeepSave/backend/internal/vault"
)

func parseEnvStrict(content string) (map[string]string, error) {
	if len(content) > 1<<20 {
		return nil, vault.ErrInvalid
	}
	result := map[string]string{}
	scanner := bufio.NewScanner(strings.NewReader(content))
	scanner.Buffer(make([]byte, 4096), 1<<20)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "export ") {
			line = strings.TrimSpace(strings.TrimPrefix(line, "export "))
		}
		key, value, ok := strings.Cut(line, "=")
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if !ok || !vault.ValidKey(key) {
			return nil, vault.ErrInvalid
		}
		if _, exists := result[key]; exists {
			return nil, vault.ErrInvalid
		}
		if strings.HasPrefix(value, "\"") {
			var err error
			value, err = strconv.Unquote(value)
			if err != nil {
				return nil, vault.ErrInvalid
			}
		} else if strings.HasPrefix(value, "'") {
			if len(value) < 2 || !strings.HasSuffix(value, "'") {
				return nil, vault.ErrInvalid
			}
			value = value[1 : len(value)-1]
		}
		result[key] = value
	}
	if scanner.Err() != nil || len(result) == 0 {
		return nil, vault.ErrInvalid
	}
	return result, nil
}
func formatEnvRecords(records []vault.Record) string {
	var b strings.Builder
	b.WriteString("# KeepSave explicit plaintext export\n")
	for _, record := range records {
		value := record.Value
		if strings.ContainsAny(value, " #\n\r\t\"'\\") {
			value = strconv.Quote(value)
		}
		b.WriteString(record.Key + "=" + value + "\n")
	}
	return b.String()
}
