package auth

import (
	"testing"
)

func TestHashPassword(t *testing.T) {
	pw := "1a3b5C7&"
	got, err := HashPassword(pw)
	if err != nil {
		t.Fatalf("Error creating hash: %v", err)
	}
	if got == pw || len(got) <= 0 {
		t.Errorf("HashPassword(%s)='%v'; unexpected", pw, got)
	}
	// Hash different password
	pw2 := pw + "x"
	got2, err2 := HashPassword(pw2)
	if err2 != nil {
		t.Fatalf("Error creating hash: %v", err2)
	}
	if got2 == pw2 || got2 == pw || got == got2 || len(got2) <= 0 {
		t.Errorf("HashPassword(%s)='%v'; unexpected", pw2, got2)
	}
	// Hash same password as first test, and compare results
	got3, err3 := HashPassword(pw)
	if err3 != nil {
		t.Fatalf("Error creating hash: %v", err3)
	}
	if got2 == pw || got != got3 || len(got3) <= 0 {
		t.Errorf("HashPassword(%s)='%v'; unexpected: hash(pw) doesn't match first hash", pw, got3)
	}
}

func TestCheckPasswordHash(t *testing.T) {
	pw := "1a3b5C7&"
	hash, err := HashPassword(pw)
	if err != nil {
		t.Fatalf("Error creating hash: %v", err)
	}
	got1, err1 := CheckPasswordHash(pw, hash)
	if err1 != nil {
		t.Fatalf("Error checking pw against hash: %v", err1)
	}
	// check pw matches hash
	if got1 == false {
		t.Errorf("CheckPasswordHash(%s, %s)='%v'; unexpected: pw doesn't match hash", pw, hash, got1)
	}
	got2, err2 := CheckPasswordHash(pw+"x", hash)
	if err2 != nil {
		t.Fatalf("Error checking pw against hash: %v", err2)
	}
	// check pw doesn't match hash
	if got2 != false {
		t.Errorf("CheckPasswordHash(%s, %s)='%v'; unexpected: pw matches hash", pw+"x", hash, got2)
	}
}
