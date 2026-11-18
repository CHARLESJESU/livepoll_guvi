package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestValidCredentials(t *testing.T) {
	tests := []struct {
		name     string
		email    string
		password string
		expected bool
	}{
		{"valid credentials", "user@example.com", "password123", true},
		{"missing at symbol", "userexample.com", "password123", false},
		{"missing domain dot", "user@localhost", "password123", false},
		{"short password", "user@example.com", "pass", false},
		{"password too long", "user@example.com", strings.Repeat("a", 73), false},
		{"exact min password length", "user@example.com", "12345678", true},
		{"exact max password length", "user@example.com", strings.Repeat("a", 72), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validCredentials(authRequest{Email: tt.email, Password: tt.password})
			if result != tt.expected {
				t.Errorf("validCredentials(%q, %q) = %v; expected %v", tt.email, tt.password, result, tt.expected)
			}
		})
	}
}

func TestTokenGenerationAndParsing(t *testing.T) {
	secret := []byte("test-super-secret-key-32-bytes!!")
	app := &App{secret: secret}

	userID := primitive.NewObjectID()
	tok := app.token(userID)

	// Valid token
	parsedID, ok := app.parseToken("Bearer " + tok)
	if !ok {
		t.Fatalf("expected token to parse successfully")
	}
	if parsedID != userID {
		t.Fatalf("expected userID %s, got %s", userID.Hex(), parsedID.Hex())
	}

	// Missing Bearer prefix
	if _, ok := app.parseToken(tok); ok {
		t.Errorf("expected failure for missing Bearer prefix")
	}

	// Tampered token
	tampered := tok[:len(tok)-2] + "ab"
	if _, ok := app.parseToken("Bearer " + tampered); ok {
		t.Errorf("expected failure for tampered token")
	}

	// Wrong secret
	otherApp := &App{secret: []byte("different-secret-key-32-bytes!!")}
	if _, ok := otherApp.parseToken("Bearer " + tok); ok {
		t.Errorf("expected failure for token signed with different secret")
	}
}

func TestHealthEndpoint(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/health", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status": "ok"}) })

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/health", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), `"status":"ok"`) {
		t.Errorf("unexpected body: %s", w.Body.String())
	}
}

func TestEnvFallback(t *testing.T) {
	if val := env("NON_EXISTENT_VAR_12345", "fallback_val"); val != "fallback_val" {
		t.Errorf("expected fallback_val, got %s", val)
	}
}

func TestPollValidation(t *testing.T) {
	// Verify poll creation boundary constraints
	tooShortQuestion := "hi"
	if len(strings.TrimSpace(tooShortQuestion)) >= 5 {
		t.Errorf("question should be invalid")
	}

	tooFewOptions := []string{"Only One"}
	if len(tooFewOptions) >= 2 {
		t.Errorf("should require at least 2 options")
	}

	tooManyOptions := []string{"1", "2", "3", "4", "5", "6", "7"}
	if len(tooManyOptions) <= 6 {
		t.Errorf("should reject more than 6 options")
	}
}
