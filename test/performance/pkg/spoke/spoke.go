package spoke

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"time"

	confluentkafka "github.com/confluentinc/confluent-kafka-go/v2/kafka"
	"github.com/openshift/library-go/pkg/controller/controllercmd"
	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/spf13/pflag"

	"github.com/stolostron/maestro-addon/pkg/helpers"
	"github.com/stolostron/maestro-addon/test/performance/pkg/util"

	"gopkg.in/yaml.v2"

	commonoptions "open-cluster-management.io/ocm/pkg/common/options"
	"open-cluster-management.io/ocm/pkg/work/spoke"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
)

const (
	CertificateBlockType   = "CERTIFICATE"
	RSAPrivateKeyBlockType = "RSA PRIVATE KEY"
)

type SpokeOptions struct {
	AgentConfigDir string

	KubeConfigPath string

	KafkaServer          string
	KafkaClusterCAPath   string
	KafkaClientCAPath    string
	KafkaClientCAKeyPath string

	ClusterBeginIndex int
	ClusterCounts     int
}

func NewSpokeOptions() *SpokeOptions {
	return &SpokeOptions{
		ClusterBeginIndex: 0,
		ClusterCounts:     1,
	}
}

func (o *SpokeOptions) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&o.AgentConfigDir, "agent-config-dir", o.AgentConfigDir, "The dir to save the agent configs")
	fs.StringVar(&o.KubeConfigPath, "kubeconfig", o.KubeConfigPath, "Location of the kubeconfig")
	fs.StringVar(&o.KafkaServer, "kafka-server", o.KafkaServer, "Address of the kafka server")
	fs.StringVar(&o.KafkaClusterCAPath, "kafka-cluster-ca", o.KafkaClusterCAPath, "Location of the Kafka cluster CA")
	fs.StringVar(&o.KafkaClientCAPath, "kafka-client-ca", o.KafkaClientCAPath, "Location of the Kafka client CA")
	fs.StringVar(&o.KafkaClientCAKeyPath, "kafka-client-ca-key", o.KafkaClientCAKeyPath, "Location of the Kafka client CA Key")
	fs.IntVar(&o.ClusterBeginIndex, "cluster-begin-index", o.ClusterBeginIndex, "Begin index of the clusters")
	fs.IntVar(&o.ClusterCounts, "cluster-counts", o.ClusterCounts, "Counts of the clusters")
}

func (o *SpokeOptions) Run(ctx context.Context) error {
	kubeConfig, err := clientcmd.BuildConfigFromFlags("", o.KubeConfigPath)
	if err != nil {
		return err
	}

	kubeClient, err := kubernetes.NewForConfig(kubeConfig)
	if err != nil {
		return err
	}

	for i := o.ClusterBeginIndex; i < o.ClusterCounts; i++ {
		clusterName := util.ClusterName(i)

		if err := o.prepareAgentConfig(clusterName); err != nil {
			return err
		}

		ns := &corev1.Namespace{}
		ns.Name = clusterName
		if _, err := kubeClient.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{}); err != nil {
			return err
		}

		klog.Infof("The namespace of cluster %s is created", clusterName)

		go func() {
			klog.Infof("Starting the work agent for cluster %s", clusterName)
			if err := o.startWorkAgent(ctx, kubeConfig, clusterName); err != nil {
				klog.Errorf("failed to start work agent for cluster %s, %v", clusterName, err)
			}
		}()
	}

	return nil
}

func (o *SpokeOptions) prepareAgentConfig(clusterName string) error {
	clientKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return err
	}

	certFile, err := os.ReadFile(o.KafkaClientCAPath)
	if err != nil {
		return err
	}

	pemBlock, _ := pem.Decode(certFile)
	caCert, err := x509.ParseCertificate(pemBlock.Bytes)
	if err != nil {
		return err
	}

	keyFile, err := os.ReadFile(o.KafkaClientCAKeyPath)
	if err != nil {
		return err
	}
	keyBlock, _ := pem.Decode(keyFile)
	caKey, err := x509.ParsePKCS1PrivateKey(keyBlock.Bytes)
	if err != nil {
		return err
	}

	clientCertDERBytes, err := x509.CreateCertificate(
		rand.Reader,
		&x509.Certificate{
			Subject: pkix.Name{
				CommonName: helpers.ToKafkaPrincipal(clusterName),
			},
			SerialNumber: big.NewInt(1),
			NotBefore:    caCert.NotBefore,
			NotAfter:     caCert.NotBefore.Add(8760 * time.Hour).UTC(),
			KeyUsage:     x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
			ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		},
		caCert,
		clientKey.Public(),
		caKey,
	)
	if err != nil {
		return err
	}

	clientCert, err := x509.ParseCertificate(clientCertDERBytes)
	if err != nil {
		return err
	}

	if err := os.WriteFile(
		filepath.Join(o.AgentConfigDir, fmt.Sprintf("client-%s.crt", clusterName)),
		pem.EncodeToMemory(&pem.Block{
			Type:  CertificateBlockType,
			Bytes: clientCert.Raw,
		}),
		0o600,
	); err != nil {
		return err
	}

	if err := os.WriteFile(
		filepath.Join(o.AgentConfigDir, fmt.Sprintf("client-%s.key", clusterName)),
		pem.EncodeToMemory(&pem.Block{
			Type:  RSAPrivateKeyBlockType,
			Bytes: x509.MarshalPKCS1PrivateKey(clientKey),
		}),
		0o600,
	); err != nil {
		return err
	}

	configData, err := yaml.Marshal(confluentkafka.ConfigMap{
		"bootstrap.servers":        o.KafkaServer,
		"security.protocol":        "ssl",
		"ssl.ca.location":          o.KafkaClusterCAPath,
		"ssl.certificate.location": filepath.Join(o.AgentConfigDir, fmt.Sprintf("client-%s.crt", clusterName)),
		"ssl.key.location":         filepath.Join(o.AgentConfigDir, fmt.Sprintf("client-%s.key", clusterName)),
	})
	if err != nil {
		return err
	}

	configFile := filepath.Join(o.AgentConfigDir, fmt.Sprintf("client-%s.config", clusterName))
	if err := os.WriteFile(configFile, configData, 0o600); err != nil {
		return err
	}

	klog.Infof("The config file %s of cluster %s is prepared", configFile, clusterName)
	return nil
}

func (o *SpokeOptions) startWorkAgent(ctx context.Context, kubeConfig *rest.Config, clusterName string) error {
	commonOptions := commonoptions.NewAgentOptions()
	commonOptions.SpokeClusterName = clusterName

	agentOptions := spoke.NewWorkloadAgentOptions()
	agentOptions.StatusSyncInterval = 3 * time.Second
	agentOptions.AppliedManifestWorkEvictionGracePeriod = 5 * time.Second
	agentOptions.WorkloadSourceDriver = "kafka"
	agentOptions.WorkloadSourceConfig = filepath.Join(o.AgentConfigDir, fmt.Sprintf("client-%s.config", clusterName))
	agentOptions.CloudEventsClientID = fmt.Sprintf("%s-agent", clusterName)
	agentOptions.CloudEventsClientCodecs = []string{"manifestbundle"}

	agentConfig := spoke.NewWorkAgentConfig(commonOptions, agentOptions)
	return agentConfig.RunWorkloadAgent(ctx, &controllercmd.ControllerContext{
		KubeConfig:    kubeConfig,
		EventRecorder: events.NewInMemoryRecorder(clusterName),
	})
}
