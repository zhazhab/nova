package config

import "testing"

func TestDefaultSettingsValues(t *testing.T) {
	s := DefaultSettings()
	if s.OpenAIBaseURL != "https://api.deepseek.com" {
		t.Fatalf("BaseURL: %s", s.OpenAIBaseURL)
	}
	if s.OpenAIModel != "deepseek-v4-pro" {
		t.Fatalf("Model: %s", s.OpenAIModel)
	}
	if s.AutoSaveEnabled == nil || *s.AutoSaveEnabled != true {
		t.Fatalf("AutoSaveEnabled default")
	}
	if s.MaxIteration == nil || *s.MaxIteration != 50 {
		t.Fatalf("MaxIteration default")
	}
}

func TestMergeOverridesNonZero(t *testing.T) {
	parent := Settings{
		OpenAIBaseURL: "https://parent",
		OpenAIModel:   "p-model",
		MaxIteration:  intPtr(10),
	}
	child := Settings{
		OpenAIModel:  "c-model", // override
		MaxIteration: nil,       // 继承 parent
	}
	out := Merge(parent, child)
	if out.OpenAIBaseURL != "https://parent" {
		t.Fatalf("BaseURL should inherit: %s", out.OpenAIBaseURL)
	}
	if out.OpenAIModel != "c-model" {
		t.Fatalf("Model should override: %s", out.OpenAIModel)
	}
	if out.MaxIteration == nil || *out.MaxIteration != 10 {
		t.Fatalf("MaxIteration should inherit parent")
	}
}

func TestMergePointerExplicitOverride(t *testing.T) {
	parent := Settings{AutoSaveEnabled: boolPtr(true)}
	child := Settings{AutoSaveEnabled: boolPtr(false)}
	out := Merge(parent, child)
	if out.AutoSaveEnabled == nil || *out.AutoSaveEnabled != false {
		t.Fatalf("explicit false should override true")
	}
}
