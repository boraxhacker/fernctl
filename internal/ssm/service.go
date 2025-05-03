package ssm

import (
	"context"
	"errors"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	awsssm "github.com/aws/aws-sdk-go-v2/service/ssm"
	awstypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"
	"gopkg.in/yaml.v3"
	"os"
	"strings"
	"time"
)

type Service struct {
	client *awsssm.Client
}

func NewService() *Service {

	awsConfig, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		panic(err)
	}

	result := awsssm.NewFromConfig(awsConfig)

	return &Service{client: result}
}

func (s *Service) Handle(flags []string) error {

	if len(flags) < 1 {
		return errors.New("no subcommand - not enough parameters")
	}

	if flags[0] == "get" {

		return s.get(flags[1])

	} else if flags[0] == "delete" {

		return s.delete(flags[1])

	} else if flags[0] == "sync" {

		if len(flags) < 3 {
			return errors.New("missing prefix and/or file - not enough parameters")
		}

		return s.sync(flags[1], flags[2])
	}

	return nil
}

func (s *Service) get(key string) error {

	var parameters []awstypes.Parameter

	if strings.HasPrefix(key, "path:") {

		resp, err := s.client.GetParametersByPath(context.TODO(), &awsssm.GetParametersByPathInput{
			Path:           aws.String(strings.TrimPrefix(key, "path:")),
			Recursive:      aws.Bool(true),
			WithDecryption: aws.Bool(true),
		})

		if err != nil {
			return err
		}

		parameters = resp.Parameters

	} else {

		resp, err := s.client.GetParameter(context.TODO(), &awsssm.GetParameterInput{
			Name:           aws.String(key),
			WithDecryption: aws.Bool(true),
		})

		if err != nil {
			return err
		}

		parameters = append(parameters, *resp.Parameter)
	}

	for _, parameter := range parameters {

		fmt.Printf("%s: '%s'\n", aws.ToString(parameter.Name), aws.ToString(parameter.Value))
	}

	return nil
}

func (s *Service) delete(key string) error {

	var parameters []string

	if strings.HasPrefix(key, "path:") {

		resp, err := s.client.GetParametersByPath(context.TODO(), &awsssm.GetParametersByPathInput{
			Path:           aws.String(strings.TrimPrefix(key, "path:")),
			Recursive:      aws.Bool(true),
			WithDecryption: aws.Bool(true),
		})

		if err != nil {
			return err
		}

		for _, parameter := range resp.Parameters {
			parameters = append(parameters, aws.ToString(parameter.Name))
		}

	} else {

		parameters = append(parameters, key)
	}

	resp, err := s.client.DeleteParameters(context.TODO(), &awsssm.DeleteParametersInput{
		Names: parameters,
	})

	if err != nil {
		return err
	}

	fmt.Printf("Success: %v\n", resp.DeletedParameters)
	fmt.Printf("Failures: %v\n", resp.InvalidParameters)

	return nil
}

func (s *Service) sync(prefix string, file string) error {

	yamlfile, err := os.ReadFile(file)
	if err != nil {
		return err
	}

	if !strings.HasPrefix(prefix, "/") {
		prefix = "/" + prefix
	}

	var entries map[string]interface{}
	err = yaml.Unmarshal(yamlfile, &entries)

	values := flatten(entries, "/")

	awsTagKey := aws.String("fernctl-last-synced")
	awsTagValue := aws.String(time.Now().UTC().Format(time.RFC3339))

	for key, value := range values {

		if !strings.HasPrefix(key, "/") {
			key = "/" + key
		}

		awskey := aws.String(prefix + key)

		fmt.Printf("Upserting %s\n", aws.ToString(awskey))

		_, err := s.client.PutParameter(context.TODO(), &awsssm.PutParameterInput{
			Name:      awskey,
			Value:     aws.String(value.(string)),
			Overwrite: aws.Bool(true),
			Type:      awstypes.ParameterTypeSecureString,
		})

		if err != nil {
			return err
		}

		_, err = s.client.AddTagsToResource(context.TODO(), &awsssm.AddTagsToResourceInput{
			ResourceId:   awskey,
			ResourceType: awstypes.ResourceTypeForTaggingParameter,
			Tags: []awstypes.Tag{
				{
					Key:   awsTagKey,
					Value: awsTagValue,
				},
			},
		})

		if err != nil {
			return err
		}
	}

	return s.prune(prefix, *awsTagKey, *awsTagValue)
}

func (s *Service) prune(prefix string, tagkey string, tagvalue string) error {

	resp, err := s.client.GetParametersByPath(context.TODO(), &awsssm.GetParametersByPathInput{
		Path:           aws.String(prefix),
		Recursive:      aws.Bool(true),
		WithDecryption: aws.Bool(true),
	})

	if err != nil {
		return err
	}

	for _, param := range resp.Parameters {

		tags, err := s.client.ListTagsForResource(context.TODO(), &awsssm.ListTagsForResourceInput{
			ResourceId:   param.Name,
			ResourceType: awstypes.ResourceTypeForTaggingParameter,
		})

		if err != nil {
			return err
		}

		found := false
		for _, tag := range tags.TagList {

			if aws.ToString(tag.Key) == tagkey && aws.ToString(tag.Value) == tagvalue {
				found = true
				break
			}
		}

		if !found {

			fmt.Printf("Pruning %s\n", aws.ToString(param.Name))
			_, err = s.client.DeleteParameter(context.TODO(), &awsssm.DeleteParameterInput{Name: param.Name})
		}

		if err != nil {
			return err
		}
	}

	return nil
}
