package marketdata

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// -----------------------------------------------------------------------------
// S3 & environment configuration helpers
// -----------------------------------------------------------------------------

type s3Config struct {
	Endpoint string
	Bucket   string
	Key      string
	Secret   string
	Region   string
}

// env returns the value of key or a fallback default.
func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// mustEnv fetches the value of an env-var or terminates the process.
func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("environment variable %s is required", key)
	}
	return v
}

// loadS3Config populates s3Config from environment variables (with sane defaults).
func loadS3Config() s3Config {
	return s3Config{
		Endpoint: env("POLYGON_S3_ENDPOINT", "https://files.polygon.io"),
		Bucket:   env("S3_BUCKET", "flatfiles"),
		Key:      mustEnv("POLYGON_S3_KEY"),
		Secret:   mustEnv("POLYGON_S3_SECRET"),
		Region:   env("AWS_REGION", "us-east-1"),
	}
}

// newS3Client returns a tuned AWS S3 client for high-throughput, low-latency transfers.
//
//nolint:staticcheck // SA1019: Using deprecated resolver until AWS SDK upgrade completes.
func newS3Client(cfg s3Config) (*s3.Client, error) {
	awsCfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithRegion(cfg.Region),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(cfg.Key, cfg.Secret, "")),
		config.WithEndpointResolverWithOptions(aws.EndpointResolverWithOptionsFunc( //nolint:staticcheck // SA1019: Using deprecated resolver until AWS SDK upgrade completes.
			func(service, region string, _ ...interface{}) (aws.Endpoint, error) {
				return aws.Endpoint{URL: cfg.Endpoint, SigningRegion: region, HostnameImmutable: true}, nil //nolint:staticcheck // SA1019: Using deprecated Endpoint struct until AWS SDK upgrade completes.
			}),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("load aws cfg: %w", err)
	}

	// Use a custom HTTP client with an extended timeout to handle slow S3 responses.
	httpClient := &http.Client{Timeout: 5 * time.Minute}
	return s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.HTTPClient = httpClient
		o.DisableLogOutputChecksumValidationSkipped = true
	}), nil
}

// -----------------------------------------------------------------------------
// Timeframe definitions – maintain strict prefix⇄table mapping.
// -----------------------------------------------------------------------------

type timeframe struct {
	name      string
	s3Prefix  string
	tableName string
}

var timeframes = []timeframe{
	{"1-day", "us_stocks_sip/day_aggs_v1", "ohlcv_1d"},
	{"1-minute", "us_stocks_sip/minute_aggs_v1", "ohlcv_1m"},
}

// -----------------------------------------------------------------------------
// Performance tuning knobs (batch size & worker count)
// -----------------------------------------------------------------------------

// copyBatchSize determines how many CSV files are streamed into a single COPY.
var copyBatchSize = func() int {
	if v := os.Getenv("COPY_BATCH_SIZE"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return 10
}()

// copyWorkerCount caps the parallel COPY operations to protect the WAL.
var copyWorkerCount = func() int {
	return 4 // TODO: remove this if we can get db stable
	/*
		cpus := runtime.NumCPU()
		if cpus > 8 {
			return 8
		}
		return cpus
	*/
}()

// -----------------------------------------------------------------------------
// Generic helper utilities (no DB side-effects)
// -----------------------------------------------------------------------------

// monthlyPrefixes yields S3 prefixes for each month in the range [from, to].
func monthlyPrefixes(base string, from, to time.Time) []string {
	var out []string
	cur := time.Date(from.Year(), from.Month(), 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(to.Year(), to.Month(), 1, 0, 0, 0, 0, time.UTC)
	for !cur.After(end) {
		out = append(out, fmt.Sprintf("%s/%04d/%02d/", base, cur.Year(), cur.Month()))
		cur = cur.AddDate(0, 1, 0)
	}
	return out
}

// listCSVObjects lists all *.csv.gz objects under prefix and returns their keys.
func listCSVObjects(ctx context.Context, s3c *s3.Client, bucket, prefix string) ([]string, error) {
	return listCSVObjectsWithRetry(ctx, s3c, bucket, prefix, 3)
}

// listCSVObjectsWithRetry lists all *.csv.gz objects under prefix with exponential backoff retry.
func listCSVObjectsWithRetry(ctx context.Context, s3c *s3.Client, bucket, prefix string, maxRetries int) ([]string, error) {
	var out []string
	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			// Exponential backoff: 2^attempt seconds, with jitter
			backoffDuration := time.Duration(1<<attempt) * time.Second
			if backoffDuration > 30*time.Second {
				backoffDuration = 30 * time.Second // Cap at 30 seconds
			}

			log.Printf("S3 rate limited (attempt %d/%d), backing off for %v...", attempt, maxRetries, backoffDuration)

			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoffDuration):
			}
		}

		out = nil // Reset output slice for retry
		success := true

		pag := s3.NewListObjectsV2Paginator(s3c, &s3.ListObjectsV2Input{Bucket: aws.String(bucket), Prefix: aws.String(prefix)})
		for pag.HasMorePages() {
			page, err := pag.NextPage(ctx)
			if err != nil {
				lastErr = err
				success = false

				// Check if this is a rate limit error (429)
				if isS3RateLimitError(err) {
					log.Printf("S3 rate limit hit during pagination for prefix %s: %v", prefix, err)
					break // Break from pagination, will retry entire operation
				}

				// For non-rate-limit errors, fail immediately
				return nil, err
			}

			for _, obj := range page.Contents {
				if obj.Key != nil && strings.HasSuffix(*obj.Key, ".csv.gz") {
					out = append(out, *obj.Key)
				}
			}
		}

		if success {
			return out, nil
		}

		// If we hit a rate limit error but haven't exhausted retries, continue to next attempt
		if !isS3RateLimitError(lastErr) {
			return nil, lastErr
		}
	}

	return nil, fmt.Errorf("S3 listing failed after %d retries, last error: %w", maxRetries, lastErr)
}

// isS3RateLimitError checks if the error is an S3 rate limiting error (429 Too Many Requests)
func isS3RateLimitError(err error) bool {
	if err == nil {
		return false
	}

	errStr := err.Error()
	return strings.Contains(errStr, "TooManyRequests") ||
		strings.Contains(errStr, "429") ||
		strings.Contains(errStr, "Too Many Requests")
}
