package service

import "testing"

func TestImageRegistryHost(t *testing.T) {
	cases := map[string]string{
		"nginx":                         "docker.io",
		"nginx:latest":                  "docker.io",
		"library/nginx":                 "docker.io",
		"user/app:1":                    "docker.io",
		"ghcr.io/org/app:latest":        "ghcr.io",
		"registry.example.com:5000/app": "registry.example.com:5000",
		"localhost:5000/app":            "localhost:5000",
	}
	for image, host := range cases {
		if got := imageRegistryHost(image); got != host {
			t.Fatalf("%s: got %s, want %s", image, got, host)
		}
	}
}

func TestSplitRegistryURL(t *testing.T) {
	host, bases := splitRegistryURL("https://GHCR.io/v2/")
	if host != "ghcr.io" {
		t.Fatalf("host %s", host)
	}
	if len(bases) != 2 || bases[0] != "https://ghcr.io" {
		t.Fatalf("bases %#v", bases)
	}

	host, bases = splitRegistryURL("http://127.0.0.1:5000")
	if host != "127.0.0.1:5000" || bases[0] != "http://127.0.0.1:5000" {
		t.Fatalf("http host %s bases %#v", host, bases)
	}
}

func TestHostsMatchDockerHub(t *testing.T) {
	if !hostsMatch("docker.io", "index.docker.io") {
		t.Fatal("docker hub aliases should match")
	}
	if hostsMatch("ghcr.io", "docker.io") {
		t.Fatal("different registries should not match")
	}
}
