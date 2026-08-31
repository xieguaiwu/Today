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
// object ({"id":"eyes","label":"Eyes","hint":"远眺 20 分钟","color":"#3B82F6",
// "group":"学习"}). The bare form keeps the zero-config promise; the object
// form adds a stable id (so history survives a rename), an optional hint, and
// the colour/group used by the visualisations.
type Item struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Hint  string `json:"hint,omitempty"`
	Color string `json:"color,omitempty"`
	Group string `json:"group,omitempty"`
	// Steps is how many sub-steps make this habit "done" for the day.
	// 0 or 1 means a plain checkbox, so existing configs are unaffected.
	Steps int `json:"steps,omitempty"`
}

// MaxSteps bounds how many sub-steps one habit may have: enough for any real
// routine, small enough that the bar and the heatmap levels stay legible.
const MaxSteps = 12

// Steps returns the effective step count, at least 1.
func (it Item) StepsOrDefault() int {
	if it.Steps < 1 {
		return 1
	}
	return it.Steps
}

// Palette is used for items that do not declare a colour. It echoes the
// saturated-on-dark scheme of the source app.
var Palette = []string{
	"#3B82F6", // blue
	"#84CC16", // lime
	"#F87171", // red
	"#2563EB", // deep blue
	"#EF4444", // bright red
	"#A855F7", // purple
	"#22C55E", // green
	"#EAB308", // yellow
	"#EC4899", // pink
	"#06B6D4", // cyan
}

// Color returns the item's colour, falling back to the palette by position.
func (it Item) ColorOrDefault(idx int) string {
	if it.Color != "" {
		return it.Color
	}
	if len(Palette) == 0 {
		return ""
	}
	return Palette[idx%len(Palette)]
}

// Config is the on-disk configuration of Today.
type Config struct {
	Title string `json:"title,omitempty"`
	Items []Item `json:"items"`
	// HistoryDays bounds how far back per-day records are kept. 0 = default.
	HistoryDays int `json:"history_days,omitempty"`
}

// DefaultConfig is what gets written on first run. It leads with the five
// habits migrated from the source streak app, then keeps the seven health
// self-check items that were hardcoded in v0.1.0.
func DefaultConfig() Config {
	items := []Item{
		{Label: "anki单词", Hint: "Anki 新词 + 复习", Color: "#3B82F6", Group: "学习"},
		{Label: "USACO", Hint: "算法训练/比赛题", Color: "#84CC16", Group: "学习"},
		{Label: "无氧锻炼", Hint: "力量训练", Color: "#F87171", Group: "健身"},
		{Label: "PMA", Hint: "数学/物理额外练习", Color: "#2563EB", Group: "学习"},
		{Label: "brain", Hint: "专注力/冥想训练", Color: "#EF4444", Group: "健身"},
		{Label: "Eyes"},
		{Label: "Nose"},
		{Label: "Skin"},
		{Label: "Lips"},
		{Label: "Anxiety"},
		{Label: "Cognition"},
		{Label: "Weight & Fat"},
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
		Color string `json:"color,omitempty"`
		Group string `json:"group,omitempty"`
		Steps int    `json:"steps,omitempty"`
	}
	var a itemAlias
	if err := json.Unmarshal(data, &a); err != nil {
		return err
	}
	it.ID = strings.TrimSpace(a.ID)
	it.Hint = strings.TrimSpace(a.Hint)
	it.Color = strings.TrimSpace(a.Color)
	it.Group = strings.TrimSpace(a.Group)
	it.Steps = a.Steps
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
		Color string `json:"color,omitempty"`
		Group string `json:"group,omitempty"`
		Steps int    `json:"steps,omitempty"`
	}
	return encodeNoHTMLEscape(itemAlias{
		ID: it.ID, Label: it.Label, Hint: it.Hint, Color: it.Color, Group: it.Group, Steps: it.Steps,
	})
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
	for _, it := range cfg.Items {
		if it.Color != "" {
			if err := validateColor(it.Color); err != nil {
				return Config{}, fmt.Errorf("parse %s: item %q: %w", path, it.Label, err)
			}
		}
		if it.Steps < 0 || it.Steps > MaxSteps {
			return Config{}, fmt.Errorf("parse %s: item %q: steps must be 0..%d, got %d", path, it.Label, MaxSteps, it.Steps)
		}
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

// validateColor accepts #RGB and #RRGGBB hex colours.
func validateColor(c string) error {
	s := strings.TrimPrefix(c, "#")
	if len(s) != 3 && len(s) != 6 {
		return fmt.Errorf("invalid color %q (want #RGB or #RRGGBB)", c)
	}
	for _, r := range s {
		isHex := (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')
		if !isHex {
			return fmt.Errorf("invalid color %q (want #RGB or #RRGGBB)", c)
		}
	}
	return nil
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
