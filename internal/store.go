package internal

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// HistoryVersion is stamped into the file so future migrations can tell formats apart.
const HistoryVersion = 1

// DefaultRetentionDays bounds how far back history is kept.
const DefaultRetentionDays = 365

// DayLayout is the key format for one day of history (local time, not UTC:
// a daily checklist must roll over at the user's midnight).
const DayLayout = "2006-01-02"

// DayRecord is one day's state.
type DayRecord struct {
	Checked []string  `json:"checked"`
	Total   int       `json:"total"` // item count in effect that day
	Updated time.Time `json:"updated_at"`
}

// UnmarshalJSON tolerates a broken or missing updated_at. The checklist itself
// is the valuable part, so a cosmetic timestamp must not be allowed to fail the
// decode of the whole history file and get it quarantined.
func (d *DayRecord) UnmarshalJSON(data []byte) error {
	type dayAlias struct {
		Checked []string `json:"checked"`
		Total   int      `json:"total"`
		Updated string   `json:"updated_at"`
	}
	var a dayAlias
	if err := json.Unmarshal(data, &a); err != nil {
		return err
	}
	d.Checked = a.Checked
	d.Total = a.Total
	if ts := strings.TrimSpace(a.Updated); ts != "" {
		if t, err := time.Parse(time.RFC3339Nano, ts); err == nil {
			d.Updated = t
		}
		// Unparseable: leave zero time, keep the checks.
	}
	return nil
}

// History is the persisted, per-day record of checklists.
type History struct {
	Version int                   `json:"version"`
	Days    map[string]*DayRecord `json:"days"`

	path      string
	retention int
	Notice    string `json:"-"` // one-shot warning for the caller, not persisted
}

// DataPath resolves the history file location.
// Precedence: flag > $TODAY_DATA > $XDG_DATA_HOME/Today/history.json > ~/.local/share/Today/history.json
func DataPath(flagValue string) (string, error) {
	if flagValue != "" {
		return expandPath(flagValue)
	}
	if env := os.Getenv("TODAY_DATA"); env != "" {
		return expandPath(env)
	}
	base := os.Getenv("XDG_DATA_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("cannot resolve home directory: %w", err)
		}
		base = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(base, "Today", "history.json"), nil
}

// LoadHistory reads history.json. A missing file is an empty history (normal on
// first run). An unparseable file is quarantined to a .corrupt-* sibling and
// replaced, rather than aborting every future run.
func LoadHistory(path string, retention int) (*History, error) {
	if retention <= 0 {
		retention = DefaultRetentionDays
	}
	h := &History{Version: HistoryVersion, Days: map[string]*DayRecord{}, path: path, retention: retention}

	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return h, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}

	var onDisk History
	if err := json.Unmarshal(raw, &onDisk); err != nil {
		quarantine := fmt.Sprintf("%s.corrupt-%d", path, time.Now().Unix())
		if rerr := os.Rename(path, quarantine); rerr == nil {
			h.Notice = fmt.Sprintf("history file was unreadable; moved to %s and started fresh", quarantine)
			return h, nil
		}
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if onDisk.Days != nil {
		h.Days = onDisk.Days
	}
	for date, rec := range h.Days {
		if rec == nil {
			delete(h.Days, date)
		}
	}
	return h, nil
}

// Day returns the record for a date, or nil.
func (h *History) Day(date string) *DayRecord { return h.Days[date] }

// CheckedSet returns the checked ids for a date as a set.
func (h *History) CheckedSet(date string) map[string]struct{} {
	out := map[string]struct{}{}
	rec := h.Days[date]
	if rec == nil {
		return out
	}
	for _, id := range rec.Checked {
		out[id] = struct{}{}
	}
	return out
}

// SetChecked records a toggle and flushes to disk immediately, so that even
// `kill -9` or a closed terminal loses nothing.
func (h *History) SetChecked(date string, ids []string, total int) error {
	if len(ids) == 0 {
		delete(h.Days, date)
	} else {
		h.Days[date] = &DayRecord{Checked: ids, Total: total, Updated: time.Now()}
	}
	return h.Save()
}

// ResetDay clears one day's checks.
func (h *History) ResetDay(date string) error {
	if _, ok := h.Days[date]; !ok {
		return nil
	}
	delete(h.Days, date)
	return h.Save()
}

// Save writes history atomically after pruning out-of-retention days.
func (h *History) Save() error {
	h.prune()
	raw, err := marshalReadable(h)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(h.path), 0o755); err != nil {
		return err
	}
	// 0600: this file records personal health self-checks.
	return atomicWrite(h.path, raw, 0o600)
}

func (h *History) prune() {
	if h.retention <= 0 || len(h.Days) == 0 {
		return
	}
	cutoff := time.Now().AddDate(0, 0, -h.retention).Format(DayLayout)
	for date := range h.Days {
		if date < cutoff {
			delete(h.Days, date)
		}
	}
}

// CheckedCount is how many days in the retained history have any check.
func (h *History) CheckedCount() int { return len(h.Days) }

// CompleteCount is how many days had every item checked, judged against the
// item count recorded for that day (so editing the list later cannot rewrite
// history).
func (h *History) CompleteCount() int {
	n := 0
	for _, rec := range h.Days {
		if rec != nil && rec.Total > 0 && len(rec.Checked) >= rec.Total {
			n++
		}
	}
	return n
}

// Streak counts consecutive days with at least one check, ending today. If
// nothing is checked yet today, the chain is allowed to end yesterday, so an
// un-started morning does not visually break an ongoing streak.
func (h *History) Streak(now time.Time) int {
	day := now
	if len(h.CheckedSet(day.Format(DayLayout))) == 0 {
		day = day.AddDate(0, 0, -1)
	}
	streak := 0
	for {
		key := day.Format(DayLayout)
		if len(h.CheckedSet(key)) == 0 {
			return streak
		}
		streak++
		day = day.AddDate(0, 0, -1)
		if streak > h.CheckedCount() { // cannot exceed days on record
			return h.CheckedCount()
		}
	}
}

// SortedDays returns day keys in ascending order (used by tests and --status).
func (h *History) SortedDays() []string {
	out := make([]string, 0, len(h.Days))
	for d := range h.Days {
		out = append(out, d)
	}
	sort.Strings(out)
	return out
}

// atomicWrite writes via a sibling temp file + rename, so readers never observe
// a partially written file and a crash cannot truncate the original.
func atomicWrite(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".atomic-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer func() {
		if _, statErr := os.Stat(tmpName); statErr == nil {
			_ = os.Remove(tmpName)
		}
	}()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpName, perm); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}
