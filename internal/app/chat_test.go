package app

import (
	"context"
	"testing"

	"llm-console/internal/llm"
)

type fakeClient struct{ messages []llm.Message }

func (f *fakeClient) Complete(_ context.Context, messages []llm.Message) (string, error) {
	f.messages = messages
	return "answer", nil
}

func TestAskSendsUserPrompt(t *testing.T) {
	fake := &fakeClient{}
	service := NewChatService(fake)

	answer, err := service.Ask(context.Background(), "  Hello  ")
	if err != nil {
		t.Fatal(err)
	}
	if answer != "answer" || len(fake.messages) != 1 || fake.messages[0].Content != "Hello" {
		t.Fatalf("unexpected result: answer=%q messages=%#v", answer, fake.messages)
	}
}
