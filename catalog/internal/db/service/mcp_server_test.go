package service_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/kubeflow/model-registry/catalog/internal/db/models"
	"github.com/kubeflow/model-registry/catalog/internal/db/service"
	"github.com/kubeflow/model-registry/internal/apiutils"
	dbmodels "github.com/kubeflow/model-registry/internal/db/models"
	"github.com/kubeflow/model-registry/internal/db/schema"
	"github.com/kubeflow/model-registry/internal/testutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestMcpServerRepository(t *testing.T) {
	sharedDB, cleanup := testutils.SetupPostgresWithMigrations(t, service.DatastoreSpec())
	defer cleanup()

	// Create or get the McpServer type ID
	typeID := getMcpServerTypeID(t, sharedDB)
	repo := service.NewMcpServerRepository(sharedDB, typeID)

	t.Run("TestSave_Create", func(t *testing.T) {
		// Test creating a new MCP server
		mcpServer := &models.McpServerImpl{
			Attributes: &models.McpServerAttributes{
				Name:       apiutils.Of("test-mcp-server"),
				ExternalID: apiutils.Of("mcp-ext-123"),
			},
			Properties: &[]dbmodels.Properties{
				{
					Name:        "source_id",
					StringValue: apiutils.Of("test-source"),
				},
				{
					Name:        "description",
					StringValue: apiutils.Of("Test MCP server description"),
				},
				{
					Name:        "provider",
					StringValue: apiutils.Of("test-provider"),
				},
				{
					Name:        "version",
					StringValue: apiutils.Of("1.0.0"),
				},
				{
					Name:       "verifiedSource",
					BoolValue:  apiutils.Of(true),
				},
			},
			CustomProperties: &[]dbmodels.Properties{
				{
					Name:        "custom-prop",
					StringValue: apiutils.Of("custom-value"),
				},
			},
		}

		saved, err := repo.Save(mcpServer)
		require.NoError(t, err)
		require.NotNil(t, saved)
		require.NotNil(t, saved.GetID())
		assert.Equal(t, "test-mcp-server", *saved.GetAttributes().Name)
		assert.Equal(t, "mcp-ext-123", *saved.GetAttributes().ExternalID)
	})

	t.Run("TestSave_Update", func(t *testing.T) {
		// Create initial server
		mcpServer := &models.McpServerImpl{
			Attributes: &models.McpServerAttributes{
				Name: apiutils.Of("update-test-server"),
			},
			Properties: &[]dbmodels.Properties{
				{
					Name:        "source_id",
					StringValue: apiutils.Of("update-source"),
				},
				{
					Name:        "version",
					StringValue: apiutils.Of("1.0.0"),
				},
			},
		}

		saved, err := repo.Save(mcpServer)
		require.NoError(t, err)
		require.NotNil(t, saved.GetID())

		// Update the server
		updateServer := &models.McpServerImpl{
			ID: saved.GetID(),
			Attributes: &models.McpServerAttributes{
				Name:                     apiutils.Of("update-test-server"),
				CreateTimeSinceEpoch:     saved.GetAttributes().CreateTimeSinceEpoch,
			},
			Properties: &[]dbmodels.Properties{
				{
					Name:        "source_id",
					StringValue: apiutils.Of("update-source"),
				},
				{
					Name:        "version",
					StringValue: apiutils.Of("2.0.0"), // Updated version
				},
				{
					Name:        "description",
					StringValue: apiutils.Of("Updated description"),
				},
			},
		}

		updated, err := repo.Save(updateServer)
		require.NoError(t, err)
		assert.Equal(t, *saved.GetID(), *updated.GetID())

		// Verify properties were updated
		require.NotNil(t, updated.GetProperties())
		versionFound := false
		descriptionFound := false
		for _, prop := range *updated.GetProperties() {
			if prop.Name == "version" && prop.StringValue != nil && *prop.StringValue == "2.0.0" {
				versionFound = true
			}
			if prop.Name == "description" && prop.StringValue != nil {
				descriptionFound = true
			}
		}
		assert.True(t, versionFound, "Version should be updated")
		assert.True(t, descriptionFound, "Description should be added")
	})

	t.Run("TestSave_UpsertByNameAndVersion_SameVersion", func(t *testing.T) {
		// Create first server with name and version
		server1 := &models.McpServerImpl{
			Attributes: &models.McpServerAttributes{
				Name: apiutils.Of("upsert-server"),
			},
			Properties: &[]dbmodels.Properties{
				{
					Name:        "source_id",
					StringValue: apiutils.Of("upsert-source"),
				},
				{
					Name:        "version",
					StringValue: apiutils.Of("1.0.0"),
				},
				{
					Name:        "description",
					StringValue: apiutils.Of("Initial description"),
				},
			},
		}

		saved1, err := repo.Save(server1)
		require.NoError(t, err)
		require.NotNil(t, saved1.GetID())

		// Save another server with same name and version (without ID)
		// This should UPDATE the existing server
		server2 := &models.McpServerImpl{
			Attributes: &models.McpServerAttributes{
				Name: apiutils.Of("upsert-server"),
			},
			Properties: &[]dbmodels.Properties{
				{
					Name:        "source_id",
					StringValue: apiutils.Of("upsert-source"),
				},
				{
					Name:        "version",
					StringValue: apiutils.Of("1.0.0"),
				},
				{
					Name:        "description",
					StringValue: apiutils.Of("Updated description"),
				},
			},
		}

		saved2, err := repo.Save(server2)
		require.NoError(t, err)
		assert.Equal(t, *saved1.GetID(), *saved2.GetID(), "Should update existing server with same name and version")

		// Verify the description was updated
		retrieved, err := repo.GetByNameAndVersion("upsert-server", "1.0.0")
		require.NoError(t, err)
		descFound := false
		for _, prop := range *retrieved.GetProperties() {
			if prop.Name == "description" && prop.StringValue != nil && *prop.StringValue == "Updated description" {
				descFound = true
				break
			}
		}
		assert.True(t, descFound, "Description should be updated")
	})

	t.Run("TestSave_UpsertByNameAndVersion_DifferentVersions", func(t *testing.T) {
		// Create first server with version 1.0.0
		server1 := &models.McpServerImpl{
			Attributes: &models.McpServerAttributes{
				Name: apiutils.Of("multi-version-server"),
			},
			Properties: &[]dbmodels.Properties{
				{
					Name:        "source_id",
					StringValue: apiutils.Of("multi-source"),
				},
				{
					Name:        "version",
					StringValue: apiutils.Of("1.0.0"),
				},
			},
		}

		saved1, err := repo.Save(server1)
		require.NoError(t, err)
		require.NotNil(t, saved1.GetID())

		// Save server with same name but different version (2.0.0)
		// This should CREATE a new server
		server2 := &models.McpServerImpl{
			Attributes: &models.McpServerAttributes{
				Name: apiutils.Of("multi-version-server"),
			},
			Properties: &[]dbmodels.Properties{
				{
					Name:        "source_id",
					StringValue: apiutils.Of("multi-source"),
				},
				{
					Name:        "version",
					StringValue: apiutils.Of("2.0.0"),
				},
			},
		}

		saved2, err := repo.Save(server2)
		require.NoError(t, err)
		assert.NotEqual(t, *saved1.GetID(), *saved2.GetID(), "Should create new server with different version")

		// Verify both versions exist
		v1, err := repo.GetByNameAndVersion("multi-version-server", "1.0.0")
		require.NoError(t, err)
		assert.Equal(t, *saved1.GetID(), *v1.GetID())

		v2, err := repo.GetByNameAndVersion("multi-version-server", "2.0.0")
		require.NoError(t, err)
		assert.Equal(t, *saved2.GetID(), *v2.GetID())
	})

	t.Run("TestGetByID", func(t *testing.T) {
		// Create a server
		mcpServer := &models.McpServerImpl{
			Attributes: &models.McpServerAttributes{
				Name: apiutils.Of("get-by-id-test"),
			},
			Properties: &[]dbmodels.Properties{
				{
					Name:        "source_id",
					StringValue: apiutils.Of("get-source"),
				},
			},
		}

		saved, err := repo.Save(mcpServer)
		require.NoError(t, err)
		require.NotNil(t, saved.GetID())

		// Retrieve by ID
		retrieved, err := repo.GetByID(*saved.GetID())
		require.NoError(t, err)
		require.NotNil(t, retrieved)
		assert.Equal(t, *saved.GetID(), *retrieved.GetID())
		assert.Equal(t, "get-by-id-test", *retrieved.GetAttributes().Name)

		// Test non-existent ID
		_, err = repo.GetByID(99999)
		assert.ErrorIs(t, err, service.ErrMcpServerNotFound)
	})

	t.Run("TestGetByNameAndVersion_WithVersion", func(t *testing.T) {
		// Create a server with version
		mcpServer := &models.McpServerImpl{
			Attributes: &models.McpServerAttributes{
				Name: apiutils.Of("versioned-server"),
			},
			Properties: &[]dbmodels.Properties{
				{
					Name:        "source_id",
					StringValue: apiutils.Of("version-source"),
				},
				{
					Name:        "version",
					StringValue: apiutils.Of("1.0.0"),
				},
			},
		}

		saved, err := repo.Save(mcpServer)
		require.NoError(t, err)
		require.NotNil(t, saved.GetID())

		// Retrieve by name and version
		retrieved, err := repo.GetByNameAndVersion("versioned-server", "1.0.0")
		require.NoError(t, err)
		require.NotNil(t, retrieved)
		assert.Equal(t, *saved.GetID(), *retrieved.GetID())
		assert.Equal(t, "versioned-server", *retrieved.GetAttributes().Name)

		// Test non-existent version
		_, err = repo.GetByNameAndVersion("versioned-server", "2.0.0")
		assert.ErrorIs(t, err, service.ErrMcpServerNotFound)

		// Test non-existent name
		_, err = repo.GetByNameAndVersion("non-existent-server", "1.0.0")
		assert.ErrorIs(t, err, service.ErrMcpServerNotFound)
	})

	t.Run("TestGetByNameAndVersion_NoVersion", func(t *testing.T) {
		// Create a server without version
		mcpServer := &models.McpServerImpl{
			Attributes: &models.McpServerAttributes{
				Name: apiutils.Of("unversioned-server"),
			},
			Properties: &[]dbmodels.Properties{
				{
					Name:        "source_id",
					StringValue: apiutils.Of("no-version-source"),
				},
				{
					Name:        "description",
					StringValue: apiutils.Of("Server without version"),
				},
			},
		}

		saved, err := repo.Save(mcpServer)
		require.NoError(t, err)
		require.NotNil(t, saved.GetID())

		// Retrieve by name with empty version
		retrieved, err := repo.GetByNameAndVersion("unversioned-server", "")
		require.NoError(t, err)
		require.NotNil(t, retrieved)
		assert.Equal(t, *saved.GetID(), *retrieved.GetID())
		assert.Equal(t, "unversioned-server", *retrieved.GetAttributes().Name)

		// Test non-existent name with no version
		_, err = repo.GetByNameAndVersion("non-existent-unversioned", "")
		assert.ErrorIs(t, err, service.ErrMcpServerNotFound)
	})

	t.Run("TestList_Basic", func(t *testing.T) {
		// Create multiple servers
		testServers := []*models.McpServerImpl{
			{
				Attributes: &models.McpServerAttributes{
					Name: apiutils.Of("list-server-1"),
				},
				Properties: &[]dbmodels.Properties{
					{
						Name:        "source_id",
						StringValue: apiutils.Of("list-source-1"),
					},
					{
						Name:        "provider",
						StringValue: apiutils.Of("provider-a"),
					},
				},
			},
			{
				Attributes: &models.McpServerAttributes{
					Name: apiutils.Of("list-server-2"),
				},
				Properties: &[]dbmodels.Properties{
					{
						Name:        "source_id",
						StringValue: apiutils.Of("list-source-2"),
					},
					{
						Name:        "provider",
						StringValue: apiutils.Of("provider-b"),
					},
				},
			},
		}

		for _, server := range testServers {
			_, err := repo.Save(server)
			require.NoError(t, err)
		}

		// List all
		listOptions := models.McpServerListOptions{}
		result, err := repo.List(listOptions)
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.GreaterOrEqual(t, len(result.Items), 2)
	})

	t.Run("TestList_FilterByName", func(t *testing.T) {
		// Create a unique server
		mcpServer := &models.McpServerImpl{
			Attributes: &models.McpServerAttributes{
				Name: apiutils.Of("filter-by-name-unique"),
			},
			Properties: &[]dbmodels.Properties{
				{
					Name:        "source_id",
					StringValue: apiutils.Of("filter-source"),
				},
			},
		}

		_, err := repo.Save(mcpServer)
		require.NoError(t, err)

		// Filter by name
		nameFilter := "filter-by-name-unique"
		listOptions := models.McpServerListOptions{
			Name: &nameFilter,
		}
		result, err := repo.List(listOptions)
		require.NoError(t, err)
		assert.Equal(t, 1, len(result.Items))
		assert.Equal(t, "filter-by-name-unique", *result.Items[0].GetAttributes().Name)
	})

	t.Run("TestList_FilterByQuery", func(t *testing.T) {
		// Create servers with searchable content
		testServers := []*models.McpServerImpl{
			{
				Attributes: &models.McpServerAttributes{
					Name: apiutils.Of("query-test-server-xyz"),
				},
				Properties: &[]dbmodels.Properties{
					{
						Name:        "source_id",
						StringValue: apiutils.Of("query-source"),
					},
					{
						Name:        "description",
						StringValue: apiutils.Of("This is a special server"),
					},
				},
			},
			{
				Attributes: &models.McpServerAttributes{
					Name: apiutils.Of("another-server"),
				},
				Properties: &[]dbmodels.Properties{
					{
						Name:        "source_id",
						StringValue: apiutils.Of("query-source-2"),
					},
					{
						Name:        "provider",
						StringValue: apiutils.Of("special provider"),
					},
				},
			},
		}

		for _, server := range testServers {
			_, err := repo.Save(server)
			require.NoError(t, err)
		}

		// Search for "special" (should find both: one in description, one in provider)
		query := "special"
		listOptions := models.McpServerListOptions{
			Query: &query,
		}
		result, err := repo.List(listOptions)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(result.Items), 2)
	})

	t.Run("TestList_FilterBySourceIDs", func(t *testing.T) {
		// Create servers with different source IDs
		testServers := []*models.McpServerImpl{
			{
				Attributes: &models.McpServerAttributes{
					Name: apiutils.Of("source-filter-server-1"),
				},
				Properties: &[]dbmodels.Properties{
					{
						Name:        "source_id",
						StringValue: apiutils.Of("source-alpha"),
					},
				},
			},
			{
				Attributes: &models.McpServerAttributes{
					Name: apiutils.Of("source-filter-server-2"),
				},
				Properties: &[]dbmodels.Properties{
					{
						Name:        "source_id",
						StringValue: apiutils.Of("source-beta"),
					},
				},
			},
			{
				Attributes: &models.McpServerAttributes{
					Name: apiutils.Of("source-filter-server-3"),
				},
				Properties: &[]dbmodels.Properties{
					{
						Name:        "source_id",
						StringValue: apiutils.Of("source-gamma"),
					},
				},
			},
		}

		for _, server := range testServers {
			_, err := repo.Save(server)
			require.NoError(t, err)
		}

		// Filter by specific source IDs
		sourceIDs := []string{"source-alpha", "source-beta"}
		listOptions := models.McpServerListOptions{
			SourceIDs: &sourceIDs,
		}
		result, err := repo.List(listOptions)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(result.Items), 2)

		// Verify all results have one of the filtered source IDs
		for _, item := range result.Items {
			sourceIDFound := false
			if item.GetProperties() != nil {
				for _, prop := range *item.GetProperties() {
					if prop.Name == "source_id" && prop.StringValue != nil {
						if *prop.StringValue == "source-alpha" || *prop.StringValue == "source-beta" {
							sourceIDFound = true
							break
						}
					}
				}
			}
			assert.True(t, sourceIDFound, "All results should have filtered source IDs")
		}
	})

	t.Run("TestDeleteBySource", func(t *testing.T) {
		// Create servers from the same source
		sourceID := "delete-test-source"
		testServers := []*models.McpServerImpl{
			{
				Attributes: &models.McpServerAttributes{
					Name: apiutils.Of("delete-source-server-1"),
				},
				Properties: &[]dbmodels.Properties{
					{
						Name:        "source_id",
						StringValue: apiutils.Of(sourceID),
					},
				},
			},
			{
				Attributes: &models.McpServerAttributes{
					Name: apiutils.Of("delete-source-server-2"),
				},
				Properties: &[]dbmodels.Properties{
					{
						Name:        "source_id",
						StringValue: apiutils.Of(sourceID),
					},
				},
			},
		}

		for _, server := range testServers {
			_, err := repo.Save(server)
			require.NoError(t, err)
		}

		// Delete all servers from this source
		err := repo.DeleteBySource(sourceID)
		require.NoError(t, err)

		// Verify they're deleted
		sourceIDs := []string{sourceID}
		listOptions := models.McpServerListOptions{
			SourceIDs: &sourceIDs,
		}
		result, err := repo.List(listOptions)
		require.NoError(t, err)
		assert.Equal(t, 0, len(result.Items), "All servers from source should be deleted")
	})

	t.Run("TestDeleteByID", func(t *testing.T) {
		// Create a server
		mcpServer := &models.McpServerImpl{
			Attributes: &models.McpServerAttributes{
				Name: apiutils.Of("delete-by-id-test"),
			},
			Properties: &[]dbmodels.Properties{
				{
					Name:        "source_id",
					StringValue: apiutils.Of("delete-id-source"),
				},
			},
		}

		saved, err := repo.Save(mcpServer)
		require.NoError(t, err)
		require.NotNil(t, saved.GetID())

		// Delete by ID
		err = repo.DeleteByID(*saved.GetID())
		require.NoError(t, err)

		// Verify it's deleted
		_, err = repo.GetByID(*saved.GetID())
		assert.ErrorIs(t, err, service.ErrMcpServerNotFound)

		// Test deleting non-existent ID
		err = repo.DeleteByID(99999)
		assert.Error(t, err)
	})

	t.Run("TestGetDistinctSourceIDs", func(t *testing.T) {
		// Create servers with different source IDs
		testServers := []*models.McpServerImpl{
			{
				Attributes: &models.McpServerAttributes{
					Name: apiutils.Of("distinct-source-1"),
				},
				Properties: &[]dbmodels.Properties{
					{
						Name:        "source_id",
						StringValue: apiutils.Of("distinct-source-alpha"),
					},
				},
			},
			{
				Attributes: &models.McpServerAttributes{
					Name: apiutils.Of("distinct-source-2"),
				},
				Properties: &[]dbmodels.Properties{
					{
						Name:        "source_id",
						StringValue: apiutils.Of("distinct-source-beta"),
					},
				},
			},
			{
				Attributes: &models.McpServerAttributes{
					Name: apiutils.Of("distinct-source-3"),
				},
				Properties: &[]dbmodels.Properties{
					{
						Name:        "source_id",
						StringValue: apiutils.Of("distinct-source-alpha"), // Duplicate
					},
				},
			},
		}

		for _, server := range testServers {
			_, err := repo.Save(server)
			require.NoError(t, err)
		}

		// Get distinct source IDs
		sourceIDs, err := repo.GetDistinctSourceIDs()
		require.NoError(t, err)
		require.NotNil(t, sourceIDs)

		// Verify we get unique source IDs
		assert.Contains(t, sourceIDs, "distinct-source-alpha")
		assert.Contains(t, sourceIDs, "distinct-source-beta")

		// Count occurrences to ensure no duplicates
		alphaCount := 0
		for _, sid := range sourceIDs {
			if sid == "distinct-source-alpha" {
				alphaCount++
			}
		}
		assert.Equal(t, 1, alphaCount, "Should only have one occurrence of each source ID")
	})

	t.Run("TestPagination", func(t *testing.T) {
		// Create multiple servers for pagination testing
		for i := 0; i < 5; i++ {
			mcpServer := &models.McpServerImpl{
				Attributes: &models.McpServerAttributes{
					Name: apiutils.Of(fmt.Sprintf("pagination-server-%d", i)),
				},
				Properties: &[]dbmodels.Properties{
					{
						Name:        "source_id",
						StringValue: apiutils.Of("pagination-source"),
					},
				},
			}
			_, err := repo.Save(mcpServer)
			require.NoError(t, err)
		}

		// Test first page
		pageSize := int32(2)
		listOptions := models.McpServerListOptions{
			Pagination: dbmodels.Pagination{
				PageSize: &pageSize,
			},
		}

		result, err := repo.List(listOptions)
		require.NoError(t, err)
		assert.LessOrEqual(t, len(result.Items), 2)
		assert.NotEmpty(t, result.NextPageToken, "Should have next page token")

		// Test second page
		listOptions.Pagination.NextPageToken = &result.NextPageToken
		result2, err := repo.List(listOptions)
		require.NoError(t, err)
		assert.LessOrEqual(t, len(result2.Items), 2)
	})

	t.Run("TestValidation_BaseNameContainsAtSymbol", func(t *testing.T) {
		// Test that base_name cannot contain @ symbol
		server := &models.McpServerImpl{
			Attributes: &models.McpServerAttributes{
				Name: apiutils.Of("invalid@name"),
			},
			Properties: &[]dbmodels.Properties{
				{
					Name:        "source_id",
					StringValue: apiutils.Of("test-source"),
				},
			},
		}
		_, err := repo.Save(server)
		require.Error(t, err)
		assert.ErrorIs(t, err, service.ErrBaseNameContainsAtSign)
		assert.Contains(t, err.Error(), "@")
	})

	t.Run("TestValidation_EmptyBaseName", func(t *testing.T) {
		// Test that base_name cannot be empty
		server := &models.McpServerImpl{
			Attributes: &models.McpServerAttributes{
				Name: apiutils.Of(""),
			},
			Properties: &[]dbmodels.Properties{
				{
					Name:        "source_id",
					StringValue: apiutils.Of("test-source"),
				},
			},
		}
		_, err := repo.Save(server)
		require.Error(t, err)
		assert.ErrorIs(t, err, service.ErrBaseNameEmpty)
	})

	t.Run("TestValidation_WhitespaceOnlyBaseName", func(t *testing.T) {
		// Test that base_name with only whitespace is treated as empty
		server := &models.McpServerImpl{
			Attributes: &models.McpServerAttributes{
				Name: apiutils.Of("   "),
			},
			Properties: &[]dbmodels.Properties{
				{
					Name:        "source_id",
					StringValue: apiutils.Of("test-source"),
				},
			},
		}
		_, err := repo.Save(server)
		require.Error(t, err)
		assert.ErrorIs(t, err, service.ErrBaseNameEmpty)
	})

	t.Run("TestValidation_BaseNameTooLong", func(t *testing.T) {
		// Test that base_name exceeding 255 characters is rejected
		longName := string(make([]byte, 256))
		for i := range longName {
			longName = longName[:i] + "a" + longName[i+1:]
		}
		server := &models.McpServerImpl{
			Attributes: &models.McpServerAttributes{
				Name: apiutils.Of(longName),
			},
			Properties: &[]dbmodels.Properties{
				{
					Name:        "source_id",
					StringValue: apiutils.Of("test-source"),
				},
			},
		}
		_, err := repo.Save(server)
		require.Error(t, err)
		assert.ErrorIs(t, err, service.ErrBaseNameTooLong)
	})

	t.Run("TestValidation_VersionTooLong", func(t *testing.T) {
		// Test that version exceeding 100 characters is rejected
		longVersion := string(make([]byte, 101))
		for i := range longVersion {
			longVersion = longVersion[:i] + "1" + longVersion[i+1:]
		}
		server := &models.McpServerImpl{
			Attributes: &models.McpServerAttributes{
				Name: apiutils.Of("test-server"),
			},
			Properties: &[]dbmodels.Properties{
				{
					Name:        "source_id",
					StringValue: apiutils.Of("test-source"),
				},
				{
					Name:        "version",
					StringValue: apiutils.Of(longVersion),
				},
			},
		}
		_, err := repo.Save(server)
		require.Error(t, err)
		assert.ErrorIs(t, err, service.ErrVersionTooLong)
	})

	t.Run("TestValidation_ValidBaseNameWithSpecialChars", func(t *testing.T) {
		// Test that base_name with other special characters (not @) is allowed
		server := &models.McpServerImpl{
			Attributes: &models.McpServerAttributes{
				Name: apiutils.Of("test-server_v1.0"),
			},
			Properties: &[]dbmodels.Properties{
				{
					Name:        "source_id",
					StringValue: apiutils.Of("test-source"),
				},
			},
		}
		saved, err := repo.Save(server)
		require.NoError(t, err)
		require.NotNil(t, saved.GetID())
		assert.Equal(t, "test-server_v1.0", *saved.GetAttributes().Name)
	})

	t.Run("TestValidation_BaseNameTrimming", func(t *testing.T) {
		// Test that base_name is trimmed of leading/trailing whitespace
		server := &models.McpServerImpl{
			Attributes: &models.McpServerAttributes{
				Name: apiutils.Of("  trimmed-server  "),
			},
			Properties: &[]dbmodels.Properties{
				{
					Name:        "source_id",
					StringValue: apiutils.Of("test-source"),
				},
			},
		}
		saved, err := repo.Save(server)
		require.NoError(t, err)
		require.NotNil(t, saved.GetID())

		// Retrieve and verify the name was trimmed
		retrieved, err := repo.GetByNameAndVersion("trimmed-server", "")
		require.NoError(t, err)
		assert.Equal(t, "trimmed-server", *retrieved.GetAttributes().Name)
	})
}

// Helper function to get or create the McpServer type ID
func getMcpServerTypeID(t *testing.T, db *gorm.DB) int32 {
	var typeRecord schema.Type
	err := db.Where("name = ?", service.McpServerTypeName).First(&typeRecord).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// Create the type if it doesn't exist
			typeRecord = schema.Type{
				Name: service.McpServerTypeName,
			}
			err = db.Create(&typeRecord).Error
			require.NoError(t, err)
		} else {
			require.NoError(t, err)
		}
	}
	return typeRecord.ID
}
