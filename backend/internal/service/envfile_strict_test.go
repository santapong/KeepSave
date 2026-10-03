package service

import (
	"errors"
	"github.com/santapong/KeepSave/backend/internal/vault"
	"testing"
)

func TestEnvImportExportRoundTripAndRejectPartial(t *testing.T) {
	records := []vault.Record{{Key: "EMPTY", Value: ""}, {Key: "MULTILINE", Value: "a\nb"}, {Key: "ESCAPES", Value: "quote\" and \\ path\r\t"}, {Key: "HASH", Value: "a#b"}}
	parsed, err := parseEnvStrict(formatEnvRecords(records))
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range records {
		if parsed[r.Key] != r.Value {
			t.Fatalf("roundtrip failed for %s", r.Key)
		}
	}
	for _, input := range []string{"A=1\nA=2\n", "A=1\nmalformed\n", "A=\"unterminated\n", "A='unterminated\n", "BAD KEY=x\n", "\n# empty"} {
		if _, err := parseEnvStrict(input); !errors.Is(err, vault.ErrInvalid) {
			t.Fatal("accepted invalid import")
		}
	}
}
