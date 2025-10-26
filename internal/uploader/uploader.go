package uploader

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type Uploader struct {
	s3Client *s3.Client
	bucket   string
}

func New(s3Client *s3.Client, bucket string) *Uploader {
	return &Uploader{
		s3Client: s3Client,
		bucket:   bucket,
	}
}

// GenerateExecutionID creates a unique execution ID
func GenerateExecutionID() string {
	return time.Now().UTC().Format("20060102-150405-") + fmt.Sprintf("%06d", time.Now().Nanosecond()/1000)
}

// UploadDirectory creates a tarball of the directory and uploads it to S3
func (u *Uploader) UploadDirectory(ctx context.Context, dir string, executionID string) error {
	// Create tarball
	tarball, err := createTarball(dir)
	if err != nil {
		return fmt.Errorf("failed to create tarball: %w", err)
	}

	// Upload to S3
	key := fmt.Sprintf("executions/%s/code.tar.gz", executionID)
	_, err = u.s3Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: &u.bucket,
		Key:    &key,
		Body:   bytes.NewReader(tarball),
	})
	if err != nil {
		return fmt.Errorf("failed to upload to S3: %w", err)
	}

	return nil
}

func createTarball(sourceDir string) ([]byte, error) {
	var buf bytes.Buffer
	gzipWriter := gzip.NewWriter(&buf)
	tarWriter := tar.NewWriter(gzipWriter)

	// Get absolute path
	absPath, err := filepath.Abs(sourceDir)
	if err != nil {
		return nil, err
	}

	// Walk the directory
	err = filepath.Walk(absPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip hidden files and common ignore patterns
		relPath, err := filepath.Rel(absPath, path)
		if err != nil {
			return err
		}

		if shouldIgnore(relPath) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// Create tar header
		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}

		// Use relative path for tar
		header.Name = relPath
		if relPath == "." {
			return nil
		}

		// Write header
		if err := tarWriter.WriteHeader(header); err != nil {
			return err
		}

		// If it's a regular file, write its content
		if info.Mode().IsRegular() {
			file, err := os.Open(path)
			if err != nil {
				return err
			}
			defer file.Close()

			if _, err := io.Copy(tarWriter, file); err != nil {
				return err
			}
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	// Close writers
	if err := tarWriter.Close(); err != nil {
		return nil, err
	}
	if err := gzipWriter.Close(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func shouldIgnore(path string) bool {
	// Common patterns to ignore
	ignorePatterns := []string{
		".git",
		".gitignore",
		".DS_Store",
		"node_modules",
		".terraform",
		"*.tfstate",
		"*.tfstate.backup",
		"venv",
		"__pycache__",
		".env",
		".venv",
		"dist",
		"build",
	}

	for _, pattern := range ignorePatterns {
		if strings.HasPrefix(path, ".") && path != "." {
			return true
		}
		if strings.Contains(path, pattern) {
			return true
		}
	}

	return false
}
