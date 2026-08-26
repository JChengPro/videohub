package auth

import "testing"

func TestGenerateTokenCreatesUniqueSession(t *testing.T) {
	t.Setenv("JWT_SECRET", "test-secret-with-enough-entropy")

	first, err := GenerateToken(7, "123456", "tester")
	if err != nil {
		t.Fatalf("generate first token: %v", err)
	}
	second, err := GenerateToken(7, "123456", "tester")
	if err != nil {
		t.Fatalf("generate second token: %v", err)
	}
	if first == second {
		t.Fatal("two logins must not receive the same token")
	}

	firstClaims, err := ParseToken(first)
	if err != nil {
		t.Fatalf("parse first token: %v", err)
	}
	secondClaims, err := ParseToken(second)
	if err != nil {
		t.Fatalf("parse second token: %v", err)
	}
	if firstClaims.ID == "" || secondClaims.ID == "" {
		t.Fatal("session token must contain a JWT ID")
	}
	if firstClaims.ID == secondClaims.ID {
		t.Fatal("each login must receive a unique JWT ID")
	}
}
