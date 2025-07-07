package marketdata

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"runtime"
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
func newS3Client(cfg s3Config) (*s3.Client, error) {
	awsCfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithRegion(cfg.Region),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(cfg.Key, cfg.Secret, "")),
		config.WithEndpointResolverWithOptions(aws.EndpointResolverWithOptionsFunc(
			func(service, region string, _ ...interface{}) (aws.Endpoint, error) {
				return aws.Endpoint{URL: cfg.Endpoint, SigningRegion: region, HostnameImmutable: true}, nil
			}),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("load aws cfg: %w", err)
	}

	// Use a custom HTTP client with a shorter timeout.
	httpClient := &http.Client{Timeout: 30 * time.Second}
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
	cpus := runtime.NumCPU()
	if cpus > 8 {
		return 8
	}
	return cpus
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
	var out []string
	pag := s3.NewListObjectsV2Paginator(s3c, &s3.ListObjectsV2Input{Bucket: aws.String(bucket), Prefix: aws.String(prefix)})
	for pag.HasMorePages() {
		page, err := pag.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, obj := range page.Contents {
			if obj.Key != nil && strings.HasSuffix(*obj.Key, ".csv.gz") {
				out = append(out, *obj.Key)
			}
		}
	}
	return out, nil
}
