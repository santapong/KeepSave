package repository

import (
	"database/sql"
	"fmt"
	"time"
)

// dbTime accepts both native driver timestamps and the UTC TEXT timestamps
// in KeepSave's original SQLite migrations. It also reads existing SQLite
// databases, without rewriting tables or changing their stored values.
func dbTime(target interface{}) sql.Scanner { return timestampScanner{target: target} }

type timestampScanner struct{ target interface{} }

func (s timestampScanner) Scan(value interface{}) error {
	var parsed time.Time
	if value != nil {
		switch v := value.(type) {
		case time.Time:
			parsed = v
		case string:
			var err error
			parsed, err = parseDBTime(v)
			if err != nil {
				return err
			}
		case []byte:
			var err error
			parsed, err = parseDBTime(string(v))
			if err != nil {
				return err
			}
		default:
			return fmt.Errorf("unsupported database timestamp type")
		}
	}
	// Never include the source bytes in an error; database contents can be
	// attacker-controlled and must not become log output.
	switch target := s.target.(type) {
	case *time.Time:
		if value == nil {
			return fmt.Errorf("unexpected null database timestamp")
		}
		*target = parsed
	case **time.Time:
		if value == nil {
			*target = nil
		} else {
			*target = &parsed
		}
	case *sql.NullTime:
		*target = sql.NullTime{Time: parsed, Valid: value != nil}
	default:
		return fmt.Errorf("unsupported database timestamp destination")
	}
	return nil
}

func parseDBTime(value string) (time.Time, error) {
	for _, layout := range []string{
		time.RFC3339Nano,
		"2006-01-02 15:04:05.999999999-07:00",
		"2006-01-02 15:04:05.999999999",
		"2006-01-02T15:04:05.999999999",
		"2006-01-02",
	} {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed, nil
		}
	}
	return time.Time{}, fmt.Errorf("invalid database timestamp")
}
