package services

import "context"

// mockLLMProvider implements LLMProvider for testing (no CGO dependency).
type mockLLMProvider struct {
	reflectionResult string
	reflectionError  error
}

func (m *mockLLMProvider) Generate(ctx context.Context, prompt string) (string, error) {
	return "", nil
}
func (m *mockLLMProvider) ExtractEntities(ctx context.Context, text string) (*ExtractionResult, error) {
	return &ExtractionResult{}, nil
}
func (m *mockLLMProvider) ScoreImportance(ctx context.Context, text string) (*ImportanceScore, error) {
	return &ImportanceScore{Score: 0.5}, nil
}
func (m *mockLLMProvider) GenerateReflection(ctx context.Context, memories []string, coreMemoryContext string) (string, error) {
	if m.reflectionError != nil {
		return "", m.reflectionError
	}
	return m.reflectionResult, nil
}
