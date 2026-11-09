package catalog

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/databricks/databricks-sdk-go/qa"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

const (
	testSecretSchemaFullName = "main.secrets"
	testSecretConnectionName = "kv_uc_secrets_demo"

	// Wire representations of secret_storage, written out literally so that the
	// tests pin the JSON field names and enum values rather than whatever the
	// structs happen to marshal to.
	testBoundSecretStorageJSON   = `{"storage_type":"STORAGE_TYPE_EXTERNAL","connection_name":"kv_uc_secrets_demo"}`
	testUnboundSecretStorageJSON = `{"storage_type":"STORAGE_TYPE_UNSPECIFIED"}`
)

var testBoundSecretStorage = SchemaSecretStorage{
	ConnectionName: testSecretConnectionName,
	StorageType:    SchemaSecretStorageTypeStorageTypeExternal,
}

// ignoreForceSendFields drops the ForceSendFields bookkeeping that unmarshaling
// records for every field present in a response, so that comparisons are about
// the decoded values only.
var ignoreForceSendFields = cmp.Options{
	cmpopts.IgnoreFields(SchemaInfo{}, "ForceSendFields"),
	cmpopts.IgnoreFields(SchemaSecretStorage{}, "ForceSendFields"),
}

// newTestSchemasAPI returns a SchemasAPI talking to a local HTTP server that
// only answers the given fixtures. The fixtures match on the raw request URI
// and, when ExpectedRequest is set, on the JSON request body.
func newTestSchemasAPI(t *testing.T, fixtures qa.HTTPFixtures) *SchemasAPI {
	t.Helper()
	client, server := fixtures.Client(t)
	t.Cleanup(server.Close)
	return NewSchemas(client)
}

func TestSchemasCreate_secretStorage(t *testing.T) {
	api := newTestSchemasAPI(t, qa.HTTPFixtures{
		{
			Method:   "POST",
			Resource: "/api/2.1/unity-catalog/schemas",
			ExpectedRequest: json.RawMessage(`{
				"catalog_name": "main",
				"name": "secrets",
				"secret_storage": ` + testBoundSecretStorageJSON + `
			}`),
			Response: `{
				"full_name": "main.secrets",
				"secret_storage": ` + testBoundSecretStorageJSON + `
			}`,
		},
	})

	got, err := api.Create(context.Background(), CreateSchema{
		CatalogName:   "main",
		Name:          "secrets",
		SecretStorage: &testBoundSecretStorage,
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	want := &SchemaInfo{
		FullName:      testSecretSchemaFullName,
		SecretStorage: &testBoundSecretStorage,
	}
	if diff := cmp.Diff(want, got, ignoreForceSendFields); diff != "" {
		t.Errorf("Create mismatch (-want +got):\n%s", diff)
	}
}

func TestSchemasUpdate_secretStorage(t *testing.T) {
	testCases := []struct {
		name          string
		secretStorage *SchemaSecretStorage
		wantRequest   string
	}{
		{
			name:          "bind to connection",
			secretStorage: &testBoundSecretStorage,
			wantRequest:   `{"secret_storage": ` + testBoundSecretStorageJSON + `}`,
		},
		{
			name: "unbind",
			secretStorage: &SchemaSecretStorage{
				StorageType: SchemaSecretStorageTypeStorageTypeUnspecified,
			},
			wantRequest: `{"secret_storage": ` + testUnboundSecretStorageJSON + `}`,
		},
		{
			name:          "unset leaves the binding out of the request",
			secretStorage: nil,
			wantRequest:   `{}`,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			api := newTestSchemasAPI(t, qa.HTTPFixtures{
				{
					Method:          "PATCH",
					Resource:        "/api/2.1/unity-catalog/schemas/main.secrets",
					ExpectedRequest: json.RawMessage(tc.wantRequest),
					Response:        `{"full_name": "main.secrets"}`,
				},
			})

			_, err := api.Update(context.Background(), UpdateSchema{
				FullName:      testSecretSchemaFullName,
				SecretStorage: tc.secretStorage,
			})
			if err != nil {
				t.Fatalf("Update returned error: %v", err)
			}
		})
	}
}

func TestSchemasGet_includeSecretStorage(t *testing.T) {
	testCases := []struct {
		name         string
		request      GetSchemaRequest
		wantResource string
		response     string
		want         *SchemaInfo
	}{
		{
			name: "explicit true is sent as a query parameter",
			request: GetSchemaRequest{
				FullName:             testSecretSchemaFullName,
				IncludeSecretStorage: true,
			},
			wantResource: "/api/2.1/unity-catalog/schemas/main.secrets?include_secret_storage=true",
			response: `{
				"full_name": "main.secrets",
				"secret_storage": ` + testBoundSecretStorageJSON + `
			}`,
			want: &SchemaInfo{
				FullName:      testSecretSchemaFullName,
				SecretStorage: &testBoundSecretStorage,
			},
		},
		{
			name: "unset is omitted from the query",
			request: GetSchemaRequest{
				FullName: testSecretSchemaFullName,
			},
			wantResource: "/api/2.1/unity-catalog/schemas/main.secrets?",
			response:     `{"full_name": "main.secrets"}`,
			want: &SchemaInfo{
				FullName: testSecretSchemaFullName,
			},
		},
		{
			name: "explicit false is sent when forced",
			request: GetSchemaRequest{
				FullName:        testSecretSchemaFullName,
				ForceSendFields: []string{"IncludeSecretStorage"},
			},
			wantResource: "/api/2.1/unity-catalog/schemas/main.secrets?include_secret_storage=false",
			response:     `{"full_name": "main.secrets"}`,
			want: &SchemaInfo{
				FullName: testSecretSchemaFullName,
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			api := newTestSchemasAPI(t, qa.HTTPFixtures{
				{
					Method:   "GET",
					Resource: tc.wantResource,
					Response: tc.response,
				},
			})

			got, err := api.Get(context.Background(), tc.request)
			if err != nil {
				t.Fatalf("Get returned error: %v", err)
			}
			if diff := cmp.Diff(tc.want, got, ignoreForceSendFields); diff != "" {
				t.Errorf("Get mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestSchemasListAll_includeSecretStorage(t *testing.T) {
	api := newTestSchemasAPI(t, qa.HTTPFixtures{
		{
			Method:   "GET",
			Resource: "/api/2.1/unity-catalog/schemas?catalog_name=main&include_secret_storage=true&max_results=0",
			Response: `{
				"schemas": [
					{
						"full_name": "main.secrets",
						"secret_storage": ` + testBoundSecretStorageJSON + `
					},
					{"full_name": "main.default"}
				]
			}`,
		},
	})

	got, err := api.ListAll(context.Background(), ListSchemasRequest{
		CatalogName:          "main",
		IncludeSecretStorage: true,
	})
	if err != nil {
		t.Fatalf("ListAll returned error: %v", err)
	}

	want := []SchemaInfo{
		{
			FullName:      testSecretSchemaFullName,
			SecretStorage: &testBoundSecretStorage,
		},
		{FullName: "main.default"},
	}
	if diff := cmp.Diff(want, got, ignoreForceSendFields); diff != "" {
		t.Errorf("ListAll mismatch (-want +got):\n%s", diff)
	}
}

func TestSchemaSecretStorageType_Set(t *testing.T) {
	testCases := []struct {
		name    string
		value   string
		want    SchemaSecretStorageType
		wantErr string
	}{
		{
			name:  "external",
			value: "STORAGE_TYPE_EXTERNAL",
			want:  SchemaSecretStorageTypeStorageTypeExternal,
		},
		{
			name:  "unspecified",
			value: "STORAGE_TYPE_UNSPECIFIED",
			want:  SchemaSecretStorageTypeStorageTypeUnspecified,
		},
		{
			name:    "unknown value is rejected",
			value:   "STORAGE_TYPE_INTERNAL",
			wantErr: `value "STORAGE_TYPE_INTERNAL" is not one of "STORAGE_TYPE_EXTERNAL", "STORAGE_TYPE_UNSPECIFIED"`,
		},
		{
			name:    "empty value is rejected",
			value:   "",
			wantErr: `value "" is not one of "STORAGE_TYPE_EXTERNAL", "STORAGE_TYPE_UNSPECIFIED"`,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var got SchemaSecretStorageType
			err := got.Set(tc.value)
			gotErr := ""
			if err != nil {
				gotErr = err.Error()
			}
			if gotErr != tc.wantErr {
				t.Fatalf("Set(%q) error = %q, want %q", tc.value, gotErr, tc.wantErr)
			}
			if got != tc.want {
				t.Errorf("Set(%q) = %q, want %q", tc.value, got, tc.want)
			}
		})
	}
}
