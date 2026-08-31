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

// HistoryVersion stamps the on-disk format. v2 added per-item step progress;
// v1 files (a bare "checked" list) are still read and treated as full credit.
const HistoryVersion = 2

// DefaultRetentionDays bounds how far back history is kept.
const DefaultRetentionDays = 365

// DayLayout is the key format for one day of history (local time, not UTC:
// a daily checklist must roll over at the user's midnight).
const DayLayout = "2006-01-02"

// DayRecord is one day's state.
type DayRecord struct {
	// Progress maps item id -> steps completed that day (v2+).
	Progress map[string]int `json:"progress,omitempty"`
	// Checked is the v1 format: ids that were fully completed. Still read so
	// existing files keep working; new writes go to Progress.
	Checked []string  `json:"checked,omitempty"`
	Total   int       `json:"total"` // item count in effect that day
	Updated time.Time `json:"updated_at"`
}

// Catalog is the id -> step-count lookup derived from the config. Statistics
// need it to tell "fully done" apart from "partially done".
type Catalog struct {
	steps map[string]int
}

// NewCatalog indexes the config's items by id.
func NewCatalog(items []Item) Catalog {
	c := Catalog{steps: make(map[string]int, len(items))}
	for _, it := range items {
		c.steps[it.ID] = it.StepsOrDefault()
	}
	return c
}

// Steps returns an item's step count, at least 1. Unknown ids count as 1.
func (c Catalog) Steps(id string) int {
	if n, ok := c.steps[id]; ok && n >= 1 {
		return n
	}
	return 1
}

// IDs lists the known item ids in a stable order.
func (c Catalog) IDs() []string {
	out := make([]string, 0, len(c.steps))
	for id := range c.steps {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

// UnmarshalJSON tolerates a broken or missing updated_at. The checklist itself
// is the valuable part, so a cosmetic timestamp must not be allowed to fail the
// decode of the whole history file and get it quarantined.
func (d *DayRecord) UnmarshalJSON(data []byte) error {
	type dayAlias struct {
		Progress map[string]int `json:"progress"`
		Checked  []string       `json:"checked"`
		Total    int            `json:"total"`
		Updated  string         `json:"updated_at"`
	}
	var a dayAlias
	if err := json.Unmarshal(data, &a); err != nil {
		return err
	}
	d.Progress = a.Progress
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
	cat       Catalog
	Notice    string `json:"-"` // one-shot warning for the caller, not persisted
}

// UseCatalog tells the history how many steps each item needs, which is what
// makes "fully done" distinguishable from "partially done". Call it right after
// LoadHistory and before touching any statistic.
func (h *History) UseCatalog(c Catalog) { h.cat = c }

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

// CheckedSet returns the ids *fully* completed on a date.
func (h *History) CheckedSet(date string) map[string]struct{} {
	out := map[string]struct{}{}
	rec := h.Days[date]
	if rec == nil {
		return out
	}
	if len(rec.Progress) > 0 {
		for id, v := range rec.Progress {
			if v >= h.cat.Steps(id) {
				out[id] = struct{}{}
			}
		}
		return out
	}
	for _, id := range rec.Checked {
		out[id] = struct{}{}
	}
	return out
}

// StepsOn reports how many steps of an item were completed on a date. A v1
// record (id present in Checked) counts as fully complete. Values are clamped
// to the item's current step count so a hand-edited file cannot wedge an item.
func (h *History) StepsOn(date, id string) int {
	rec := h.Days[date]
	if rec == nil {
		return 0
	}
	if v, ok := rec.Progress[id]; ok {
		if v <= 0 {
			return 0
		}
		if max := h.cat.Steps(id); v > max {
			return max
		}
		return v
	}
	for _, c := range rec.Checked {
		if c == id {
			return h.cat.Steps(id)
		}
	}
	return 0
}

// Ratio returns completion in 0..1. For AllItems it takes the single best item
// that day, which is what the source app shades its 全部 heatmap by.
func (h *History) Ratio(date, id string) float64 {
	if id != AllItems {
		max := h.cat.Steps(id)
		if max <= 0 {
			return 0
		}
		return clampRatio(float64(h.StepsOn(date, id)) / float64(max))
	}
	rec := h.Days[date]
	if rec == nil {
		return 0
	}
	// Candidate ids come from the catalog *and* from the record itself, so a
	// caller that forgot UseCatalog cannot make the 全部 view read as all-zero.
	best := 0.0
	seen := map[string]bool{}
	consider := func(other string) {
		if seen[other] {
			return
		}
		seen[other] = true
		if r := h.Ratio(date, other); r > best {
			best = r
		}
	}
	for _, other := range h.cat.IDs() {
		consider(other)
	}
	for other := range rec.Progress {
		consider(other)
	}
	for _, other := range rec.Checked {
		consider(other)
	}
	return clampRatio(best)
}

func clampRatio(r float64) float64 {
	if r < 0 {
		return 0
	}
	if r > 1 {
		return 1
	}
	return r
}

// ProgressSet returns today's raw step counts, so the UI can restore partial
// progress (not just completed items) when it reopens on the same day.
func (h *History) ProgressSet(date string) map[string]int {
	out := map[string]int{}
	rec := h.Days[date]
	if rec == nil {
		return out
	}
	for id, v := range rec.Progress {
		out[id] = v
	}
	// v1 records only name the fully-completed ids.
	for _, id := range rec.Checked {
		if _, ok := out[id]; !ok {
			out[id] = h.cat.Steps(id)
		}
	}
	return out
}

// SetProgress records step counts for a date and flushes immediately, so that
// even `kill -9` or a closed terminal loses nothing. Zero-step entries are
// dropped and an empty day removes the record entirely.
func (h *History) SetProgress(date string, prog map[string]int, total int) error {
	clean := make(map[string]int, len(prog))
	for id, v := range prog {
		if v <= 0 {
			continue
		}
		if max := h.cat.Steps(id); v > max {
			v = max
		}
		clean[id] = v
	}
	if len(clean) == 0 {
		delete(h.Days, date)
	} else {
		h.Days[date] = &DayRecord{Progress: clean, Total: total, Updated: time.Now()}
	}
	return h.Save()
}

// SetChecked records a set of fully-completed ids, so callers that think in
// checkbox terms never have to care about steps.
func (h *History) SetChecked(date string, ids []string, total int) error {
	prog := make(map[string]int, len(ids))
	for _, id := range ids {
		prog[id] = h.cat.Steps(id)
	}
	return h.SetProgress(date, prog, total)
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
	for date, rec := range h.Days {
		if rec == nil || rec.Total <= 0 {
			continue
		}
		if len(h.CheckedSet(date)) >= rec.Total {
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
