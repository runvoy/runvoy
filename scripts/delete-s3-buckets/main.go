// Package main provides a utility script to delete non-empty S3 buckets.
// It deletes all object versions, delete markers, and objects before deleting the bucket.
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/runvoy/runvoy/internal/constants"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

func main() {
	if len(os.Args) < constants.MinimumArgsDeleteS3Buckets {
		log.Fatalf("error: usage: %s <bucket-name> [bucket-name...]", os.Args[0])
	}

	bucketNames := os.Args[1:]
	if len(bucketNames) == 0 {
		log.Fatalf("error: at least one bucket name is required")
	}

	deleteCtx := context.Background()
	for _, bucketName := range bucketNames {
		if bucketName == "" {
			log.Printf("warning: skipping empty bucket name")
			continue
		}

		region := extractRegionFromBucketName(bucketName)
		if region == "" {
			regionCtx, cancel := context.WithTimeout(context.Background(), constants.ScriptContextTimeout)
			var err error
			region, err = detectBucketRegion(regionCtx, bucketName)
			cancel()
			if err != nil {
				log.Fatalf("error: failed to detect region for bucket %s: %v", bucketName, err)
			}
		}

		log.Printf("using region %s for bucket %s", region, bucketName)
		regionCfg, err := awsconfig.LoadDefaultConfig(deleteCtx, awsconfig.WithRegion(region))
		if err != nil {
			log.Fatalf("error: failed to load AWS configuration for region %s: %v", region, err)
		}

		client := s3.NewFromConfig(regionCfg)

		if deleteErr := deleteBucket(deleteCtx, client, bucketName); deleteErr != nil {
			log.Fatalf("error: failed to delete bucket %s: %v", bucketName, deleteErr)
		}

		log.Printf("successfully deleted bucket: %s", bucketName)
	}
}

func detectBucketRegion(ctx context.Context, bucketName string) (string, error) {
	cfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion("us-east-1"))
	if err != nil {
		return "", fmt.Errorf("failed to load AWS config: %w", err)
	}

	client := s3.NewFromConfig(cfg)

	output, err := client.GetBucketLocation(ctx, &s3.GetBucketLocationInput{
		Bucket: aws.String(bucketName),
	})
	if err != nil {
		return "", fmt.Errorf("failed to get bucket location: %w", err)
	}

	region := string(output.LocationConstraint)
	if region == "" || region == "None" {
		region = "us-east-1"
	}

	return region, nil
}

func extractRegionFromBucketName(bucketName string) string {
	regions := []string{
		"us-east-1", "us-east-2", "us-west-1", "us-west-2",
		"eu-west-1", "eu-west-2", "eu-west-3", "eu-central-1", "eu-north-1", "eu-south-1",
		"ap-southeast-1", "ap-southeast-2", "ap-south-1", "ap-northeast-1", "ap-northeast-2", "ap-northeast-3",
		"ca-central-1", "sa-east-1",
	}

	for _, region := range regions {
		if len(bucketName) >= len(region) && bucketName[len(bucketName)-len(region):] == region {
			return region
		}
		if len(bucketName) > len(region)+1 && bucketName[len(bucketName)-len(region)-1:] == "-"+region {
			return region
		}
	}

	return ""
}

func deleteBucket(ctx context.Context, client *s3.Client, bucketName string) error {
	log.Printf("deleting bucket: %s", bucketName)

	if err := deleteAllObjectVersions(ctx, client, bucketName); err != nil {
		return fmt.Errorf("failed to delete object versions: %w", err)
	}

	if err := deleteAllObjects(ctx, client, bucketName); err != nil {
		return fmt.Errorf("failed to delete objects: %w", err)
	}

	_, err := client.DeleteBucket(ctx, &s3.DeleteBucketInput{
		Bucket: aws.String(bucketName),
	})
	if err != nil {
		return fmt.Errorf("failed to delete bucket: %w", err)
	}

	return nil
}

func buildObjectIdentifiersFromVersions(versions []types.ObjectVersion) []types.ObjectIdentifier {
	var objectsToDelete []types.ObjectIdentifier
	for i := range versions {
		version := &versions[i]
		if version.Key != nil && version.VersionId != nil {
			objectsToDelete = append(objectsToDelete, types.ObjectIdentifier{
				Key:       version.Key,
				VersionId: version.VersionId,
			})
		}
	}
	return objectsToDelete
}

func buildObjectIdentifiersFromDeleteMarkers(markers []types.DeleteMarkerEntry) []types.ObjectIdentifier {
	var objectsToDelete []types.ObjectIdentifier
	for i := range markers {
		marker := &markers[i]
		if marker.Key != nil && marker.VersionId != nil {
			objectsToDelete = append(objectsToDelete, types.ObjectIdentifier{
				Key:       marker.Key,
				VersionId: marker.VersionId,
			})
		}
	}
	return objectsToDelete
}

func deleteObjectsBatch(
	ctx context.Context,
	client *s3.Client,
	bucketName string,
	batch []types.ObjectIdentifier,
) error {
	deleteOutput, deleteErr := client.DeleteObjects(ctx, &s3.DeleteObjectsInput{
		Bucket: aws.String(bucketName),
		Delete: &types.Delete{
			Objects: batch,
			Quiet:   aws.Bool(true),
		},
	})
	if deleteErr != nil {
		return fmt.Errorf("failed to delete objects batch: %w", deleteErr)
	}

	if len(deleteOutput.Errors) > 0 {
		for _, delErr := range deleteOutput.Errors {
			if delErr.Key != nil {
				log.Printf("warning: failed to delete object %s: %s", *delErr.Key, aws.ToString(delErr.Message))
			}
		}
	}

	log.Printf("deleted batch of %d objects", len(batch))
	return nil
}

func deleteObjectsInBatches(
	ctx context.Context,
	client *s3.Client,
	bucketName string,
	objectsToDelete []types.ObjectIdentifier,
) error {
	if len(objectsToDelete) == 0 {
		return nil
	}

	log.Printf("deleting %d object versions and delete markers", len(objectsToDelete))

	batchSize := 1000
	for i := 0; i < len(objectsToDelete); i += batchSize {
		end := min(i+batchSize, len(objectsToDelete))
		batch := objectsToDelete[i:end]

		if err := deleteObjectsBatch(ctx, client, bucketName, batch); err != nil {
			return err
		}
	}
	return nil
}

func deleteAllObjectVersions(ctx context.Context, client *s3.Client, bucketName string) error {
	log.Printf("listing object versions for bucket: %s", bucketName)

	var continuationToken *string
	var versionIDMarker *string

loop:
	for {
		listOutput, err := client.ListObjectVersions(ctx, &s3.ListObjectVersionsInput{
			Bucket:          aws.String(bucketName),
			KeyMarker:       continuationToken,
			VersionIdMarker: versionIDMarker,
		})
		if err != nil {
			return fmt.Errorf("failed to list object versions: %w", err)
		}

		objectsToDelete := buildObjectIdentifiersFromVersions(listOutput.Versions)
		objectsToDelete = append(objectsToDelete, buildObjectIdentifiersFromDeleteMarkers(listOutput.DeleteMarkers)...)

		if batchErr := deleteObjectsInBatches(ctx, client, bucketName, objectsToDelete); batchErr != nil {
			return batchErr
		}

		if listOutput.IsTruncated == nil || !*listOutput.IsTruncated {
			break
		}
		switch {
		case len(listOutput.Versions) > 0:
			lastVersion := listOutput.Versions[len(listOutput.Versions)-1]
			continuationToken = lastVersion.Key
			versionIDMarker = lastVersion.VersionId
		case len(listOutput.DeleteMarkers) > 0:
			lastMarker := listOutput.DeleteMarkers[len(listOutput.DeleteMarkers)-1]
			continuationToken = lastMarker.Key
			versionIDMarker = lastMarker.VersionId
		default:
			break loop
		}
	}

	return nil
}

func buildObjectIdentifiersFromContents(contents []types.Object) []types.ObjectIdentifier {
	var objectsToDelete []types.ObjectIdentifier
	for i := range contents {
		obj := &contents[i]
		if obj.Key != nil {
			objectsToDelete = append(objectsToDelete, types.ObjectIdentifier{
				Key: obj.Key,
			})
		}
	}
	return objectsToDelete
}

func deleteAllObjects(ctx context.Context, client *s3.Client, bucketName string) error {
	log.Printf("listing objects for bucket: %s", bucketName)

	var continuationToken *string

	for {
		listOutput, err := client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
			Bucket:            aws.String(bucketName),
			ContinuationToken: continuationToken,
		})
		if err != nil {
			return fmt.Errorf("failed to list objects: %w", err)
		}

		if len(listOutput.Contents) == 0 {
			break
		}

		objectsToDelete := buildObjectIdentifiersFromContents(listOutput.Contents)

		if len(objectsToDelete) > 0 {
			log.Printf("deleting %d objects", len(objectsToDelete))
			if batchErr := deleteObjectsInBatches(ctx, client, bucketName, objectsToDelete); batchErr != nil {
				return batchErr
			}
		}

		if listOutput.IsTruncated == nil || !*listOutput.IsTruncated {
			break
		}

		continuationToken = listOutput.NextContinuationToken
	}

	return nil
}
