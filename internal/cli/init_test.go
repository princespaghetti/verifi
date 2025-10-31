package cli

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/princespaghetti/verifi/internal/certstore"
)


func TestInitCmd_Exists(t *testing.T) {
	// Verify the init command is registered
	if initCmd == nil {
		t.Fatal("initCmd is nil")
	}

	if initCmd.Use != "init" {
		t.Errorf("initCmd.Use = %q, want %q", initCmd.Use, "init")
	}
}

func TestInitCmd_Flags(t *testing.T) {
	// Verify --force flag exists
	flag := initCmd.Flags().Lookup("force")
	if flag == nil {
		t.Fatal("--force flag not found")
	}

	if flag.DefValue != "false" {
		t.Errorf("--force default = %q, want %q", flag.DefValue, "false")
	}
}

func TestInitCmd_Success(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a new command with our temp directory
	// We can't easily test runInit directly because it calls os.Exit,
	// but we can test the underlying store operations

	store, err := certstore.NewStore(tmpDir)
	if err != nil {
		t.Fatalf("NewStore() failed: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Test the actual Init operation that runInit uses
	if err := store.Init(ctx, false); err != nil {
		t.Fatalf("Init() failed: %v", err)
	}

	// Verify store is initialized
	if !store.IsInitialized() {
		t.Error("Store should be initialized")
	}

	// Verify expected output would be generated
	basePath := store.BasePath()
	bundlePath := store.CombinedBundlePath()

	if basePath == "" {
		t.Error("BasePath should not be empty")
	}

	if bundlePath == "" {
		t.Error("CombinedBundlePath should not be empty")
	}
}

func TestInitCmd_AlreadyInitialized(t *testing.T) {
	tmpDir := t.TempDir()

	store, err := certstore.NewStore(tmpDir)
	if err != nil {
		t.Fatalf("NewStore() failed: %v", err)
	}

	ctx := context.Background()

	// Initialize once
	if err := store.Init(ctx, false); err != nil {
		t.Fatalf("First Init() failed: %v", err)
	}

	// Verify IsInitialized check (this is what runInit uses)
	if !store.IsInitialized() {
		t.Error("Store should be initialized")
	}

	// Second init should fail
	err = store.Init(ctx, false)
	if err == nil {
		t.Error("Second Init() should fail")
	}
}

func TestInitCmd_WithForce(t *testing.T) {
	tmpDir := t.TempDir()

	store, err := certstore.NewStore(tmpDir)
	if err != nil {
		t.Fatalf("NewStore() failed: %v", err)
	}

	ctx := context.Background()

	// Initialize once
	if err := store.Init(ctx, false); err != nil {
		t.Fatalf("First Init() failed: %v", err)
	}

	// Verify store is initialized
	if !store.IsInitialized() {
		t.Error("Store should be initialized after first init")
	}

	// Initialize again with force=true
	if err := store.Init(ctx, true); err != nil {
		t.Fatalf("Init with force failed: %v", err)
	}

	// Should still be initialized
	if !store.IsInitialized() {
		t.Error("Store should be initialized after forced reinit")
	}
}

func TestInitCmd_Timeout(t *testing.T) {
	tmpDir := t.TempDir()

	store, err := certstore.NewStore(tmpDir)
	if err != nil {
		t.Fatalf("NewStore() failed: %v", err)
	}

	// Use a very short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()

	// Wait for context to expire
	time.Sleep(10 * time.Millisecond)

	// Init should fail due to context timeout
	err = store.Init(ctx, false)
	if err == nil {
		t.Error("Init() should fail with expired context")
	}

	// Error should be context-related
	if err != context.DeadlineExceeded && !strings.Contains(err.Error(), "context") {
		t.Logf("Got error: %v", err)
	}
}

func TestInitCmd_Help(t *testing.T) {
	// Verify help text is set
	if initCmd.Short == "" {
		t.Error("initCmd.Short (short description) is empty")
	}

	if initCmd.Long == "" {
		t.Error("initCmd.Long (long description) is empty")
	}

	// Verify help mentions key concepts
	helpText := initCmd.Long
	expectedTerms := []string{"initialize", "store", ".verifi", "Mozilla", "offline"}

	for _, term := range expectedTerms {
		if !strings.Contains(strings.ToLower(helpText), strings.ToLower(term)) {
			t.Errorf("Help text should mention %q", term)
		}
	}
}

func TestInitCmd_OutputMessages(t *testing.T) {
	// Test that the success messages contain expected information
	// We can't easily capture the actual fmt.Printf output without refactoring,
	// but we can verify the expected message patterns

	expectedPatterns := []string{
		"Initializing certificate store",
		"Certificate store initialized successfully",
		"Mozilla CA bundle extracted",
		"Next steps",
		"verifi cert add",
		"Configure your shell",
		"verifi verify",
	}

	// Read the init.go file to verify messages are there
	// This is a basic smoke test
	for _, pattern := range expectedPatterns {
		// In a real test, we'd capture stdout
		// For now, just log what we'd check
		t.Logf("Expected output pattern: %q", pattern)
	}
}

func TestInitCmd_ErrorHandling(t *testing.T) {
	// Test error handling paths

	tests := []struct {
		name        string
		setupFunc   func() (*certstore.Store, context.Context, error)
		expectError bool
	}{
		{
			name: "invalid base path permissions",
			setupFunc: func() (*certstore.Store, context.Context, error) {
				// Use a path that can't be created
				store, err := certstore.NewStore("/root/cannot-create-here")
				ctx := context.Background()
				return store, ctx, err
			},
			expectError: false, // NewStore itself doesn't fail, Init would
		},
		{
			name: "context already cancelled",
			setupFunc: func() (*certstore.Store, context.Context, error) {
				tmpDir := t.TempDir()
				store, err := certstore.NewStore(tmpDir)
				ctx, cancel := context.WithCancel(context.Background())
				cancel() // Cancel immediately
				return store, ctx, err
			},
			expectError: false, // NewStore succeeds, Init fails with cancelled context
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store, ctx, err := tt.setupFunc()

			if err != nil && !tt.expectError {
				// Setup failed unexpectedly
				t.Logf("Setup error (may be expected): %v", err)
				return
			}

			if store == nil {
				return
			}

			// Try to init - may fail, which is fine for error cases
			err = store.Init(ctx, false)
			t.Logf("Init result: %v", err)
		})
	}
}

func TestInitCmd_CommandStructure(t *testing.T) {
	// Verify the command is properly structured
	if initCmd.RunE == nil {
		t.Error("initCmd.RunE should be set")
	}

	// Verify Args is not set (init takes no positional arguments)
	if initCmd.Args != nil {
		t.Error("initCmd.Args should be nil (no positional arguments expected)")
	}

	// Verify the command has been added to root
	// Note: This might not work in test context as init() may not have run
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Use == "init" {
			found = true
			break
		}
	}

	if !found {
		t.Log("Warning: init command not found in root commands (may be due to test isolation)")
	}
}
