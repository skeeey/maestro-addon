package mq

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
	"gopkg.in/yaml.v2"
	"k8s.io/klog/v2"

	"github.com/stolostron/maestro-addon/pkg/common"
	"github.com/stolostron/maestro-addon/pkg/helpers"
)

const sourceID = "maestro"

type MessageQueueAuthzCreator interface {
	CreateAuthorizations(ctx context.Context, clusterName string) error
	DeleteAuthorizations(ctx context.Context, clusterName string) error
}

func NewMessageQueueAuthzCreator(mqType, mqConfigPath string) (MessageQueueAuthzCreator, error) {
	switch mqType {
	case common.BrokerKafka:
		config, err := ToKafkaConfigMap(mqConfigPath)
		if err != nil {
			return nil, err
		}

		if err := helpers.CreteKafkaTopics(context.Background(), config, sourceID); err != nil {
			return nil, err
		}

		return &KafkaAuthzCreator{config: config}, nil
	case common.BrokerGRPC:
		return &DumbAuthzCreator{}, nil
	default:
		klog.Warningf("unsupported message queue driver: %s, will not create message queue authorizations", mqType)
		return nil, nil
	}
}

type KafkaConfig struct {
	// BootstrapServer is the host of the Kafka broker (hostname:port).
	BootstrapServer string `json:"bootstrapServer" yaml:"bootstrapServer"`

	// CAFile is the file path to a cert file for the MQTT broker certificate authority.
	CAFile string `json:"caFile,omitempty" yaml:"caFile,omitempty"`
	// ClientCertFile is the file path to a client cert file for TLS.
	ClientCertFile string `json:"clientCertFile,omitempty" yaml:"clientCertFile,omitempty"`
	// ClientKeyFile is the file path to a client key file for TLS.
	ClientKeyFile string `json:"clientKeyFile,omitempty" yaml:"clientKeyFile,omitempty"`
}

func ToKafkaConfigMap(configPath string) (*kafka.ConfigMap, error) {
	configData, err := os.ReadFile(filepath.Clean(configPath))
	if err != nil {
		return nil, err
	}

	config := &KafkaConfig{}
	if err := yaml.Unmarshal(configData, config); err != nil {
		return nil, err
	}

	if config.BootstrapServer == "" {
		return nil, fmt.Errorf("bootstrapServer is required")
	}

	if (config.ClientCertFile == "" && config.ClientKeyFile != "") ||
		(config.ClientCertFile != "" && config.ClientKeyFile == "") {
		return nil, fmt.Errorf("either both or none of clientCertFile and clientKeyFile must be set")
	}
	if config.ClientCertFile != "" && config.ClientKeyFile != "" && config.CAFile == "" {
		return nil, fmt.Errorf("setting clientCertFile and clientKeyFile requires caFile")
	}

	configMap := &kafka.ConfigMap{
		"bootstrap.servers": config.BootstrapServer,
	}

	if config.ClientCertFile != "" {
		_ = configMap.SetKey("security.protocol", "ssl")
		_ = configMap.SetKey("ssl.ca.location", config.CAFile)
		_ = configMap.SetKey("ssl.certificate.location", config.ClientCertFile)
		_ = configMap.SetKey("ssl.key.location", config.ClientKeyFile)
	}

	return configMap, nil
}

type DumbAuthzCreator struct{}

func (d *DumbAuthzCreator) CreateAuthorizations(ctx context.Context, clusterName string) error {
	return nil
}

func (d *DumbAuthzCreator) DeleteAuthorizations(ctx context.Context, clusterName string) error {
	return nil
}

type KafkaAuthzCreator struct {
	config *kafka.ConfigMap
}

func (c *KafkaAuthzCreator) CreateAuthorizations(ctx context.Context, clusterName string) error {
	return helpers.CreateACLs(ctx, c.config, sourceID, clusterName)
}

func (c *KafkaAuthzCreator) DeleteAuthorizations(ctx context.Context, clusterName string) error {
	// TODO Delete Kafka ACLs for topics created for the given cluster
	return nil
}
