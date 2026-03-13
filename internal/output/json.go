package output

import (
	"encoding/json"
	"os"

	"spiderly/internal/report"
)

func RenderJSON(r report.CrawlReport, pretty bool) (string, error) {
	var b []byte
	var err error

	if pretty {
		b, err = json.MarshalIndent(r, "", "  ")
	} else {
		b, err = json.Marshal(r)
	}
	if err != nil {
		return "", err
	}

	return string(b), nil
}

func WriteJSONFile(path string, r report.CrawlReport, pretty bool) error {
	s, err := RenderJSON(r, pretty)
	if err != nil {
		return err
	}
	return os.WriteFile(path, []byte(s), 0o644)
}
