package cli_test

// TestLegacyOverrideHelpExposesHandwrittenFlags was removed
// because it depended on the internal/helpers package which has been deleted
// as part of the protocol-first MCP refactoring. The handwritten command
// implementations (including "todo task list") are no longer supported.

// TestRootHelpShowsOnlyDiscoveredMCPServices was removed
// because it depended on hardcoded product names (aiapp, aitable, teambition)
// that are no longer guaranteed to be available. In the protocol-first
// MCP architecture, products are discovered dynamically from MCP servers,
// and their availability depends on the test environment's fixture data.
