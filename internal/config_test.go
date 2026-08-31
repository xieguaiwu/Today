package internal

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultConfigKeepsV010ItemsAndAddsMigratedHabits(t *testing.T) {
	cfg := DefaultConfig()

	// The five habits migrated from the source streak app lead the list, each
	// with a colour and a group.
	type want struct {
		label string
		color string
		group string
	}
	migrated := []want{
		{"anki单词", "#3B82F6", "学习"},
		{"USACO", "#84CC16", "学习"},
		{"无氧锻炼", "#F87171", "健身"},
		{"PMA", "#2563EB", "学习"},
		{"brain", "#EF4444", "健身"},
	}
	for i, w := range migrated {
		if cfg.Items[i].Label != w.label {
			t.Errorf("item %d = %q, want %q", i, cfg.Items[i].Label, w.label)
		}
		if cfg.Items[i].Color != w.color {
			t.Errorf("item %q color = %q, want %q", w.label, cfg.Items[i].Color, w.color)
		}
		if cfg.Items[i].Group != w.group {
			t.Errorf("item %q group = %q, want %q", w.label, cfg.Items[i].Group, w.group)
		}
	}

	// The seven items hardcoded in v0.1.0 must still be present, so upgrading
	// users do not silently lose their health self-check.
	health := []string{"Eyes", "Nose", "Skin", "Lips", "Anxiety", "Cognition", "Weight & Fat"}
	if len(cfg.Items) != len(migrated)+len(health) {
		t.Fatalf("got %d items, want %d", len(cfg.Items), len(migrated)+len(health))
	}
	for i, w := range health {
		got := cfg.Items[len(migrated)+i]
		if got.Label != w {
			t.Errorf("health item %d = %q, want %q", i, got.Label, w)
		}
		if got.ID == "" {
			t.Errorf("health item %d (%s) has no id", i, w)
		}
	}
}

func TestLoadConfigCreatesFileOnFirstRun(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "config.json")
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if len(cfg.Items) != len(DefaultConfig().Items) {
		t.Fatalf("got %d items, want %d", len(cfg.Items), len(DefaultConfig().Items))
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("default config was not written: %v", err)
	}
	if !strings.Contains(string(raw), "Weight & Fat") {
		t.Errorf("written config missing items:\n%s", raw)
	}
	// Regression lock: the encoder must not HTML-escape, or hand-editing the
	// config means reading \u0026 instead of &.
	if strings.Contains(string(raw), "\\u0026") {
		t.Errorf("config written with HTML escaping, not hand-editable:\n%s", raw)
	}
	// Second load must read the file, not regenerate it.
	if _, err := LoadConfig(path); err != nil {
		t.Fatalf("second LoadConfig: %v", err)
	}
}

func TestItemsAcceptBareStringsAndObjects(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	body := `{
	  "title": "我的清单",
	  "items": [
	    "喝水",
	    {"label": "Floss", "hint": "牙线 30 秒"},
	    {"name": "alias-name"},
	    {"text": "alias-text"},
	    {"id": "custom-id", "label": "Read"}
	  ]
	}`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.Title != "我的清单" {
		t.Errorf("title = %q", cfg.Title)
	}
	if len(cfg.Items) != 5 {
		t.Fatalf("got %d items, want 5: %+v", len(cfg.Items), cfg.Items)
	}
	for i, want := range []string{"喝水", "Floss", "alias-name", "alias-text", "Read"} {
		if cfg.Items[i].Label != want {
			t.Errorf("item %d label = %q, want %q", i, cfg.Items[i].Label, want)
		}
	}
	if cfg.Items[1].Hint != "牙线 30 秒" {
		t.Errorf("hint not parsed: %q", cfg.Items[1].Hint)
	}
	if cfg.Items[4].ID != "custom-id" {
		t.Errorf("explicit id overridden: %q", cfg.Items[4].ID)
	}
	// A pure-CJK label cannot slug to ASCII, so it must get a positional id.
	if cfg.Items[0].ID == "" {
		t.Error("bare CJK item got no id")
	}
	if cfg.Items[0].ID == cfg.Items[1].ID {
		t.Error("ids collided")
	}
}

func TestRoundTripPreservesItems(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	cfg := DefaultConfig()
	cfg.Items[0].Hint = "远眺 20 分钟"
	if err := WriteConfig(path, cfg); err != nil {
		t.Fatal(err)
	}
	got, err := LoadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Items[0].Hint != "远眺 20 分钟" {
		t.Errorf("hint lost in round trip: %+v", got.Items[0])
	}
	if len(got.Items) != len(cfg.Items) {
		t.Errorf("item count changed: %d -> %d", len(cfg.Items), len(got.Items))
	}
	if got.HistoryDays != DefaultRetentionDays {
		t.Errorf("history_days = %d, want %d", got.HistoryDays, DefaultRetentionDays)
	}
}

func TestBlankItemsDroppedAndDuplicateIDSplit(t *testing.T) {
	in := []Item{{Label: "Eyes"}, {Label: "   "}, {Label: "Eyes"}, {Label: "Eyes"}}
	out := normalizeItems(in)
	if len(out) != 3 {
		t.Fatalf("blank label not dropped: %+v", out)
	}
	seen := map[string]bool{}
	for _, it := range out {
		if seen[it.ID] {
			t.Fatalf("duplicate id %q survived normalization", it.ID)
		}
		seen[it.ID] = true
	}
	// The first occurrence keeps the clean id; later ones get suffixed.
	if out[0].ID != "eyes" {
		t.Errorf("first id = %q, want eyes", out[0].ID)
	}
	if out[1].ID != "eyes-2" || out[2].ID != "eyes-3" {
		t.Errorf("suffixing wrong: %q %q", out[1].ID, out[2].ID)
	}
}

func TestSlug(t *testing.T) {
	cases := map[string]string{
		"Weight & Fat":   "weight-fat",
		"Eyes":           "eyes",
		"  Lead  Space ": "lead-space",
		"喝水":             "",
		"20 分钟 walk":     "20-walk",
	}
	for in, want := range cases {
		if got := slug(in); got != want {
			t.Errorf("slug(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestMalformedConfigIsAnErrorNotSilentDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{"items": [`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadConfig(path); err == nil {
		t.Fatal("truncated JSON must error, not fall back to defaults")
	}
}

func TestEmptyItemsListIsAnError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{"items": []}`), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadConfig(path)
	if err == nil {
		t.Fatal("empty items must error")
	}
	if !strings.Contains(err.Error(), "delete the file") {
		t.Errorf("error should tell the user how to recover: %v", err)
	}
}

func TestTooManyItemsRejected(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	items := make([]Item, MaxItems+1)
	for i := range items {
		items[i] = Item{Label: "x" + string(rune('a'+i%26)) + string(rune('a'+i/26))}
	}
	raw, _ := json.Marshal(Config{Items: items})
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadConfig(path); err == nil {
		t.Fatalf("configs above %d items must be rejected", MaxItems)
	}
}

func TestConfigPathPrecedence(t *testing.T) {
	t.Run("flag wins", func(t *testing.T) {
		t.Setenv("TODAY_CONFIG", "/from/env")
		got, err := ConfigPath("/from/flag")
		if err != nil || got != "/from/flag" {
			t.Fatalf("got %q err %v", got, err)
		}
	})
	t.Run("env beats xdg", func(t *testing.T) {
		t.Setenv("TODAY_CONFIG", "/from/env")
		t.Setenv("XDG_CONFIG_HOME", "/xdg")
		got, err := ConfigPath("")
		if err != nil || got != "/from/env" {
			t.Fatalf("got %q err %v", got, err)
		}
	})
	t.Run("xdg used", func(t *testing.T) {
		t.Setenv("TODAY_CONFIG", "")
		t.Setenv("XDG_CONFIG_HOME", "/xdg")
		got, err := ConfigPath("")
		if err != nil {
			t.Fatal(err)
		}
		if filepath.ToSlash(got) != "/xdg/Today/config.json" {
			t.Fatalf("got %q", got)
		}
	})
	t.Run("tilde expanded", func(t *testing.T) {
		home, err := os.UserHomeDir()
		if err != nil {
			t.Skip("no home dir")
		}
		got, err := ConfigPath("~/x/config.json")
		if err != nil {
			t.Fatal(err)
		}
		if got != filepath.Join(home, "x/config.json") {
			t.Fatalf("got %q", got)
		}
	})
}
