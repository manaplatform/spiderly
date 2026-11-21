package output

import (
	"bufio"
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
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	bw := bufio.NewWriterSize(f, 64*1024)
	enc := json.NewEncoder(bw)
	if pretty {
		enc.SetIndent("", "  ")
	}
	if err := enc.Encode(r); err != nil {
		return err
	}
	return bw.Flush()
}
