package main

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/athena"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/cenkalti/backoff"
	"github.com/mobingi/gosdk/pkg/util/cmdline"
	"github.com/mobingi/gosdk/pkg/util/simplelog"
	"github.com/spf13/cobra"
)

var (
	queryBucket = "queryathena2csv-query-bucket"
	database    string
	queryFile   string

	rootCmd = &cobra.Command{
		Use:   "queryathena2csv [query]",
		Short: "execute athena query and download csv result",
		Run: func(cmd *cobra.Command, args []string) {
			if queryFile != "" {
				b, err := ioutil.ReadFile(queryFile)
				if err != nil {
					die(err)
				}

				do(string(b))
				return
			}

			if len(args) == 0 {
				die("no input query")
			}

			do(args[0])
		},
	}
)

func do(query string) {
	if database == "" {
		die("database is empty")
	}

	simplelog.Infof("query=%v", query)
	simplelog.Infof("dir=%v", cmdline.Dir())
	simplelog.Infof("qb=%v", queryBucket)

	sess, _ := session.NewSession(&aws.Config{
		Region: aws.String(os.Getenv("AWS_REGION")),
	})

	svc := athena.New(sess)

	var s athena.StartQueryExecutionInput
	s.SetQueryString(query)

	var q athena.QueryExecutionContext
	q.SetDatabase(database)
	s.SetQueryExecutionContext(&q)

	var r athena.ResultConfiguration
	outLoc := "s3://" + queryBucket
	r.SetOutputLocation(outLoc)
	s.SetResultConfiguration(&r)

	var result *athena.StartQueryExecutionOutput
	var err error
	var rerr error
	start, tries := time.Now(), 0
	op := func() error {
		tries += 1
		if tries > 1 {
			simplelog.Infof("StartQueryExecution throttled, retry call %v after first run", time.Since(start))
		}

		result, err = svc.StartQueryExecution(&s)
		rerr = err
		if err != nil {
			if isAthenaThrottleErr(err) {
				return err // will cause retry with backoff
			}
		}

		return nil // final err is rerr
	}

	err = backoff.Retry(op, backoff.NewExponentialBackOff())
	if err != nil {
		dief("StartQueryExecution failed, retry exhausted, err=%v", err)
	} else {
		err = rerr
	}

	if err != nil {
		dief("StartQueryExecution failed, err=%v", err)
	}

	simplelog.Infof("StartQueryExecution result=%v", result.GoString())

	var qri athena.GetQueryExecutionInput
	qri.SetQueryExecutionId(*result.QueryExecutionId)

	var qrop *athena.GetQueryExecutionOutput
	duration := time.Duration(2) * time.Second // pause for 2 seconds
	start = time.Now()
	less1h := true

	for {
		var rerr error
		start, tries := time.Now(), 0
		op := func() error {
			tries += 1
			if tries > 1 {
				simplelog.Infof("GetQueryExecution throttled, retry call %v after first run", time.Since(start))
			}

			qrop, err = svc.GetQueryExecution(&qri)
			rerr = err
			if err != nil {
				if isAthenaThrottleErr(err) {
					return err // will cause retry with backoff
				}
			}

			return nil // final err is rerr
		}

		err = backoff.Retry(op, backoff.NewExponentialBackOff())
		if err != nil {
			dief("GetQueryExecution failed, retry exhausted, err=%v", err)
		} else {
			err = rerr
		}

		if err != nil {
			dief("GetQueryExecution failed, err=%v", err)
		}

		if *qrop.QueryExecution.Status.State != "RUNNING" {
			break
		}

		simplelog.Infof("waiting state to finish %v since start, current=%v", time.Since(start), *qrop.QueryExecution.Status.State)

		if time.Since(start) > (time.Hour * 1) {
			less1h = false
			break
		}

		time.Sleep(duration)
	}

	if !less1h {
		dief("query has gone beyond 1hour!")
	}

	if *qrop.QueryExecution.Status.State == "SUCCEEDED" {
		srcFile := queryBucket + "/" + *result.QueryExecutionId + ".csv"
		simplelog.Infof("output=s3://%v", srcFile)

		downloader := s3manager.NewDownloader(session.New(&aws.Config{
			Region: aws.String(os.Getenv("AWS_REGION")),
		}))

		toDownload := filepath.Join(cmdline.Dir(), "output.csv")
		file, err := os.Create(toDownload)
		if err != nil {
			die(err)
		}

		defer file.Close()
		numBytes, err := downloader.Download(file,
			&s3.GetObjectInput{
				Bucket: aws.String(queryBucket),
				Key:    aws.String(*result.QueryExecutionId + ".csv"),
			})

		if err != nil {
			die(err)
		}

		simplelog.Infof("downloaded=%v, bytes=%v", file.Name(), numBytes)
		return
	}

	dief("unexpected state, val=%v, details=%v",
		*qrop.QueryExecution.Status.State,
		*qrop.QueryExecution.Status.StateChangeReason)
}

func isAthenaThrottleErr(err error) bool {
	if aerr, ok := err.(awserr.Error); ok {
		switch aerr.Code() {
		case athena.ErrCodeTooManyRequestsException: // throttled
			return true
		}
	}

	// At this point, we resort to string matching error string, which is not really
	// a good idea but if you have a better alternative, please update.
	// Strings to check: 'throttling', rate exceeded' (tolower)
	if strings.Contains(strings.ToLower(err.Error()), "throttling") ||
		strings.Contains(strings.ToLower(err.Error()), "rate exceeded") {
		return true
	}

	return false
}

func die(v ...interface{}) {
	simplelog.Error(v...)
	os.Exit(1)
}

func dief(f string, v ...interface{}) {
	simplelog.Errorf(f, v...)
	os.Exit(1)
}

func main() {
	rootCmd.PersistentFlags().StringVar(&queryBucket, "query-bucket", queryBucket, "bucket to save queries")
	rootCmd.PersistentFlags().StringVar(&queryFile, "query-file", queryFile, "filename containing your query, high priority than args")
	rootCmd.PersistentFlags().StringVar(&database, "database", database, "athena database to query from")
	if err := rootCmd.Execute(); err != nil {
		die("root cmd execute failed: %v", err)
	}
}
