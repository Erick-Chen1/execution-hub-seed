package user

import "testing"

func TestValidateUsername(t *testing.T) {
	ok := []string{"alice1", "alice_01", "a1234", "john-doe", "alice.dev"}
	for _, v := range ok {
		if err := ValidateUsername(v); err != nil {
			t.Fatalf("expected valid username %q: %v", v, err)
		}
	}
	bad := []string{"", "1alice", "a", "ab", "a_", "a..", "a*", "toolongusername_over_32_chars_abc"}
	for _, v := range bad {
		if err := ValidateUsername(v); err == nil {
			t.Fatalf("expected invalid username %q", v)
		}
	}
}

func TestValidatePassword(t *testing.T) {
	if err := ValidatePassword("S3cure!Passw0rd", "alice"); err != nil {
		t.Fatalf("expected valid password: %v", err)
	}
	if err := ValidatePassword("short1!", "alice"); err == nil {
		t.Fatalf("expected error for short password")
	}
	if err := ValidatePassword("alllowercase123!", "alice"); err == nil {
		t.Fatalf("expected error for missing upper")
	}
	if err := ValidatePassword("ALLUPPERCASE123!", "alice"); err == nil {
		t.Fatalf("expected error for missing lower")
	}
	if err := ValidatePassword("NoDigits!!!!!!!", "alice"); err == nil {
		t.Fatalf("expected error for missing digit")
	}
	if err := ValidatePassword("NoSpecial12345", "alice"); err == nil {
		t.Fatalf("expected error for missing special")
	}
	if err := ValidatePassword("Alice!Passw0rd", "alice"); err == nil {
		t.Fatalf("expected error for containing username")
	}
}
