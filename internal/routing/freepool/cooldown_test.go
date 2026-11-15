package freepool

import (
	"testing"
	"time"
)

func TestStoreParkAndExpiry(t *testing.T) {
	s := &Store{}
	if s.Parked("openai", "gpt-4o") {
		t.Fatal("zero Store reports a parked model")
	}

	s.Park("openai", "gpt-4o", 40*time.Millisecond)
	if !s.Parked("openai", "gpt-4o") {
		t.Fatal("Park did not take effect")
	}
	if s.Parked("openai", "gpt-4o-turbo") {
		t.Fatal("Park leaked to a different model")
	}
	if s.Parked("deepseek", "gpt-4o") {
		t.Fatal("Park leaked to a different provider")
	}

	time.Sleep(60 * time.Millisecond)
	if s.Parked("openai", "gpt-4o") {
		t.Fatal("model still parked after its deadline")
	}
	if _, ok := s.until[key("openai", "gpt-4o")]; ok {
		t.Fatal("expired entry was not deleted")
	}
}

func TestStoreParkIgnoresNonPositive(t *testing.T) {
	s := &Store{}
	s.Park("openai", "gpt-4o", 0)
	s.Park("openai", "gpt-4o", -time.Second)
	if s.Parked("openai", "gpt-4o") {
		t.Fatal("non-positive duration parked the model")
	}
}

func TestStoreLongerDeadlineWins(t *testing.T) {
	s := &Store{}
	s.Park("openai", "gpt-4o", 100*time.Millisecond)
	s.Park("openai", "gpt-4o", 10*time.Millisecond) // shorter: ignored
	time.Sleep(30 * time.Millisecond)
	if !s.Parked("openai", "gpt-4o") {
		t.Fatal("shorter Park shortened the existing deadline")
	}

	s.Park("openai", "gpt-4o", 300*time.Millisecond) // longer: replaces
	time.Sleep(90 * time.Millisecond)
	if !s.Parked("openai", "gpt-4o") {
		t.Fatal("longer Park did not extend the deadline")
	}
}

func TestStoreReset(t *testing.T) {
	s := &Store{}
	s.Park("openai", "gpt-4o", time.Minute)
	if !s.Parked("openai", "gpt-4o") {
		t.Fatal("Park did not take effect")
	}
	s.Reset()
	if s.Parked("openai", "gpt-4o") {
		t.Fatal("Reset did not clear the store")
	}
}
