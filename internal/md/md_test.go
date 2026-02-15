package md

import "testing"

func TestParseName(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		data string
		want string
	}{
		{
			name: "name field present",
			data: "---\nname: my-agent\n---\n# Agent\n",
			want: "my-agent",
		},
		{
			name: "name field with other fields",
			data: "---\nname: custom\ndescription: test\n---\n",
			want: "custom",
		},
		{
			name: "no name field",
			data: "---\ndescription: test\n---\n# Agent\n",
			want: "",
		},
		{
			name: "no frontmatter",
			data: "# Agent\nSome content\n",
			want: "",
		},
		{
			name: "empty input",
			data: "",
			want: "",
		},
		{
			name: "invalid YAML frontmatter",
			data: "---\n: invalid: yaml:\n---\n",
			want: "",
		},
		{
			name: "name is not a string",
			data: "---\nname: 123\n---\n",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := ParseName([]byte(tt.data))
			if got != tt.want {
				t.Errorf("ParseName() = %q, want %q", got, tt.want)
			}
		})
	}
}
