package ssm

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"
	"github.com/towardsthecloud/aws-toolbox/internal/cliutil"
)

type mockClient struct {
	deleteParameterFn func(context.Context, *ssm.DeleteParameterInput, ...func(*ssm.Options)) (*ssm.DeleteParameterOutput, error)
	putParameterFn    func(context.Context, *ssm.PutParameterInput, ...func(*ssm.Options)) (*ssm.PutParameterOutput, error)
}

func (m *mockClient) DeleteParameter(ctx context.Context, in *ssm.DeleteParameterInput, optFns ...func(*ssm.Options)) (*ssm.DeleteParameterOutput, error) {
	if m.deleteParameterFn == nil {
		return nil, errors.New("DeleteParameter not mocked")
	}
	return m.deleteParameterFn(ctx, in, optFns...)
}

func (m *mockClient) PutParameter(ctx context.Context, in *ssm.PutParameterInput, optFns ...func(*ssm.Options)) (*ssm.PutParameterOutput, error) {
	if m.putParameterFn == nil {
		return nil, errors.New("PutParameter not mocked")
	}
	return m.putParameterFn(ctx, in, optFns...)
}

func withMockDeps(t *testing.T, loader func(string, string) (awssdk.Config, error), factory func(awssdk.Config) API) {
	t.Helper()

	oldLoader := loadAWSConfig
	oldFactory := newClient

	loadAWSConfig = loader
	newClient = factory

	t.Cleanup(func() {
		loadAWSConfig = oldLoader
		newClient = oldFactory
	})
}

func executeCommand(t *testing.T, args ...string) (string, error) {
	t.Helper()
	return executeCommandWithInput(t, "", args...)
}

func executeCommandWithInput(t *testing.T, input string, args ...string) (string, error) {
	t.Helper()

	root := cliutil.NewTestRootCommand(NewCommand())
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetIn(strings.NewReader(input))
	root.SetArgs(args)

	err := root.Execute()
	return buf.String(), err
}

func TestImportParametersReadsFromInputFile(t *testing.T) {
	inputPath := filepath.Join(t.TempDir(), "params.json")
	content := `[
  {"Name":"/service/foo","Type":"String","Value":"one"},
  {"Name":"/service/bar","Type":"SecureString","Value":"two","Overwrite":true}
]`
	if err := os.WriteFile(inputPath, []byte(content), 0o600); err != nil {
		t.Fatalf("write params file: %v", err)
	}

	putCalls := make([]string, 0)
	client := &mockClient{
		putParameterFn: func(_ context.Context, in *ssm.PutParameterInput, _ ...func(*ssm.Options)) (*ssm.PutParameterOutput, error) {
			putCalls = append(putCalls, cliutil.PointerToString(in.Name))
			return &ssm.PutParameterOutput{Version: int64(1)}, nil
		},
	}

	withMockDeps(
		t,
		func(_, _ string) (awssdk.Config, error) { return awssdk.Config{Region: "us-east-1"}, nil },
		func(awssdk.Config) API { return client },
	)

	output, err := executeCommand(t, "--output", "json", "ssm", "import-parameters", "--input-file", inputPath)
	if err != nil {
		t.Fatalf("execute ssm import-parameters: %v", err)
	}

	if len(putCalls) != 2 {
		t.Fatalf("expected 2 put calls, got %d", len(putCalls))
	}
	for _, expected := range []string{"/service/foo", "/service/bar", `"action": "imported"`} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected output to contain %q\n%s", expected, output)
		}
	}
}

func TestImportParametersRequiresInputFile(t *testing.T) {
	output, err := executeCommand(t, "ssm", "import-parameters")
	if err == nil {
		t.Fatalf("expected error, got nil and output=%s", output)
	}
	if !strings.Contains(err.Error(), "--input-file is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestImportParametersParsesTypeCaseInsensitive(t *testing.T) {
	inputPath := filepath.Join(t.TempDir(), "params-lower.json")
	content := `[
  {"name":"/service/sample","type":"stringlist","value":"one,two,three"}
]`
	if err := os.WriteFile(inputPath, []byte(content), 0o600); err != nil {
		t.Fatalf("write params file: %v", err)
	}

	client := &mockClient{
		putParameterFn: func(_ context.Context, in *ssm.PutParameterInput, _ ...func(*ssm.Options)) (*ssm.PutParameterOutput, error) {
			if in.Type != ssmtypes.ParameterTypeStringList {
				t.Fatalf("unexpected type: %s", in.Type)
			}
			return &ssm.PutParameterOutput{Version: int64(1)}, nil
		},
	}

	withMockDeps(
		t,
		func(_, _ string) (awssdk.Config, error) { return awssdk.Config{Region: "us-east-1"}, nil },
		func(awssdk.Config) API { return client },
	)

	if _, err := executeCommand(t, "--output", "json", "ssm", "import-parameters", "--input-file", inputPath); err != nil {
		t.Fatalf("execute ssm import-parameters: %v", err)
	}
}

// ---------------------------------------------------------------------------
// delete-parameters: require --input-file
// ---------------------------------------------------------------------------

func TestDeleteParametersRequiresInputFile(t *testing.T) {
	withMockDeps(
		t,
		func(_, _ string) (awssdk.Config, error) { return awssdk.Config{Region: "us-east-1"}, nil },
		func(awssdk.Config) API { return &mockClient{} },
	)

	_, err := executeCommand(t, "ssm", "delete-parameters")
	if err == nil {
		t.Fatal("expected error for missing --input-file")
	}
	if !strings.Contains(err.Error(), "--input-file is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// delete-parameters: dry-run shows would-delete without calling API
// ---------------------------------------------------------------------------

func TestDeleteParametersDryRun(t *testing.T) {
	inputPath := filepath.Join(t.TempDir(), "names.json")
	content := `["/app/param-a", "/app/param-b"]`
	if err := os.WriteFile(inputPath, []byte(content), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	deleteCalls := 0
	client := &mockClient{
		deleteParameterFn: func(_ context.Context, _ *ssm.DeleteParameterInput, _ ...func(*ssm.Options)) (*ssm.DeleteParameterOutput, error) {
			deleteCalls++
			return &ssm.DeleteParameterOutput{}, nil
		},
	}

	withMockDeps(
		t,
		func(_, _ string) (awssdk.Config, error) { return awssdk.Config{Region: "us-east-1"}, nil },
		func(awssdk.Config) API { return client },
	)

	output, err := executeCommand(t, "--output", "json", "--dry-run", "ssm", "delete-parameters", "--input-file", inputPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if deleteCalls != 0 {
		t.Fatalf("expected 0 delete calls in dry-run, got %d", deleteCalls)
	}
	if !strings.Contains(output, "would-delete") {
		t.Fatalf("expected would-delete in output:\n%s", output)
	}
}

// ---------------------------------------------------------------------------
// delete-parameters: no-confirm executes deletes automatically
// ---------------------------------------------------------------------------

func TestDeleteParametersNoConfirm(t *testing.T) {
	inputPath := filepath.Join(t.TempDir(), "names.json")
	content := `["/app/alpha", "/app/beta"]`
	if err := os.WriteFile(inputPath, []byte(content), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	deleted := make([]string, 0)
	client := &mockClient{
		deleteParameterFn: func(_ context.Context, in *ssm.DeleteParameterInput, _ ...func(*ssm.Options)) (*ssm.DeleteParameterOutput, error) {
			deleted = append(deleted, cliutil.PointerToString(in.Name))
			return &ssm.DeleteParameterOutput{}, nil
		},
	}

	withMockDeps(
		t,
		func(_, _ string) (awssdk.Config, error) { return awssdk.Config{Region: "us-east-1"}, nil },
		func(awssdk.Config) API { return client },
	)

	output, err := executeCommand(t, "--output", "json", "--no-confirm", "ssm", "delete-parameters", "--input-file", inputPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(deleted) != 2 {
		t.Fatalf("expected 2 deletes, got %d", len(deleted))
	}
	if !strings.Contains(output, "deleted") {
		t.Fatalf("expected 'deleted' in output:\n%s", output)
	}
}

// ---------------------------------------------------------------------------
// delete-parameters: user declines confirmation -> cancelled
// ---------------------------------------------------------------------------

func TestDeleteParametersUserDeclinesConfirmation(t *testing.T) {
	inputPath := filepath.Join(t.TempDir(), "names.json")
	content := `["/app/param-x"]`
	if err := os.WriteFile(inputPath, []byte(content), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	deleteCalls := 0
	client := &mockClient{
		deleteParameterFn: func(_ context.Context, _ *ssm.DeleteParameterInput, _ ...func(*ssm.Options)) (*ssm.DeleteParameterOutput, error) {
			deleteCalls++
			return &ssm.DeleteParameterOutput{}, nil
		},
	}

	withMockDeps(
		t,
		func(_, _ string) (awssdk.Config, error) { return awssdk.Config{Region: "us-east-1"}, nil },
		func(awssdk.Config) API { return client },
	)

	output, err := executeCommandWithInput(t, "n\n", "--output", "json", "ssm", "delete-parameters", "--input-file", inputPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if deleteCalls != 0 {
		t.Fatalf("expected 0 delete calls when user declines, got %d", deleteCalls)
	}
	if !strings.Contains(output, "cancelled") {
		t.Fatalf("expected 'cancelled' in output:\n%s", output)
	}
}

// ---------------------------------------------------------------------------
// delete-parameters: API error surfaces in output
// ---------------------------------------------------------------------------

func TestDeleteParametersAPIError(t *testing.T) {
	inputPath := filepath.Join(t.TempDir(), "names.json")
	content := `["/app/broken"]`
	if err := os.WriteFile(inputPath, []byte(content), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	client := &mockClient{
		deleteParameterFn: func(_ context.Context, _ *ssm.DeleteParameterInput, _ ...func(*ssm.Options)) (*ssm.DeleteParameterOutput, error) {
			return nil, errors.New("ParameterNotFound")
		},
	}

	withMockDeps(
		t,
		func(_, _ string) (awssdk.Config, error) { return awssdk.Config{Region: "us-east-1"}, nil },
		func(awssdk.Config) API { return client },
	)

	output, err := executeCommand(t, "--output", "json", "--no-confirm", "ssm", "delete-parameters", "--input-file", inputPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(output, "failed:") {
		t.Fatalf("expected 'failed:' in output:\n%s", output)
	}
}

// ---------------------------------------------------------------------------
// delete-parameters: input file not found
// ---------------------------------------------------------------------------

func TestDeleteParametersFileNotFound(t *testing.T) {
	withMockDeps(
		t,
		func(_, _ string) (awssdk.Config, error) { return awssdk.Config{Region: "us-east-1"}, nil },
		func(awssdk.Config) API { return &mockClient{} },
	)

	_, err := executeCommand(t, "--output", "json", "ssm", "delete-parameters", "--input-file", "/nonexistent/file.json")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
	if !strings.Contains(err.Error(), "read input file") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// delete-parameters: AWS config load error
// ---------------------------------------------------------------------------

func TestDeleteParametersAWSConfigError(t *testing.T) {
	inputPath := filepath.Join(t.TempDir(), "names.json")
	if err := os.WriteFile(inputPath, []byte(`["/app/x"]`), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	withMockDeps(
		t,
		func(_, _ string) (awssdk.Config, error) { return awssdk.Config{}, errors.New("no credentials") },
		func(awssdk.Config) API { return &mockClient{} },
	)

	_, err := executeCommand(t, "--output", "json", "ssm", "delete-parameters", "--input-file", inputPath)
	if err == nil {
		t.Fatal("expected error for AWS config failure")
	}
	if !strings.Contains(err.Error(), "no credentials") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// import-parameters: dry-run shows would-import without calling PutParameter
// ---------------------------------------------------------------------------

func TestImportParametersDryRun(t *testing.T) {
	inputPath := filepath.Join(t.TempDir(), "params.json")
	content := `[{"Name":"/svc/key","Type":"String","Value":"val"}]`
	if err := os.WriteFile(inputPath, []byte(content), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	putCalls := 0
	client := &mockClient{
		putParameterFn: func(_ context.Context, _ *ssm.PutParameterInput, _ ...func(*ssm.Options)) (*ssm.PutParameterOutput, error) {
			putCalls++
			return &ssm.PutParameterOutput{Version: int64(1)}, nil
		},
	}

	withMockDeps(
		t,
		func(_, _ string) (awssdk.Config, error) { return awssdk.Config{Region: "us-east-1"}, nil },
		func(awssdk.Config) API { return client },
	)

	output, err := executeCommand(t, "--output", "json", "--dry-run", "ssm", "import-parameters", "--input-file", inputPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if putCalls != 0 {
		t.Fatalf("expected 0 put calls in dry-run, got %d", putCalls)
	}
	if !strings.Contains(output, "would-import") {
		t.Fatalf("expected 'would-import' in output:\n%s", output)
	}
}

// ---------------------------------------------------------------------------
// import-parameters: PutParameter API error surfaces as failed
// ---------------------------------------------------------------------------

func TestImportParametersPutError(t *testing.T) {
	inputPath := filepath.Join(t.TempDir(), "params.json")
	content := `[{"Name":"/svc/bad","Type":"String","Value":"x"}]`
	if err := os.WriteFile(inputPath, []byte(content), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	client := &mockClient{
		putParameterFn: func(_ context.Context, _ *ssm.PutParameterInput, _ ...func(*ssm.Options)) (*ssm.PutParameterOutput, error) {
			return nil, errors.New("AccessDenied")
		},
	}

	withMockDeps(
		t,
		func(_, _ string) (awssdk.Config, error) { return awssdk.Config{Region: "us-east-1"}, nil },
		func(awssdk.Config) API { return client },
	)

	output, err := executeCommand(t, "--output", "json", "ssm", "import-parameters", "--input-file", inputPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(output, "failed:") {
		t.Fatalf("expected 'failed:' in output:\n%s", output)
	}
}

// ---------------------------------------------------------------------------
// import-parameters: AWS config load error
// ---------------------------------------------------------------------------

func TestImportParametersAWSConfigError(t *testing.T) {
	inputPath := filepath.Join(t.TempDir(), "params.json")
	if err := os.WriteFile(inputPath, []byte(`[{"Name":"/x","Type":"String","Value":"v"}]`), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	withMockDeps(
		t,
		func(_, _ string) (awssdk.Config, error) { return awssdk.Config{}, errors.New("config boom") },
		func(awssdk.Config) API { return &mockClient{} },
	)

	_, err := executeCommand(t, "--output", "json", "ssm", "import-parameters", "--input-file", inputPath)
	if err == nil {
		t.Fatal("expected error for AWS config failure")
	}
	if !strings.Contains(err.Error(), "config boom") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// import-parameters: file not found
// ---------------------------------------------------------------------------

func TestImportParametersFileNotFound(t *testing.T) {
	withMockDeps(
		t,
		func(_, _ string) (awssdk.Config, error) { return awssdk.Config{Region: "us-east-1"}, nil },
		func(awssdk.Config) API { return &mockClient{} },
	)

	_, err := executeCommand(t, "--output", "json", "ssm", "import-parameters", "--input-file", "/nonexistent/params.json")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
	if !strings.Contains(err.Error(), "read input file") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// import-parameters: missing Name in record
// ---------------------------------------------------------------------------

func TestImportParametersMissingName(t *testing.T) {
	inputPath := filepath.Join(t.TempDir(), "params.json")
	content := `[{"Type":"String","Value":"val"}]`
	if err := os.WriteFile(inputPath, []byte(content), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	withMockDeps(
		t,
		func(_, _ string) (awssdk.Config, error) { return awssdk.Config{Region: "us-east-1"}, nil },
		func(awssdk.Config) API { return &mockClient{} },
	)

	_, err := executeCommand(t, "--output", "json", "ssm", "import-parameters", "--input-file", inputPath)
	if err == nil {
		t.Fatal("expected error for missing Name")
	}
	if !strings.Contains(err.Error(), "missing Name") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// import-parameters: unsupported type
// ---------------------------------------------------------------------------

func TestImportParametersUnsupportedType(t *testing.T) {
	inputPath := filepath.Join(t.TempDir(), "params.json")
	content := `[{"Name":"/svc/key","Type":"BogusType","Value":"val"}]`
	if err := os.WriteFile(inputPath, []byte(content), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	withMockDeps(
		t,
		func(_, _ string) (awssdk.Config, error) { return awssdk.Config{Region: "us-east-1"}, nil },
		func(awssdk.Config) API { return &mockClient{} },
	)

	_, err := executeCommand(t, "--output", "json", "ssm", "import-parameters", "--input-file", inputPath)
	if err == nil {
		t.Fatal("expected error for unsupported type")
	}
	if !strings.Contains(err.Error(), "unsupported parameter type") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// import-parameters: overwrite from lowercase field
// ---------------------------------------------------------------------------

func TestImportParametersOverwriteLowercase(t *testing.T) {
	inputPath := filepath.Join(t.TempDir(), "params.json")
	content := `[{"name":"/svc/key","type":"String","value":"val","overwrite":true,"description":"desc"}]`
	if err := os.WriteFile(inputPath, []byte(content), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	var capturedOverwrite bool
	client := &mockClient{
		putParameterFn: func(_ context.Context, in *ssm.PutParameterInput, _ ...func(*ssm.Options)) (*ssm.PutParameterOutput, error) {
			capturedOverwrite = cliutil.PointerToString((*string)(nil)) == "" // just to use the import
			if in.Overwrite != nil {
				capturedOverwrite = *in.Overwrite
			}
			return &ssm.PutParameterOutput{Version: int64(1)}, nil
		},
	}

	withMockDeps(
		t,
		func(_, _ string) (awssdk.Config, error) { return awssdk.Config{Region: "us-east-1"}, nil },
		func(awssdk.Config) API { return client },
	)

	_, err := executeCommand(t, "--output", "json", "ssm", "import-parameters", "--input-file", inputPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !capturedOverwrite {
		t.Fatal("expected overwrite to be true from lowercase field")
	}
}

// ---------------------------------------------------------------------------
// readParameterNamesFile: simple string array
// ---------------------------------------------------------------------------

func TestReadParameterNamesFileStringArray(t *testing.T) {
	path := filepath.Join(t.TempDir(), "names.json")
	content := `["/param/a", "/param/b", "/param/a"]`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	names, err := readParameterNamesFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// duplicates should be removed
	if len(names) != 2 {
		t.Fatalf("expected 2 unique names, got %d: %v", len(names), names)
	}
}

// ---------------------------------------------------------------------------
// readParameterNamesFile: string array with empty entry
// ---------------------------------------------------------------------------

func TestReadParameterNamesFileEmptyEntry(t *testing.T) {
	path := filepath.Join(t.TempDir(), "names.json")
	content := `["/param/a", "  "]`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	_, err := readParameterNamesFile(path)
	if err == nil {
		t.Fatal("expected error for empty entry")
	}
	if !strings.Contains(err.Error(), "empty name") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// readParameterNamesFile: object array format (extracts names)
// ---------------------------------------------------------------------------

func TestReadParameterNamesFileObjectArray(t *testing.T) {
	path := filepath.Join(t.TempDir(), "names.json")
	content := `[{"Name":"/param/x"},{"name":"/param/y"}]`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	names, err := readParameterNamesFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(names) != 2 {
		t.Fatalf("expected 2 names, got %d: %v", len(names), names)
	}
}

// ---------------------------------------------------------------------------
// readParameterNamesFile: object with missing name
// ---------------------------------------------------------------------------

func TestReadParameterNamesFileMissingName(t *testing.T) {
	path := filepath.Join(t.TempDir(), "names.json")
	content := `[{"Type":"String"}]`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	_, err := readParameterNamesFile(path)
	if err == nil {
		t.Fatal("expected error for missing name in object")
	}
	if !strings.Contains(err.Error(), "missing Name") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// readParameterNamesFile: file not found
// ---------------------------------------------------------------------------

func TestReadParameterNamesFileNotFound(t *testing.T) {
	_, err := readParameterNamesFile("/nonexistent/path.json")
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
	if !strings.Contains(err.Error(), "read input file") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// parseParameterRecords: envelope with lowercase "parameters" key
// ---------------------------------------------------------------------------

func TestParseParameterRecordsEnvelopeLowercase(t *testing.T) {
	data := []byte(`{"parameters":[{"Name":"/p1"},{"Name":"/p2"}]}`)
	records, err := parseParameterRecords(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("expected 2 records, got %d", len(records))
	}
}

// ---------------------------------------------------------------------------
// parseParameterRecords: envelope with uppercase "Parameters" key
// ---------------------------------------------------------------------------

func TestParseParameterRecordsEnvelopeUppercase(t *testing.T) {
	data := []byte(`{"Parameters":[{"Name":"/p1"}]}`)
	records, err := parseParameterRecords(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}
}

// ---------------------------------------------------------------------------
// parseParameterRecords: invalid JSON
// ---------------------------------------------------------------------------

func TestParseParameterRecordsInvalidJSON(t *testing.T) {
	_, err := parseParameterRecords([]byte(`not-json`))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
	if !strings.Contains(err.Error(), "parse input file") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// parseParameterRecords: empty envelope (no parameters key populated)
// ---------------------------------------------------------------------------

func TestParseParameterRecordsEmptyEnvelope(t *testing.T) {
	_, err := parseParameterRecords([]byte(`{"other":"data"}`))
	if err == nil {
		t.Fatal("expected error for empty envelope")
	}
	if !strings.Contains(err.Error(), "parse input file") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// parseParameterType: all valid types and edge cases
// ---------------------------------------------------------------------------

func TestParseParameterType(t *testing.T) {
	cases := []struct {
		input    string
		expected ssmtypes.ParameterType
		wantErr  bool
	}{
		{"", ssmtypes.ParameterTypeString, false},
		{"String", ssmtypes.ParameterTypeString, false},
		{"string", ssmtypes.ParameterTypeString, false},
		{"StringList", ssmtypes.ParameterTypeStringList, false},
		{"stringlist", ssmtypes.ParameterTypeStringList, false},
		{"SecureString", ssmtypes.ParameterTypeSecureString, false},
		{"securestring", ssmtypes.ParameterTypeSecureString, false},
		{"  String  ", ssmtypes.ParameterTypeString, false},
		{"InvalidType", "", true},
	}

	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			got, err := parseParameterType(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.expected {
				t.Fatalf("expected %s, got %s", tc.expected, got)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// uniqueStrings: deduplication and ordering
// ---------------------------------------------------------------------------

func TestUniqueStrings(t *testing.T) {
	input := []string{"b", "a", "b", "c", "a"}
	got := uniqueStrings(input)
	// preserves first-seen order, removes duplicates
	expected := []string{"b", "a", "c"}
	if len(got) != len(expected) {
		t.Fatalf("expected %d, got %d: %v", len(expected), len(got), got)
	}
	for i, v := range expected {
		if got[i] != v {
			t.Fatalf("index %d: expected %q, got %q", i, v, got[i])
		}
	}
}

func TestUniqueStringsEmpty(t *testing.T) {
	got := uniqueStrings(nil)
	if len(got) != 0 {
		t.Fatalf("expected empty, got %v", got)
	}
}

// ---------------------------------------------------------------------------
// firstNonEmpty
// ---------------------------------------------------------------------------

func TestFirstNonEmpty(t *testing.T) {
	if got := firstNonEmpty("", "  ", "hello"); got != "hello" {
		t.Fatalf("expected 'hello', got %q", got)
	}
	if got := firstNonEmpty("", ""); got != "" {
		t.Fatalf("expected empty, got %q", got)
	}
	if got := firstNonEmpty("first", "second"); got != "first" {
		t.Fatalf("expected 'first', got %q", got)
	}
}

// ---------------------------------------------------------------------------
// delete-parameters: reads object-format names file
// ---------------------------------------------------------------------------

func TestDeleteParametersObjectFormatFile(t *testing.T) {
	inputPath := filepath.Join(t.TempDir(), "names.json")
	content := `[{"Name":"/app/obj-param"}]`
	if err := os.WriteFile(inputPath, []byte(content), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	deleted := make([]string, 0)
	client := &mockClient{
		deleteParameterFn: func(_ context.Context, in *ssm.DeleteParameterInput, _ ...func(*ssm.Options)) (*ssm.DeleteParameterOutput, error) {
			deleted = append(deleted, cliutil.PointerToString(in.Name))
			return &ssm.DeleteParameterOutput{}, nil
		},
	}

	withMockDeps(
		t,
		func(_, _ string) (awssdk.Config, error) { return awssdk.Config{Region: "us-east-1"}, nil },
		func(awssdk.Config) API { return client },
	)

	output, err := executeCommand(t, "--output", "json", "--no-confirm", "ssm", "delete-parameters", "--input-file", inputPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(deleted) != 1 || deleted[0] != "/app/obj-param" {
		t.Fatalf("unexpected deletes: %v", deleted)
	}
	if !strings.Contains(output, "deleted") {
		t.Fatalf("expected 'deleted' in output:\n%s", output)
	}
}

// ---------------------------------------------------------------------------
// delete-parameters: duplicate names are deduplicated
// ---------------------------------------------------------------------------

func TestDeleteParametersDeduplicatesNames(t *testing.T) {
	inputPath := filepath.Join(t.TempDir(), "names.json")
	content := `["/app/dup", "/app/dup", "/app/other"]`
	if err := os.WriteFile(inputPath, []byte(content), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	deleted := make([]string, 0)
	client := &mockClient{
		deleteParameterFn: func(_ context.Context, in *ssm.DeleteParameterInput, _ ...func(*ssm.Options)) (*ssm.DeleteParameterOutput, error) {
			deleted = append(deleted, cliutil.PointerToString(in.Name))
			return &ssm.DeleteParameterOutput{}, nil
		},
	}

	withMockDeps(
		t,
		func(_, _ string) (awssdk.Config, error) { return awssdk.Config{Region: "us-east-1"}, nil },
		func(awssdk.Config) API { return client },
	)

	_, err := executeCommand(t, "--output", "json", "--no-confirm", "ssm", "delete-parameters", "--input-file", inputPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(deleted) != 2 {
		t.Fatalf("expected 2 unique deletes, got %d: %v", len(deleted), deleted)
	}
}

// ---------------------------------------------------------------------------
// import-parameters: envelope format with uppercase Parameters key
// ---------------------------------------------------------------------------

func TestImportParametersEnvelopeFormat(t *testing.T) {
	inputPath := filepath.Join(t.TempDir(), "params.json")
	content := `{"Parameters":[{"Name":"/svc/env-param","Type":"String","Value":"envval"}]}`
	if err := os.WriteFile(inputPath, []byte(content), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	putCalls := make([]string, 0)
	client := &mockClient{
		putParameterFn: func(_ context.Context, in *ssm.PutParameterInput, _ ...func(*ssm.Options)) (*ssm.PutParameterOutput, error) {
			putCalls = append(putCalls, cliutil.PointerToString(in.Name))
			return &ssm.PutParameterOutput{Version: int64(1)}, nil
		},
	}

	withMockDeps(
		t,
		func(_, _ string) (awssdk.Config, error) { return awssdk.Config{Region: "us-east-1"}, nil },
		func(awssdk.Config) API { return client },
	)

	output, err := executeCommand(t, "--output", "json", "ssm", "import-parameters", "--input-file", inputPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(putCalls) != 1 || putCalls[0] != "/svc/env-param" {
		t.Fatalf("unexpected put calls: %v", putCalls)
	}
	if !strings.Contains(output, "imported") {
		t.Fatalf("expected 'imported' in output:\n%s", output)
	}
}

// ---------------------------------------------------------------------------
// import-parameters: SecureString type default is empty -> defaults to String
// ---------------------------------------------------------------------------

func TestImportParametersDefaultType(t *testing.T) {
	inputPath := filepath.Join(t.TempDir(), "params.json")
	content := `[{"Name":"/svc/noTypeField","Value":"val"}]`
	if err := os.WriteFile(inputPath, []byte(content), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	var capturedType ssmtypes.ParameterType
	client := &mockClient{
		putParameterFn: func(_ context.Context, in *ssm.PutParameterInput, _ ...func(*ssm.Options)) (*ssm.PutParameterOutput, error) {
			capturedType = in.Type
			return &ssm.PutParameterOutput{Version: int64(1)}, nil
		},
	}

	withMockDeps(
		t,
		func(_, _ string) (awssdk.Config, error) { return awssdk.Config{Region: "us-east-1"}, nil },
		func(awssdk.Config) API { return client },
	)

	_, err := executeCommand(t, "--output", "json", "ssm", "import-parameters", "--input-file", inputPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedType != ssmtypes.ParameterTypeString {
		t.Fatalf("expected String type default, got %s", capturedType)
	}
}

// ---------------------------------------------------------------------------
// delete-parameters: envelope format file
// ---------------------------------------------------------------------------

func TestDeleteParametersEnvelopeFormat(t *testing.T) {
	inputPath := filepath.Join(t.TempDir(), "names.json")
	content := `{"parameters":[{"Name":"/env/p1"},{"Name":"/env/p2"}]}`
	if err := os.WriteFile(inputPath, []byte(content), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	deleted := make([]string, 0)
	client := &mockClient{
		deleteParameterFn: func(_ context.Context, in *ssm.DeleteParameterInput, _ ...func(*ssm.Options)) (*ssm.DeleteParameterOutput, error) {
			deleted = append(deleted, cliutil.PointerToString(in.Name))
			return &ssm.DeleteParameterOutput{}, nil
		},
	}

	withMockDeps(
		t,
		func(_, _ string) (awssdk.Config, error) { return awssdk.Config{Region: "us-east-1"}, nil },
		func(awssdk.Config) API { return client },
	)

	_, err := executeCommand(t, "--output", "json", "--no-confirm", "ssm", "delete-parameters", "--input-file", inputPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(deleted) != 2 {
		t.Fatalf("expected 2 deletes, got %d: %v", len(deleted), deleted)
	}
}

// ---------------------------------------------------------------------------
// readParameterNamesFile: invalid JSON (not array, not envelope)
// ---------------------------------------------------------------------------

func TestReadParameterNamesFileInvalidJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.json")
	if err := os.WriteFile(path, []byte(`{invalid json`), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	_, err := readParameterNamesFile(path)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

// ---------------------------------------------------------------------------
// import-parameters: envelope format with lowercase "parameters" key
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// import-parameters: invalid JSON file (not array, not envelope)
// ---------------------------------------------------------------------------

func TestImportParametersInvalidJSON(t *testing.T) {
	inputPath := filepath.Join(t.TempDir(), "params.json")
	content := `{"randomKey": "this is not parameters"}`
	if err := os.WriteFile(inputPath, []byte(content), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	withMockDeps(
		t,
		func(_, _ string) (awssdk.Config, error) { return awssdk.Config{Region: "us-east-1"}, nil },
		func(awssdk.Config) API { return &mockClient{} },
	)

	_, err := executeCommand(t, "--output", "json", "ssm", "import-parameters", "--input-file", inputPath)
	if err == nil {
		t.Fatal("expected error for invalid JSON format")
	}
	if !strings.Contains(err.Error(), "parse input file") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestImportParametersEnvelopeLowercase(t *testing.T) {
	inputPath := filepath.Join(t.TempDir(), "params.json")
	content := `{"parameters":[{"Name":"/svc/low","Type":"SecureString","Value":"secret"}]}`
	if err := os.WriteFile(inputPath, []byte(content), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	var capturedType ssmtypes.ParameterType
	client := &mockClient{
		putParameterFn: func(_ context.Context, in *ssm.PutParameterInput, _ ...func(*ssm.Options)) (*ssm.PutParameterOutput, error) {
			capturedType = in.Type
			return &ssm.PutParameterOutput{Version: int64(1)}, nil
		},
	}

	withMockDeps(
		t,
		func(_, _ string) (awssdk.Config, error) { return awssdk.Config{Region: "us-east-1"}, nil },
		func(awssdk.Config) API { return client },
	)

	_, err := executeCommand(t, "--output", "json", "ssm", "import-parameters", "--input-file", inputPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedType != ssmtypes.ParameterTypeSecureString {
		t.Fatalf("expected SecureString, got %s", capturedType)
	}
}
