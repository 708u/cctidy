package cctidy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/708u/cctidy/internal/testutil"
)

func TestFormat(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		input   string
		checker PathChecker
		want    string
		wantErr bool
	}{
		{
			name:    "recursive key sorting",
			input:   `{"z": 1, "a": {"c": 2, "b": 1}}`,
			checker: testutil.AllPathsExist{},
			want:    "{\n  \"a\": {\n    \"b\": 1,\n    \"c\": 2\n  },\n  \"githubRepoPaths\": {},\n  \"projects\": {},\n  \"z\": 1\n}\n",
		},
		{
			name:    "remove non-existent project paths",
			input:   `{"projects": {"/exists": {}, "/gone": {}}}`,
			checker: testutil.CheckerFor("/exists"),
			want:    "{\n  \"githubRepoPaths\": {},\n  \"projects\": {\n    \"/exists\": {}\n  }\n}\n",
		},
		{
			name:    "remove non-existent github repo paths",
			input:   `{"githubRepoPaths": {"org/repo": ["/exists", "/gone"]}}`,
			checker: testutil.CheckerFor("/exists"),
			want:    "{\n  \"githubRepoPaths\": {\n    \"org/repo\": [\n      \"/exists\"\n    ]\n  },\n  \"projects\": {}\n}\n",
		},
		{
			name:    "remove empty repo keys",
			input:   `{"githubRepoPaths": {"org/repo": ["/gone"]}}`,
			checker: testutil.NoPathsExist{},
			want:    "{\n  \"githubRepoPaths\": {},\n  \"projects\": {}\n}\n",
		},
		{
			name:    "preserve string array order",
			input:   `{"arr": ["c", "a", "b"]}`,
			checker: testutil.AllPathsExist{},
			want:    "{\n  \"arr\": [\n    \"c\",\n    \"a\",\n    \"b\"\n  ],\n  \"githubRepoPaths\": {},\n  \"projects\": {}\n}\n",
		},
		{
			name:    "preserve number array order",
			input:   `{"arr": [9, 10, 2]}`,
			checker: testutil.AllPathsExist{},
			want:    "{\n  \"arr\": [\n    9,\n    10,\n    2\n  ],\n  \"githubRepoPaths\": {},\n  \"projects\": {}\n}\n",
		},
		{
			name:    "preserve bool array order",
			input:   `{"arr": [true, false, true]}`,
			checker: testutil.AllPathsExist{},
			want:    "{\n  \"arr\": [\n    true,\n    false,\n    true\n  ],\n  \"githubRepoPaths\": {},\n  \"projects\": {}\n}\n",
		},
		{
			name:    "preserve object array order",
			input:   `{"arr": [{"b": 1}, {"a": 2}]}`,
			checker: testutil.AllPathsExist{},
			want:    "{\n  \"arr\": [\n    {\n      \"b\": 1\n    },\n    {\n      \"a\": 2\n    }\n  ],\n  \"githubRepoPaths\": {},\n  \"projects\": {}\n}\n",
		},
		{
			name:    "preserve nested array order",
			input:   `{"arr": [[3, 1], [2, 4]]}`,
			checker: testutil.AllPathsExist{},
			want:    "{\n  \"arr\": [\n    [\n      3,\n      1\n    ],\n    [\n      2,\n      4\n    ]\n  ],\n  \"githubRepoPaths\": {},\n  \"projects\": {}\n}\n",
		},
		{
			name:    "preserve mixed type array order",
			input:   `{"arr": ["a", 1, true]}`,
			checker: testutil.AllPathsExist{},
			want:    "{\n  \"arr\": [\n    \"a\",\n    1,\n    true\n  ],\n  \"githubRepoPaths\": {},\n  \"projects\": {}\n}\n",
		},
		{
			name:    "preserve integer representation",
			input:   `{"num": 42}`,
			checker: testutil.AllPathsExist{},
			want:    "{\n  \"githubRepoPaths\": {},\n  \"num\": 42,\n  \"projects\": {}\n}\n",
		},
		{
			name:    "no HTML escaping",
			input:   `{"url": "a&b<c>d"}`,
			checker: testutil.AllPathsExist{},
			want:    "{\n  \"githubRepoPaths\": {},\n  \"projects\": {},\n  \"url\": \"a&b<c>d\"\n}\n",
		},
		{
			name:    "trailing newline",
			input:   `{"a": 1}`,
			checker: testutil.AllPathsExist{},
			want:    "{\n  \"a\": 1,\n  \"githubRepoPaths\": {},\n  \"projects\": {}\n}\n",
		},
		{
			name:    "invalid JSON",
			input:   `{broken`,
			checker: testutil.AllPathsExist{},
			wantErr: true,
		},
		{
			name:    "empty file",
			input:   ``,
			checker: testutil.AllPathsExist{},
			wantErr: true,
		},
		{
			name:    "whitespace only",
			input:   `   `,
			checker: testutil.AllPathsExist{},
			wantErr: true,
		},
		{
			name:    "projects key missing sets empty object",
			input:   `{"foo": 1}`,
			checker: testutil.AllPathsExist{},
			want:    "{\n  \"foo\": 1,\n  \"githubRepoPaths\": {},\n  \"projects\": {}\n}\n",
		},
		{
			name:    "projects is array skips cleaning",
			input:   `{"projects": [1, 2]}`,
			checker: testutil.AllPathsExist{},
			want:    "{\n  \"githubRepoPaths\": {},\n  \"projects\": [\n    1,\n    2\n  ]\n}\n",
		},
		{
			name:    "githubRepoPaths value is string skips cleaning",
			input:   `{"githubRepoPaths": {"repo": "not-array"}}`,
			checker: testutil.AllPathsExist{},
			want:    "{\n  \"githubRepoPaths\": {\n    \"repo\": \"not-array\"\n  },\n  \"projects\": {}\n}\n",
		},
		{
			name:    "empty object input",
			input:   `{}`,
			checker: testutil.AllPathsExist{},
			want:    "{\n  \"githubRepoPaths\": {},\n  \"projects\": {}\n}\n",
		},
		{
			name:    "paths with special characters",
			input:   `{"projects": {"/path with spaces/project": {}, "/path/日本語": {}}}`,
			checker: testutil.CheckerFor("/path with spaces/project", "/path/日本語"),
			want:    "{\n  \"githubRepoPaths\": {},\n  \"projects\": {\n    \"/path with spaces/project\": {},\n    \"/path/日本語\": {}\n  }\n}\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			f := NewClaudeJSONFormatter(tt.checker)
			result, err := f.Format(t.Context(), []byte(tt.input))
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			got := string(result.Data)
			if got != tt.want {
				t.Errorf("mismatch:\ngot:\n%s\nwant:\n%s", got, tt.want)
			}
		})
	}
}

func TestSettingsJSONFormatter(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		input   string
		checker PathChecker
		want    string
		wantErr bool
	}{
		{
			name:    "settings-style key sorting",
			input:   `{"permissions":{"allow":["Bash","Read"]},"env":{"Z_VAR":"z","A_VAR":"a"},"hooks":{"preToolUse":[]}}`,
			checker: testutil.AllPathsExist{},
			want:    "{\n  \"env\": {\n    \"A_VAR\": \"a\",\n    \"Z_VAR\": \"z\"\n  },\n  \"hooks\": {\n    \"preToolUse\": []\n  },\n  \"permissions\": {\n    \"allow\": [\n      \"Bash\",\n      \"Read\"\n    ]\n  }\n}\n",
		},
		{
			name:    "sort string arrays in permissions",
			input:   `{"permissions":{"allow":["Write","Bash","Read"],"deny":["mcp__dangerous","mcp__admin"]}}`,
			checker: testutil.AllPathsExist{},
			want:    "{\n  \"permissions\": {\n    \"allow\": [\n      \"Bash\",\n      \"Read\",\n      \"Write\"\n    ],\n    \"deny\": [\n      \"mcp__admin\",\n      \"mcp__dangerous\"\n    ]\n  }\n}\n",
		},
		{
			name:    "no projects or githubRepoPaths added",
			input:   `{"apiKey":"test"}`,
			checker: testutil.AllPathsExist{},
			want:    "{\n  \"apiKey\": \"test\"\n}\n",
		},
		{
			name:    "invalid JSON",
			input:   `{broken`,
			checker: testutil.AllPathsExist{},
			wantErr: true,
		},
		{
			name:    "empty object",
			input:   `{}`,
			checker: testutil.AllPathsExist{},
			want:    "{}\n",
		},
		{
			name:    "nested objects sorted recursively",
			input:   `{"z":{"b":2,"a":1},"a":{"d":4,"c":3}}`,
			checker: testutil.AllPathsExist{},
			want:    "{\n  \"a\": {\n    \"c\": 3,\n    \"d\": 4\n  },\n  \"z\": {\n    \"a\": 1,\n    \"b\": 2\n  }\n}\n",
		},
		{
			name:    "sweep dead permission paths",
			input:   `{"permissions":{"allow":["Read(//dead/path)","Read"]}}`,
			checker: testutil.CheckerFor(),
			want:    "{\n  \"permissions\": {\n    \"allow\": [\n      \"Read\"\n    ]\n  }\n}\n",
		},
		{
			name:    "keep alive permission paths",
			input:   `{"permissions":{"allow":["Read(//alive/path)","Read"]}}`,
			checker: testutil.CheckerFor("/alive/path"),
			want:    "{\n  \"permissions\": {\n    \"allow\": [\n      \"Read\",\n      \"Read(//alive/path)\"\n    ]\n  }\n}\n",
		},
		{
			name:    "sweep allow and ask but keep deny",
			input:   `{"permissions":{"allow":["Read(//dead/a)"],"deny":["Read(//dead/b)"],"ask":["Edit(//dead/c)"]}}`,
			checker: testutil.CheckerFor(),
			want:    "{\n  \"permissions\": {\n    \"allow\": [],\n    \"ask\": [],\n    \"deny\": [\n      \"Read(//dead/b)\"\n    ]\n  }\n}\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result, err := NewSettingsJSONFormatter(NewPermissionSweeper(tt.checker, "", nil)).Format(t.Context(), []byte(tt.input))
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			got := string(result.Data)
			if got != tt.want {
				t.Errorf("mismatch:\ngot:\n%s\nwant:\n%s", got, tt.want)
			}
		})
	}
}

func TestSettingsJSONFormatterDoesNotAddProjects(t *testing.T) {
	t.Parallel()
	input := `{"key": "value"}`
	result, err := NewSettingsJSONFormatter(NewPermissionSweeper(testutil.AllPathsExist{}, "", nil)).Format(t.Context(), []byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := string(result.Data)
	assertNoKey(t, got, "projects")
	assertNoKey(t, got, "githubRepoPaths")
}

func assertNoKey(t *testing.T, jsonStr, key string) {
	t.Helper()
	if len(jsonStr) > 0 && json.Valid([]byte(jsonStr)) &&
		bytes.Contains([]byte(jsonStr), []byte(`"`+key+`"`)) {
		t.Errorf("should not contain key %q", key)
	}
}

func TestFormatStats(t *testing.T) {
	t.Parallel()
	input := `{"projects": {"/exists": {}, "/gone": {}}, "githubRepoPaths": {"r": ["/exists", "/gone"]}}`
	f := NewClaudeJSONFormatter(testutil.CheckerFor("/exists"))
	result, err := f.Format(t.Context(), []byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	s := result.Stats.(*ClaudeJSONFormatterStats)
	if removed := s.ProjectsBefore - s.ProjectsAfter; removed != 1 {
		t.Errorf("projects removed = %d, want 1", removed)
	}
	if removed := s.RepoBefore - s.RepoAfter; removed != 1 {
		t.Errorf("repo paths removed = %d, want 1", removed)
	}
}

func TestFormatComma(t *testing.T) {
	t.Parallel()
	tests := []struct {
		n    int64
		want string
	}{
		{0, "0"},
		{999, "999"},
		{1000, "1,000"},
		{52049, "52,049"},
		{1234567890, "1,234,567,890"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			t.Parallel()
			got := formatComma(tt.n)
			if got != tt.want {
				t.Errorf("formatComma(%d) = %q, want %q", tt.n, got, tt.want)
			}
		})
	}
}

func TestFormatRespectsContextCancellation(t *testing.T) {
	t.Parallel()
	input := `{"projects": {"/a": {}, "/b": {}}, "githubRepoPaths": {"r": ["/a"]}}`
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	f := NewClaudeJSONFormatter(testutil.AllPathsExist{})
	_, err := f.Format(ctx, []byte(input))
	if err == nil {
		t.Fatal("expected error from canceled context, got nil")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got: %v", err)
	}
}

func TestFormatOutputValidity(t *testing.T) {
	t.Parallel()
	input := `{"key": "value", "num": 42, "arr": [3, 1, 2]}`
	f := NewClaudeJSONFormatter(testutil.AllPathsExist{})
	result, err := f.Format(t.Context(), []byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !json.Valid(result.Data) {
		t.Error("output is not valid JSON")
	}
}
