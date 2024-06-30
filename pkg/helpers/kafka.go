package helpers

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/klog/v2"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
	"github.com/stolostron/maestro-addon/pkg/common"
)

// an interface for kafka.AdminClient, this will help with testing
type KafkaAdminClient interface {
	DescribeTopics(ctx context.Context, topics kafka.TopicCollection,
		options ...kafka.DescribeTopicsAdminOption) (result kafka.DescribeTopicsResult, err error)
	DescribeACLs(ctx context.Context, aclBindingFilter kafka.ACLBindingFilter,
		options ...kafka.DescribeACLsAdminOption) (result *kafka.DescribeACLsResult, err error)
	CreateTopics(ctx context.Context, topics []kafka.TopicSpecification,
		options ...kafka.CreateTopicsAdminOption) (result []kafka.TopicResult, err error)
	CreateACLs(ctx context.Context, aclBindings kafka.ACLBindings,
		options ...kafka.CreateACLsAdminOption) (result []kafka.CreateACLResult, err error)
}

// CreteKafkaPlaceholderTopics creates placeholder topics.
// This avoids unknown topic error when subscribing to wildcard topics
func CreteKafkaPlaceholderTopics(ctx context.Context, config *kafka.ConfigMap, sourceID string) error {
	client, err := kafka.NewAdminClient(config)
	if err != nil {
		return err
	}
	defer client.Close()

	return createKafkaTopics(ctx, client, kafkaPlaceholderTopics(sourceID)...)
}

func CreateKafkaTopicsWithACLs(ctx context.Context, config *kafka.ConfigMap, sourceID, clusterName string) error {
	adminClient, err := kafka.NewAdminClient(config)
	if err != nil {
		return err
	}
	defer adminClient.Close()

	// each cluster has four topics
	topics := kafkaClusterTopics(sourceID, clusterName)

	if err := createKafkaTopics(ctx, adminClient, topics...); err != nil {
		return err
	}

	return createKafkaACLs(ctx, adminClient, clusterName, topics...)
}

func createKafkaTopics(ctx context.Context, adminClient KafkaAdminClient, newTopics ...string) error {
	logger := klog.FromContext(ctx)

	topics, err := adminClient.DescribeTopics(ctx, kafka.NewTopicCollectionOfTopicNames(newTopics))
	if err != nil {
		return err
	}

	topicSpecs := []kafka.TopicSpecification{}
	for _, topic := range newTopics {
		if hasKafkaTopic(topics.TopicDescriptions, topic) {
			logger.V(4).Info(fmt.Sprintf("topic %s already exists", topic))
			continue
		}

		topicSpecs = append(topicSpecs, kafka.TopicSpecification{
			Topic:             topic,
			NumPartitions:     1,
			ReplicationFactor: 1,
		})
	}

	if len(topicSpecs) == 0 {
		return nil
	}

	results, err := adminClient.CreateTopics(ctx, topicSpecs)
	if err != nil {
		return err
	}

	errs := []error{}
	for _, r := range results {
		if r.Error.Code() == kafka.ErrNoError {
			logger.V(4).Info(fmt.Sprintf("topic %s created successfully", r.Topic))
			continue
		}

		errs = append(errs, fmt.Errorf("failed to create topic %s, %s", r.Topic, r.Error.String()))
	}

	return errors.NewAggregate(errs)
}

func createKafkaACLs(ctx context.Context, adminClient KafkaAdminClient, clusterName string, topics ...string) error {
	logger := klog.FromContext(ctx)

	principal := ToKafkaPrincipal(clusterName)

	expectedACLBindings := []kafka.ACLBinding{{
		Type:                kafka.ResourceGroup,
		Name:                "*",
		ResourcePatternType: kafka.ResourcePatternTypeLiteral,
		Principal:           principal,
		Host:                "*",
		Operation:           kafka.ACLOperationAll,
		PermissionType:      kafka.ACLPermissionTypeAllow,
	}}

	for _, topic := range topics {
		expectedACLBindings = append(expectedACLBindings, kafka.ACLBinding{
			Type:                kafka.ResourceTopic,
			Name:                topic,
			ResourcePatternType: kafka.ResourcePatternTypeLiteral,
			Principal:           principal,
			Host:                "*",
			Operation:           kafka.ACLOperationAll,
			PermissionType:      kafka.ACLPermissionTypeAllow,
		})
	}

	aclBindings := []kafka.ACLBinding{}
	for _, acl := range expectedACLBindings {
		result, err := adminClient.DescribeACLs(ctx, acl)
		if err != nil {
			return err
		}

		if hasKafkaACL(result, acl) {
			logger.V(4).Info(fmt.Sprintf("acl %s/%s already exists for %s", acl.Type, acl.Name, acl.Principal))
			continue
		}

		aclBindings = append(aclBindings, acl)
	}

	if len(aclBindings) == 0 {
		return nil
	}

	results, err := adminClient.CreateACLs(ctx, aclBindings)
	if err != nil {
		return err
	}

	errs := []error{}
	for _, r := range results {
		if r.Error.Code() != kafka.ErrNoError {
			errs = append(errs, fmt.Errorf("failed to create acl %s", r.Error.String()))
		}
	}
	if len(errs) == 0 {
		logger.V(4).Info(fmt.Sprintf("acls is created successfully for agent %s", principal))
	}

	return errors.NewAggregate(errs)
}

func hasKafkaTopic(topics []kafka.TopicDescription, topic string) bool {
	for _, t := range topics {
		if t.Error.Code() == kafka.ErrNoError && t.Name == topic {
			return true
		}
	}

	return false
}

func hasKafkaACL(acls *kafka.DescribeACLsResult, binding kafka.ACLBinding) bool {
	if acls.Error.Code() == kafka.ErrNoError {
		for _, a := range acls.ACLBindings {
			if a.Name == binding.Name {
				return true
			}
		}
	}
	return false
}

func kafkaPlaceholderTopics(sourceID string) []string {
	return []string{
		fmt.Sprintf("sourceevents.%s.agent", sourceID),
		fmt.Sprintf("sourcebroadcast.%s", sourceID),
		fmt.Sprintf("agentevents.%s.agent", sourceID),
		"agentbroadcast.agent",
	}
}

func kafkaClusterTopics(sourceID, clusterName string) []string {
	return []string{
		fmt.Sprintf("sourceevents.%s.%s", sourceID, clusterName),
		fmt.Sprintf("sourcebroadcast.%s", sourceID),
		fmt.Sprintf("agentevents.%s.%s", sourceID, clusterName),
		fmt.Sprintf("agentbroadcast.%s", clusterName),
	}
}

func ToKafkaPrincipal(clusterName string) string {
	commonName := fmt.Sprintf("system:open-cluster-management:cluster:%s:addon:%s:agent:%s-agent",
		clusterName, common.AddOnName, common.AddOnName)
	authGroup := "system:authenticated"
	addOnGroup := fmt.Sprintf("system:open-cluster-management:addon:%s", common.AddOnName)
	clusterGroup := fmt.Sprintf("system:open-cluster-management:cluster:%s:addon:%s", clusterName, common.AddOnName)
	return fmt.Sprintf("User:CN=%s,O=%s+O=%s+O=%s", commonName, authGroup, addOnGroup, clusterGroup)
}
