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

	if strings.HasPrefix(key, "path:") {

		isdone := false
		var nexttoken *string
		for !isdone {

			resp, err := s.client.GetParametersByPath(context.TODO(), &awsssm.GetParametersByPathInput{
				Path:           aws.String(strings.TrimPrefix(key, "path:")),
				Recursive:      aws.Bool(true),
				WithDecryption: aws.Bool(true),
				NextToken:      nexttoken,
			})

			if err != nil {
				return err
			}

			for i, _ := range resp.Parameters {

				printParameter(&resp.Parameters[i])
			}

			isdone = resp.NextToken == nil || aws.ToString(resp.NextToken) == ""
			nexttoken = resp.NextToken
		}

	} else {

		resp, err := s.client.GetParameter(context.TODO(), &awsssm.GetParameterInput{
			Name:           aws.String(key),
			WithDecryption: aws.Bool(true),
		})

		if err != nil {
			return err
		}

		printParameter(resp.Parameter)
	}

	return nil
}

func printParameter(parameter *awstypes.Parameter) {

	fmt.Printf("%s: '%s'\n", aws.ToString(parameter.Name), aws.ToString(parameter.Value))
}

func (s *Service) delete(key string) error {

	var deletedParameters []string
	var invalidParameters []string

	if strings.HasPrefix(key, "path:") {

		isdone := false
		var nexttoken *string
		for !isdone {

			resp, err := s.client.GetParametersByPath(context.TODO(), &awsssm.GetParametersByPathInput{
				Path:           aws.String(strings.TrimPrefix(key, "path:")),
				Recursive:      aws.Bool(true),
				WithDecryption: aws.Bool(true),
				NextToken:      nexttoken,
			})

			if err != nil {
				return err
			}

			var parameters []string

			for _, parameter := range resp.Parameters {
				parameters = append(parameters, aws.ToString(parameter.Name))
			}

			delresp, err := s.client.DeleteParameters(context.TODO(), &awsssm.DeleteParametersInput{
				Names: parameters,
			})

			if err != nil {

				invalidParameters = append(invalidParameters, parameters...)

			} else {

				deletedParameters = append(deletedParameters, delresp.DeletedParameters...)
				invalidParameters = append(invalidParameters, delresp.InvalidParameters...)
			}

			isdone = resp.NextToken == nil || aws.ToString(resp.NextToken) == ""
			nexttoken = resp.NextToken
		}

	} else {

		_, err := s.client.DeleteParameter(context.TODO(), &awsssm.DeleteParameterInput{
			Name: aws.String(key),
		})

		if err != nil {

			invalidParameters = append(invalidParameters, key)

		} else {

			deletedParameters = append(deletedParameters, key)
		}
	}

	fmt.Printf("Success: %v\n", deletedParameters)
	fmt.Printf("Failures: %v\n", invalidParameters)

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

	isdone := false
	var nexttoken *string
	for !isdone {

		resp, err := s.client.GetParametersByPath(context.TODO(), &awsssm.GetParametersByPathInput{
			Path:           aws.String(prefix),
			Recursive:      aws.Bool(true),
			WithDecryption: aws.Bool(true),
			NextToken:      nexttoken,
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

			prune := false
			for _, tag := range tags.TagList {

				if aws.ToString(tag.Key) == tagkey && aws.ToString(tag.Value) != tagvalue {
					prune = true
					break
				}
			}

			if prune {

				fmt.Printf("Pruning %s\n", aws.ToString(param.Name))
				_, err = s.client.DeleteParameter(context.TODO(), &awsssm.DeleteParameterInput{Name: param.Name})
			}

			if err != nil {
				return err
			}
		}

		isdone = resp.NextToken == nil || aws.ToString(resp.NextToken) == ""
		nexttoken = resp.NextToken
	}

	return nil
}
