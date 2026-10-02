package compiler

// Versions stamped on every Flow JSON (SPEC v0.3 §8). One place, so the
// release bump (sub-project F of the LAYA mission: CompilerVersion 0.4.0 ->
// 0.5.0, plus cmd/tac, CHANGELOG, README, SPEC) touches exactly this file.
const (
	// LanguageVersion is the language level this compiler implements.
	LanguageVersion = "0.5"
	// CompilerVersion is the release of this compiler. It moves at release
	// time only (F), never with a feature.
	CompilerVersion = "0.4.0"
	// IRVersion is the Flow JSON shape. 1.2 adds edge label/range, flow
	// requires and flow schedules.
	IRVersion = "1.2"
	// LayaLanguage is the language level the LAYA constructs need; a flow
	// using any of them carries "requires": LayaLanguage.
	LayaLanguage = "0.5"
)
