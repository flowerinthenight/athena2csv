[![CircleCI](https://circleci.com/gh/flowerinthenight/athena2csv/tree/master.svg?style=svg)](https://circleci.com/gh/flowerinthenight/athena2csv/tree/master)

## Overview
This tool can download an [Athena](https://aws.amazon.com/athena/) SQL query results in CSV format.

## Installation
Using [Homebrew](https://brew.sh/):
```bash
$ brew tap flowerinthenight/tap
$ brew install athena2csv
```

If you have a Go environment:
```bash
$ go get -u -v github.com/flowerinthenight/athena2csv
```

## Run the tool
Running this tool will create an [S3 bucket](https://aws.amazon.com/s3/) `queryathena2csv-query-bucket`.

Required environment variables:

```bash
# The following should have at least Athena read and S3 read/write access.
AWS_REGION={your-aws-region}
AWS_ACCESS_KEY_ID={aws-key-id}
AWS_SECRET_ACCESS_KEY={aws-secret}
```

If your query string is quite long, you can write it in a file:
```bash
$ athena2csv --database aws-billing --query-file query.txt
```

If your query is not that long, you can input directly in command line:
```bash
$ athena2csv --database aws-billing "select \"identity/lineitemid\" \
      from \"aws_billing\".\"mobingilabs_aws_billing_formatted_development\" \
      limit 10"
```

Output file is downloaded to the current directory, named `output.csv`.
