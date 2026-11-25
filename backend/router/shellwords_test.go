package router

import (
	"reflect"
	"testing"
)

func TestSplitShellWords(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		want    []string
		wantErr bool
	}{
		{
			name:  "simple",
			input: "docker run -d nginx",
			want:  []string{"docker", "run", "-d", "nginx"},
		},
		{
			name:  "double quoted with space",
			input: `docker run -e "FOO=a b" nginx`,
			want:  []string{"docker", "run", "-e", "FOO=a b", "nginx"},
		},
		{
			name:  "single quoted literal",
			input: `docker run -e 'FOO=a $HOME b' nginx`,
			want:  []string{"docker", "run", "-e", "FOO=a $HOME b", "nginx"},
		},
		{
			name:  "metacharacters become literal",
			input: "docker run nginx; rm -rf /",
			want:  []string{"docker", "run", "nginx;", "rm", "-rf", "/"},
		},
		{
			name:  "backslash escape outside quotes",
			input: `docker run -e A\ B nginx`,
			want:  []string{"docker", "run", "-e", "A B", "nginx"},
		},
		{
			name:  "escaped quote inside double quotes",
			input: `docker run -e "A=\"x\"" nginx`,
			want:  []string{"docker", "run", "-e", `A="x"`, "nginx"},
		},
		{
			name:  "empty quoted arg preserved",
			input: `docker run --name "" nginx`,
			want:  []string{"docker", "run", "--name", "", "nginx"},
		},
		{
			name:    "unclosed single quote",
			input:   "docker run 'abc",
			wantErr: true,
		},
		{
			name:    "unclosed double quote",
			input:   `docker run "abc`,
			wantErr: true,
		},
		{
			name:    "trailing backslash",
			input:   `docker run nginx\`,
			wantErr: true,
		},
		{
			name:  "extra whitespace",
			input: "  docker   run   nginx  ",
			want:  []string{"docker", "run", "nginx"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := splitShellWords(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got %v", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}
