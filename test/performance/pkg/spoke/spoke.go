package spoke

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"embed"
	"encoding/json"
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

	ocmfeature "open-cluster-management.io/api/feature"

	commonoptions "open-cluster-management.io/ocm/pkg/common/options"
	"open-cluster-management.io/ocm/pkg/features"
	"open-cluster-management.io/ocm/pkg/work/spoke"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
)

const (
	CertificateBlockType   = "CERTIFICATE"
	RSAPrivateKeyBlockType = "RSA PRIVATE KEY"
)

const jobFile = "manifests/job.yaml"

//go:embed manifests
var ManifestFiles embed.FS

type SpokeOptions struct {
	AgentConfigDir string

	HubKubeConfigPath   string
	SpokeKubeConfigPath string

	MaestroNamespace string

	KafkaServer          string
	KafkaClusterCAPath   string
	KafkaClientCAPath    string
	KafkaClientCAKeyPath string

	ClusterBeginIndex int
	ClusterCounts     int
	ClusterWithWorks  bool
}

func NewSpokeOptions() *SpokeOptions {
	return &SpokeOptions{
		MaestroNamespace:  "maestro",
		ClusterBeginIndex: 0,
		ClusterCounts:     1,
	}
}

func (o *SpokeOptions) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&o.AgentConfigDir, "agent-config-dir", o.AgentConfigDir, "The dir to save the agent configs")
	fs.StringVar(&o.HubKubeConfigPath, "hub-kubeconfig", o.HubKubeConfigPath, "Location of the Hub kubeconfig")
	fs.StringVar(&o.SpokeKubeConfigPath, "spoke-kubeconfig", o.SpokeKubeConfigPath, "Location of the Spoke kubeconfig")
	fs.StringVar(&o.KafkaServer, "kafka-server", o.KafkaServer, "Address of the kafka server")
	fs.StringVar(&o.KafkaClusterCAPath, "kafka-cluster-ca", o.KafkaClusterCAPath, "Location of the Kafka cluster CA")
	fs.StringVar(&o.KafkaClientCAPath, "kafka-client-ca", o.KafkaClientCAPath, "Location of the Kafka client CA")
	fs.StringVar(&o.KafkaClientCAKeyPath, "kafka-client-ca-key", o.KafkaClientCAKeyPath, "Location of the Kafka client CA Key")
	fs.IntVar(&o.ClusterBeginIndex, "cluster-begin-index", o.ClusterBeginIndex, "Begin index of the clusters")
	fs.IntVar(&o.ClusterCounts, "cluster-counts", o.ClusterCounts, "Counts of the clusters")
	fs.BoolVar(&o.ClusterWithWorks, "cluster-with-works", o.ClusterWithWorks, "Create cluster with initialized works")
}

func (o *SpokeOptions) Run(ctx context.Context) error {
	hubKubeConfig, err := clientcmd.BuildConfigFromFlags("", o.HubKubeConfigPath)
	if err != nil {
		return err
	}

	spokeKubeConfig, err := clientcmd.BuildConfigFromFlags("", o.SpokeKubeConfigPath)
	if err != nil {
		return err
	}

	hubKubeClient, err := kubernetes.NewForConfig(hubKubeConfig)
	if err != nil {
		return err
	}

	spokeKubeClient, err := kubernetes.NewForConfig(spokeKubeConfig)
	if err != nil {
		return err
	}

	// prepare clusters
	if err := o.PrepareClusters(ctx, hubKubeClient); err != nil {
		return err
	}

	// start agents
	utilruntime.Must(features.SpokeMutableFeatureGate.Add(ocmfeature.DefaultSpokeWorkFeatureGates))
	utilruntime.Must(features.SpokeMutableFeatureGate.Set(fmt.Sprintf("%s=true", ocmfeature.RawFeedbackJsonString)))

	index := o.ClusterBeginIndex
	for i := 0; i < o.ClusterCounts; i++ {
		clusterName := util.ClusterName(index)

		if err := o.prepareAgentConfig(clusterName); err != nil {
			return err
		}

		_, err := spokeKubeClient.CoreV1().Namespaces().Get(ctx, clusterName, metav1.GetOptions{})
		switch {
		case errors.IsNotFound(err):
			if _, err := spokeKubeClient.CoreV1().Namespaces().Create(
				ctx,
				&corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: clusterName,
					},
				},
				metav1.CreateOptions{},
			); err != nil {
				return err
			}
		case err != nil:
			return err
		}

		klog.Infof("The namespace of cluster %s is created", clusterName)

		go func() {
			klog.Infof("Starting the work agent for cluster %s", clusterName)
			if err := o.startWorkAgent(ctx, spokeKubeConfig, clusterName); err != nil {
				klog.Errorf("failed to start work agent for cluster %s, %v", clusterName, err)
			}
		}()

		index = index + 1
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
	caKey, err := x509.ParsePKCS8PrivateKey(keyBlock.Bytes)
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
		"bootstrapServer": o.KafkaServer,
		"caFile":          o.KafkaClusterCAPath,
		"clientCertFile":  filepath.Join(o.AgentConfigDir, fmt.Sprintf("client-%s.crt", clusterName)),
		"clientKeyFile":   filepath.Join(o.AgentConfigDir, fmt.Sprintf("client-%s.key", clusterName)),
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
	agentOptions.MaxJSONRawLength = 1024 * 1024 // 1M
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

func (o *SpokeOptions) PrepareClusters(ctx context.Context, kubeClient kubernetes.Interface) error {
	lastIndex := o.ClusterBeginIndex + o.ClusterCounts - 1
	name := fmt.Sprintf("clusters-%d-%d", o.ClusterBeginIndex, lastIndex)
	if o.ClusterWithWorks {
		name = fmt.Sprintf("clusters-with-works-%d-%d", o.ClusterBeginIndex, lastIndex)
	}

	_, err := kubeClient.BatchV1().Jobs("maestro").Get(ctx, name, metav1.GetOptions{})
	switch {
	case errors.IsNotFound(err):
		data, err := ManifestFiles.ReadFile(jobFile)
		if err != nil {
			return err
		}

		raw, err := util.Render(
			jobFile,
			data,
			&struct {
				Name              string
				Namespace         string
				ClusterBeginIndex int
				ClusterCounts     int
				ClusterWithWorks  bool
			}{
				Name:              name,
				Namespace:         o.MaestroNamespace,
				ClusterBeginIndex: o.ClusterBeginIndex,
				ClusterCounts:     o.ClusterCounts,
				ClusterWithWorks:  o.ClusterWithWorks,
			},
		)
		if err != nil {
			return err
		}

		job := &batchv1.Job{}
		if err := json.Unmarshal(raw, job); err != nil {
			return err
		}

		_, err = kubeClient.BatchV1().Jobs("maestro").Create(ctx, job, metav1.CreateOptions{})
		if err != nil {
			return err
		}
	case err != nil:
		return err
	}

	return util.Eventually(
		func() error {
			job, err := kubeClient.BatchV1().Jobs("maestro").Get(ctx, name, metav1.GetOptions{})
			if err != nil {
				return err
			}

			for _, cond := range job.Status.Conditions {
				if cond.Type != batchv1.JobComplete {
					continue
				}

				if cond.Status == corev1.ConditionTrue {
					return nil
				}
			}

			return fmt.Errorf("the job %s/%s is not completed", o.MaestroNamespace, name)
		},
		300*time.Second,
		time.Second,
	)
}
