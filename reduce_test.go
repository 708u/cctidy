package cctidy

import (
	"slices"
	"testing"
)

func TestIsBareToolName(t *testing.T) {
	t.Parallel()
	tests := []struct {
		entry string
		want  bool
	}{
		{"Bash", true},
		{"Read", true},
		{"Write", true},
		{"Edit", true},
		{"WebFetch", true},
		{"Bash(git add *)", false},
		{"Read(/path)", false},
		{"mcp__slack", false},
		{"mcp__slack__post", false},
		{"", false},
		{"123", false},
		{"mcp__plugin_github_github__tool", false},
	}
	for _, tt := range tests {
		t.Run(tt.entry, func(t *testing.T) {
			t.Parallel()
			if got := isBareToolName(tt.entry); got != tt.want {
				t.Errorf("isBareToolName(%q) = %v, want %v", tt.entry, got, tt.want)
			}
		})
	}
}

func TestIsBareMCPServer(t *testing.T) {
	t.Parallel()
	tests := []struct {
		entry string
		want  bool
	}{
		{"mcp__slack", true},
		{"mcp__github", true},
		{"mcp__slack__post_message", false},
		{"mcp__plugin_github_github__tool", false},
		{"mcp__slack__tool(query)", false},
		{"Bash", false},
		{"Read(/path)", false},
		{"mcp__", false},
		{"", false},
	}
	for _, tt := range tests {
		t.Run(tt.entry, func(t *testing.T) {
			t.Parallel()
			if got := isBareMCPServer(tt.entry); got != tt.want {
				t.Errorf("isBareMCPServer(%q) = %v, want %v", tt.entry, got, tt.want)
			}
		})
	}
}

func TestDedupReducer(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		entries     []string
		wantEntries []string
		wantRemoved int
		wantMsgLen  int
	}{
		{
			name:        "no duplicates",
			entries:     []string{"Read", "Write", "Bash(git *)"},
			wantEntries: []string{"Read", "Write", "Bash(git *)"},
			wantRemoved: 0,
			wantMsgLen:  0,
		},
		{
			name:        "single duplicate",
			entries:     []string{"Read", "Write", "Read"},
			wantEntries: []string{"Read", "Write"},
			wantRemoved: 1,
			wantMsgLen:  1,
		},
		{
			name:        "multiple kinds of duplicates",
			entries:     []string{"Read", "Write", "Read", "Write", "Bash"},
			wantEntries: []string{"Read", "Write", "Bash"},
			wantRemoved: 2,
			wantMsgLen:  2,
		},
		{
			name:        "triple duplicate",
			entries:     []string{"Read", "Read", "Read"},
			wantEntries: []string{"Read"},
			wantRemoved: 2,
			wantMsgLen:  1,
		},
		{
			name:        "empty list",
			entries:     []string{},
			wantEntries: []string{},
			wantRemoved: 0,
			wantMsgLen:  0,
		},
		{
			name:        "single entry",
			entries:     []string{"Bash"},
			wantEntries: []string{"Bash"},
			wantRemoved: 0,
			wantMsgLen:  0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			d := &DedupReducer{}
			got, result := d.Reduce(t.Context(), tt.entries)
			if !slices.Equal(got, tt.wantEntries) {
				t.Errorf("entries = %v, want %v", got, tt.wantEntries)
			}
			if result.Removed != tt.wantRemoved {
				t.Errorf("Removed = %d, want %d", result.Removed, tt.wantRemoved)
			}
			if len(result.Msgs) != tt.wantMsgLen {
				t.Errorf("Msgs len = %d, want %d: %v", len(result.Msgs), tt.wantMsgLen, result.Msgs)
			}
		})
	}
}

func TestSubsumptionReducer(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		entries     []string
		wantEntries []string
		wantRemoved int
		wantMsgLen  int
	}{
		{
			name:        "bare tool subsumes specifier",
			entries:     []string{"Bash", "Bash(git add *)"},
			wantEntries: []string{"Bash"},
			wantRemoved: 1,
			wantMsgLen:  1,
		},
		{
			name:        "bare tool does not subsume itself",
			entries:     []string{"Bash", "Read"},
			wantEntries: []string{"Bash", "Read"},
			wantRemoved: 0,
			wantMsgLen:  0,
		},
		{
			name:        "bare MCP subsumes specific tool",
			entries:     []string{"mcp__github", "mcp__github__create_pr"},
			wantEntries: []string{"mcp__github"},
			wantRemoved: 1,
			wantMsgLen:  1,
		},
		{
			name:        "bare MCP subsumes tool with parens",
			entries:     []string{"mcp__github", "mcp__github__search_code(query)"},
			wantEntries: []string{"mcp__github"},
			wantRemoved: 1,
			wantMsgLen:  1,
		},
		{
			name:        "bare MCP does not subsume itself",
			entries:     []string{"mcp__github", "mcp__slack"},
			wantEntries: []string{"mcp__github", "mcp__slack"},
			wantRemoved: 0,
			wantMsgLen:  0,
		},
		{
			name:        "plugin MCP not subsumed",
			entries:     []string{"mcp__github", "mcp__plugin_github_github__tool"},
			wantEntries: []string{"mcp__github", "mcp__plugin_github_github__tool"},
			wantRemoved: 0,
			wantMsgLen:  0,
		},
		{
			name:        "multiple bare tools subsume multiple entries",
			entries:     []string{"Bash", "Read", "Bash(git *)", "Read(/path)", "Write"},
			wantEntries: []string{"Bash", "Read", "Write"},
			wantRemoved: 2,
			wantMsgLen:  2,
		},
		{
			name:        "no subsumption without bare entry",
			entries:     []string{"Bash(git *)", "Bash(npm *)"},
			wantEntries: []string{"Bash(git *)", "Bash(npm *)"},
			wantRemoved: 0,
			wantMsgLen:  0,
		},
		{
			name:        "empty list",
			entries:     []string{},
			wantEntries: []string{},
			wantRemoved: 0,
			wantMsgLen:  0,
		},
		{
			name:        "single entry",
			entries:     []string{"Read"},
			wantEntries: []string{"Read"},
			wantRemoved: 0,
			wantMsgLen:  0,
		},
		{
			name:        "bare MCP with multiple tools subsumed",
			entries:     []string{"mcp__slack", "mcp__slack__post_message", "mcp__slack__list_channels"},
			wantEntries: []string{"mcp__slack"},
			wantRemoved: 2,
			wantMsgLen:  2,
		},
		{
			name:        "bare MCP with wildcard notation subsumed",
			entries:     []string{"mcp__github", "mcp__github__*"},
			wantEntries: []string{"mcp__github"},
			wantRemoved: 1,
			wantMsgLen:  1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			s := &SubsumptionReducer{}
			got, result := s.Reduce(t.Context(), tt.entries)
			if !slices.Equal(got, tt.wantEntries) {
				t.Errorf("entries = %v, want %v", got, tt.wantEntries)
			}
			if result.Removed != tt.wantRemoved {
				t.Errorf("Removed = %d, want %d", result.Removed, tt.wantRemoved)
			}
			if len(result.Msgs) != tt.wantMsgLen {
				t.Errorf("Msgs len = %d, want %d: %v", len(result.Msgs), tt.wantMsgLen, result.Msgs)
			}
		})
	}
}

func TestSplitStringEntries(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		arr        []any
		wantStrs   []string
		wantOthers []any
	}{
		{
			name:       "all strings",
			arr:        []any{"Read", "Write"},
			wantStrs:   []string{"Read", "Write"},
			wantOthers: nil,
		},
		{
			name:       "mixed types",
			arr:        []any{"Read", 42, "Write"},
			wantStrs:   []string{"Read", "Write"},
			wantOthers: []any{42},
		},
		{
			name:       "empty",
			arr:        []any{},
			wantStrs:   nil,
			wantOthers: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			strs, others := splitStringEntries(tt.arr)
			if !slices.Equal(strs, tt.wantStrs) {
				t.Errorf("strs = %v, want %v", strs, tt.wantStrs)
			}
			if len(others) != len(tt.wantOthers) {
				t.Errorf("others len = %d, want %d", len(others), len(tt.wantOthers))
			}
		})
	}
}
