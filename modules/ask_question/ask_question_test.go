// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-only

package ask_question

import (
	"context"
	"testing"

	"github.com/devproje/mininaru/modules"
)

func TestQuestionRequiresQuestion(t *testing.T) {
	var ask modules.AskFunc
	var err error

	ask = func(ctx context.Context, question string, options []string) (string, error) {
		return "answer", nil
	}

	_, err = Question(ask).Execute(context.Background(), `{"question":""}`)
	if err == nil {
		t.Fatal("expected an error for an empty question")
	}
}

func TestQuestionErrorsWithNoAskFunc(t *testing.T) {
	var err error

	_, err = Question(nil).Execute(context.Background(), `{"question":"pick one"}`)
	if err == nil {
		t.Fatal("expected an error when no interactive user is available")
	}
}

func TestQuestionRoundTripsTheAnswer(t *testing.T) {
	var ask modules.AskFunc
	var gotQuestion string
	var gotOptions []string
	var result string

	var err error

	ask = func(ctx context.Context, question string, options []string) (string, error) {
		gotQuestion = question
		gotOptions = options

		return "blue", nil
	}

	result, err = Question(ask).Execute(context.Background(), `{"question":"favorite color?","options":["red","blue"]}`)
	if err != nil {
		t.Fatal(err)
	}
	if result != "blue" {
		t.Fatalf("result = %q, want %q", result, "blue")
	}
	if gotQuestion != "favorite color?" {
		t.Fatalf("question passed to ask = %q, want %q", gotQuestion, "favorite color?")
	}
	if len(gotOptions) != 2 || gotOptions[0] != "red" || gotOptions[1] != "blue" {
		t.Fatalf("options passed to ask = %v, want [red blue]", gotOptions)
	}
}
