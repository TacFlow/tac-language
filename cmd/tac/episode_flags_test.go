package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// M-7: `tac episode` validates the file like `tac compile` does, so it must
// honour the same flags: --tasks, --mode and --registry.
func TestEpisodeCommand_HonoursFlags(t *testing.T) {
	dir := t.TempDir()
	write := func(name, content string) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	episode := "episode \"e\" {\n  group \"g\"\n  question q: binary \"B?\"\n  target q = true\n}\n"
	gateSrc := write("gate.tac", episode+`flow "f" {
  node "gate" -> skill laya.decide(task: "acao", input: payload)
  node "a" -> skill laya.tasks.list()
  gate[nope] -> a
  gate[*] -> a
}
`)
	customSrc := write("custom.tac", episode+`flow "f" {
  node "a" -> skill custom_skill(x: "1")
}
`)
	tasks := write("tasks.json", `[{"name":"acao","questions":[{"id":"acao","type":"choice","instructions":"?","options":[{"label":"proceed"}]}]}]`)
	registry := write("registry.json", `[{"name":"custom_skill","version":"1.0.0","returns":{"type":"string","trust":"Fact"},"params":[{"name":"x","type":"string","trust":"Untrusted"}]}]`)

	run := func(args ...string) (int, string) {
		t.Helper()
		saved := os.Args
		defer func() { os.Args = saved }()
		os.Args = append([]string{"tac", "episode"}, args...)
		var out, errb bytes.Buffer
		code := runEpisode(args[0], flagValue(args, "--id"), &out, &errb)
		return code, errb.String()
	}

	if code, _ := run(gateSrc); code != 0 {
		t.Fatalf("without --tasks the gate labels are not checked: exit %d", code)
	}
	if code, stderr := run(gateSrc, "--tasks", tasks); code != 1 || !strings.Contains(stderr, "TAC-LAYA-002") {
		t.Errorf("--tasks: exit %d, stderr %s; want 1 and TAC-LAYA-002 for [nope]", code, stderr)
	}
	if code, _ := run(customSrc); code != 0 {
		t.Fatalf("development mode: an unknown skill is a warning: exit %d", code)
	}
	if code, _ := run(customSrc, "--mode", "production"); code != 1 {
		t.Errorf("--mode production: an unknown skill must fail: exit %d", code)
	}
	if code, stderr := run(customSrc, "--mode", "production", "--registry", registry); code != 0 {
		t.Errorf("--registry declares custom_skill: exit %d, stderr %s", code, stderr)
	}
}
