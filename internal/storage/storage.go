package storage

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"time"
)

const (
	csvTimeFormat = time.RFC3339
	stateFileName = "state.json"
)

type Sample struct {
	Timestamp time.Time
	Port      string
	Name      string
	Owner     string
	BytesIn   uint64
	BytesOut  uint64
}

type State struct {
	UpdatedAt time.Time               `json:"updated_at"`
	Ports     map[string]StateCounter `json:"ports"`
}

type StateCounter struct {
	BytesIn  uint64 `json:"bytes_in"`
	BytesOut uint64 `json:"bytes_out"`
}

type ReportOptions struct {
	From    time.Time
	To      time.Time
	ByOwner bool
	Monthly bool
}

type SummaryRow struct {
	Key      string
	Name     string
	Owner    string
	BytesIn  uint64
	BytesOut uint64
}

func AppendSamples(dataDir string, samples []Sample) error {
	if len(samples) == 0 {
		return nil
	}
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return fmt.Errorf("create data dir %q: %w", dataDir, err)
	}

	byDay := make(map[string][]Sample)
	for _, sample := range samples {
		day := sample.Timestamp.Format("2006-01-02")
		byDay[day] = append(byDay[day], sample)
	}

	for day, daySamples := range byDay {
		path := filepath.Join(dataDir, day+".csv")
		if err := appendDay(path, daySamples); err != nil {
			return err
		}
	}
	return nil
}

func LoadState(dataDir string) (State, error) {
	path := filepath.Join(dataDir, stateFileName)
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return State{Ports: make(map[string]StateCounter)}, nil
	}
	if err != nil {
		return State{}, fmt.Errorf("read state %q: %w", path, err)
	}

	var state State
	if err := json.Unmarshal(raw, &state); err != nil {
		return State{}, fmt.Errorf("parse state %q: %w", path, err)
	}
	if state.Ports == nil {
		state.Ports = make(map[string]StateCounter)
	}
	return state, nil
}

func SaveState(dataDir string, state State) error {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return fmt.Errorf("create data dir %q: %w", dataDir, err)
	}
	state.UpdatedAt = time.Now()

	raw, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("encode state: %w", err)
	}

	path := filepath.Join(dataDir, stateFileName)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0644); err != nil {
		return fmt.Errorf("write state %q: %w", tmp, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("replace state %q: %w", path, err)
	}
	return nil
}

func CleanupOldCSVs(dataDir string, retentionDays int, now time.Time) (int, error) {
	if retentionDays <= 0 {
		return 0, nil
	}

	entries, err := os.ReadDir(dataDir)
	if errors.Is(err, os.ErrNotExist) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("read data dir %q: %w", dataDir, err)
	}

	cutoff := startOfDay(now.AddDate(0, 0, -retentionDays))
	removed := 0
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".csv" {
			continue
		}
		day, err := time.ParseInLocation("2006-01-02", entry.Name()[:len(entry.Name())-4], now.Location())
		if err != nil || !day.Before(cutoff) {
			continue
		}
		if err := os.Remove(filepath.Join(dataDir, entry.Name())); err != nil {
			return removed, fmt.Errorf("remove old csv %q: %w", entry.Name(), err)
		}
		removed++
	}
	return removed, nil
}

func Summarize(dataDir string, options ReportOptions) ([]SummaryRow, error) {
	from, to := normalizeRange(options.From, options.To)
	rows := make(map[string]SummaryRow)

	for day := startOfDay(from); !day.After(to); day = day.AddDate(0, 0, 1) {
		path := filepath.Join(dataDir, day.Format("2006-01-02")+".csv")
		if err := readDay(path, from, to, options, rows); err != nil {
			return nil, err
		}
	}

	out := make([]SummaryRow, 0, len(rows))
	for _, row := range rows {
		out = append(out, row)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Key < out[j].Key
	})
	return out, nil
}

func appendDay(path string, samples []Sample) error {
	needsHeader := true
	if stat, err := os.Stat(path); err == nil && stat.Size() > 0 {
		needsHeader = false
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("stat csv %q: %w", path, err)
	}

	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("open csv %q: %w", path, err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	if needsHeader {
		if err := writer.Write([]string{"timestamp", "port", "name", "owner", "bytes_in", "bytes_out"}); err != nil {
			return fmt.Errorf("write csv header %q: %w", path, err)
		}
	}

	for _, sample := range samples {
		record := []string{
			sample.Timestamp.Format(csvTimeFormat),
			sample.Port,
			sample.Name,
			sample.Owner,
			strconv.FormatUint(sample.BytesIn, 10),
			strconv.FormatUint(sample.BytesOut, 10),
		}
		if err := writer.Write(record); err != nil {
			return fmt.Errorf("write csv row %q: %w", path, err)
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return fmt.Errorf("flush csv %q: %w", path, err)
	}
	return nil
}

func readDay(path string, from time.Time, to time.Time, options ReportOptions, rows map[string]SummaryRow) error {
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("open csv %q: %w", path, err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	header, err := reader.Read()
	if errors.Is(err, io.EOF) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read csv header %q: %w", path, err)
	}
	if len(header) < 6 {
		return fmt.Errorf("csv %q has invalid header", path)
	}

	for {
		record, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return fmt.Errorf("read csv row %q: %w", path, err)
		}
		if len(record) < 6 {
			return fmt.Errorf("csv %q has short row", path)
		}

		timestamp, err := time.Parse(csvTimeFormat, record[0])
		if err != nil {
			return fmt.Errorf("parse timestamp %q in %q: %w", record[0], path, err)
		}
		if timestamp.Before(from) || timestamp.After(to) {
			continue
		}

		bytesIn, err := strconv.ParseUint(record[4], 10, 64)
		if err != nil {
			return fmt.Errorf("parse bytes_in %q in %q: %w", record[4], path, err)
		}
		bytesOut, err := strconv.ParseUint(record[5], 10, 64)
		if err != nil {
			return fmt.Errorf("parse bytes_out %q in %q: %w", record[5], path, err)
		}

		key, baseRow := reportKey(record, timestamp, options)
		row, ok := rows[key]
		if !ok {
			row = baseRow
		}
		row.BytesIn += bytesIn
		row.BytesOut += bytesOut
		rows[key] = row
	}

	return nil
}

func reportKey(record []string, timestamp time.Time, options ReportOptions) (string, SummaryRow) {
	port := record[1]
	name := record[2]
	owner := record[3]

	if options.Monthly {
		key := timestamp.Format("2006-01")
		return key, SummaryRow{Key: key, Name: "monthly", Owner: ""}
	}
	if options.ByOwner {
		return owner, SummaryRow{Key: owner, Name: "", Owner: owner}
	}
	return port, SummaryRow{Key: port, Name: name, Owner: owner}
}

func normalizeRange(from time.Time, to time.Time) (time.Time, time.Time) {
	if from.IsZero() {
		from = time.Now()
	}
	if to.IsZero() {
		to = from
	}
	if to.Before(from) {
		from, to = to, from
	}
	return startOfDay(from), endOfDay(to)
}

func startOfDay(t time.Time) time.Time {
	year, month, day := t.Date()
	return time.Date(year, month, day, 0, 0, 0, 0, t.Location())
}

func endOfDay(t time.Time) time.Time {
	return startOfDay(t).AddDate(0, 0, 1).Add(-time.Nanosecond)
}
