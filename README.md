## Overview
This tool can download an [Athena](https://aws.amazon.com/athena/) SQL query results in CSV format.

## Installation
```bash
$ go get -u -v github.com/flowerinthenight/athena2csv
```
Or you can clone and build:
```bash
$ git clone https://github.com/flowerinthenight/athena2csv
$ cd athena2csv/
$ go build -v
```

## Run the tool
Required environment variables:

```bash
# The following should have at least Athena read and S3 read/write access.
AWS_REGION={your-aws-region}
AWS_ACCESS_KEY_ID={aws-key-id}
AWS_SECRET_ACCESS_KEY={aws-secret}
```

If your query string is quite long, you can write it in a file:
```bash
$ ./athena2csv --database aws-billing --query-file query.txt
```

If your query is not that long, you can input directly in command line:
```bash
$ ./athena2csv --database aws-billing "select \"identity/lineitemid\" \
      from \"aws_billing\".\"mobingilabs_aws_billing_formatted_development\" \
      limit 10"
```

Output file is downloaded to the current directory, named `output.csv`.
