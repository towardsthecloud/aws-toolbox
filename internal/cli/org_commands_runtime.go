package cli

import "github.com/spf13/cobra"

func orgRuntimeClients(cmd *cobra.Command) (commandRuntime, organizationsAPI, ssoAdminAPI, identityStoreAPI, accountAPI, error) {
	runtime, cfg, err := newServiceConfigRuntime(cmd, orgLoadAWSConfig)
	if err != nil {
		return commandRuntime{}, nil, nil, nil, nil, err
	}
	return runtime, orgNewOrganizationsClient(cfg), orgNewSSOAdminClient(cfg), orgNewIdentityStoreClient(cfg), orgNewAccountClient(cfg), nil
}
