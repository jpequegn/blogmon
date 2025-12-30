package llm

import (
	"testing"
	"time"
)

func TestNewClient(t *testing.T) {
	client := NewClient("http://localhost:11434", "llama3.2", 30*time.Second)

	if client.baseURL != "http://localhost:11434" {
		t.Errorf("expected baseURL http://localhost:11434, got %s", client.baseURL)
	}
	if client.model != "llama3.2" {
		t.Errorf("expected model llama3.2, got %s", client.model)
	}
}

func TestGeneratePrompt(t *testing.T) {
	client := NewClient("http://localhost:11434", "llama3.2", 30*time.Second)

	prompt := client.BuildExtractionPrompt("Test Title", "Test content about Go programming.")

	if prompt == "" {
		t.Error("expected non-empty prompt")
	}
}
