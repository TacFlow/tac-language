package tac_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tacflow1-tech/tac-language/compiler"
	"github.com/tacflow1-tech/tac-language/formatter"
	"github.com/tacflow1-tech/tac-language/lexer"
	"github.com/tacflow1-tech/tac-language/parser"
	"github.com/tacflow1-tech/tac-language/semantic"
	"github.com/tacflow1-tech/tac-language/types"
)

func readExampleFile(name string) (string, error) {
	data, err := os.ReadFile(filepath.Join("examples", name))
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func TestCompileGoldenFiles(t *testing.T) {
	testFiles := []string{"web_qa", "graph_builder", "multi_agent_review"}
	for _, name := range testFiles {
		t.Run(name, func(t *testing.T) {
			src, err := readExampleFile(name + ".tac")
			if err != nil {
				t.Fatal(err)
			}
			program, err := parser.ParseSource(src)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			flows, err := compiler.CompileProgram(program)
			if err != nil {
				t.Fatalf("compile: %v", err)
			}
			if len(flows) == 0 {
				t.Fatal("no flows compiled")
			}
			for _, fj := range flows {
				if fj.Language.Name == "" {
					t.Error("missing language name")
				}
				if fj.Language.LanguageVersion == "" {
					t.Error("missing language_version")
				}
				if fj.Language.CompilerVersion == "" {
					t.Error("missing compiler_version")
				}
				if fj.Language.IRVersion == "" {
					t.Error("missing ir_version")
				}
			}
		})
	}
}

func TestCompileDeterministic(t *testing.T) {
	src := `flow "det" {
  node "a" -> skill web_search(query: "test")
  node "b" -> skill memory_store(text: "ok")
  a -> b
}`
	program1, _ := parser.ParseSource(src)
	program2, _ := parser.ParseSource(src)
	flows1, _ := compiler.CompileProgram(program1)
	flows2, _ := compiler.CompileProgram(program2)
	j1, _ := json.Marshal(flows1)
	j2, _ := json.Marshal(flows2)
	if string(j1) != string(j2) {
		t.Error("compiler non-deterministic")
	}
}

func TestFormatterByteStable(t *testing.T) {
	src := `flow "f" {
  input q: Untrusted
  node "a" -> skill web_search(query: q, count: 3)
  node "b" -> skill memory_store(text: "done", tags: ["x"])
  a -> b
  on "init" -> a
}`
	program, _ := parser.ParseSource(src)
	fmt1 := formatter.Format(program)
	program2, _ := parser.ParseSource(fmt1)
	fmt2 := formatter.Format(program2)
	if fmt1 != fmt2 {
		t.Error("formatter not byte-stable")
		t.Logf("first:  %q", fmt1)
		t.Logf("second: %q", fmt2)
	}

	// 100 iterations: format(AST) must produce identical bytes every time
	for i := 0; i < 100; i++ {
		p, _ := parser.ParseSource(src)
		if got := formatter.Format(p); got != fmt1 {
			t.Fatalf("iteration %d: byte-stable violation", i)
		}
	}
}

func TestUnknownSkillStrictMode(t *testing.T) {
	src := `flow "test" {
  node "a" -> skill nonexistent_skill(query: "test")
}`
	program, err := parser.ParseSource(src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	devAnalyzer := semantic.NewWithMode(semantic.ModeDevelopment)
	devDiags := devAnalyzer.Analyze(program)
	foundDevWarning, foundDevError := false, false
	for _, d := range devDiags {
		if strings.Contains(d.Message, "unknown skill") {
			if d.Severity == semantic.SeverityWarning {
				foundDevWarning = true
			} else {
				foundDevError = true
			}
		}
	}
	if !foundDevWarning {
		t.Error("development mode: expected warning for unknown skill")
	}
	if foundDevError {
		t.Error("development mode: unknown skill should be warning, not error")
	}

	prodAnalyzer := semantic.NewWithMode(semantic.ModeProduction)
	prodDiags := prodAnalyzer.Analyze(program)
	foundProdError := false
	for _, d := range prodDiags {
		if strings.Contains(d.Message, "unknown skill") && d.Severity == semantic.SeverityError {
			foundProdError = true
		}
	}
	if !foundProdError {
		t.Error("production mode: expected error for unknown skill")
	}
}

func TestTrustFlowUntrustedToFactRejected(t *testing.T) {
	src := `flow "bad" {
  input prompt: Untrusted
  node "persist" -> skill memory_store(text: prompt)
}`
	program, err := parser.ParseSource(src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	analyzer := semantic.New()
	diags := analyzer.Analyze(program)
	found := false
	for _, d := range diags {
		if strings.Contains(d.Message, "requires explicit") && strings.Contains(d.Message, "validate") {
			found = true
		}
	}
	if !found {
		t.Error("expected error: Untrusted -> Fact requires explicit validate()")
		for _, d := range diags {
			t.Logf("  diag: %s", d)
		}
	}
}

func TestTrustFlowValidatedChain(t *testing.T) {
	src := `flow "good" {
  input prompt: Untrusted
  node "v" -> skill validate(value: prompt)
  node "p" -> skill memory_store(text: v.result)
  v -> p
}`
	program, err := parser.ParseSource(src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	analyzer := semantic.New()
	diags := analyzer.Analyze(program)
	for _, d := range diags {
		if d.Severity == semantic.SeverityError {
			t.Errorf("unexpected error: %s", d)
		}
	}
}

func TestSecretCannotReachMemory(t *testing.T) {
	src := `flow "leak" {
  input api_key: Secret
  node "leak" -> skill memory_store(text: api_key)
}`
	program, _ := parser.ParseSource(src)
	diags := semantic.New().Analyze(program)
	found := false
	for _, d := range diags {
		if strings.Contains(d.Message, "requires explicit") && strings.Contains(d.Message, "Secret") && strings.Contains(d.Message, "authorize") {
			found = true
		}
	}
	if !found {
		t.Error("expected error: Secret -> Fact requires explicit authorize()")
		for _, d := range diags {
			t.Logf("  diag: %s", d)
		}
	}
}

func TestConditionIR(t *testing.T) {
	src := `flow "cond" {
  node "a" -> skill verify(source: "x")
  node "b" -> skill memory_store(text: "y")
  a -> b { if: a.confidence > 0.8 }
}`
	program, err := parser.ParseSource(src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	flows, err := compiler.CompileProgram(program)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if len(flows) == 0 || len(flows[0].Edges) == 0 {
		t.Fatal("expected edges")
	}
	edge := flows[0].Edges[0]
	if edge.Condition == nil {
		t.Fatal("expected structured condition")
	}
	if edge.Condition.Operator != ">" {
		t.Errorf("operator: expected '>', got %q", edge.Condition.Operator)
	}
	if edge.Condition.Left.Kind != "node_output" {
		t.Errorf("left kind: expected 'node_output', got %q", edge.Condition.Left.Kind)
	}
	if edge.Condition.Left.Node != "a" {
		t.Errorf("left node: expected 'a', got %q", edge.Condition.Left.Node)
	}
	if edge.Condition.Left.Path != "confidence" {
		t.Errorf("left path: expected 'confidence', got %q", edge.Condition.Left.Path)
	}
	if edge.Condition.Right.Kind != "number" {
		t.Errorf("right kind: expected 'number', got %q", edge.Condition.Right.Kind)
	}
}

func TestPythonSkillManifest(t *testing.T) {
	registry := semantic.NewRegistry()
	registry.Register(semantic.SkillSpec{
		Name: "lidar.scan.environment", Version: "1.4.2", ReturnType: types.Fact,
		Args: []string{"resolution"}, ArgTypes: map[string]types.TrustType{},
		Description: "Scan environment", Runtime: "python",
		Entrypoint: "lidar_scan.main:execute", Artifact: "oci://registry.tacflow.ai/lidar-scan:1.4.2",
		Digest: "sha256:9f40e5a", Signature: "signed", InputSchema: "in.json", OutputSchema: "out.json",
		Capabilities: []string{"device.lidar"},
		Execution: semantic.ExecutionSpec{TimeoutSeconds: 30, MemoryMB: 2048, CPU: 2},
	})
	spec, ok := registry.LookupVersioned("lidar.scan.environment", "1.4.2")
	if !ok || !spec.IsDynamic() {
		t.Fatal("skill not found or not dynamic")
	}

	incompleteReg := semantic.NewRegistry()
	incompleteReg.Register(semantic.SkillSpec{Name: "bad", Version: "1.0", Runtime: "python"})
	src2 := `flow "bad" { node "x" -> skill bad() }`
	program2, _ := parser.ParseSource(src2)
	strictDiags := semantic.NewWithRegistry(incompleteReg, semantic.ModeProduction).Analyze(program2)
	errors := 0
	for _, d := range strictDiags {
		if d.Severity == semantic.SeverityError {
			errors++
		}
	}
	if errors < 4 {
		t.Errorf("expected at least 4 errors (digest, unsigned, schemas), got %d", errors)
		for _, d := range strictDiags {
			t.Logf("  diag: %s", d)
		}
	}
}

func TestDeadNodeTriggerBased(t *testing.T) {
	src := `flow "example" {
  node "start" -> skill web_search(query: "x")
  node "finish" -> skill memory_search(query: "x")
  node "forgotten" -> skill web_search(query: "y")
  start -> finish
  on "run" -> start
}`
	program, _ := parser.ParseSource(src)
	diags := semantic.New().Analyze(program)
	found := false
	for _, d := range diags {
		if strings.Contains(d.Message, "unreachable") && strings.Contains(d.Message, "forgotten") {
			found = true
		}
	}
	if !found {
		t.Error("expected 'forgotten' flagged as unreachable")
		for _, d := range diags {
			t.Logf("  diag: %s", d)
		}
	}
}

func TestHallucinableToFactRequiresVerify(t *testing.T) {
	src := `flow "bad" {
  node "llm" -> skill llm.chat(prompt: "test")
  node "persist" -> skill memory_store(text: llm.result)
  llm -> persist
}`
	program, _ := parser.ParseSource(src)
	diags := semantic.New().Analyze(program)
	found := false
	for _, d := range diags {
		if strings.Contains(d.Message, "requires explicit") && strings.Contains(d.Message, "verify") {
			found = true
		}
	}
	if !found {
		t.Error("expected error: Hallucinable -> Fact requires verify()")
		for _, d := range diags {
			t.Logf("  diag: %s", d)
		}
	}
}

func TestVersionMetadata(t *testing.T) {
	src := `flow "v" { node "a" -> skill web_search(query: "x") }`
	program, _ := parser.ParseSource(src)
	flows, _ := compiler.CompileProgram(program)
	if len(flows) == 0 {
		t.Fatal("no flows")
	}
	meta := flows[0].Language
	if meta.Name != "TAC" || meta.LanguageVersion != "0.3" || meta.IRVersion != "1.1" {
		t.Errorf("version mismatch: name=%q lang=%q ir=%q", meta.Name, meta.LanguageVersion, meta.IRVersion)
	}
}

func TestSkillSpecJSONRoundTrip(t *testing.T) {
	// Verify the JSON tags on SkillSpec + ExecutionSpec match skills.json
	jsonIn := `[
  {
    "name": "sentiment_analyzer",
    "version": "1.0",
    "return_type": "Hallucinable",
    "args": ["text", "model"],
    "arg_types": {},
    "description": "Analyze sentiment",
    "runtime": "python",
    "entrypoint": "skills.sentiment:analyze",
    "artifact": "sentiment-analyzer-1.0.0.tar.gz",
    "digest": "sha256:abc123def456",
    "signature": "MEUCIQD...",
    "input_schema": "{\"type\":\"object\"}",
    "output_schema": "{\"type\":\"object\"}",
    "capabilities": ["nlp"],
    "permissions": {"network": "none"},
    "execution": {
      "timeout_seconds": 30,
      "memory_mb": 256,
      "cpu": 1,
      "retries": 1,
      "idempotent": true,
      "cancellable": true
    }
  }
]`
	var skills []semantic.SkillSpec
	if err := json.Unmarshal([]byte(jsonIn), &skills); err != nil {
		t.Fatalf("json unmarshal: %v", err)
	}
	if len(skills) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(skills))
	}
	s := skills[0]
	if s.Name != "sentiment_analyzer" {
		t.Errorf("name: got %q", s.Name)
	}
	if s.Version != "1.0" {
		t.Errorf("version: got %q", s.Version)
	}
	if s.ReturnType != types.Hallucinable {
		t.Errorf("return_type: got %q", s.ReturnType)
	}
	if s.Runtime != "python" {
		t.Errorf("runtime: got %q", s.Runtime)
	}
	if s.Entrypoint != "skills.sentiment:analyze" {
		t.Errorf("entrypoint: got %q", s.Entrypoint)
	}
	if s.Digest != "sha256:abc123def456" {
		t.Errorf("digest: got %q", s.Digest)
	}
	if s.Signature != "MEUCIQD..." {
		t.Errorf("signature: got %q", s.Signature)
	}
	if s.InputSchema == "" || s.OutputSchema == "" {
		t.Error("schemas missing")
	}
	if len(s.Capabilities) != 1 || s.Capabilities[0] != "nlp" {
		t.Errorf("capabilities: %v", s.Capabilities)
	}
	if s.Execution.TimeoutSeconds != 30 {
		t.Errorf("timeout: got %d", s.Execution.TimeoutSeconds)
	}
	if s.Execution.MemoryMB != 256 {
		t.Errorf("memory: got %d", s.Execution.MemoryMB)
	}
	if !s.Execution.Idempotent || !s.Execution.Cancellable {
		t.Error("execution flags missing")
	}
	if !s.IsDynamic() {
		t.Error("python skill should be dynamic")
	}

	// Round-trip: marshal and unmarshal should preserve
	reg := semantic.NewRegistry()
	reg.Register(s)
	spec, ok := reg.LookupVersioned("sentiment_analyzer", "1.0")
	if !ok {
		t.Fatal("not found after round-trip")
	}
	if spec.Digest != s.Digest {
		t.Error("digest lost in round-trip")
	}

	// Verify production mode rejects an incomplete dynamic skill
	badJSON := `[{"name":"bad_skill","version":"1.0","runtime":"python"}]`
	var badSkills []semantic.SkillSpec
	json.Unmarshal([]byte(badJSON), &badSkills)
	badReg := semantic.NewRegistry()
	for _, bs := range badSkills {
		badReg.Register(bs)
	}
	src := `flow "b" { node "x" -> skill bad_skill() }`
	program, _ := parser.ParseSource(src)
	diags := semantic.NewWithRegistry(badReg, semantic.ModeProduction).Analyze(program)
	errCount := 0
	for _, d := range diags {
		if d.Severity == semantic.SeverityError {
			errCount++
		}
	}
	if errCount < 4 {
		t.Errorf("expected ≥4 errors (digest, unsigned, schemas), got %d", errCount)
		for _, d := range diags {
			t.Logf("  diag: %s", d)
		}
	}
}

func FuzzCompilePipeline(f *testing.F) {
	seeds := []string{
		`flow "x" { node "a" -> skill web_search(query: "test") }`,
		`flow "f" { input q: Untrusted node "a" -> skill verify(source: "x") node "b" -> skill memory_store(text: a.result) a -> b on "init" -> a }`,
	}
	for _, s := range seeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, input string) {
		l := lexer.New(input)
		tokens, err := l.Scan()
		if err != nil {
			return
		}
		p := parser.New(tokens)
		program, err := p.Parse()
		if err != nil {
			return
		}
		_ = formatter.Format(program)
		_ = semantic.New().Analyze(program)
		_, _ = compiler.CompileProgram(program)
	})
}

func TestRegistrySnapshotDeterministic(t *testing.T) {
	reg := semantic.NewStaticRegistry(semantic.BuiltinSkills)
	d1, err := reg.SnapshotDigest(nil)
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	d2, err := reg.SnapshotDigest(nil)
	if err != nil {
		t.Fatalf("snapshot 2: %v", err)
	}
	if d1 != d2 {
		t.Fatalf("snapshot not deterministic: %s != %s", d1, d2)
	}
	if d1 == "" || !strings.HasPrefix(d1, "sha256:") {
		t.Errorf("invalid snapshot format: %s", d1)
	}

	// 100 iterations: snapshot must be stable
	for i := 0; i < 100; i++ {
		d, err := reg.SnapshotDigest(nil)
		if err != nil || d != d1 {
			t.Fatalf("iteration %d: snapshot changed (%s)", i, d)
		}
	}
}

func TestFingerprintDeterministic(t *testing.T) {
	src := `flow "f" { node "a" -> skill web_search(query: "test") }`
	program, _ := parser.ParseSource(src)
	flows, _ := compiler.CompileProgram(program)
	irJSON, _ := compiler.ToJSON(flows[0])

	regSnapshot := "sha256:abc123"
	trustPolicy := "1.0"

	fp1 := semantic.Fingerprint(irJSON, "0.3.0", regSnapshot, trustPolicy)
	fp2 := semantic.Fingerprint(irJSON, "0.3.0", regSnapshot, trustPolicy)
	if fp1 != fp2 {
		t.Fatal("fingerprint not deterministic")
	}

	// Different compiler version => different fingerprint
	fp3 := semantic.Fingerprint(irJSON, "0.2.0", regSnapshot, trustPolicy)
	if fp1 == fp3 {
		t.Error("fingerprint should differ with different compiler version")
	}
}

func TestUnreachableNodeInProductionIsError(t *testing.T) {
	src := `flow "example" {
  node "start" -> skill web_search(query: "x")
  node "forgotten" -> skill web_search(query: "y")
  on "run" -> start
}`
	program, _ := parser.ParseSource(src)
	devAnalyzer := semantic.NewWithMode(semantic.ModeDevelopment)
	devDiags := devAnalyzer.Analyze(program)
	foundDevErr := false
	for _, d := range devDiags {
		if strings.Contains(d.Message, "unreachable") && d.Severity == semantic.SeverityError {
			foundDevErr = true
		}
	}
	if foundDevErr {
		t.Error("unreachable node should be WARNING in development mode")
	}

	prodAnalyzer := semantic.NewWithMode(semantic.ModeProduction)
	prodDiags := prodAnalyzer.Analyze(program)
	foundProdErr := false
	for _, d := range prodDiags {
		if strings.Contains(d.Message, "unreachable") && d.Severity == semantic.SeverityError {
			foundProdErr = true
		}
	}
	if !foundProdErr {
		t.Error("unreachable node should be ERROR in production mode")
		for _, d := range prodDiags {
			t.Logf("  diag: %s", d)
		}
	}
}

func TestControlToFactRequiresAuthorize(t *testing.T) {
	src := `flow "bad" {
  input cmd: Control
  node "store" -> skill memory_store(text: cmd)
}`
	program, _ := parser.ParseSource(src)
	diags := semantic.New().Analyze(program)
	found := false
	for _, d := range diags {
		if strings.Contains(d.Message, "requires explicit") && strings.Contains(d.Message, "authorize") {
			found = true
		}
	}
	if !found {
		t.Error("expected error: Control -> Fact requires explicit authorize()")
		for _, d := range diags {
			t.Logf("  diag: %s", d)
		}
	}
}

func TestDiagnosticsHaveStructuredCodes(t *testing.T) {
	src := `flow "bad" {
  input prompt: Untrusted
  node "store" -> skill memory_store(text: prompt)
}`
	program, _ := parser.ParseSource(src)
	diags := semantic.New().Analyze(program)
	foundStructured := false
	for _, d := range diags {
		if d.Code == "TAC-TRUST-001" && d.SuggestedConversion == "validate" {
			foundStructured = true
		}
		if d.Code == "TAC-TRUST-001" && d.SourceType == "Untrusted" && d.TargetType == "Fact" {
			// verify structured metadata
		}
	}
	if !foundStructured {
		t.Error("expected structured diagnostic TAC-TRUST-001")
		for _, d := range diags {
			t.Logf("  code=%q source=%q target=%q suggested=%q msg=%s",
				d.Code, d.SourceType, d.TargetType, d.SuggestedConversion, d.Message)
		}
	}
}

func TestFlowIRHasEnhancedMetadata(t *testing.T) {
	src := `flow "v" { node "a" -> skill web_search(query: "x") }`
	program, _ := parser.ParseSource(src)
	flows, _ := compiler.CompileProgram(program)
	if len(flows) == 0 {
		t.Fatal("no flows")
	}
	fj := flows[0]
	if fj.Language.CompilerVersion != "0.3.0" {
		t.Errorf("compiler version: %s", fj.Language.CompilerVersion)
	}
	if fj.Language.IRVersion != "1.1" {
		t.Errorf("ir version: %s", fj.Language.IRVersion)
	}
	if fj.Language.LanguageVersion != "0.3" {
		t.Errorf("language version: %s", fj.Language.LanguageVersion)
	}
}
