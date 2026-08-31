package internal

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"
)

// DefaultTitle is used when no config file exists yet.
const DefaultTitle = "每日自查清单"

// MaxItems guards against a config that would overflow a single terminal screen.
const MaxItems = 64

// Item is one entry of the daily checklist.
//
// In JSON an item may be written either as a bare string ("Eyes") or as an
// object ({"id":"eyes","label":"Eyes","hint":"远眺 20 分钟"}). The bare form
// keeps the zero-config promise; the object form adds a stable id (so history
// survives a rename) and an optional hint line.
type Item struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Hint  string `json:"hint,omitempty"`
}

// Config is the on-disk configuration of Today.
type Config struct {
	Title string `json:"title,omitempty"`
	Items []Item `json:"items"`
	// HistoryDays bounds how far back per-day records are kept. 0 = default.
	HistoryDays int `json:"history_days,omitempty"`
}

// DefaultConfig is what gets written on first run. It preserves the seven
// items that were hardcoded in v0.1.0, so upgrading users see no change.
func DefaultConfig() Config {
	labels := []string{"Eyes", "Nose", "Skin", "Lips", "Anxiety", "Cognition", "Weight & Fat"}
	items := make([]Item, 0, len(labels))
	for _, l := range labels {
		items = append(items, Item{Label: l})
	}
	return Config{Title: DefaultTitle, Items: normalizeItems(items), HistoryDays: DefaultRetentionDays}
}

// UnmarshalJSON accepts both the bare-string and the object form.
func (it *Item) UnmarshalJSON(data []byte) error {
	trimmed := strings.TrimSpace(string(data))
	if len(trimmed) > 0 && trimmed[0] == '"' {
		var s string
		if err := json.Unmarshal(data, &s); err != nil {
			return err
		}
		it.Label = strings.TrimSpace(s)
		return nil
	}
	// Alias struct prevents infinite recursion back into this method.
	type itemAlias struct {
		ID    string `json:"id"`
		Label string `json:"label"`
		Name  string `json:"name"`
		Text  string `json:"text"`
		Hint  string `json:"hint,omitempty"`
	}
	var a itemAlias
	if err := json.Unmarshal(data, &a); err != nil {
		return err
	}
	it.ID = strings.TrimSpace(a.ID)
	it.Hint = strings.TrimSpace(a.Hint)
	switch {
	case strings.TrimSpace(a.Label) != "":
		it.Label = strings.TrimSpace(a.Label)
	case strings.TrimSpace(a.Name) != "":
		it.Label = strings.TrimSpace(a.Name)
	case strings.TrimSpace(a.Text) != "":
		it.Label = strings.TrimSpace(a.Text)
	}
	return nil
}

// MarshalJSON emits the compact object form.
func (it Item) MarshalJSON() ([]byte, error) {
	type itemAlias struct {
		ID    string `json:"id"`
		Label string `json:"label"`
		Hint  string `json:"hint,omitempty"`
	}
	return encodeNoHTMLEscape(itemAlias{ID: it.ID, Label: it.Label, Hint: it.Hint})
}

// ConfigPath resolves the config file location.
//
// Precedence: explicit flag > $TODAY_CONFIG > $XDG_CONFIG_HOME/Today/config.json
// > ~/.config/Today/config.json
func ConfigPath(flagValue string) (string, error) {
	if flagValue != "" {
		return expandPath(flagValue)
	}
	if env := os.Getenv("TODAY_CONFIG"); env != "" {
		return expandPath(env)
	}
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("cannot resolve home directory: %w", err)
		}
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "Today", "config.json"), nil
}

// LoadConfig reads the config, creating the default file when it is absent.
// A malformed config is a hard error: silently falling back to defaults would
// make a typo look like "my items disappeared".
func LoadConfig(path string) (Config, error) {
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		cfg := DefaultConfig()
		if err := WriteConfig(path, cfg); err != nil {
			// A missing/unwritable config dir should not block using the tool.
			return cfg, fmt.Errorf("using built-in defaults, could not create %s: %w", path, err)
		}
		return cfg, nil
	}
	if err != nil {
		return Config{}, fmt.Errorf("read %s: %w", path, err)
	}

	var cfg Config
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse %s: %w", path, err)
	}
	if strings.TrimSpace(cfg.Title) == "" {
		cfg.Title = DefaultTitle
	}
	cfg.Items = normalizeItems(cfg.Items)
	if len(cfg.Items) == 0 {
		return Config{}, fmt.Errorf("parse %s: \"items\" is empty (delete the file to get the built-in defaults back)", path)
	}
	if len(cfg.Items) > MaxItems {
		return Config{}, fmt.Errorf("parse %s: %d items exceeds the %d limit", path, len(cfg.Items), MaxItems)
	}
	if cfg.HistoryDays <= 0 {
		cfg.HistoryDays = DefaultRetentionDays
	}
	return cfg, nil
}

// WriteConfig stores the config atomically so a crash mid-write cannot leave a
// truncated file that the next run would refuse to parse.
func WriteConfig(path string, cfg Config) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	raw, err := marshalReadable(cfg)
	if err != nil {
		return err
	}
	return atomicWrite(path, raw, 0o644)
}

// marshalReadable encodes indented JSON without HTML escaping. The default
// encoder turns "Weight & Fat" into "Weight \u0026 Fat", which is correct but
// hostile to a human editing this file by hand.
func marshalReadable(v any) ([]byte, error) {
	raw, err := encodeNoHTMLEscape(v)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := json.Indent(&buf, raw, "", "  "); err != nil {
		return nil, err
	}
	buf.WriteByte('\n')
	return buf.Bytes(), nil
}

// encodeNoHTMLEscape is the single escaping-off encoder used by every writer in
// this package, including Item.MarshalJSON. Encoder.SetEscapeHTML does not
// propagate into a custom MarshalJSON, so the option has to be applied here and
// used everywhere -- otherwise nested values are re-escaped by json.Marshal.
func encodeNoHTMLEscape(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimRight(buf.Bytes(), "\n"), nil
}

// normalizeItems drops blank labels, derives ids, and enforces id uniqueness.
func normalizeItems(in []Item) []Item {
	out := make([]Item, 0, len(in))
	seen := make(map[string]struct{}, len(in))
	for i, it := range in {
		it.Label = strings.TrimSpace(it.Label)
		if it.Label == "" {
			continue
		}
		it.ID = strings.TrimSpace(it.ID)
		if it.ID == "" {
			it.ID = slug(it.Label)
		}
		if it.ID == "" {
			it.ID = fmt.Sprintf("item%d", i+1)
		}
		if _, dup := seen[it.ID]; dup {
			// History is keyed by id, so a collision would silently merge two
			// items. Disambiguate instead of rejecting the whole config.
			for n := 2; ; n++ {
				cand := fmt.Sprintf("%s-%d", it.ID, n)
				if _, taken := seen[cand]; !taken {
					it.ID = cand
					break
				}
			}
		}
		seen[it.ID] = struct{}{}
		out = append(out, it)
	}
	return out
}

// slug turns a label into a stable id fragment. ASCII labels yield readable
// ids ("weight & fat" -> "weight-fat"); labels with no ASCII word characters
// (e.g. pure Chinese) return "" so the caller can fall back to a positional id.
func slug(s string) string {
	var b strings.Builder
	prevDash := true
	for _, r := range strings.ToLower(s) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			prevDash = false
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			// Non-ASCII letter/digit: skip, it cannot appear in a portable id.
			continue
		case r == ' ' || r == '-' || r == '_' || r == '&' || r == '/':
			if !prevDash {
				b.WriteRune('-')
				prevDash = true
			}
		}
	}
	return strings.Trim(b.String(), "-")
}

func expandPath(p string) (string, error) {
	if p == "~" || strings.HasPrefix(p, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		if p == "~" {
			return home, nil
		}
		return filepath.Join(home, strings.TrimPrefix(p, "~/")), nil
	}
	return p, nil
}
