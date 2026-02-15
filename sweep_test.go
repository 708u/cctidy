package cctidy

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/708u/cctidy/internal/set"
	"github.com/708u/cctidy/internal/testutil"
)

func TestExtractToolEntry(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		entry string
		want  ToolEntry
	}{
		{
			name:  "Bash tool",
			entry: "Bash(git -C /repo status)",
			want:  StandardEntry{Tool: ToolBash, Specifier: "git -C /repo status"},
		},
		{
			name:  "Write tool",
			entry: "Write(/some/path)",
			want:  StandardEntry{Tool: ToolWrite, Specifier: "/some/path"},
		},
		{
			name:  "Read tool",
			entry: "Read(/some/path)",
			want:  StandardEntry{Tool: ToolRead, Specifier: "/some/path"},
		},
		{
			name:  "mcp tool routed to MCPEntry",
			entry: "mcp__github__search_code(query)",
			want:  MCPEntry{ServerName: "github", RawEntry: "mcp__github__search_code(query)"},
		},
		{
			name:  "bare mcp entry routed to MCPEntry",
			entry: "mcp__slack__post_message",
			want:  MCPEntry{ServerName: "slack", RawEntry: "mcp__slack__post_message"},
		},
		{
			name:  "mcp plugin entry returns nil",
			entry: "mcp__plugin_github_github__search_code",
			want:  nil,
		},
		{
			name:  "bare tool name without parens",
			entry: "Bash",
			want:  nil,
		},
		{
			name:  "empty string",
			entry: "",
			want:  nil,
		},
		{
			name:  "starts with number",
			entry: "1Tool(arg)",
			want:  nil,
		},
		{
			name:  "WebFetch tool",
			entry: "WebFetch(domain:github.com)",
			want:  StandardEntry{Tool: "WebFetch", Specifier: "domain:github.com"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := extractToolEntry(tt.entry)
			if got != tt.want {
				t.Errorf("extractToolEntry(%q) = %v, want %v", tt.entry, got, tt.want)
			}
		})
	}
}

func TestContainsGlob(t *testing.T) {
	t.Parallel()
	tests := []struct {
		s    string
		want bool
	}{
		{"**/*.ts", true},
		{"/path/to/file", false},
		{"src/[a-z]/*.go", true},
		{"file?.txt", true},
		{"normal/path", false},
		{"", false},
	}
	for _, tt := range tests {
		t.Run(tt.s, func(t *testing.T) {
			t.Parallel()
			if got := containsGlob(tt.s); got != tt.want {
				t.Errorf("containsGlob(%q) = %v, want %v", tt.s, got, tt.want)
			}
		})
	}
}

func TestExtractAbsolutePaths(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{
			name:  "single absolute path",
			input: "git -C /home/user/repo status",
			want:  []string{"/home/user/repo"},
		},
		{
			name:  "glob stops extraction",
			input: "npm run *",
			want:  nil,
		},
		{
			name:  "shell metachar stops at &&",
			input: "cd /path && make",
			want:  []string{"/path"},
		},
		{
			name:  "semicolon stops extraction",
			input: "cd /path;make",
			want:  []string{"/path"},
		},
		{
			name:  "equals-prefixed path",
			input: "--config=/etc/app.conf",
			want:  []string{"/etc/app.conf"},
		},
		{
			name:  "multiple paths",
			input: "cp /src/a /dst/b",
			want:  []string{"/src/a", "/dst/b"},
		},
		{
			name:  "no absolute paths",
			input: "echo hello",
			want:  nil,
		},
		{
			name:  "trailing slash trimmed",
			input: "ls /some/dir/",
			want:  []string{"/some/dir"},
		},
		{
			name:  "trailing dot preserved",
			input: "ls /some/path.",
			want:  []string{"/some/path."},
		},
		{
			name:  "root only path filtered",
			input: "ls /",
			want:  nil,
		},
		{
			name:  "path with underscores and dots",
			input: "cat /home/user/.config/app_v2/settings.json",
			want:  []string{"/home/user/.config/app_v2/settings.json"},
		},
		{
			name:  "dot-slash relative path is not extracted",
			input: "./bin/run",
			want:  nil,
		},
		{
			name:  "dot-dot-slash relative path is not extracted",
			input: "../src/main.go",
			want:  nil,
		},
		{
			name:  "tilde home path is not extracted",
			input: "cat ~/config.json",
			want:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := extractAbsolutePaths(tt.input)
			if len(got) != len(tt.want) {
				t.Fatalf("extractAbsolutePaths(%q) = %v, want %v", tt.input, got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("extractAbsolutePaths(%q)[%d] = %q, want %q", tt.input, i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestExtractRelativePaths(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{
			name:  "dot-slash path",
			input: "cat ./src/main.go",
			want:  []string{"./src/main.go"},
		},
		{
			name:  "dot-dot-slash path",
			input: "cat ../other/file",
			want:  []string{"../other/file"},
		},
		{
			name:  "tilde path",
			input: "cat ~/config.json",
			want:  []string{"~/config.json"},
		},
		{
			name:  "mixed relative paths",
			input: "cat ~/file ./local ../parent",
			want:  []string{"~/file", "./local", "../parent"},
		},
		{
			name:  "bare relative path not extracted",
			input: "cat bare/path",
			want:  nil,
		},
		{
			name:  "no paths",
			input: "echo hello",
			want:  nil,
		},
		{
			name:  "trailing slash trimmed",
			input: "ls ./dir/",
			want:  []string{"./dir"},
		},
		{
			name:  "trailing dot preserved",
			input: "ls ./path.",
			want:  []string{"./path."},
		},
		{
			name:  "equals-prefixed dot-slash",
			input: "--config=./app.conf",
			want:  []string{"./app.conf"},
		},
		{
			name:  "equals-prefixed tilde",
			input: "--config=~/app.conf",
			want:  []string{"~/app.conf"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := extractRelativePaths(tt.input)
			if len(got) != len(tt.want) {
				t.Fatalf("extractRelativePaths(%q) = %v, want %v", tt.input, got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("extractRelativePaths(%q)[%d] = %q, want %q", tt.input, i, got[i], tt.want[i])
				}
			}
		})
	}
}

var noExcludes = NewBashExcluder(BashSweepConfig{})

func TestBashToolSweeperShouldSweep(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		sweeper   *BashToolSweeper
		specifier string
		wantSweep bool
	}{
		{
			name:      "all paths dead",
			sweeper:   &BashToolSweeper{checker: testutil.NoPathsExist{}, excluder: noExcludes},
			specifier: "git -C /dead/repo status",
			wantSweep: true,
		},
		{
			name:      "one path alive",
			sweeper:   &BashToolSweeper{checker: testutil.CheckerFor("/alive/src"), excluder: noExcludes},
			specifier: "cp /alive/src /dead/dst",
			wantSweep: false,
		},
		{
			name:      "no absolute paths keeps entry",
			sweeper:   &BashToolSweeper{checker: testutil.NoPathsExist{}, excluder: noExcludes},
			specifier: "npm run *",
			wantSweep: false,
		},
		{
			name:      "all paths alive",
			sweeper:   &BashToolSweeper{checker: testutil.AllPathsExist{}, excluder: noExcludes},
			specifier: "cp /src/a /dst/b",
			wantSweep: false,
		},
		{
			name:      "multiple dead paths",
			sweeper:   &BashToolSweeper{checker: testutil.NoPathsExist{}, excluder: noExcludes},
			specifier: "cp /dead/a /dead/b",
			wantSweep: true,
		},
		{
			name:      "tilde path dead with homeDir",
			sweeper:   &BashToolSweeper{checker: testutil.NoPathsExist{}, homeDir: "/home/user", excluder: noExcludes},
			specifier: "cat ~/dead/config",
			wantSweep: true,
		},
		{
			name:      "tilde path alive with homeDir",
			sweeper:   &BashToolSweeper{checker: testutil.CheckerFor("/home/user/alive/config"), homeDir: "/home/user", excluder: noExcludes},
			specifier: "cat ~/alive/config",
			wantSweep: false,
		},
		{
			name:      "tilde path without homeDir is skipped",
			sweeper:   &BashToolSweeper{checker: testutil.NoPathsExist{}, excluder: noExcludes},
			specifier: "cat ~/config",
			wantSweep: false,
		},
		{
			name:      "dot-slash path dead with baseDir",
			sweeper:   &BashToolSweeper{checker: testutil.NoPathsExist{}, baseDir: "/project", excluder: noExcludes},
			specifier: "cat ./src/main.go",
			wantSweep: true,
		},
		{
			name:      "dot-slash path without baseDir is skipped",
			sweeper:   &BashToolSweeper{checker: testutil.NoPathsExist{}, excluder: noExcludes},
			specifier: "cat ./src/main.go",
			wantSweep: false,
		},
		{
			name:      "dot-dot-slash path dead with baseDir",
			sweeper:   &BashToolSweeper{checker: testutil.NoPathsExist{}, baseDir: "/project", excluder: noExcludes},
			specifier: "cat ../other/file",
			wantSweep: true,
		},
		{
			name:      "mixed absolute and relative all dead",
			sweeper:   &BashToolSweeper{checker: testutil.NoPathsExist{}, homeDir: "/home/user", baseDir: "/project", excluder: noExcludes},
			specifier: "cp /dead/src ./dead/dst",
			wantSweep: true,
		},
		{
			name:      "mixed absolute and relative one alive",
			sweeper:   &BashToolSweeper{checker: testutil.CheckerFor("/alive/src"), homeDir: "/home/user", baseDir: "/project", excluder: noExcludes},
			specifier: "cp /alive/src ./dead/dst",
			wantSweep: false,
		},
		{
			name:      "only unresolvable relative paths keeps entry",
			sweeper:   &BashToolSweeper{checker: testutil.NoPathsExist{}, excluder: noExcludes},
			specifier: "cat ./local ../parent ~/home",
			wantSweep: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			entry := StandardEntry{Tool: ToolBash, Specifier: tt.specifier}
			result := tt.sweeper.ShouldSweep(t.Context(), entry)
			if result.Sweep != tt.wantSweep {
				t.Errorf("ShouldSweep(%q) = %v, want %v", tt.specifier, result.Sweep, tt.wantSweep)
			}
		})
	}
}

func TestBashExcluderIsExcluded(t *testing.T) {
	t.Parallel()

	excl := NewBashExcluder(BashSweepConfig{
		ExcludeEntries:  []string{"mkdir -p /opt/myapp/logs", "touch /opt/myapp/.initialized"},
		ExcludeCommands: []string{"mkdir", "touch", "ln"},
		ExcludePaths:    []string{"/opt/myapp/", "/var/log/myapp/"},
	})

	tests := []struct {
		name      string
		specifier string
		want      bool
	}{
		{
			name:      "exact entry match",
			specifier: "mkdir -p /opt/myapp/logs",
			want:      true,
		},
		{
			name:      "exact entry no match",
			specifier: "cat /other/path",
			want:      false,
		},
		{
			name:      "command match mkdir",
			specifier: "mkdir /some/new/dir",
			want:      true,
		},
		{
			name:      "command match touch",
			specifier: "touch /some/file",
			want:      true,
		},
		{
			name:      "command match ln",
			specifier: "ln -s /src /dst",
			want:      true,
		},
		{
			name:      "command no match",
			specifier: "git -C /dead/repo status",
			want:      false,
		},
		{
			name:      "path prefix match",
			specifier: "cat /opt/myapp/config/app.conf",
			want:      true,
		},
		{
			name:      "path prefix match var log",
			specifier: "tail /var/log/myapp/error.log",
			want:      true,
		},
		{
			name:      "path prefix no match",
			specifier: "cat /opt/otherapp/config",
			want:      false,
		},
		{
			name:      "path prefix boundary no match",
			specifier: "cat /opt/myappdata/file",
			want:      false,
		},
		{
			name:      "path exact match",
			specifier: "cat /opt/myapp/",
			want:      true,
		},
		{
			name:      "no paths no command match",
			specifier: "npm run build",
			want:      false,
		},
		{
			name:      "single token command match",
			specifier: "mkdir",
			want:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := excl.IsExcluded(tt.specifier, extractAbsolutePaths(tt.specifier)); got != tt.want {
				t.Errorf("IsExcluded(%q) = %v, want %v", tt.specifier, got, tt.want)
			}
		})
	}
}

func TestBashExcluderPathBoundary(t *testing.T) {
	t.Parallel()
	excl := NewBashExcluder(BashSweepConfig{
		ExcludePaths: []string{"/home/user"},
	})
	tests := []struct {
		name      string
		specifier string
		want      bool
	}{
		{
			name:      "exact path",
			specifier: "cat /home/user",
			want:      true,
		},
		{
			name:      "subpath",
			specifier: "cat /home/user/file",
			want:      true,
		},
		{
			name:      "similar prefix different dir",
			specifier: "cat /home/username/file",
			want:      false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := excl.IsExcluded(tt.specifier, extractAbsolutePaths(tt.specifier)); got != tt.want {
				t.Errorf("IsExcluded(%q) = %v, want %v", tt.specifier, got, tt.want)
			}
		})
	}
}

func TestBashExcluderEmpty(t *testing.T) {
	t.Parallel()
	excl := NewBashExcluder(BashSweepConfig{})
	if excl.IsExcluded("git -C /dead/repo status", extractAbsolutePaths("git -C /dead/repo status")) {
		t.Error("empty excluder should not exclude anything")
	}
}

func TestBashToolSweeperWithExcluder(t *testing.T) {
	t.Parallel()

	excl := NewBashExcluder(BashSweepConfig{
		ExcludeCommands: []string{"mkdir"},
	})

	tests := []struct {
		name      string
		sweeper   *BashToolSweeper
		specifier string
		wantSweep bool
	}{
		{
			name:      "excluded command keeps entry even with dead paths",
			sweeper:   &BashToolSweeper{checker: testutil.NoPathsExist{}, excluder: excl},
			specifier: "mkdir -p /dead/path",
			wantSweep: false,
		},
		{
			name:      "non-excluded command with dead paths is swept",
			sweeper:   &BashToolSweeper{checker: testutil.NoPathsExist{}, excluder: excl},
			specifier: "git -C /dead/repo status",
			wantSweep: true,
		},
		{
			name:      "empty excluder does not affect sweeping",
			sweeper:   &BashToolSweeper{checker: testutil.NoPathsExist{}, excluder: noExcludes},
			specifier: "git -C /dead/repo status",
			wantSweep: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			entry := StandardEntry{Tool: ToolBash, Specifier: tt.specifier}
			result := tt.sweeper.ShouldSweep(t.Context(), entry)
			if result.Sweep != tt.wantSweep {
				t.Errorf("ShouldSweep(%q) = %v, want %v", tt.specifier, result.Sweep, tt.wantSweep)
			}
		})
	}
}

func TestTaskToolSweeperShouldSweep(t *testing.T) {
	t.Parallel()

	agentsWithMyAgent := set.New("my-agent")
	agentsWithProjAgent := set.New("proj-agent")
	emptyAgents := set.New[string]()

	tests := []struct {
		name      string
		sweeper   *TaskToolSweeper
		specifier string
		wantSweep bool
	}{
		{
			name:      "built-in Explore is kept",
			sweeper:   NewTaskToolSweeper(emptyAgents),
			specifier: "Explore",
			wantSweep: false,
		},
		{
			name:      "built-in statusline-setup is kept",
			sweeper:   NewTaskToolSweeper(emptyAgents),
			specifier: "statusline-setup",
			wantSweep: false,
		},
		{
			name:      "plugin agent with colon is kept",
			sweeper:   NewTaskToolSweeper(emptyAgents),
			specifier: "plugin:agent",
			wantSweep: false,
		},
		{
			name:      "agent in name set is kept",
			sweeper:   NewTaskToolSweeper(agentsWithMyAgent),
			specifier: "my-agent",
			wantSweep: false,
		},
		{
			name:      "agent not in name set is swept",
			sweeper:   NewTaskToolSweeper(agentsWithProjAgent),
			specifier: "my-agent",
			wantSweep: true,
		},
		{
			name:      "agent with project .md file is kept",
			sweeper:   NewTaskToolSweeper(agentsWithProjAgent),
			specifier: "proj-agent",
			wantSweep: false,
		},
		{
			name:      "dead agent is swept when agents set non-empty",
			sweeper:   NewTaskToolSweeper(set.New("other-agent")),
			specifier: "dead-agent",
			wantSweep: true,
		},
		{
			name:      "frontmatter name is kept",
			sweeper:   NewTaskToolSweeper(set.New("custom-name")),
			specifier: "custom-name",
			wantSweep: false,
		},
		{
			name:      "nil agents keeps entry conservatively",
			sweeper:   NewTaskToolSweeper(nil),
			specifier: "unknown",
			wantSweep: false,
		},
		{
			name:      "empty agents keeps unknown conservatively",
			sweeper:   NewTaskToolSweeper(emptyAgents),
			specifier: "unknown",
			wantSweep: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			entry := StandardEntry{Tool: ToolTask, Specifier: tt.specifier}
			result := tt.sweeper.ShouldSweep(t.Context(), entry)
			if result.Sweep != tt.wantSweep {
				t.Errorf("ShouldSweep(%q) = %v, want %v", tt.specifier, result.Sweep, tt.wantSweep)
			}
		})
	}
}

func TestReadEditToolSweeperShouldSweep(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		sweeper   ReadEditToolSweeper
		specifier string
		wantSweep bool
	}{
		{
			name:      "absolute path with // prefix is resolved",
			sweeper:   ReadEditToolSweeper{checker: testutil.NoPathsExist{}},
			specifier: "//dead/path",
			wantSweep: true,
		},
		{
			name:      "existing absolute path is kept",
			sweeper:   ReadEditToolSweeper{checker: testutil.CheckerFor("/alive/path")},
			specifier: "//alive/path",
			wantSweep: false,
		},
		{
			name:      "home-relative path with homeDir is resolved",
			sweeper:   ReadEditToolSweeper{checker: testutil.NoPathsExist{}, homeDir: "/home/user"},
			specifier: "~/config.json",
			wantSweep: true,
		},
		{
			name:      "existing home-relative path is kept",
			sweeper:   ReadEditToolSweeper{checker: testutil.CheckerFor("/home/user/config.json"), homeDir: "/home/user"},
			specifier: "~/config.json",
			wantSweep: false,
		},
		{
			name:      "home-relative path without homeDir is skipped",
			sweeper:   ReadEditToolSweeper{checker: testutil.NoPathsExist{}},
			specifier: "~/config.json",
			wantSweep: false,
		},
		{
			name:      "relative path with baseDir is resolved",
			sweeper:   ReadEditToolSweeper{checker: testutil.NoPathsExist{}, baseDir: "/project"},
			specifier: "./src/main.go",
			wantSweep: true,
		},
		{
			name:      "relative path without baseDir is skipped",
			sweeper:   ReadEditToolSweeper{checker: testutil.NoPathsExist{}},
			specifier: "./src/main.go",
			wantSweep: false,
		},
		{
			name:      "glob pattern is skipped",
			sweeper:   ReadEditToolSweeper{checker: testutil.NoPathsExist{}},
			specifier: "**/*.ts",
			wantSweep: false,
		},
		{
			name:      "parent-relative path with baseDir is resolved",
			sweeper:   ReadEditToolSweeper{checker: testutil.NoPathsExist{}, baseDir: "/project"},
			specifier: "../other/file.go",
			wantSweep: true,
		},
		{
			name:      "slash-prefixed path with baseDir is resolved",
			sweeper:   ReadEditToolSweeper{checker: testutil.CheckerFor("/project/src/file.go"), baseDir: "/project"},
			specifier: "/src/file.go",
			wantSweep: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			entry := StandardEntry{Tool: ToolRead, Specifier: tt.specifier}
			result := tt.sweeper.ShouldSweep(t.Context(), entry)
			if result.Sweep != tt.wantSweep {
				t.Errorf("ShouldSweep(%q) = %v, want %v", tt.specifier, result.Sweep, tt.wantSweep)
			}
		})
	}
}

func TestSweepPermissions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		entries        []any
		checker        PathChecker
		homeDir        string
		opts           []SweepOption
		wantAllowLen   int
		wantSweptAllow int
	}{
		{
			name:           "dead absolute path entry is removed",
			entries:        []any{"Read(//dead/path)"},
			checker:        testutil.NoPathsExist{},
			wantAllowLen:   0,
			wantSweptAllow: 1,
		},
		{
			name:           "existing absolute path entry is kept",
			entries:        []any{"Read(//alive/path)"},
			checker:        testutil.CheckerFor("/alive/path"),
			wantAllowLen:   1,
			wantSweptAllow: 0,
		},
		{
			name:           "home-relative path with homeDir is swept when dead",
			entries:        []any{"Read(~/dead/config)"},
			checker:        testutil.NoPathsExist{},
			homeDir:        "/home/user",
			wantAllowLen:   0,
			wantSweptAllow: 1,
		},
		{
			name:           "home-relative path with homeDir is kept when exists",
			entries:        []any{"Read(~/config)"},
			checker:        testutil.CheckerFor("/home/user/config"),
			homeDir:        "/home/user",
			wantAllowLen:   1,
			wantSweptAllow: 0,
		},
		{
			name:           "relative path without baseDir is kept",
			entries:        []any{"Edit(/src/file.go)"},
			checker:        testutil.NoPathsExist{},
			wantAllowLen:   1,
			wantSweptAllow: 0,
		},
		{
			name:           "relative path with baseDir is swept when dead",
			entries:        []any{"Edit(./src/file.go)"},
			checker:        testutil.NoPathsExist{},
			opts:           []SweepOption{WithBaseDir("/project")},
			wantAllowLen:   0,
			wantSweptAllow: 1,
		},
		{
			name:           "glob pattern entry is kept",
			entries:        []any{"Read(**/*.ts)"},
			checker:        testutil.NoPathsExist{},
			wantAllowLen:   1,
			wantSweptAllow: 0,
		},
		{
			name: "Write entries are never swept",
			entries: []any{
				"Write(//dead/path)",
				"Write(~/dead/notes.md)",
			},
			checker:        testutil.NoPathsExist{},
			homeDir:        "/home/user",
			wantAllowLen:   2,
			wantSweptAllow: 0,
		},
		{
			name: "unregistered tool entries are kept",
			entries: []any{
				"Bash(git -C /dead/path status)",
				"WebFetch(domain:example.com)",
			},
			checker:        testutil.NoPathsExist{},
			wantAllowLen:   2,
			wantSweptAllow: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			obj := map[string]any{
				"permissions": map[string]any{
					"allow": tt.entries,
				},
			}
			result := NewPermissionSweeper(tt.checker, tt.homeDir, nil, tt.opts...).Sweep(t.Context(), obj)
			allow := obj["permissions"].(map[string]any)["allow"].([]any)
			if len(allow) != tt.wantAllowLen {
				t.Errorf("allow len = %d, want %d, got %v", len(allow), tt.wantAllowLen, allow)
			}
			if result.SweptAllow != tt.wantSweptAllow {
				t.Errorf("SweptAllow = %d, want %d", result.SweptAllow, tt.wantSweptAllow)
			}
		})
	}

	t.Run("relative path with baseDir is kept when exists", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "file.go"), []byte(""), 0o644)

		obj := map[string]any{
			"permissions": map[string]any{
				"allow": []any{"Edit(./file.go)"},
			},
		}
		result := NewPermissionSweeper(testutil.CheckerFor(filepath.Join(dir, "file.go")), "", nil, WithBaseDir(dir)).Sweep(t.Context(), obj)
		allow := obj["permissions"].(map[string]any)["allow"].([]any)
		if len(allow) != 1 {
			t.Errorf("allow should have 1 entry, got %v", allow)
		}
		if result.SweptAllow != 0 {
			t.Errorf("SweptAllow = %d, want 0", result.SweptAllow)
		}
	})

	t.Run("missing permissions key is no-op", func(t *testing.T) {
		t.Parallel()
		obj := map[string]any{"key": "value"}
		result := NewPermissionSweeper(testutil.AllPathsExist{}, "", nil).Sweep(t.Context(), obj)
		if result.SweptAllow != 0 || result.SweptAsk != 0 {
			t.Errorf("expected zero counts, got allow=%d ask=%d",
				result.SweptAllow, result.SweptAsk)
		}
		if len(result.Warns) != 0 {
			t.Errorf("expected no warnings, got %v", result.Warns)
		}
	})

	t.Run("bash entries swept when enabled and all paths dead", func(t *testing.T) {
		t.Parallel()
		obj := map[string]any{
			"permissions": map[string]any{
				"allow": []any{
					"Bash(git -C /dead/repo status)",
					"Bash(npm run *)",
					"Read",
				},
			},
		}
		result := NewPermissionSweeper(testutil.NoPathsExist{}, "", nil, WithBashSweep(BashSweepConfig{})).Sweep(t.Context(), obj)
		allow := obj["permissions"].(map[string]any)["allow"].([]any)
		if len(allow) != 2 {
			t.Errorf("allow len = %d, want 2, got %v", len(allow), allow)
		}
		if result.SweptAllow != 1 {
			t.Errorf("SweptAllow = %d, want 1", result.SweptAllow)
		}
	})

	t.Run("bash entries kept when one path alive", func(t *testing.T) {
		t.Parallel()
		obj := map[string]any{
			"permissions": map[string]any{
				"allow": []any{
					"Bash(cp /alive/src /dead/dst)",
				},
			},
		}
		result := NewPermissionSweeper(testutil.CheckerFor("/alive/src"), "", nil, WithBashSweep(BashSweepConfig{})).Sweep(t.Context(), obj)
		allow := obj["permissions"].(map[string]any)["allow"].([]any)
		if len(allow) != 1 {
			t.Errorf("allow len = %d, want 1, got %v", len(allow), allow)
		}
		if result.SweptAllow != 0 {
			t.Errorf("SweptAllow = %d, want 0", result.SweptAllow)
		}
	})

	t.Run("bash entries kept when sweep-bash not enabled", func(t *testing.T) {
		t.Parallel()
		obj := map[string]any{
			"permissions": map[string]any{
				"allow": []any{
					"Bash(git -C /dead/repo status)",
				},
			},
		}
		result := NewPermissionSweeper(testutil.NoPathsExist{}, "", nil).Sweep(t.Context(), obj)
		allow := obj["permissions"].(map[string]any)["allow"].([]any)
		if len(allow) != 1 {
			t.Errorf("allow len = %d, want 1, got %v", len(allow), allow)
		}
		if result.SweptAllow != 0 {
			t.Errorf("SweptAllow = %d, want 0", result.SweptAllow)
		}
	})

	t.Run("bash entries with tilde path swept when homeDir set and dead", func(t *testing.T) {
		t.Parallel()
		obj := map[string]any{
			"permissions": map[string]any{
				"allow": []any{
					"Bash(cat ~/dead/config)",
				},
			},
		}
		result := NewPermissionSweeper(testutil.NoPathsExist{}, "/home/user", nil, WithBashSweep(BashSweepConfig{})).Sweep(t.Context(), obj)
		allow := obj["permissions"].(map[string]any)["allow"].([]any)
		if len(allow) != 0 {
			t.Errorf("allow len = %d, want 0, got %v", len(allow), allow)
		}
		if result.SweptAllow != 1 {
			t.Errorf("SweptAllow = %d, want 1", result.SweptAllow)
		}
	})

	t.Run("bash entries with dot-slash path swept when baseDir set and dead", func(t *testing.T) {
		t.Parallel()
		obj := map[string]any{
			"permissions": map[string]any{
				"allow": []any{
					"Bash(cat ./dead/file)",
				},
			},
		}
		result := NewPermissionSweeper(testutil.NoPathsExist{}, "", nil, WithBashSweep(BashSweepConfig{}), WithBaseDir("/project")).Sweep(t.Context(), obj)
		allow := obj["permissions"].(map[string]any)["allow"].([]any)
		if len(allow) != 0 {
			t.Errorf("allow len = %d, want 0, got %v", len(allow), allow)
		}
		if result.SweptAllow != 1 {
			t.Errorf("SweptAllow = %d, want 1", result.SweptAllow)
		}
	})

	t.Run("task entries swept when agent dead", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		agentsDir := filepath.Join(dir, ".claude", "agents")
		os.MkdirAll(agentsDir, 0o755)
		os.WriteFile(filepath.Join(agentsDir, "alive-agent.md"), []byte("---\nname: alive-agent\n---\n# Alive"), 0o644)

		obj := map[string]any{
			"permissions": map[string]any{
				"allow": []any{
					"Task(dead-agent)",
					"Task(Explore)",
					"Read",
				},
			},
		}
		result := NewPermissionSweeper(testutil.NoPathsExist{}, "", nil, WithBaseDir(dir)).Sweep(t.Context(), obj)
		allow := obj["permissions"].(map[string]any)["allow"].([]any)
		if len(allow) != 2 {
			t.Errorf("allow len = %d, want 2, got %v", len(allow), allow)
		}
		if result.SweptAllow != 1 {
			t.Errorf("SweptAllow = %d, want 1", result.SweptAllow)
		}
	})

	t.Run("task built-in agent kept", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		agentsDir := filepath.Join(dir, ".claude", "agents")
		os.MkdirAll(agentsDir, 0o755)
		os.WriteFile(filepath.Join(agentsDir, "stub.md"), []byte("---\nname: stub\n---\n# Stub"), 0o644)

		obj := map[string]any{
			"permissions": map[string]any{
				"allow": []any{
					"Task(Explore)",
					"Task(Plan)",
					"Task(general-purpose)",
				},
			},
		}
		result := NewPermissionSweeper(testutil.NoPathsExist{}, "", nil, WithBaseDir(dir)).Sweep(t.Context(), obj)
		allow := obj["permissions"].(map[string]any)["allow"].([]any)
		if len(allow) != 3 {
			t.Errorf("allow len = %d, want 3, got %v", len(allow), allow)
		}
		if result.SweptAllow != 0 {
			t.Errorf("SweptAllow = %d, want 0", result.SweptAllow)
		}
	})

	t.Run("bash entries with relative path kept when no baseDir", func(t *testing.T) {
		t.Parallel()
		obj := map[string]any{
			"permissions": map[string]any{
				"allow": []any{
					"Bash(cat ./local/file)",
				},
			},
		}
		result := NewPermissionSweeper(testutil.NoPathsExist{}, "", nil, WithBashSweep(BashSweepConfig{})).Sweep(t.Context(), obj)
		allow := obj["permissions"].(map[string]any)["allow"].([]any)
		if len(allow) != 1 {
			t.Errorf("allow len = %d, want 1, got %v", len(allow), allow)
		}
		if result.SweptAllow != 0 {
			t.Errorf("SweptAllow = %d, want 0", result.SweptAllow)
		}
	})

	t.Run("deny entries are never swept", func(t *testing.T) {
		t.Parallel()
		obj := map[string]any{
			"permissions": map[string]any{
				"allow": []any{"Read(//dead/allow)"},
				"deny":  []any{"Read(//dead/deny)"},
				"ask":   []any{"Edit(//dead/ask)"},
			},
		}
		result := NewPermissionSweeper(testutil.NoPathsExist{}, "", nil).Sweep(t.Context(), obj)
		if result.SweptAllow != 1 {
			t.Errorf("SweptAllow = %d, want 1", result.SweptAllow)
		}
		if result.SweptAsk != 1 {
			t.Errorf("SweptAsk = %d, want 1", result.SweptAsk)
		}
		deny := obj["permissions"].(map[string]any)["deny"].([]any)
		if len(deny) != 1 {
			t.Errorf("deny should be kept unchanged, got %v", deny)
		}
	})

	t.Run("MCP bare entry swept when server missing", func(t *testing.T) {
		t.Parallel()
		servers := set.New("slack")
		obj := map[string]any{
			"permissions": map[string]any{
				"allow": []any{
					"mcp__slack__post_message",
					"mcp__jira__create_issue",
					"Read",
				},
			},
		}
		result := NewPermissionSweeper(testutil.AllPathsExist{}, "", servers).Sweep(t.Context(), obj)
		allow := obj["permissions"].(map[string]any)["allow"].([]any)
		if len(allow) != 2 {
			t.Errorf("allow len = %d, want 2, got %v", len(allow), allow)
		}
		if result.SweptAllow != 1 {
			t.Errorf("SweptAllow = %d, want 1", result.SweptAllow)
		}
	})

	t.Run("MCP bare entry kept when server exists", func(t *testing.T) {
		t.Parallel()
		servers := set.New("slack")
		obj := map[string]any{
			"permissions": map[string]any{
				"allow": []any{"mcp__slack__post_message"},
			},
		}
		result := NewPermissionSweeper(testutil.AllPathsExist{}, "", servers).Sweep(t.Context(), obj)
		allow := obj["permissions"].(map[string]any)["allow"].([]any)
		if len(allow) != 1 {
			t.Errorf("allow len = %d, want 1", len(allow))
		}
		if result.SweptAllow != 0 {
			t.Errorf("SweptAllow = %d, want 0", result.SweptAllow)
		}
	})

	t.Run("MCP entry with parens swept when server missing", func(t *testing.T) {
		t.Parallel()
		servers := set.New("slack")
		obj := map[string]any{
			"permissions": map[string]any{
				"allow": []any{"mcp__jira__create_issue(query)"},
			},
		}
		result := NewPermissionSweeper(testutil.AllPathsExist{}, "", servers).Sweep(t.Context(), obj)
		allow := obj["permissions"].(map[string]any)["allow"].([]any)
		if len(allow) != 0 {
			t.Errorf("allow len = %d, want 0, got %v", len(allow), allow)
		}
		if result.SweptAllow != 1 {
			t.Errorf("SweptAllow = %d, want 1", result.SweptAllow)
		}
	})

	t.Run("MCP plugin entry kept", func(t *testing.T) {
		t.Parallel()
		servers := set.New[string]()
		obj := map[string]any{
			"permissions": map[string]any{
				"allow": []any{"mcp__plugin_github_github__search_code"},
			},
		}
		result := NewPermissionSweeper(testutil.AllPathsExist{}, "", servers).Sweep(t.Context(), obj)
		allow := obj["permissions"].(map[string]any)["allow"].([]any)
		if len(allow) != 1 {
			t.Errorf("plugin entry should be kept, got %v", allow)
		}
		if result.SweptAllow != 0 {
			t.Errorf("SweptAllow = %d, want 0", result.SweptAllow)
		}
	})

	t.Run("MCP in ask category swept", func(t *testing.T) {
		t.Parallel()
		servers := set.New("slack")
		obj := map[string]any{
			"permissions": map[string]any{
				"ask": []any{"mcp__jira__create_issue"},
			},
		}
		result := NewPermissionSweeper(testutil.AllPathsExist{}, "", servers).Sweep(t.Context(), obj)
		ask := obj["permissions"].(map[string]any)["ask"].([]any)
		if len(ask) != 0 {
			t.Errorf("ask len = %d, want 0", len(ask))
		}
		if result.SweptAsk != 1 {
			t.Errorf("SweptAsk = %d, want 1", result.SweptAsk)
		}
	})
}
