package cmd

import (
	"fmt"

	"github.com/Azure/dcos-engine/pkg/armhelpers"
	"github.com/Azure/go-autorest/autorest/azure"
	uuid "github.com/satori/go.uuid"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"
)

const (
	rootName             = "dcos-engine"
	rootShortDescription = "DCOS-Engine deploys and manages DC/OS clusters in Azure"
	rootLongDescription  = "DCOS-Engine deploys and manages DC/OS clusters in Azure"
)

var (
	debug bool
)

// NewRootCmd returns the root command for DCOS-Engine.
func NewRootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   rootName,
		Short: rootShortDescription,
		Long:  rootLongDescription,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			if debug {
				log.SetLevel(log.DebugLevel)
			}
		},
	}

	p := rootCmd.PersistentFlags()
	p.BoolVar(&debug, "debug", false, "enable verbose debug logs")

	rootCmd.AddCommand(newVersionCmd())
	rootCmd.AddCommand(newGenerateCmd())
	rootCmd.AddCommand(newDeployCmd())
	rootCmd.AddCommand(newOrchestratorsCmd())
	rootCmd.AddCommand(newDcosUpgradeCmd())

	return rootCmd
}

type authArgs struct {
	RawAzureEnvironment string
	rawSubscriptionID   string
	SubscriptionID      uuid.UUID
	AuthMethod          string
	rawClientID         string

	ClientID        uuid.UUID
	ClientSecret    string
	CertificatePath string
	PrivateKeyPath  string
	language        string
}

func addAuthFlags(authArgs *authArgs, f *flag.FlagSet) {
	f.StringVar(&authArgs.RawAzureEnvironment, "azure-env", "AzurePublicCloud", "the target Azure cloud")
	f.StringVar(&authArgs.rawSubscriptionID, "subscription-id", "", "azure subscription id (required)")
	f.StringVar(&authArgs.AuthMethod, "auth-method", "device", "auth method (default:`device`, `client_secret`, `client_certificate`)")
	f.StringVar(&authArgs.rawClientID, "client-id", "", "client id (used with --auth-method=[client_secret|client_certificate])")
	f.StringVar(&authArgs.ClientSecret, "client-secret", "", "client secret (used with --auth-mode=client_secret)")
	f.StringVar(&authArgs.CertificatePath, "certificate-path", "", "path to client certificate (used with --auth-method=client_certificate)")
	f.StringVar(&authArgs.PrivateKeyPath, "private-key-path", "", "path to private key (used with --auth-method=client_certificate)")
	f.StringVar(&authArgs.language, "language", "en-us", "language to return error messages in")
}

func (authArgs *authArgs) validateAuthArgs() error {
	authArgs.ClientID, _ = uuid.FromString(authArgs.rawClientID)
	authArgs.SubscriptionID, _ = uuid.FromString(authArgs.rawSubscriptionID)

	if authArgs.AuthMethod == "client_secret" {
		if authArgs.ClientID.String() == "00000000-0000-0000-0000-000000000000" || authArgs.ClientSecret == "" {
			return fmt.Errorf(`--client-id and --client-secret must be specified when --auth-method="client_secret"`)
		}
		// try parse the UUID
	} else if authArgs.AuthMethod == "client_certificate" {
		if authArgs.ClientID.String() == "00000000-0000-0000-0000-000000000000" || authArgs.CertificatePath == "" || authArgs.PrivateKeyPath == "" {
			return fmt.Errorf(`--client-id and --certificate-path, and --private-key-path must be specified when --auth-method="client_certificate"`)
		}
	}

	if authArgs.SubscriptionID.String() == "00000000-0000-0000-0000-000000000000" {
		return fmt.Errorf("--subscription-id is required (and must be a valid UUID)")
	}

	_, err := azure.EnvironmentFromName(authArgs.RawAzureEnvironment)
	if err != nil {
		return fmt.Errorf("failed to parse --azure-env as a valid target Azure cloud environment")
	}
	return nil
}

func (authArgs *authArgs) getClient() (*armhelpers.AzureClient, error) {
	var client *armhelpers.AzureClient
	env, err := azure.EnvironmentFromName(authArgs.RawAzureEnvironment)
	if err != nil {
		return nil, err
	}
	switch authArgs.AuthMethod {
	case "device":
		client, err = armhelpers.NewAzureClientWithDeviceAuth(env, authArgs.SubscriptionID.String())
	case "client_secret":
		client, err = armhelpers.NewAzureClientWithClientSecret(env, authArgs.SubscriptionID.String(), authArgs.ClientID.String(), authArgs.ClientSecret)
	case "client_certificate":
		client, err = armhelpers.NewAzureClientWithClientCertificateFile(env, authArgs.SubscriptionID.String(), authArgs.ClientID.String(), authArgs.CertificatePath, authArgs.PrivateKeyPath)
	default:
		return nil, fmt.Errorf("--auth-method: ERROR: method unsupported. method=%q", authArgs.AuthMethod)
	}
	if err != nil {
		return nil, err
	}
	err = client.EnsureProvidersRegistered(authArgs.SubscriptionID.String())
	if err != nil {
		return nil, err
	}
	client.AddAcceptLanguages([]string{authArgs.language})
	return client, nil
}
