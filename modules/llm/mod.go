package llm

type LLMModule struct {
}

var LLM *LLMModule = &LLMModule{}

func (m *LLMModule) Name() string {
	return "llm-module"
}

func (m *LLMModule) Load() error {
	return nil
}

func (m *LLMModule) Unload() error {
	return nil
}
