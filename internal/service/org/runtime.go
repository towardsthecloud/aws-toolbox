package org

import (
	"github.com/spf13/cobra"
	"github.com/towardsthecloud/aws-toolbox/internal/cliutil"
)

func runtimeClients(cmd *cobra.Command) (cliutil.CommandRuntime, OrganizationsAPI, SSOAdminAPI, IdentityStoreAPI, AccountAPI, error) {
	runtime, cfg, err := cliutil.NewServiceConfigRuntime(cmd, loadAWSConfig)
	if err != nil {
		return cliutil.CommandRuntime{}, nil, nil, nil, nil, err
	}
	return runtime, newOrganizationsClient(cfg), newSSOAdminClient(cfg), newIdentityStoreClient(cfg), newAccountClient(cfg), nil
}
