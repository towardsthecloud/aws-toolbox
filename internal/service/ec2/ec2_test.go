package ec2

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/smithy-go"
	"github.com/towardsthecloud/aws-toolbox/internal/cliutil"
)

type mockClient struct {
	describeAddressesFn         func(context.Context, *ec2.DescribeAddressesInput, ...func(*ec2.Options)) (*ec2.DescribeAddressesOutput, error)
	describeImagesFn            func(context.Context, *ec2.DescribeImagesInput, ...func(*ec2.Options)) (*ec2.DescribeImagesOutput, error)
	describeInstancesFn         func(context.Context, *ec2.DescribeInstancesInput, ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error)
	describeKeyPairsFn          func(context.Context, *ec2.DescribeKeyPairsInput, ...func(*ec2.Options)) (*ec2.DescribeKeyPairsOutput, error)
	describeNetworkInterfacesFn func(context.Context, *ec2.DescribeNetworkInterfacesInput, ...func(*ec2.Options)) (*ec2.DescribeNetworkInterfacesOutput, error)
	describeRegionsFn           func(context.Context, *ec2.DescribeRegionsInput, ...func(*ec2.Options)) (*ec2.DescribeRegionsOutput, error)
	describeSecurityGroupsFn    func(context.Context, *ec2.DescribeSecurityGroupsInput, ...func(*ec2.Options)) (*ec2.DescribeSecurityGroupsOutput, error)
	describeSnapshotsFn         func(context.Context, *ec2.DescribeSnapshotsInput, ...func(*ec2.Options)) (*ec2.DescribeSnapshotsOutput, error)
	describeVolumesFn           func(context.Context, *ec2.DescribeVolumesInput, ...func(*ec2.Options)) (*ec2.DescribeVolumesOutput, error)
	deleteKeyPairFn             func(context.Context, *ec2.DeleteKeyPairInput, ...func(*ec2.Options)) (*ec2.DeleteKeyPairOutput, error)
	deleteSecurityGroupFn       func(context.Context, *ec2.DeleteSecurityGroupInput, ...func(*ec2.Options)) (*ec2.DeleteSecurityGroupOutput, error)
	deleteSnapshotFn            func(context.Context, *ec2.DeleteSnapshotInput, ...func(*ec2.Options)) (*ec2.DeleteSnapshotOutput, error)
	deleteVolumeFn              func(context.Context, *ec2.DeleteVolumeInput, ...func(*ec2.Options)) (*ec2.DeleteVolumeOutput, error)
	deregisterImageFn           func(context.Context, *ec2.DeregisterImageInput, ...func(*ec2.Options)) (*ec2.DeregisterImageOutput, error)
	releaseAddressFn            func(context.Context, *ec2.ReleaseAddressInput, ...func(*ec2.Options)) (*ec2.ReleaseAddressOutput, error)
	revokeSecurityIngressFn     func(context.Context, *ec2.RevokeSecurityGroupIngressInput, ...func(*ec2.Options)) (*ec2.RevokeSecurityGroupIngressOutput, error)
}

func (m *mockClient) DescribeAddresses(ctx context.Context, in *ec2.DescribeAddressesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeAddressesOutput, error) {
	if m.describeAddressesFn == nil {
		return nil, errors.New("DescribeAddresses not mocked")
	}
	return m.describeAddressesFn(ctx, in, optFns...)
}

func (m *mockClient) DescribeImages(ctx context.Context, in *ec2.DescribeImagesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeImagesOutput, error) {
	if m.describeImagesFn == nil {
		return nil, errors.New("DescribeImages not mocked")
	}
	return m.describeImagesFn(ctx, in, optFns...)
}

func (m *mockClient) DescribeInstances(ctx context.Context, in *ec2.DescribeInstancesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
	if m.describeInstancesFn == nil {
		return nil, errors.New("DescribeInstances not mocked")
	}
	return m.describeInstancesFn(ctx, in, optFns...)
}

func (m *mockClient) DescribeKeyPairs(ctx context.Context, in *ec2.DescribeKeyPairsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeKeyPairsOutput, error) {
	if m.describeKeyPairsFn == nil {
		return nil, errors.New("DescribeKeyPairs not mocked")
	}
	return m.describeKeyPairsFn(ctx, in, optFns...)
}

func (m *mockClient) DescribeNetworkInterfaces(ctx context.Context, in *ec2.DescribeNetworkInterfacesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeNetworkInterfacesOutput, error) {
	if m.describeNetworkInterfacesFn == nil {
		return nil, errors.New("DescribeNetworkInterfaces not mocked")
	}
	return m.describeNetworkInterfacesFn(ctx, in, optFns...)
}

func (m *mockClient) DescribeRegions(ctx context.Context, in *ec2.DescribeRegionsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeRegionsOutput, error) {
	if m.describeRegionsFn == nil {
		return nil, errors.New("DescribeRegions not mocked")
	}
	return m.describeRegionsFn(ctx, in, optFns...)
}

func (m *mockClient) DescribeSecurityGroups(ctx context.Context, in *ec2.DescribeSecurityGroupsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeSecurityGroupsOutput, error) {
	if m.describeSecurityGroupsFn == nil {
		return nil, errors.New("DescribeSecurityGroups not mocked")
	}
	return m.describeSecurityGroupsFn(ctx, in, optFns...)
}

func (m *mockClient) DescribeSnapshots(ctx context.Context, in *ec2.DescribeSnapshotsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeSnapshotsOutput, error) {
	if m.describeSnapshotsFn == nil {
		return nil, errors.New("DescribeSnapshots not mocked")
	}
	return m.describeSnapshotsFn(ctx, in, optFns...)
}

func (m *mockClient) DescribeVolumes(ctx context.Context, in *ec2.DescribeVolumesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeVolumesOutput, error) {
	if m.describeVolumesFn == nil {
		return nil, errors.New("DescribeVolumes not mocked")
	}
	return m.describeVolumesFn(ctx, in, optFns...)
}

func (m *mockClient) DeleteKeyPair(ctx context.Context, in *ec2.DeleteKeyPairInput, optFns ...func(*ec2.Options)) (*ec2.DeleteKeyPairOutput, error) {
	if m.deleteKeyPairFn == nil {
		return nil, errors.New("DeleteKeyPair not mocked")
	}
	return m.deleteKeyPairFn(ctx, in, optFns...)
}

func (m *mockClient) DeleteSecurityGroup(ctx context.Context, in *ec2.DeleteSecurityGroupInput, optFns ...func(*ec2.Options)) (*ec2.DeleteSecurityGroupOutput, error) {
	if m.deleteSecurityGroupFn == nil {
		return nil, errors.New("DeleteSecurityGroup not mocked")
	}
	return m.deleteSecurityGroupFn(ctx, in, optFns...)
}

func (m *mockClient) DeleteSnapshot(ctx context.Context, in *ec2.DeleteSnapshotInput, optFns ...func(*ec2.Options)) (*ec2.DeleteSnapshotOutput, error) {
	if m.deleteSnapshotFn == nil {
		return nil, errors.New("DeleteSnapshot not mocked")
	}
	return m.deleteSnapshotFn(ctx, in, optFns...)
}

func (m *mockClient) DeleteVolume(ctx context.Context, in *ec2.DeleteVolumeInput, optFns ...func(*ec2.Options)) (*ec2.DeleteVolumeOutput, error) {
	if m.deleteVolumeFn == nil {
		return nil, errors.New("DeleteVolume not mocked")
	}
	return m.deleteVolumeFn(ctx, in, optFns...)
}

func (m *mockClient) DeregisterImage(ctx context.Context, in *ec2.DeregisterImageInput, optFns ...func(*ec2.Options)) (*ec2.DeregisterImageOutput, error) {
	if m.deregisterImageFn == nil {
		return nil, errors.New("DeregisterImage not mocked")
	}
	return m.deregisterImageFn(ctx, in, optFns...)
}

func (m *mockClient) ReleaseAddress(ctx context.Context, in *ec2.ReleaseAddressInput, optFns ...func(*ec2.Options)) (*ec2.ReleaseAddressOutput, error) {
	if m.releaseAddressFn == nil {
		return nil, errors.New("ReleaseAddress not mocked")
	}
	return m.releaseAddressFn(ctx, in, optFns...)
}

func (m *mockClient) RevokeSecurityGroupIngress(ctx context.Context, in *ec2.RevokeSecurityGroupIngressInput, optFns ...func(*ec2.Options)) (*ec2.RevokeSecurityGroupIngressOutput, error) {
	if m.revokeSecurityIngressFn == nil {
		return nil, errors.New("RevokeSecurityGroupIngress not mocked")
	}
	return m.revokeSecurityIngressFn(ctx, in, optFns...)
}

func withMockDeps(t *testing.T, loader func(string, string) (awssdk.Config, error), nc func(awssdk.Config) API, newRegional func(awssdk.Config, string) API) {
	t.Helper()

	oldLoader := loadAWSConfig
	oldNewClient := newClient
	oldNewRegional := newRegionalClient

	loadAWSConfig = loader
	newClient = nc
	newRegionalClient = newRegional

	t.Cleanup(func() {
		loadAWSConfig = oldLoader
		newClient = oldNewClient
		newRegionalClient = oldNewRegional
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

func TestEC2DeleteAMIsDryRunFiltersUnusedAndRetention(t *testing.T) {
	oldDate := time.Now().UTC().AddDate(0, 0, -45).Format(time.RFC3339)
	newDate := time.Now().UTC().AddDate(0, 0, -5).Format(time.RFC3339)

	client := &mockClient{
		describeImagesFn: func(_ context.Context, _ *ec2.DescribeImagesInput, _ ...func(*ec2.Options)) (*ec2.DescribeImagesOutput, error) {
			return &ec2.DescribeImagesOutput{Images: []ec2types.Image{
				{ImageId: cliutil.Ptr("ami-old-unused"), Name: cliutil.Ptr("old-unused"), CreationDate: cliutil.Ptr(oldDate)},
				{ImageId: cliutil.Ptr("ami-old-used"), Name: cliutil.Ptr("old-used"), CreationDate: cliutil.Ptr(oldDate)},
				{ImageId: cliutil.Ptr("ami-new-unused"), Name: cliutil.Ptr("new-unused"), CreationDate: cliutil.Ptr(newDate)},
			}}, nil
		},
		describeInstancesFn: func(_ context.Context, _ *ec2.DescribeInstancesInput, _ ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
			return &ec2.DescribeInstancesOutput{Reservations: []ec2types.Reservation{{Instances: []ec2types.Instance{{ImageId: cliutil.Ptr("ami-old-used")}}}}}, nil
		},
	}

	withMockDeps(
		t,
		func(_, _ string) (awssdk.Config, error) { return awssdk.Config{Region: "us-east-1"}, nil },
		func(awssdk.Config) API { return client },
		func(awssdk.Config, string) API { return client },
	)

	output, err := executeCommand(t, "--output", "json", "--dry-run", "ec2", "delete-amis", "--unused", "--retention-days", "30")
	if err != nil {
		t.Fatalf("execute delete-amis: %v", err)
	}

	if !strings.Contains(output, "ami-old-unused") || strings.Contains(output, "ami-old-used") || strings.Contains(output, "ami-new-unused") {
		t.Fatalf("unexpected output: %s", output)
	}
	if !strings.Contains(output, "would-delete") {
		t.Fatalf("expected dry-run action in output: %s", output)
	}
}

func TestEC2DeleteAMIsExecutesWhenNoConfirm(t *testing.T) {
	deleted := 0
	client := &mockClient{
		describeImagesFn: func(_ context.Context, _ *ec2.DescribeImagesInput, _ ...func(*ec2.Options)) (*ec2.DescribeImagesOutput, error) {
			return &ec2.DescribeImagesOutput{Images: []ec2types.Image{
				{ImageId: cliutil.Ptr("ami-old"), Name: cliutil.Ptr("old"), CreationDate: cliutil.Ptr(time.Now().UTC().AddDate(0, 0, -40).Format(time.RFC3339))},
			}}, nil
		},
		deregisterImageFn: func(_ context.Context, _ *ec2.DeregisterImageInput, _ ...func(*ec2.Options)) (*ec2.DeregisterImageOutput, error) {
			deleted++
			return &ec2.DeregisterImageOutput{}, nil
		},
	}

	withMockDeps(
		t,
		func(_, _ string) (awssdk.Config, error) { return awssdk.Config{Region: "us-east-1"}, nil },
		func(awssdk.Config) API { return client },
		func(awssdk.Config, string) API { return client },
	)

	output, err := executeCommand(t, "--output", "json", "--no-confirm", "ec2", "delete-amis", "--retention-days", "30")
	if err != nil {
		t.Fatalf("execute delete-amis: %v", err)
	}
	if deleted != 1 || !strings.Contains(output, "deleted") {
		t.Fatalf("unexpected output: %s", output)
	}
}

func TestEC2DeleteEIPsDryRun(t *testing.T) {
	client := &mockClient{
		describeAddressesFn: func(_ context.Context, _ *ec2.DescribeAddressesInput, _ ...func(*ec2.Options)) (*ec2.DescribeAddressesOutput, error) {
			return &ec2.DescribeAddressesOutput{Addresses: []ec2types.Address{
				{AllocationId: cliutil.Ptr("eipalloc-1"), PublicIp: cliutil.Ptr("1.1.1.1")},
				{AllocationId: cliutil.Ptr("eipalloc-2"), PublicIp: cliutil.Ptr("2.2.2.2"), AssociationId: cliutil.Ptr("eipassoc-1")},
			}}, nil
		},
	}

	withMockDeps(
		t,
		func(_, _ string) (awssdk.Config, error) { return awssdk.Config{Region: "us-east-1"}, nil },
		func(awssdk.Config) API { return client },
		func(awssdk.Config, string) API { return client },
	)

	output, err := executeCommand(t, "--output", "text", "--dry-run", "ec2", "delete-eips")
	if err != nil {
		t.Fatalf("execute delete-eips: %v", err)
	}

	if !strings.Contains(output, "eipalloc-1") || strings.Contains(output, "eipalloc-2") {
		t.Fatalf("unexpected output: %s", output)
	}
}

func TestEC2DeleteEIPsExecutesWhenNoConfirm(t *testing.T) {
	released := 0
	client := &mockClient{
		describeAddressesFn: func(_ context.Context, _ *ec2.DescribeAddressesInput, _ ...func(*ec2.Options)) (*ec2.DescribeAddressesOutput, error) {
			return &ec2.DescribeAddressesOutput{Addresses: []ec2types.Address{{AllocationId: cliutil.Ptr("eipalloc-1"), PublicIp: cliutil.Ptr("1.1.1.1")}}}, nil
		},
		releaseAddressFn: func(_ context.Context, _ *ec2.ReleaseAddressInput, _ ...func(*ec2.Options)) (*ec2.ReleaseAddressOutput, error) {
			released++
			return &ec2.ReleaseAddressOutput{}, nil
		},
	}

	withMockDeps(
		t,
		func(_, _ string) (awssdk.Config, error) { return awssdk.Config{Region: "us-east-1"}, nil },
		func(awssdk.Config) API { return client },
		func(awssdk.Config, string) API { return client },
	)

	output, err := executeCommand(t, "--output", "json", "--no-confirm", "ec2", "delete-eips")
	if err != nil {
		t.Fatalf("execute delete-eips: %v", err)
	}
	if released != 1 || !strings.Contains(output, "deleted") {
		t.Fatalf("unexpected output: %s", output)
	}
}

func TestEC2DeleteEIPsStatusMatchesSortedAllocationIDRows(t *testing.T) {
	client := &mockClient{
		describeAddressesFn: func(_ context.Context, _ *ec2.DescribeAddressesInput, _ ...func(*ec2.Options)) (*ec2.DescribeAddressesOutput, error) {
			return &ec2.DescribeAddressesOutput{Addresses: []ec2types.Address{
				{AllocationId: cliutil.Ptr("eipalloc-z"), PublicIp: cliutil.Ptr("2.2.2.2")},
				{AllocationId: cliutil.Ptr("eipalloc-a"), PublicIp: cliutil.Ptr("1.1.1.1")},
			}}, nil
		},
		releaseAddressFn: func(_ context.Context, in *ec2.ReleaseAddressInput, _ ...func(*ec2.Options)) (*ec2.ReleaseAddressOutput, error) {
			if cliutil.PointerToString(in.AllocationId) == "eipalloc-a" {
				return nil, errors.New("release blocked")
			}
			return &ec2.ReleaseAddressOutput{}, nil
		},
	}

	withMockDeps(
		t,
		func(_, _ string) (awssdk.Config, error) { return awssdk.Config{Region: "us-east-1"}, nil },
		func(awssdk.Config) API { return client },
		func(awssdk.Config, string) API { return client },
	)

	output, err := executeCommand(t, "--output", "text", "--no-confirm", "ec2", "delete-eips")
	if err != nil {
		t.Fatalf("execute delete-eips: %v", err)
	}

	if !strings.Contains(output, "allocation_id=eipalloc-a public_ip=1.1.1.1 region=us-east-1 action=failed:release blocked") {
		t.Fatalf("expected failed action to remain mapped to eipalloc-a: %s", output)
	}
	if !strings.Contains(output, "allocation_id=eipalloc-z public_ip=2.2.2.2 region=us-east-1 action=deleted") {
		t.Fatalf("expected deleted action to remain mapped to eipalloc-z: %s", output)
	}
}

func TestEC2DeleteKeypairsAllRegionsDryRun(t *testing.T) {
	clientByRegion := map[string]*mockClient{
		"us-east-1": {
			describeKeyPairsFn: func(_ context.Context, _ *ec2.DescribeKeyPairsInput, _ ...func(*ec2.Options)) (*ec2.DescribeKeyPairsOutput, error) {
				return &ec2.DescribeKeyPairsOutput{KeyPairs: []ec2types.KeyPairInfo{{KeyName: cliutil.Ptr("unused-east")}, {KeyName: cliutil.Ptr("used-east")}}}, nil
			},
			describeInstancesFn: func(_ context.Context, _ *ec2.DescribeInstancesInput, _ ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
				return &ec2.DescribeInstancesOutput{Reservations: []ec2types.Reservation{{Instances: []ec2types.Instance{{KeyName: cliutil.Ptr("used-east")}}}}}, nil
			},
		},
		"eu-west-1": {
			describeKeyPairsFn: func(_ context.Context, _ *ec2.DescribeKeyPairsInput, _ ...func(*ec2.Options)) (*ec2.DescribeKeyPairsOutput, error) {
				return &ec2.DescribeKeyPairsOutput{KeyPairs: []ec2types.KeyPairInfo{{KeyName: cliutil.Ptr("unused-eu")}}}, nil
			},
			describeInstancesFn: func(_ context.Context, _ *ec2.DescribeInstancesInput, _ ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
				return &ec2.DescribeInstancesOutput{}, nil
			},
		},
	}

	baseClient := &mockClient{
		describeRegionsFn: func(_ context.Context, _ *ec2.DescribeRegionsInput, _ ...func(*ec2.Options)) (*ec2.DescribeRegionsOutput, error) {
			return &ec2.DescribeRegionsOutput{Regions: []ec2types.Region{{RegionName: cliutil.Ptr("eu-west-1")}, {RegionName: cliutil.Ptr("us-east-1")}}}, nil
		},
	}

	withMockDeps(
		t,
		func(_, _ string) (awssdk.Config, error) { return awssdk.Config{Region: "us-east-1"}, nil },
		func(awssdk.Config) API { return baseClient },
		func(_ awssdk.Config, region string) API { return clientByRegion[region] },
	)

	output, err := executeCommand(t, "--output", "json", "--dry-run", "ec2", "delete-keypairs", "--all-regions")
	if err != nil {
		t.Fatalf("execute delete-keypairs: %v", err)
	}

	if !strings.Contains(output, "unused-east") || !strings.Contains(output, "unused-eu") || strings.Contains(output, "\"key_name\": \"used-east\"") {
		t.Fatalf("unexpected output: %s", output)
	}
}

func TestEC2DeleteKeypairsExecutesWhenNoConfirm(t *testing.T) {
	deleted := 0
	client := &mockClient{
		describeKeyPairsFn: func(_ context.Context, _ *ec2.DescribeKeyPairsInput, _ ...func(*ec2.Options)) (*ec2.DescribeKeyPairsOutput, error) {
			return &ec2.DescribeKeyPairsOutput{KeyPairs: []ec2types.KeyPairInfo{{KeyName: cliutil.Ptr("unused")}}}, nil
		},
		describeInstancesFn: func(_ context.Context, _ *ec2.DescribeInstancesInput, _ ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
			return &ec2.DescribeInstancesOutput{}, nil
		},
		deleteKeyPairFn: func(_ context.Context, _ *ec2.DeleteKeyPairInput, _ ...func(*ec2.Options)) (*ec2.DeleteKeyPairOutput, error) {
			deleted++
			return &ec2.DeleteKeyPairOutput{}, nil
		},
	}

	withMockDeps(
		t,
		func(_, _ string) (awssdk.Config, error) { return awssdk.Config{Region: "us-east-1"}, nil },
		func(awssdk.Config) API { return client },
		func(awssdk.Config, string) API { return client },
	)

	output, err := executeCommand(t, "--output", "json", "--no-confirm", "ec2", "delete-keypairs")
	if err != nil {
		t.Fatalf("execute delete-keypairs: %v", err)
	}
	if deleted != 1 || !strings.Contains(output, "deleted") {
		t.Fatalf("unexpected output: %s", output)
	}
}

func TestEC2DeleteKeypairsCancelledPrompt(t *testing.T) {
	client := &mockClient{
		describeKeyPairsFn: func(_ context.Context, _ *ec2.DescribeKeyPairsInput, _ ...func(*ec2.Options)) (*ec2.DescribeKeyPairsOutput, error) {
			return &ec2.DescribeKeyPairsOutput{KeyPairs: []ec2types.KeyPairInfo{{KeyName: cliutil.Ptr("unused")}}}, nil
		},
		describeInstancesFn: func(_ context.Context, _ *ec2.DescribeInstancesInput, _ ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
			return &ec2.DescribeInstancesOutput{}, nil
		},
	}

	withMockDeps(
		t,
		func(_, _ string) (awssdk.Config, error) { return awssdk.Config{Region: "us-east-1"}, nil },
		func(awssdk.Config) API { return client },
		func(awssdk.Config, string) API { return client },
	)

	output, err := executeCommandWithInput(t, "n\n", "--output", "json", "ec2", "delete-keypairs")
	if err != nil {
		t.Fatalf("execute delete-keypairs with prompt: %v", err)
	}
	if !strings.Contains(output, "cancelled") {
		t.Fatalf("expected cancelled action: %s", output)
	}
}

func TestEC2DeleteKeypairsErrorsWhenRegionMissing(t *testing.T) {
	client := &mockClient{}

	withMockDeps(
		t,
		func(_, _ string) (awssdk.Config, error) { return awssdk.Config{}, nil },
		func(awssdk.Config) API { return client },
		func(awssdk.Config, string) API { return client },
	)

	if _, err := executeCommand(t, "ec2", "delete-keypairs"); err == nil || !strings.Contains(err.Error(), "set --region") {
		t.Fatalf("expected missing region error, got: %v", err)
	}
}

func TestEC2DeleteSecurityGroupsSSHRulesDryRun(t *testing.T) {
	client := &mockClient{
		describeSecurityGroupsFn: func(_ context.Context, _ *ec2.DescribeSecurityGroupsInput, _ ...func(*ec2.Options)) (*ec2.DescribeSecurityGroupsOutput, error) {
			return &ec2.DescribeSecurityGroupsOutput{SecurityGroups: []ec2types.SecurityGroup{
				{
					GroupId:   cliutil.Ptr("sg-ssh"),
					GroupName: cliutil.Ptr("app-sg"),
					IpPermissions: []ec2types.IpPermission{{
						IpProtocol: cliutil.Ptr("tcp"),
						FromPort:   cliutil.Ptr(int32(22)),
						ToPort:     cliutil.Ptr(int32(22)),
					}},
				},
				{GroupId: cliutil.Ptr("sg-no-ssh"), GroupName: cliutil.Ptr("db-sg")},
			}}, nil
		},
		describeNetworkInterfacesFn: func(_ context.Context, _ *ec2.DescribeNetworkInterfacesInput, _ ...func(*ec2.Options)) (*ec2.DescribeNetworkInterfacesOutput, error) {
			return &ec2.DescribeNetworkInterfacesOutput{}, nil
		},
	}

	withMockDeps(
		t,
		func(_, _ string) (awssdk.Config, error) { return awssdk.Config{Region: "us-east-1"}, nil },
		func(awssdk.Config) API { return client },
		func(awssdk.Config, string) API { return client },
	)

	output, err := executeCommand(t, "--output", "json", "--dry-run", "ec2", "delete-security-groups", "--ssh-rules", "--unused")
	if err != nil {
		t.Fatalf("execute delete-security-groups: %v", err)
	}

	if !strings.Contains(output, "sg-ssh") || strings.Contains(output, "sg-no-ssh") || !strings.Contains(output, "would-delete") {
		t.Fatalf("unexpected output: %s", output)
	}
}

func TestEC2DeleteSecurityGroupsDeleteWithTagAndType(t *testing.T) {
	deleted := 0
	client := &mockClient{
		describeSecurityGroupsFn: func(_ context.Context, _ *ec2.DescribeSecurityGroupsInput, _ ...func(*ec2.Options)) (*ec2.DescribeSecurityGroupsOutput, error) {
			return &ec2.DescribeSecurityGroupsOutput{SecurityGroups: []ec2types.SecurityGroup{
				{GroupId: cliutil.Ptr("sg-rds"), GroupName: cliutil.Ptr("rds-main"), Tags: []ec2types.Tag{{Key: cliutil.Ptr("env"), Value: cliutil.Ptr("prod")}}},
				{GroupId: cliutil.Ptr("sg-ec2"), GroupName: cliutil.Ptr("app-main"), Tags: []ec2types.Tag{{Key: cliutil.Ptr("env"), Value: cliutil.Ptr("prod")}}},
			}}, nil
		},
		describeNetworkInterfacesFn: func(_ context.Context, _ *ec2.DescribeNetworkInterfacesInput, _ ...func(*ec2.Options)) (*ec2.DescribeNetworkInterfacesOutput, error) {
			return &ec2.DescribeNetworkInterfacesOutput{}, nil
		},
		deleteSecurityGroupFn: func(_ context.Context, _ *ec2.DeleteSecurityGroupInput, _ ...func(*ec2.Options)) (*ec2.DeleteSecurityGroupOutput, error) {
			deleted++
			return &ec2.DeleteSecurityGroupOutput{}, nil
		},
	}

	withMockDeps(
		t,
		func(_, _ string) (awssdk.Config, error) { return awssdk.Config{Region: "us-east-1"}, nil },
		func(awssdk.Config) API { return client },
		func(awssdk.Config, string) API { return client },
	)

	output, err := executeCommand(t, "--output", "json", "--no-confirm", "ec2", "delete-security-groups", "--unused", "--filter-tag", "env=prod", "--type", "rds")
	if err != nil {
		t.Fatalf("execute delete-security-groups: %v", err)
	}
	if deleted != 1 || !strings.Contains(output, "sg-rds") || strings.Contains(output, "sg-ec2") {
		t.Fatalf("unexpected output: %s", output)
	}
}

func TestEC2DeleteSnapshotsDryRun(t *testing.T) {
	oldStart := time.Now().UTC().AddDate(0, 0, -60)

	client := &mockClient{
		describeSnapshotsFn: func(_ context.Context, _ *ec2.DescribeSnapshotsInput, _ ...func(*ec2.Options)) (*ec2.DescribeSnapshotsOutput, error) {
			return &ec2.DescribeSnapshotsOutput{Snapshots: []ec2types.Snapshot{
				{SnapshotId: cliutil.Ptr("snap-orphan"), VolumeId: cliutil.Ptr("vol-missing"), StartTime: &oldStart},
				{SnapshotId: cliutil.Ptr("snap-in-use")},
			}}, nil
		},
		describeImagesFn: func(_ context.Context, _ *ec2.DescribeImagesInput, _ ...func(*ec2.Options)) (*ec2.DescribeImagesOutput, error) {
			return &ec2.DescribeImagesOutput{Images: []ec2types.Image{{
				BlockDeviceMappings: []ec2types.BlockDeviceMapping{{Ebs: &ec2types.EbsBlockDevice{SnapshotId: cliutil.Ptr("snap-in-use")}}},
			}}}, nil
		},
		describeVolumesFn: func(_ context.Context, in *ec2.DescribeVolumesInput, _ ...func(*ec2.Options)) (*ec2.DescribeVolumesOutput, error) {
			if len(in.VolumeIds) == 1 && in.VolumeIds[0] == "vol-missing" {
				return nil, &smithy.GenericAPIError{Code: "InvalidVolume.NotFound", Message: "not found"}
			}
			return &ec2.DescribeVolumesOutput{}, nil
		},
	}

	withMockDeps(
		t,
		func(_, _ string) (awssdk.Config, error) { return awssdk.Config{Region: "us-east-1"}, nil },
		func(awssdk.Config) API { return client },
		func(awssdk.Config, string) API { return client },
	)

	output, err := executeCommand(t, "--output", "json", "--dry-run", "ec2", "delete-snapshots", "--retention-days", "30")
	if err != nil {
		t.Fatalf("execute delete-snapshots: %v", err)
	}

	if !strings.Contains(output, "snap-orphan") || strings.Contains(output, "snap-in-use") {
		t.Fatalf("unexpected output: %s", output)
	}
}

func TestEC2DeleteSnapshotsExecutesWhenNoConfirm(t *testing.T) {
	deleted := 0
	start := time.Now().UTC().AddDate(0, 0, -60)
	client := &mockClient{
		describeSnapshotsFn: func(_ context.Context, _ *ec2.DescribeSnapshotsInput, _ ...func(*ec2.Options)) (*ec2.DescribeSnapshotsOutput, error) {
			return &ec2.DescribeSnapshotsOutput{Snapshots: []ec2types.Snapshot{{SnapshotId: cliutil.Ptr("snap-1"), VolumeId: cliutil.Ptr("vol-x"), StartTime: &start}}}, nil
		},
		describeImagesFn: func(_ context.Context, _ *ec2.DescribeImagesInput, _ ...func(*ec2.Options)) (*ec2.DescribeImagesOutput, error) {
			return &ec2.DescribeImagesOutput{}, nil
		},
		describeVolumesFn: func(_ context.Context, _ *ec2.DescribeVolumesInput, _ ...func(*ec2.Options)) (*ec2.DescribeVolumesOutput, error) {
			return nil, &smithy.GenericAPIError{Code: "InvalidVolume.NotFound", Message: "missing"}
		},
		deleteSnapshotFn: func(_ context.Context, _ *ec2.DeleteSnapshotInput, _ ...func(*ec2.Options)) (*ec2.DeleteSnapshotOutput, error) {
			deleted++
			return &ec2.DeleteSnapshotOutput{}, nil
		},
	}

	withMockDeps(
		t,
		func(_, _ string) (awssdk.Config, error) { return awssdk.Config{Region: "us-east-1"}, nil },
		func(awssdk.Config) API { return client },
		func(awssdk.Config, string) API { return client },
	)

	output, err := executeCommand(t, "--output", "json", "--no-confirm", "ec2", "delete-snapshots")
	if err != nil {
		t.Fatalf("execute delete-snapshots: %v", err)
	}
	if deleted != 1 || !strings.Contains(output, "deleted") {
		t.Fatalf("unexpected output: %s", output)
	}
}

func TestEC2DeleteVolumesDryRun(t *testing.T) {
	client := &mockClient{
		describeVolumesFn: func(_ context.Context, _ *ec2.DescribeVolumesInput, _ ...func(*ec2.Options)) (*ec2.DescribeVolumesOutput, error) {
			return &ec2.DescribeVolumesOutput{Volumes: []ec2types.Volume{{VolumeId: cliutil.Ptr("vol-1"), Size: cliutil.Ptr(int32(20))}}}, nil
		},
	}

	withMockDeps(
		t,
		func(_, _ string) (awssdk.Config, error) { return awssdk.Config{Region: "us-east-1"}, nil },
		func(awssdk.Config) API { return client },
		func(awssdk.Config, string) API { return client },
	)

	output, err := executeCommand(t, "--output", "json", "--dry-run", "ec2", "delete-volumes")
	if err != nil {
		t.Fatalf("execute delete-volumes: %v", err)
	}

	if !strings.Contains(output, "vol-1") || !strings.Contains(output, "would-delete") {
		t.Fatalf("unexpected output: %s", output)
	}
}

func TestEC2DeleteVolumesExecutesWhenNoConfirm(t *testing.T) {
	deleted := 0
	client := &mockClient{
		describeVolumesFn: func(_ context.Context, _ *ec2.DescribeVolumesInput, _ ...func(*ec2.Options)) (*ec2.DescribeVolumesOutput, error) {
			return &ec2.DescribeVolumesOutput{Volumes: []ec2types.Volume{{VolumeId: cliutil.Ptr("vol-1"), Size: cliutil.Ptr(int32(20))}}}, nil
		},
		deleteVolumeFn: func(_ context.Context, _ *ec2.DeleteVolumeInput, _ ...func(*ec2.Options)) (*ec2.DeleteVolumeOutput, error) {
			deleted++
			return &ec2.DeleteVolumeOutput{}, nil
		},
	}

	withMockDeps(
		t,
		func(_, _ string) (awssdk.Config, error) { return awssdk.Config{Region: "us-east-1"}, nil },
		func(awssdk.Config) API { return client },
		func(awssdk.Config, string) API { return client },
	)

	output, err := executeCommand(t, "--output", "json", "--no-confirm", "ec2", "delete-volumes")
	if err != nil {
		t.Fatalf("execute delete-volumes: %v", err)
	}
	if deleted != 1 || !strings.Contains(output, "deleted") {
		t.Fatalf("unexpected output: %s", output)
	}
}

func TestEC2ListEIPsAllOutputFormats(t *testing.T) {
	client := &mockClient{
		describeAddressesFn: func(_ context.Context, _ *ec2.DescribeAddressesInput, _ ...func(*ec2.Options)) (*ec2.DescribeAddressesOutput, error) {
			return &ec2.DescribeAddressesOutput{Addresses: []ec2types.Address{{AllocationId: cliutil.Ptr("eipalloc-xyz"), PublicIp: cliutil.Ptr("3.3.3.3")}}}, nil
		},
	}

	withMockDeps(
		t,
		func(_, _ string) (awssdk.Config, error) { return awssdk.Config{Region: "us-east-1"}, nil },
		func(awssdk.Config) API { return client },
		func(awssdk.Config, string) API { return client },
	)

	for _, format := range []string{"table", "json", "text"} {
		output, err := executeCommand(t, "--output", format, "ec2", "list-eips")
		if err != nil {
			t.Fatalf("execute list-eips (%s): %v", format, err)
		}
		if !strings.Contains(output, "eipalloc-xyz") {
			t.Fatalf("expected EIP in output for format=%s: %s", format, output)
		}
	}
}

func TestEC2HelperParsers(t *testing.T) {
	if _, _, err := cliutil.ParseTagFilter("invalid"); err == nil {
		t.Fatal("expected ParseTagFilter error")
	}

	key, value, err := cliutil.ParseTagFilter("env=prod")
	if err != nil || key != "env" || value != "prod" {
		t.Fatalf("unexpected ParseTagFilter result: %q %q %v", key, value, err)
	}

	if !matchesSecurityGroupType("rds-main", "rds") || matchesSecurityGroupType("app-main", "rds") {
		t.Fatal("unexpected matchesSecurityGroupType behavior")
	}
	if !matchesSecurityGroupType("app-main", "ec2") {
		t.Fatal("expected ec2 group type match")
	}
	if matchesSecurityGroupType("app-main", "unknown") {
		t.Fatal("unexpected unknown type match")
	}

	if !hasTagMatch([]ec2types.Tag{{Key: cliutil.Ptr("env"), Value: cliutil.Ptr("production")}}, "env", "prod") {
		t.Fatal("expected tag match")
	}
	if hasTagMatch([]ec2types.Tag{{Key: cliutil.Ptr("team"), Value: cliutil.Ptr("platform")}}, "env", "prod") {
		t.Fatal("unexpected tag match")
	}
}

func TestEC2DeleteAMIsValidationAndCancelledPrompt(t *testing.T) {
	client := &mockClient{
		describeImagesFn: func(_ context.Context, _ *ec2.DescribeImagesInput, _ ...func(*ec2.Options)) (*ec2.DescribeImagesOutput, error) {
			return &ec2.DescribeImagesOutput{Images: []ec2types.Image{
				{ImageId: cliutil.Ptr("ami-old"), Name: cliutil.Ptr("old"), CreationDate: cliutil.Ptr(time.Now().UTC().AddDate(0, 0, -40).Format(time.RFC3339))},
			}}, nil
		},
		deregisterImageFn: func(_ context.Context, _ *ec2.DeregisterImageInput, _ ...func(*ec2.Options)) (*ec2.DeregisterImageOutput, error) {
			return &ec2.DeregisterImageOutput{}, nil
		},
	}

	withMockDeps(
		t,
		func(_, _ string) (awssdk.Config, error) { return awssdk.Config{Region: "us-east-1"}, nil },
		func(awssdk.Config) API { return client },
		func(awssdk.Config, string) API { return client },
	)

	if _, err := executeCommand(t, "ec2", "delete-amis"); err == nil {
		t.Fatal("expected missing-filter error")
	}

	output, err := executeCommandWithInput(t, "n\n", "--output", "json", "ec2", "delete-amis", "--retention-days", "30")
	if err != nil {
		t.Fatalf("execute delete-amis with prompt: %v", err)
	}
	if !strings.Contains(output, "cancelled") {
		t.Fatalf("expected cancelled action: %s", output)
	}
}

func TestEC2DeleteSecurityGroupsValidationErrors(t *testing.T) {
	if _, err := executeCommand(t, "ec2", "delete-security-groups"); err == nil {
		t.Fatal("expected missing-filter error")
	}
	if _, err := executeCommand(t, "ec2", "delete-security-groups", "--ssh-rules", "--type", "bad"); err == nil {
		t.Fatal("expected invalid-type error")
	}
}

func TestEC2DeleteSecurityGroupsRevokeExecutesWhenNoConfirm(t *testing.T) {
	revoked := 0
	client := &mockClient{
		describeSecurityGroupsFn: func(_ context.Context, _ *ec2.DescribeSecurityGroupsInput, _ ...func(*ec2.Options)) (*ec2.DescribeSecurityGroupsOutput, error) {
			return &ec2.DescribeSecurityGroupsOutput{SecurityGroups: []ec2types.SecurityGroup{
				{
					GroupId:   cliutil.Ptr("sg-1"),
					GroupName: cliutil.Ptr("app"),
					IpPermissions: []ec2types.IpPermission{{
						IpProtocol: cliutil.Ptr("tcp"),
						FromPort:   cliutil.Ptr(int32(22)),
						ToPort:     cliutil.Ptr(int32(22)),
					}},
				},
			}}, nil
		},
		revokeSecurityIngressFn: func(_ context.Context, _ *ec2.RevokeSecurityGroupIngressInput, _ ...func(*ec2.Options)) (*ec2.RevokeSecurityGroupIngressOutput, error) {
			revoked++
			return &ec2.RevokeSecurityGroupIngressOutput{}, nil
		},
	}

	withMockDeps(
		t,
		func(_, _ string) (awssdk.Config, error) { return awssdk.Config{Region: "us-east-1"}, nil },
		func(awssdk.Config) API { return client },
		func(awssdk.Config, string) API { return client },
	)

	output, err := executeCommand(t, "--output", "json", "--no-confirm", "ec2", "delete-security-groups", "--ssh-rules")
	if err != nil {
		t.Fatalf("execute delete-security-groups --ssh-rules: %v", err)
	}
	if revoked != 1 || !strings.Contains(output, "deleted") {
		t.Fatalf("unexpected output: %s", output)
	}
}

func TestEC2DeleteVolumesCancelledPrompt(t *testing.T) {
	client := &mockClient{
		describeVolumesFn: func(_ context.Context, _ *ec2.DescribeVolumesInput, _ ...func(*ec2.Options)) (*ec2.DescribeVolumesOutput, error) {
			return &ec2.DescribeVolumesOutput{Volumes: []ec2types.Volume{{VolumeId: cliutil.Ptr("vol-1"), Size: cliutil.Ptr(int32(20))}}}, nil
		},
	}

	withMockDeps(
		t,
		func(_, _ string) (awssdk.Config, error) { return awssdk.Config{Region: "us-east-1"}, nil },
		func(awssdk.Config) API { return client },
		func(awssdk.Config, string) API { return client },
	)

	output, err := executeCommandWithInput(t, "n\n", "--output", "json", "ec2", "delete-volumes")
	if err != nil {
		t.Fatalf("execute delete-volumes with prompt: %v", err)
	}
	if !strings.Contains(output, "cancelled") {
		t.Fatalf("expected cancelled action: %s", output)
	}
}
