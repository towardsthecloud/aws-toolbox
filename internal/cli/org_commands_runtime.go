package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func orgRuntimeClients(cmd *cobra.Command) (commandRuntime, organizationsAPI, ssoAdminAPI, identityStoreAPI, accountAPI, error) {
	runtime, err := newCommandRuntime(cmd)
	if err != nil {
		return commandRuntime{}, nil, nil, nil, nil, err
	}
	cfg, err := orgLoadAWSConfig(runtime.Options.Profile, runtime.Options.Region)
	if err != nil {
		return commandRuntime{}, nil, nil, nil, nil, fmt.Errorf("load AWS config: %w", err)
	}
	return runtime, orgNewOrganizationsClient(cfg), orgNewSSOAdminClient(cfg), orgNewIdentityStoreClient(cfg), orgNewAccountClient(cfg), nil
}
